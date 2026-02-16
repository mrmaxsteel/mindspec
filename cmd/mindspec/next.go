package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/next"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Discover, claim, and start the next piece of work",
	Long: `Queries Beads for ready work, claims it, updates MindSpec state,
and emits mode-appropriate guidance — going from "what should I do?"
to "here's your bead, here's the mode, here are your rules" in one step.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pick, _ := cmd.Flags().GetInt("pick")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
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

		// Step 2: Query ready work
		items, err := next.QueryReady()
		if err != nil {
			return fmt.Errorf("querying ready work: %w", err)
		}

		// Step 3: Handle no-work case
		if len(items) == 0 {
			fmt.Println("No ready work found.")
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
		wtPath, wtErr := next.EnsureWorktree(selected.ID)
		if wtErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: worktree setup failed: %v\n", wtErr)
		} else if wtPath != "" {
			fmt.Printf("Worktree: %s\n", wtPath)
			fmt.Printf("  cd %s\n", wtPath)
		}

		// Step 6: Resolve mode and spec ID
		resolved := next.ResolveMode(root, selected)

		// Step 6.5: Propagate in_progress to parent beads
		if resolved.SpecID != "" {
			bead.PropagateStart(resolved.SpecID)
		}

		// Step 7: Update state
		if err := state.SetMode(root, resolved.Mode, resolved.SpecID, selected.ID); err != nil {
			return fmt.Errorf("updating state: %w", err)
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

func init() {
	nextCmd.Flags().Int("pick", 0, "Pick a specific item by number (1-based) when multiple are ready")
}
