package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
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

An optional positional bead ID is honored on BOTH paths:
  - claim path (default): claim that bead — or error clearly if it is not
    found / not in the ready set — instead of defaulting to the first ready item.
  - --emit-only: build the primer for that bead without claiming.
A positional bead ID and --pick are mutually exclusive selectors; supplying
both is an error.

Use --emit-only to build and emit a bead primer without claiming the bead,
creating a worktree, or updating state. Ideal for multi-agent mode where a
team lead spawns fresh agents per bead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pick, _ := cmd.Flags().GetInt("pick")
		specFlag, _ := cmd.Flags().GetString("spec")
		force, _ := cmd.Flags().GetBool("force")
		emitOnly, _ := cmd.Flags().GetBool("emit-only")

		// Spec 101 R1 (mindspec-mfe0): a positional bead ID and --pick are
		// mutually exclusive selectors — the positional names a specific bead,
		// --pick is an index into the ready set, and they may disagree. Reject
		// the conflict up front (pure arg validation), before any gate or claim.
		if len(args) > 0 && pick > 0 {
			return fmt.Errorf("supply exactly one selector: a positional bead ID OR --pick, not both")
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		exec := newExecutor(root)

		// Emit-only mode: build and print primer without claiming or state changes
		if emitOnly {
			return runEmitOnly(specFlag, args)
		}

		// Step 0a: --spec prefix resolution (e.g., "079" → "079-location-agnostic-commands")
		if specFlag != "" {
			resolved, resolveErr := resolve.ResolveSpecPrefix(specFlag)
			if resolveErr != nil {
				return resolveErr
			}
			specFlag = resolved
		}

		// Step 0b: Bead worktree informational note
		kind, _, _ := workspace.DetectWorktreeContext(cwd)
		if kind == workspace.WorktreeBead {
			fmt.Fprintf(os.Stderr, "Note: you're in a bead worktree. Run `mindspec complete <bead-id>` when done.\n")
		}

		// Step 0c: Session freshness gate
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

		// Step 1: Artifact-aware dirty-tree guard (ADR-0025).
		// `.beads/issues.jsonl` is a build artifact co-managed by bd; dirt on
		// that path alone is auto-handled via `bd export`. User-authored dirt
		// still blocks — the guard's purpose is to protect user code.
		userDirt, err := next.CheckDirtyTree(root, cwd)
		if err != nil {
			return fmt.Errorf("checking working tree: %w", err)
		}
		if len(userDirt) > 0 {
			// Spec 092 Reqs 8/12 (mindspec-tjat): the failure carries the
			// worktree-context line and ends with a `recovery:` line. The
			// old multi-line "Recovery steps: 1..3" stderr block advised
			// `git restore .` — destructive over the HUMAN's dirt when the
			// agent merely ran `next` from the wrong directory. The active
			// worktree (when one exists) is the steer-to target; resolving
			// it costs bd calls only on this failure path.
			return next.DirtyTreeFailure(cwd, userDirt, guard.ActiveWorktreePath(root))
		}

		// Step 1.5: Resolve target spec if ambiguous
		if specFlag == "" {
			targetSpec, err := resolve.ResolveTarget(root, "")
			if err != nil {
				if ambErr, ok := err.(*resolve.ErrAmbiguousTarget); ok {
					fmt.Fprintf(os.Stderr, "Multiple active specs have ready beads:\n\n")
					for i, s := range ambErr.Active {
						fmt.Fprintf(os.Stderr, "  %d. %s  (phase: %s)\n", i+1, idrender.Spec(s.SpecID), s.Mode)
					}
					fmt.Fprintf(os.Stderr, "\nAsk the user which spec to work on, then re-run:\n")
					fmt.Fprintf(os.Stderr, "  mindspec next --spec=<number>\n")
					os.Exit(1)
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

		// Step 3: Handle no-work case — also check for in-progress bead with
		// missing worktree (stale recovery). If a bead is in_progress but its
		// worktree was deleted, recreate the worktree so the agent can resume.
		if len(items) == 0 {
			if specFlag != "" {
				activeBead, resolveErr := next.ResolveActiveBead(root, specFlag)
				if resolveErr == nil && activeBead != "" {
					fmt.Printf("No ready work, but bead %s is in-progress with missing worktree. Recovering...\n", idrender.Bead(activeBead))
					wtPath, wtErr := next.EnsureWorktree(root, activeBead, specFlag, exec)
					if wtErr != nil {
						return fmt.Errorf("recovering worktree for in-progress bead %s: %w", idrender.Bead(activeBead), wtErr)
					}
					if wtPath != "" {
						fmt.Printf("Worktree recovered: %s\n", wtPath)
						fmt.Printf("  %s\n", containment.EmitCd(wtPath))
					}
					resolved := next.ResolveMode(root, next.BeadInfo{ID: activeBead, Title: "[" + specFlag + "] recovered"})
					resolved.SpecID = specFlag
					fmt.Print(formatStateLine(resolved.Mode, resolved.SpecID, activeBead))
					return nil
				}
			}
			fmt.Println("No ready work found.")
			if specFlag != "" {
				fmt.Printf("(filtered to spec: %s)\n", idrender.Spec(specFlag))
			}
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  - Create a new spec: mindspec spec create <slug>")
			fmt.Println("  - Check blocked items: bd blocked")
			fmt.Println("  - List all open work: bd list --status=open")
			return nil
		}

		// Step 4: Display and select. Spec 101 R1 (mindspec-mfe0): when a
		// positional bead ID is given it names the SPECIFIC bead the caller
		// intends — resolve it from the already-fetched ready set (claim that
		// bead or error), never fall through to items[0]/"Defaulting to first".
		named := ""
		if len(args) > 0 {
			named = args[0]
		}
		if len(items) > 1 {
			fmt.Printf("Ready work (%d items):\n", len(items))
			fmt.Print(next.FormatWorkList(items))
			fmt.Println()
			if pick == 0 && named == "" {
				fmt.Println("Defaulting to first item. Use --pick=N to select a specific item.")
			}
		}

		var selected next.BeadInfo
		if named != "" {
			selected, err = next.SelectWorkByName(items, named)
		} else {
			selected, err = next.SelectWork(items, pick)
		}
		if err != nil {
			return fmt.Errorf("selecting work: %w", err)
		}

		// Step 5: Claim
		fmt.Print(formatClaimLine(selected.ID, selected.Title))
		if err := next.ClaimBead(selected.ID); err != nil {
			// Spec 093 Req 3: the recovery recipe lives at this caller —
			// it has the root/specFlag context the interpolation needs.
			// The wiring (which error to return) is pinned by
			// claimFailureError in next_recovery_test.go.
			return claimFailureError(root, specFlag, selected, err)
		}

		// Step 5.1: Record bead claim time for session freshness gate
		if sess, readErr := state.ReadSession(root); readErr == nil {
			sess.BeadClaimedAt = time.Now().UTC().Format(time.RFC3339)
			_ = state.WriteSessionFile(root, sess)
		}

		// Step 5.5: Create or reuse worktree
		wtPath, wtErr := next.EnsureWorktree(root, selected.ID, specFlag, exec)
		if wtErr != nil {
			// Spec 093 Req 4: warn-and-continue is preserved (zero
			// behavior change), but the message now carries the concrete
			// `git worktree add` recipe and the auto-recovery re-run —
			// the agent is claimed-but-homeless without it. The
			// warn-AND-continue wiring is pinned by warnWorktreeSetupFailure
			// in next_recovery_test.go.
			warnWorktreeSetupFailure(os.Stderr, root, specFlag, selected, wtErr)
		} else if wtPath != "" {
			fmt.Printf("Worktree: %s\n", wtPath)
			fmt.Printf("  %s\n", containment.EmitCd(wtPath))
		}

		// Step 6: Resolve mode and spec ID
		resolved := next.ResolveMode(root, selected)
		if specFlag != "" {
			// Explicit --spec flag always wins over title parsing
			resolved.SpecID = specFlag
		}

		// Note: parent status propagation handled natively by beads epics

		// ADR-0023: no focus write — state is derived from beads.
		fmt.Print(formatStateLine(resolved.Mode, resolved.SpecID, selected.ID))
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
				// Completion reminder — ensures agents see "how to finish" right
				// before they start coding. Without this, the implement template's
				// completion section may scroll out of small-context models.
				fmt.Print(completionGuidance(selected.ID))
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

// completionGuidance renders the "When done" tail appended to the bead
// context. Spec 092 Req 5 (mindspec-qxsy / mindspec-tjat): the guidance
// is location-agnostic — it must NOT instruct agents to `cd` into the
// bead worktree before running `mindspec complete`. The command resolves
// the bead worktree itself (internal/complete/complete.go) and may be
// run from the repo root; it removes the bead worktree when it succeeds,
// so a shell that followed a cd-then-complete instruction is stranded in
// a deleted directory.
func completionGuidance(beadID string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	// R4: beadID is an ID-typed position — idrender.Bead.
	fmt.Fprintf(&sb, "**When done**: do the work and commit it in the worktree. Then run `mindspec complete %s \"describe what you did\"` to close this bead — run it from the repo root (it resolves the bead worktree itself) and note it will remove the bead worktree when it succeeds.\n", idrender.Bead(beadID))
	sb.WriteString("Do NOT use `bd close` or raw git — `mindspec complete` handles merge, cleanup, and state transitions.\n")
	// Spec 092 Req 14 (mindspec-pi24): anti-merge-main warning in the
	// bead-context channel too — the implement template's completion
	// section may scroll out of small-context models.
	sb.WriteString("Do NOT merge `main` into the bead branch mid-implementation — bead work flows bead → spec → main, and pulling `main` in creates merge conflicts at `mindspec impl approve`.\n")
	return sb.String()
}

// runEmitOnly handles the --emit-only path: build and print primer without claiming.
func runEmitOnly(specFlag string, args []string) error {
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

// formatClaimLine renders the "Claiming [<id>] <title> ..." line printed
// just before a bead is claimed. id is an ID-typed position (routed
// through idrender.Bead so a malformed value is forced-quoted rather than
// rendered raw); title is agent-writable free text from the bd Title
// field (routed through termsafe.Escape per R4).
func formatClaimLine(id, title string) string {
	return fmt.Sprintf("Claiming [%s] %s ...\n", idrender.Bead(id), termsafe.Escape(title))
}

// formatStateLine renders the "State updated: mode=..., spec=..., bead=..."
// line printed after mode resolution. specID and beadID are ID-typed
// positions routed through idrender (AC-24): a malformed value renders
// forced-quoted; a clean value (including the dotted-child shape) renders
// byte-identically. The empty-sentinel discipline (round-6 F1) matters
// here specifically: specID == "" is the legitimate spec-mode value
// (today's "spec=" with nothing after the "=") — idrender.Spec treats the
// empty string as identity, so this NEVER renders as `spec=""`.
func formatStateLine(mode, specID, beadID string) string {
	return fmt.Sprintf("State updated: mode=%s, spec=%s, bead=%s\n", mode, idrender.Spec(specID), idrender.Bead(beadID))
}

// claimFailureError builds the spec 093 Req 3 claim-failure recovery
// error for the `mindspec next` ClaimBead caller. Extracted from the
// RunE closure so the wiring — the FULL recipe is returned, never the
// pre-093 bare `fmt.Errorf("claiming bead: %w")` — is unit-testable
// (next_recovery_test.go); reverting the call site to the bare wrap
// fails TestClaimFailureError_FullRecipe.
func claimFailureError(root, specFlag string, selected next.BeadInfo, claimErr error) error {
	return next.ClaimFailure(root, recoveryConfig(root),
		selected.ID, recoverySpecSlug(root, specFlag, selected), claimErr)
}

// warnWorktreeSetupFailure writes the spec 093 Req 4 worktree-setup
// recovery recipe to w and returns — it never fails the command
// (warn-AND-continue, the claimed-but-homeless agent still proceeds to
// the state update + auto-recovery hint). Extracted from the RunE
// closure so BOTH halves are pinned: the FULL recipe text
// (TestWarnWorktreeSetupFailure_FullRecipe) and the continue semantics
// — the function returns nothing, so flipping the call site to a fatal
// `return ...` is a visible signature change that fails
// TestWarnWorktreeSetupFailure_DoesNotFatal.
func warnWorktreeSetupFailure(w io.Writer, root, specFlag string, selected next.BeadInfo, wtErr error) {
	fmt.Fprintf(w, "Warning: %v\n", next.WorktreeSetupFailure(root, recoveryConfig(root),
		selected.ID, recoverySpecSlug(root, specFlag, selected), wtErr))
}

// recoveryConfig loads config for failure-path message interpolation
// (spec 093 Reqs 3-4). Falls back to defaults so a config problem never
// masks the original failure. Runs only on failure paths.
func recoveryConfig(root string) *config.Config {
	cfg, err := config.Load(root)
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

// recoverySpecSlug resolves the spec slug for the Req 3/4 recovery
// recipes: the explicit --spec flag wins; otherwise the slug is parsed
// from the selected bead's title (next.ResolveMode). May return "" —
// the recipe constructors then fall back to placeholders. Runs only on
// failure paths.
func recoverySpecSlug(root, specFlag string, selected next.BeadInfo) string {
	if specFlag != "" {
		return specFlag
	}
	return next.ResolveMode(root, selected).SpecID
}

// checkUnmergedBeads checks for closed sibling beads that still have a bead/<id>
// branch which is NOT merged into the spec branch, indicating they were closed
// via `bd close` without `mindspec complete`. Returns an error to block
// `mindspec next` until cleanup is performed.
//
// It delegates to the shared lifecycle.FindOrphanedClosedBeads predicate so the
// IsAncestor confirmation (skipping a benign merged-but-branch-undeleted bead)
// stays in lockstep with `mindspec complete` and `mindspec doctor`.
func checkUnmergedBeads(specID string) error {
	orphans := lifecycle.FindOrphanedClosedBeads(specID, ".", "")
	if len(orphans) == 0 {
		return nil
	}
	return unmergedBeadError(orphans[0])
}

// unmergedBeadError formats the checkUnmergedBeads refusal for a single
// orphan. Extracted so a hostile o.BeadID (unvalidated bd-list data — see
// lifecycle.Orphan) can be pinned directly: idrender.Bead forces a
// malformed-but-printable ID through strconv.Quote so it can never be
// mistaken for — or forge output around — a genuine bead ID (R4 display-
// forgery sweep, spec 120).
func unmergedBeadError(o lifecycle.Orphan) error {
	safeBeadID := idrender.Bead(o.BeadID)
	return fmt.Errorf("bead %s was closed without `mindspec complete` — merge topology is broken.\nRun `mindspec complete %s` to recover, then retry `mindspec next`.", safeBeadID, safeBeadID)
}

func init() {
	nextCmd.Flags().Int("pick", 0, "Pick a specific item by number (1-based) when multiple are ready")
	nextCmd.Flags().String("spec", "", "Target spec ID to filter ready work (auto-detected if exactly one active spec)")
	nextCmd.Flags().Bool("force", false, "Bypass the context clear gate (use when you know your context is clean)")
	nextCmd.Flags().Bool("emit-only", false, "Emit bead primer without claiming, creating worktree, or updating state (for multi-agent mode)")
}
