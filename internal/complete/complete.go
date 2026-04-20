package complete

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Package-level function variables for testability.
var (
	closeBeadFn       = bead.Close
	worktreeListFn    = bead.WorktreeList
	runBDFn           = bead.RunBD
	listJSONFn        = bead.ListJSON
	resolveTargetFn   = resolve.ResolveTarget
	findLocalRootFn   = defaultFindLocalRoot
	fetchBeadByIDFn   = next.FetchBeadByID
	findEpicForBeadFn = phase.FindEpicForBead
)

// Result summarizes what mindspec complete did.
type Result struct {
	BeadID          string
	BeadClosed      bool
	WorktreeRemoved bool
	NextMode        string
	NextBead        string
	NextSpec        string
	SpecWorktree    string
}

func defaultFindLocalRoot() (string, error) {
	return workspace.FindLocalRoot(".")
}

// Run orchestrates bead completion: close bead, remove worktree, advance state.
// root is the main repo root (for spec dirs, lifecycle, merges).
// beadID is required — it must always be provided by the caller.
// exec is the Executor used for all git/workspace operations.
// specIDHint is optional and typically comes from --spec for disambiguation.
func Run(root, beadID, specIDHint, commitMsg string, exec executor.Executor) (*Result, error) {
	// Determine local root for per-worktree context resolution.
	localRoot := root
	if lr, err := findLocalRootFn(); err == nil {
		localRoot = lr
	}

	// 1. Derive activeSpec from resolver.
	// Try localRoot first (per-worktree context) then fall back to root.
	specID, err := resolveTargetFn(localRoot, specIDHint)
	if err != nil && localRoot != root {
		specID, err = resolveTargetFn(root, specIDHint)
	}
	// If still ambiguous but we have a bead ID, resolve spec from the bead's parent epic.
	if err != nil && beadID != "" {
		if _, derivedSpec, beadErr := findEpicForBeadFn(beadID); beadErr == nil && derivedSpec != "" {
			specID = derivedSpec
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("resolving active spec: %w", err)
	}

	// 1.5. Impl-only guard: verify the epic phase is implement or review.
	epicID, epicErr := phase.FindEpicBySpecID(specID)
	if epicErr == nil && epicID != "" {
		epicPhase, phaseErr := phase.DerivePhase(epicID)
		if phaseErr == nil && epicPhase != state.ModeImplement && epicPhase != state.ModeReview {
			return nil, fmt.Errorf("bead %s belongs to spec %s which is in '%s' phase.\nmindspec complete is for implementation beads only.", beadID, specID, epicPhase)
		}
	}

	// Derive spec branch from conventions
	specBranch := state.SpecBranch(specID)

	// 2. Find worktree matching bead (needed for commit/clean-tree paths)
	var wtPath string
	entries, err := worktreeListFn()
	if err == nil {
		expectedName := "worktree-" + beadID
		expectedBranch := "bead/" + beadID
		for _, e := range entries {
			if e.Name == expectedName || e.Branch == expectedBranch {
				wtPath = e.Path
				break
			}
		}
	}

	// 2.5. Auto-commit if commit message provided (via Executor)
	commitPath := wtPath
	if commitPath == "" {
		commitPath = root
	}
	if commitMsg != "" {
		msg := fmt.Sprintf("impl(%s): %s", beadID, commitMsg)
		if err := exec.CommitAll(commitPath, msg); err != nil {
			return nil, fmt.Errorf("auto-commit failed: %w", err)
		}
	}

	// 3. Check clean tree
	checkPath := wtPath
	if checkPath == "" {
		checkPath = root // No worktree — check main tree
	}
	if err := exec.IsTreeClean(checkPath); err != nil {
		if wtPath == "" {
			return nil, fmt.Errorf("%w\nhint: no active bead worktree is set. Run `mindspec next`, `cd` into the printed worktree path, then commit and rerun `mindspec complete`", err)
		}
		return nil, fmt.Errorf("%w\nhint: use `mindspec complete %s \"describe what you did\"` to auto-commit", err, beadID)
	}

	// 4. Close bead (idempotent: tolerate already-closed beads)
	if err := closeBeadFn(beadID); err != nil {
		// Check if the bead is already closed — if so, warn and continue cleanup.
		info, fetchErr := fetchBeadByIDFn(beadID)
		if fetchErr == nil && strings.EqualFold(strings.TrimSpace(info.Status), "closed") {
			fmt.Printf("Warning: bead %s already closed — performing merge and cleanup.\n", beadID)
		} else {
			return nil, fmt.Errorf("closing bead: %w", err)
		}
	}

	// 4.5. Emit recording bead marker (best-effort)
	if specID != "" {
		_ = recording.EmitBeadMarker(root, specID, "complete", beadID)
	}

	result := &Result{
		BeadID:       beadID,
		BeadClosed:   true,
		SpecWorktree: filepath.Join(root, ".worktrees", "worktree-spec-"+specID),
	}

	// 5. Merge bead→spec, remove worktree, delete branch (via Executor).
	// Pass empty msg since we already handled commit+clean-tree above.
	if err := exec.CompleteBead(beadID, specBranch, ""); err != nil {
		fmt.Printf("Warning: bead cleanup: %v\n", err)
	} else {
		result.WorktreeRemoved = true
	}

	// 6. Advance state
	nextMode, nextBead := advanceState(specID)
	result.NextMode = nextMode
	result.NextBead = nextBead
	result.NextSpec = specID

	// 6.5. Sync stored phase (Spec 080): keep epic mindspec_phase in sync
	// so that DerivePhase (metadata-first) returns the correct phase for
	// downstream commands like `mindspec impl approve`.
	if nextMode != "" {
		if eid, findErr := phase.FindEpicBySpecID(specID); findErr == nil && eid != "" {
			_ = bead.MergeMetadata(eid, map[string]interface{}{"mindspec_phase": nextMode})
		}
	}

	// ADR-0023: no focus write — state is derived from beads.
	if nextMode == state.ModeIdle {
		result.NextSpec = ""
	}

	return result, nil
}

// FormatResult returns a human-readable summary of the completion.
func FormatResult(r *Result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Bead %s closed.\n", r.BeadID)
	if r.WorktreeRemoved {
		sb.WriteString("Worktree removed.\n")
	}
	switch r.NextMode {
	case state.ModeImplement:
		fmt.Fprintf(&sb, "Next bead ready: %s\n", r.NextBead)
		fmt.Fprintf(&sb, "Mode: implement (spec: %s)\n", r.NextSpec)
		sb.WriteString("\nSTOP HERE. Do NOT run `mindspec next` or claim another bead.\nTell the user: run `/clear` (or start a fresh agent), then `mindspec next` to continue.\n")
	case state.ModePlan:
		fmt.Fprintf(&sb, "Remaining beads are blocked. Mode: plan (spec: %s)\n", r.NextSpec)
		if r.WorktreeRemoved && r.SpecWorktree != "" {
			fmt.Fprintf(&sb, "Run: `cd %s`\n", r.SpecWorktree)
		}
	case state.ModeReview:
		fmt.Fprintf(&sb, "All beads complete. Mode: review (spec: %s)\n", r.NextSpec)
		if r.WorktreeRemoved && r.SpecWorktree != "" {
			fmt.Fprintf(&sb, "Run: `cd %s`\n", r.SpecWorktree)
		}
		sb.WriteString("Run `mindspec instruct` for review guidance and next steps.\n")
	default:
		sb.WriteString("All beads complete. Mode: idle\n")
	}
	return sb.String()
}

// advanceState determines the next mode after completing a bead.
//
// Phase is derived authoritatively via phase.DerivePhaseFromChildren against
// the full child-status mix (open, in_progress, blocked, closed, and any
// custom bd statuses the repo declares — e.g. this repo's `resolved` gate
// status). Earlier revisions only queried `--status=open`, which silently
// dropped in_progress beads held by a parallel agent and any custom status,
// causing premature flips to review mode.
//
// If phase derives to implement, a `bd ready` call resolves a specific next
// bead; otherwise nextBead stays empty and the caller prints the right
// guidance for plan / review / idle.
func advanceState(specID string) (mode, nextBead string) {
	if specID == "" {
		return state.ModeIdle, ""
	}

	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return state.ModeIdle, ""
	}

	children := queryAllChildren(epicID)
	derivedPhase := phase.DerivePhaseFromChildren(children)

	if derivedPhase == state.ModeImplement {
		if out, rerr := runBDFn("ready", "--parent", epicID, "--json"); rerr == nil {
			var ready []bead.BeadInfo
			if json.Unmarshal(out, &ready) == nil && len(ready) > 0 {
				return state.ModeImplement, ready[0].ID
			}
		}
		// Implement phase without a ready bead: we're between beads (next one
		// is blocked on a dep that just closed but hasn't propagated, or the
		// only remaining work is in_progress with a peer). Stay in implement
		// without a concrete next bead rather than flipping to review.
		return state.ModeImplement, ""
	}

	return derivedPhase, ""
}

// queryAllChildren pulls child beads under an epic across every status bd
// recognises — including custom statuses declared in the project's
// .beads/config.yaml. Mirrors phase.queryChildren (package-private there).
func queryAllChildren(epicID string) []phase.ChildInfo {
	var all []phase.ChildInfo
	seen := map[string]bool{}

	// Fast path: one list call with no status filter returns every child.
	// bd list without --status defaults to open-only, so we still fall
	// through to per-status fan-out for complete coverage if that ever
	// changes. The extra cost is a handful of Dolt calls, amortised by
	// Dolt's in-memory cache on the mindspec server.
	for _, status := range []string{"open", "in_progress", "blocked", "closed", "resolved"} {
		out, err := listJSONFn("--parent", epicID, "--status="+status)
		if err != nil {
			continue
		}
		var batch []phase.ChildInfo
		if json.Unmarshal(out, &batch) != nil {
			continue
		}
		for _, c := range batch {
			if !seen[c.ID] {
				seen[c.ID] = true
				all = append(all, c)
			}
		}
	}
	return all
}
