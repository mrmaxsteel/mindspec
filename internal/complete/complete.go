package complete

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/state"
)

// Package-level function variables for testability.
var (
	readStateFn      = state.Read
	writeStateFn     = state.Write
	setModeFn        = state.SetMode
	closeBeadFn      = bead.Close
	worktreeListFn   = bead.WorktreeList
	worktreeRemoveFn = bead.WorktreeRemove
	runBDFn          = bead.RunBD
	execCommandFn    = exec.Command
	mergeBranchFn    = gitops.MergeBranch
	deleteBranchFn   = gitops.DeleteBranch
)

// Result summarizes what mindspec complete did.
type Result struct {
	BeadID          string
	BeadClosed      bool
	WorktreeRemoved bool
	NextMode        string
	NextBead        string
	NextSpec        string
}

// Run orchestrates bead completion: close bead, remove worktree, advance state.
func Run(root, beadID string) (*Result, error) {
	// 1. Read state → get activeBead if beadID empty
	s, err := readStateFn(root)
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	specID := s.ActiveSpec
	if beadID == "" {
		beadID = s.ActiveBead
	}
	if beadID == "" {
		return nil, fmt.Errorf("no bead ID provided and no activeBead in state")
	}

	// 2. Find worktree matching bead
	var wtName, wtPath string
	entries, err := worktreeListFn()
	if err == nil {
		expectedName := "worktree-" + beadID
		expectedBranch := "bead/" + beadID
		for _, e := range entries {
			if e.Name == expectedName || e.Branch == expectedBranch {
				wtName = e.Name
				wtPath = e.Path
				break
			}
		}
	}

	// 3. Check clean tree
	checkPath := wtPath
	if checkPath == "" {
		checkPath = root // No worktree — check main tree
	}
	if err := checkCleanWorktree(checkPath); err != nil {
		return nil, err
	}

	// 4. Close bead
	if err := closeBeadFn(beadID); err != nil {
		return nil, fmt.Errorf("closing bead: %w", err)
	}

	// 4.5. Emit recording bead marker (best-effort)
	if specID != "" {
		_ = recording.EmitBeadMarker(root, specID, "complete", beadID)
	}

	// Note: parent status propagation is handled natively by beads molecules

	result := &Result{
		BeadID:     beadID,
		BeadClosed: true,
	}

	// 4.7. Merge bead branch back to spec branch (ADR-0006).
	beadBranch := "bead/" + beadID
	if s.SpecBranch != "" && wtPath != "" {
		if err := mergeBranchFn(wtPath, beadBranch, s.SpecBranch); err != nil {
			fmt.Printf("Warning: could not merge %s → %s: %v\n", beadBranch, s.SpecBranch, err)
		}
	}

	// 5. Remove worktree (if found)
	if wtName != "" {
		if err := worktreeRemoveFn(wtName); err != nil {
			// Non-fatal: worktree might already be removed
			fmt.Printf("Warning: could not remove worktree %s: %v\n", wtName, err)
		} else {
			result.WorktreeRemoved = true
		}
	}

	// 5.5. Delete the bead branch after merge + worktree removal (best-effort).
	if s.SpecBranch != "" {
		if err := deleteBranchFn(beadBranch); err != nil {
			fmt.Printf("Warning: could not delete branch %s: %v\n", beadBranch, err)
		}
	}

	// 5.7. Reset activeWorktree to spec worktree (so agent returns to spec context).
	if s.SpecBranch != "" && s.ActiveWorktree != "" {
		// Find the spec worktree (worktree-spec-<specID>).
		specWtName := "worktree-spec-" + specID
		entries2, listErr := worktreeListFn()
		if listErr == nil {
			for _, e := range entries2 {
				if e.Name == specWtName || e.Branch == s.SpecBranch {
					s.ActiveWorktree = e.Path
					if writeErr := writeStateFn(root, s); writeErr != nil {
						fmt.Printf("Warning: could not update activeWorktree: %v\n", writeErr)
					}
					break
				}
			}
		}
	}

	// 6. Advance state
	nextMode, nextBead := advanceState(root, specID)
	result.NextMode = nextMode
	result.NextBead = nextBead
	result.NextSpec = specID

	switch nextMode {
	case state.ModeImplement:
		if err := setModeFn(root, state.ModeImplement, specID, nextBead); err != nil {
			return result, fmt.Errorf("advancing state to implement: %w", err)
		}
		// Set needs_clear flag so the next `mindspec next` requires a context reset.
		if cur, readErr := readStateFn(root); readErr == nil {
			cur.NeedsClear = true
			if writeErr := writeStateFn(root, cur); writeErr != nil {
				fmt.Printf("Warning: could not set needs_clear flag: %v\n", writeErr)
			}
		}
	case state.ModePlan:
		if err := setModeFn(root, state.ModePlan, specID, ""); err != nil {
			return result, fmt.Errorf("advancing state to plan: %w", err)
		}
	case state.ModeReview:
		if err := setModeFn(root, state.ModeReview, specID, ""); err != nil {
			return result, fmt.Errorf("advancing state to review: %w", err)
		}
	default:
		// idle
		result.NextSpec = ""
		s := &state.State{Mode: state.ModeIdle}
		if err := state.Write(root, s); err != nil {
			return result, fmt.Errorf("advancing state to idle: %w", err)
		}
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
		fmt.Fprintf(&sb, "Next bead: %s (mode: implement)\n", r.NextBead)
		sb.WriteString("Run `mindspec next` to claim and start.\n")
	case state.ModePlan:
		fmt.Fprintf(&sb, "Remaining beads are blocked. Mode: plan (spec: %s)\n", r.NextSpec)
	case state.ModeReview:
		fmt.Fprintf(&sb, "All beads complete. Mode: review (spec: %s)\n", r.NextSpec)
		sb.WriteString("Review implementation against acceptance criteria, then use `/impl-approve` to accept.\n")
	default:
		sb.WriteString("All beads complete. Mode: idle\n")
	}
	return sb.String()
}

// checkCleanWorktree verifies a worktree has no uncommitted changes.
func checkCleanWorktree(path string) error {
	cmd := execCommandFn("git", "-C", path, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("worktree has uncommitted changes — commit before completing:\n%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// advanceState determines the next mode after completing a bead.
func advanceState(root, specID string) (mode, nextBead string) {
	if specID == "" {
		return state.ModeIdle, ""
	}

	// Read molecule ID from state
	s, err := readStateFn(root)
	if err != nil {
		return state.ModeIdle, ""
	}
	molParentID := s.ActiveMolecule
	if molParentID == "" {
		return state.ModeIdle, ""
	}

	// Check for ready children in the molecule
	out, err := runBDFn("ready", "--parent", molParentID, "--json")
	if err == nil {
		var ready []bead.BeadInfo
		if json.Unmarshal(out, &ready) == nil && len(ready) > 0 {
			return state.ModeImplement, ready[0].ID
		}
	}

	// Check for open (but blocked) children
	implPrefix := "[IMPL " + specID + "."
	out, err = runBDFn("search", implPrefix, "--json", "--status=open")
	if err == nil {
		var open []bead.BeadInfo
		if json.Unmarshal(out, &open) == nil && len(open) > 0 {
			return state.ModePlan, ""
		}
	}

	// All beads done → review gate (human must approve before idle)
	return state.ModeReview, ""
}
