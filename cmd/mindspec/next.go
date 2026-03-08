package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/gitops"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Discover, claim, and start the next piece of work",
	Long: `Queries Beads for ready work, claims it, updates MindSpec state,
and emits mode-appropriate guidance — going from "what should I do?"
to "here's your bead, here's the mode, here are your rules" in one step.

Use --spec to filter ready work to a specific spec. If multiple active specs
exist and no --spec is given, the command fails with a list of candidates.

Use --emit-only to build and emit a bead primer without claiming the bead,
creating a worktree, or updating state. Ideal for multi-agent mode where a
team lead spawns fresh agents per bead. Accepts an optional positional bead ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pick, _ := cmd.Flags().GetInt("pick")
		specFlag, _ := cmd.Flags().GetString("spec")
		force, _ := cmd.Flags().GetBool("force")
		emitOnly, _ := cmd.Flags().GetBool("emit-only")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		// Emit-only mode: build and print primer without claiming or state changes
		if emitOnly {
			return runEmitOnly(root, specFlag, args)
		}

		// Step 0a: Worktree scoping guard
		kind, _, _ := workspace.DetectWorktreeContext(cwd)
		switch kind {
		case workspace.WorktreeMain:
			// Auto-resolve: if exactly one active spec has a worktree, cd there.
			specs, discErr := phase.DiscoverActiveSpecs()
			if discErr == nil && len(specs) == 1 {
				specWt := state.SpecWorktreePath(root, specs[0].SpecID)
				if fi, statErr := os.Stat(specWt); statErr == nil && fi.IsDir() {
					fmt.Fprintf(os.Stderr, "Auto-resolving to spec worktree: %s\n", specWt)
					if err := os.Chdir(specWt); err == nil {
						cwd = specWt
						break
					}
				}
			}
			// Build helpful message listing available spec worktrees
			msg := "mindspec next must run from a spec worktree."
			if discErr == nil {
				for _, s := range specs {
					wt := state.SpecWorktreePath(root, s.SpecID)
					if info, statErr := os.Stat(wt); statErr == nil && info.IsDir() {
						msg += fmt.Sprintf("\n  cd %s", wt)
					}
				}
			}
			if msg == "mindspec next must run from a spec worktree." {
				msg += "\nUse `mindspec spec create <slug>` or cd into an existing spec worktree"
			}
			return fmt.Errorf("%s", msg)
		case workspace.WorktreeBead:
			return fmt.Errorf("you're already in a bead worktree — run `mindspec complete \"msg\"` when done")
		}

		// Step 0b: Session freshness gate
		if sess, readErr := state.ReadSession(root); readErr == nil && sess.SessionStartedAt != "" {
			stale := false
			reason := ""
			if sess.SessionSource == "resume" || sess.SessionSource == "compact" {
				stale = true
				reason = "session was " + sess.SessionSource + " (not fresh)"
			} else if sess.BeadClaimedAt != "" && sess.BeadClaimedAt >= sess.SessionStartedAt {
				stale = true
				reason = "a bead was already claimed in this session"
			}
			if stale {
				if !force {
					return fmt.Errorf("session freshness gate: %s.\nYou MUST run /clear to reset your context, then retry.\nDo NOT attempt workarounds — /clear is required.", reason)
				}
				fmt.Fprintf(os.Stderr, "warning: bypassing session freshness gate (%s)\n", reason)
			}
		}

		// Step 1: Check clean tree
		if err := next.CheckCleanTree(); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot claim work: %s\n\n", err)
			fmt.Fprintln(os.Stderr, "Recovery steps:")
			fmt.Fprintln(os.Stderr, "  1. Commit your changes: mindspec complete \"wip\"")
			fmt.Fprintln(os.Stderr, "  2. Or discard them: git restore .")
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

		// Step 1.7: Unmerged-bead guard — block if a predecessor bead was closed
		// without `mindspec complete` (bead branch still exists).
		if specFlag != "" {
			if err := checkUnmergedBeads(specFlag); err != nil {
				return err
			}
		}

		// Step 2: Query ready work (scoped to spec's epic if available)
		var items []next.BeadInfo
		if specFlag != "" {
			// Find epic via beads metadata query (ADR-0023).
			epicID, epicErr := phase.FindEpicBySpecID(specFlag)
			if epicErr == nil && epicID != "" {
				items, err = next.QueryReadyForEpic(epicID)
			} else {
				items, err = next.QueryReady()
			}
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
			fmt.Println("  - Create a new spec: mindspec spec create <slug>")
			fmt.Println("  - Check blocked items- bd blocked")
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

		// Step 5.1: Record bead claim time for session freshness gate
		if sess, readErr := state.ReadSession(root); readErr == nil {
			sess.BeadClaimedAt = time.Now().UTC().Format(time.RFC3339)
			_ = state.WriteSessionFile(root, sess)
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
		if specFlag != "" {
			// Explicit --spec flag always wins over title parsing
			resolved.SpecID = specFlag
		}

		// Note: parent status propagation handled natively by beads epics

		// ADR-0023: no focus write — state is derived from beads.
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

		// Step 8: Emit bead context from bd show (Spec 074)
		if selected.ID != "" {
			rendered, renderErr := contextpack.RenderBeadContext(selected.ID)
			if renderErr == nil {
				fmt.Println()
				fmt.Print(rendered)
			} else {
				fmt.Fprintf(os.Stderr, "warning: could not render bead context: %v (falling back to generic guidance)\n", renderErr)
				if err := emitInstruct(root); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
				}
			}
		} else {
			if err := emitInstruct(root); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
			}
		}

		return nil
	},
}

