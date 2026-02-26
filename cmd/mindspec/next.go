package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mindspec/mindspec/internal/next"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/resolve"
	"github.com/mindspec/mindspec/internal/specmeta"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Discover, claim, and start the next piece of work",
	Long: `Queries Beads for ready work, claims it, updates MindSpec state,
and emits mode-appropriate guidance — going from "what should I do?"
to "here's your bead, here's the mode, here are your rules" in one step.

Use --spec to filter ready work to a specific spec. If multiple active specs
exist and no --spec is given, the command fails with a list of candidates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pick, _ := cmd.Flags().GetInt("pick")
		specFlag, _ := cmd.Flags().GetString("spec")
		force, _ := cmd.Flags().GetBool("force")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		// Step 0: Check needs_clear gate
		if cur, readErr := state.Read(root); readErr == nil && cur.NeedsClear {
			if !force {
				return fmt.Errorf("context clear required. Run /clear to reset your context, then retry.\nUse --force to bypass")
			}
			fmt.Fprintln(os.Stderr, "warning: bypassing context clear gate (--force)")
			cur.NeedsClear = false
			_ = state.Write(root, cur)
		}

		// Step 1: Check clean tree
		if err := next.CheckCleanTree(); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot claim work: %s\n\n", err)
			fmt.Fprintln(os.Stderr, "Recovery steps:")
			fmt.Fprintln(os.Stderr, "  1. Commit your changes: git add -A && git commit -m \"wip\"")
			fmt.Fprintln(os.Stderr, "  2. Or discard them: git checkout -- .")
			fmt.Fprintln(os.Stderr, "  3. Then re-run: mindspec next")
			return fmt.Errorf("dirty working tree")
		}

		// Step 1.5: Resolve target spec if ambiguous
		if specFlag == "" {
			targetSpec, err := resolve.ResolveTarget(root, "")
			if err != nil {
				if _, ok := err.(*resolve.ErrAmbiguousTarget); ok {
					return fmt.Errorf("multiple active specs found; use --spec to target one:\n%s", err)
				}
				// Non-ambiguous errors are OK — next will query all ready work
			} else {
				specFlag = targetSpec
			}
		}

		var boundMeta *specmeta.Meta
		if specFlag != "" {
			boundMeta, err = specmeta.EnsureFullyBound(root, specFlag)
			if err != nil {
				return fmt.Errorf("spec %s requires a valid molecule binding before claiming work: %w", specFlag, err)
			}
		}

		// Step 2: Query ready work
		var items []next.BeadInfo
		if boundMeta != nil && boundMeta.MoleculeID != "" {
			items, err = next.QueryReadyForMolecule(boundMeta.MoleculeID)
		} else {
			items, err = next.QueryReady()
		}
		if err != nil {
			return fmt.Errorf("querying ready work: %w", err)
		}

		// Step 2.5: Filter by target spec if specified
		if specFlag != "" {
			var filtered []next.BeadInfo
			for _, item := range items {
				if containsSpecID(item.Title, specFlag) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		// Step 3: Handle no-work case
		if len(items) == 0 {
			fmt.Println("No ready work found.")
			if specFlag != "" {
				fmt.Printf("(filtered to spec: %s)\n", specFlag)
			}
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  - Create a new spec: mindspec spec-init")
			fmt.Println("  - Check blocked items: bd blocked")
			fmt.Println("  - List all open work: bd list --status=open")
			return nil
		}

		// Step 4: Display and select
		if len(items) > 1 {
			fmt.Printf("Ready work (%d items):\n", len(items))
			fmt.Print(next.FormatWorkList(items))
			fmt.Println()
			if pick == 0 {
				fmt.Println("Defaulting to first item. Use --pick=N to select a specific item.")
			}
		}

		selected, err := next.SelectWork(items, pick)
		if err != nil {
			return fmt.Errorf("selecting work: %w", err)
		}

		// Step 5: Claim
		fmt.Printf("Claiming [%s] %s ...\n", selected.ID, selected.Title)
		if err := next.ClaimBead(selected.ID); err != nil {
			return fmt.Errorf("claiming bead: %w", err)
		}

		// Step 5.5: Create or reuse worktree
		wtPath, wtErr := next.EnsureWorktree(root, selected.ID)
		if wtErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: worktree setup failed: %v\n", wtErr)
		} else if wtPath != "" {
			fmt.Printf("Worktree: %s\n", wtPath)
			fmt.Printf("  cd %s\n", wtPath)
		}

		// Step 6: Resolve mode and spec ID
		resolved := next.ResolveMode(root, selected)
		if resolved.SpecID == "" && specFlag != "" {
			resolved.SpecID = specFlag
		}

		// Note: parent status propagation handled natively by beads molecules

		// Step 7: Update state (cursor)
		if boundMeta != nil && resolved.SpecID == specFlag {
			err = state.SetModeWithMetadata(root, resolved.Mode, resolved.SpecID, selected.ID, boundMeta.MoleculeID, boundMeta.StepMapping)
		} else {
			err = state.SetMode(root, resolved.Mode, resolved.SpecID, selected.ID)
		}
		if err != nil {
			return fmt.Errorf("updating state: %w", err)
		}

		// Update activeWorktree if a bead worktree was created.
		if wtErr == nil && wtPath != "" {
			if curState, readErr := state.Read(root); readErr == nil {
				curState.ActiveWorktree = wtPath
				if writeErr := state.Write(root, curState); writeErr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not update activeWorktree in state: %v\n", writeErr)
				}
			}
		}

		fmt.Printf("State updated: mode=%s, spec=%s, bead=%s\n", resolved.Mode, resolved.SpecID, selected.ID)
		fmt.Println()

		// Step 7.5: Emit recording bead marker (best-effort)
		if resolved.SpecID != "" {
			if err := recording.EmitBeadMarker(root, resolved.SpecID, "start", selected.ID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not emit recording marker: %v\n", err)
			}
			if err := recording.AddBeadToPhase(root, resolved.SpecID, selected.ID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not update recording manifest: %v\n", err)
			}
		}

		// Step 8: Emit guidance (instruct-tail convention)
		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

// containsSpecID checks if a bead title references the given spec ID.
func containsSpecID(title, specID string) bool {
	return strings.Contains(title, specID)
}

func init() {
	nextCmd.Flags().Int("pick", 0, "Pick a specific item by number (1-based) when multiple are ready")
	nextCmd.Flags().String("spec", "", "Target spec ID to filter ready work (auto-detected if exactly one active spec)")
	nextCmd.Flags().Bool("force", false, "Bypass the context clear gate (use when you know your context is clean)")
}
