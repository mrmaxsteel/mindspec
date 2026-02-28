package complete

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/next"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/resolve"
	"github.com/mindspec/mindspec/internal/state"
)

// Package-level function variables for testability.
var (
	closeBeadFn         = bead.Close
	worktreeListFn      = bead.WorktreeList
	worktreeRemoveFn    = bead.WorktreeRemove
	runBDFn             = bead.RunBD
	execCommandFn       = exec.Command
	mergeBranchFn       = gitops.MergeBranch
	deleteBranchFn      = gitops.DeleteBranch
	resolveTargetFn     = resolve.ResolveTarget
	resolveActiveBeadFn = next.ResolveActiveBead
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
	// 1. Derive activeSpec from resolver, activeBead from arg or Beads query
	specID, err := resolveTargetFn(root, "")
	if err != nil {
		return nil, fmt.Errorf("resolving active spec: %w", err)
	}
	if beadID == "" {
		beadID, err = resolveActiveBeadFn(root, specID)
		if err != nil {
			return nil, fmt.Errorf("resolving active bead: %w", err)
		}
	}
	if beadID == "" {
		return nil, fmt.Errorf("no bead ID provided and no in-progress bead found for spec %s", specID)
	}

	// Derive spec branch and worktree paths from conventions
	specBranch := state.SpecBranch(specID)

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

	result := &Result{
		BeadID:     beadID,
		BeadClosed: true,
	}

	// 4.7. Merge bead branch back to spec branch (ADR-0006).
	beadBranch := "bead/" + beadID
	if wtPath != "" {
		if err := mergeBranchFn(wtPath, beadBranch, specBranch); err != nil {
			fmt.Printf("Warning: could not merge %s → %s: %v\n", beadBranch, specBranch, err)
		}
	}

	// 5. Remove worktree (if found)
	if wtName != "" {
		if err := worktreeRemoveFn(wtName); err != nil {
			fmt.Printf("Warning: could not remove worktree %s: %v\n", wtName, err)
		} else {
			result.WorktreeRemoved = true
		}
	}

	// 5.5. Delete the bead branch after merge + worktree removal (best-effort).
	if err := deleteBranchFn(beadBranch); err != nil {
		fmt.Printf("Warning: could not delete branch %s: %v\n", beadBranch, err)
	}

	// 6. Advance state
	nextMode, nextBead := advanceState(root, specID)
	result.NextMode = nextMode
	result.NextBead = nextBead
	result.NextSpec = specID

	// 6.5. Write focus with next state
	specWtPath := state.SpecWorktreePath(root, specID)
	mc := &state.Focus{
		Mode:       nextMode,
		ActiveSpec: specID,
		ActiveBead: nextBead,
		SpecBranch: specBranch,
	}
	if nextMode == state.ModeIdle {
		result.NextSpec = ""
		mc.ActiveSpec = ""
		mc.SpecBranch = ""
		mc.ActiveWorktree = ""
	} else {
		mc.ActiveWorktree = specWtPath
	}
	if err := state.WriteFocus(root, mc); err != nil {
		return result, fmt.Errorf("writing focus: %w", err)
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
		sb.WriteString("Review implementation against acceptance criteria, then use `/ms-impl-approve` to accept.\n")
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

	// Read epic_id from lifecycle.yaml
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	if _, err := os.Stat(specDir); err != nil {
		specDir = filepath.Join(root, "docs", "specs", specID)
	}

	lc, err := state.ReadLifecycle(specDir)
	if err != nil || lc == nil || lc.EpicID == "" {
		return state.ModeIdle, ""
	}

	// Check for ready children under the epic
	out, err := runBDFn("ready", "--parent", lc.EpicID, "--json")
	if err == nil {
		var ready []bead.BeadInfo
		if json.Unmarshal(out, &ready) == nil && len(ready) > 0 {
			return state.ModeImplement, ready[0].ID
		}
	}

	// Check for open (but blocked) children — match actual title format [specID]
	implPrefix := "[" + specID + "]"
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