// runEmitOnly handles the --emit-only path: build and print primer without claiming.
func runEmitOnly(root, specFlag string, args []string) error {
	var selected next.BeadInfo

	if len(args) > 0 {
		// Explicit bead ID provided as positional argument
		info, err := next.FetchBeadByID(args[0])
		if err != nil {
			return fmt.Errorf("fetching bead %s: %w", args[0], err)
		}
		selected = info
	} else {
		// Query ready work and pick the first item
		var items []next.BeadInfo
		var err error
		if specFlag != "" {
			epicID, epicErr := phase.FindEpicBySpecID(specFlag)
			if epicErr == nil && epicID != "" {
				items, err = next.QueryReadyForEpic(epicID)
			} else {
				items, err = next.QueryReady()
			}
		} else {
			items, err = next.QueryReady()
		}
		if err != nil {
			return fmt.Errorf("querying ready work: %w", err)
		}

		if specFlag != "" {
			var filtered []next.BeadInfo
			for _, item := range items {
				if containsSpecID(item.Title, specFlag) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		if len(items) == 0 {
			return fmt.Errorf("no ready work found for --emit-only")
		}
		selected = items[0]
	}

	rendered, err := contextpack.RenderBeadContext(selected.ID)
	if err != nil {
		return fmt.Errorf("rendering bead context: %w", err)
	}

	fmt.Print(rendered)
	return nil
}

// containsSpecID checks if a bead title references the given spec ID.
func containsSpecID(title, specID string) bool {
	return strings.Contains(title, specID)
}

// checkUnmergedBeads checks for closed sibling beads that still have a bead/<id>
// branch, indicating they were closed via `bd close` without `mindspec complete`.
// Returns an error to block `mindspec next` until cleanup is performed.
func checkUnmergedBeads(specID string) error {
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return nil
	}

	out, err := bead.RunBD("list", "--parent", epicID, "--status=closed", "--json")
	if err != nil {
		return nil
	}

	var items []bead.BeadInfo
	if json.Unmarshal(out, &items) != nil {
		return nil
	}

	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id != "" && gitops.BranchExists("bead/"+id) {
			return fmt.Errorf("bead %s was closed without `mindspec complete` — merge topology is broken.\nRun `mindspec complete --spec=%s` to recover, then retry `mindspec next`.", id, specID)
		}
	}
	return nil
}

func init() {
	nextCmd.Flags().Int("pick", 0, "Pick a specific item by number (1-based) when multiple are ready")
	nextCmd.Flags().String("spec", "", "Target spec ID to filter ready work (auto-detected if exactly one active spec)")
	nextCmd.Flags().Bool("force", false, "Bypass the context clear gate (use when you know your context is clean)")
	nextCmd.Flags().Bool("emit-only", false, "Emit bead primer without claiming, creating worktree, or updating state (for multi-agent mode)")
}
