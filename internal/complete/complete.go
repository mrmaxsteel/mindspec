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
	"github.com/mindspec/mindspec/internal/workspace"
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
	findLocalRootFn     = defaultFindLocalRoot
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

func defaultFindLocalRoot() (string, error) {
	return workspace.FindLocalRoot(".")
}

// Run orchestrates bead completion: close bead, remove worktree, advance state.
// root is the main repo root (for spec dirs, lifecycle, merges).
// Focus is read from localRoot (per-worktree focus).
func Run(root, beadID string) (*Result, error) {
	// Determine local root for per-worktree focus reads.
	localRoot := root
	if lr, err := findLocalRootFn(); err == nil {
		localRoot = lr
	}

	// 1. Derive activeSpec from resolver, activeBead from arg or Beads query
	specID, err := resolveTargetFn(root, "")
	if err != nil {
		return nil, fmt.Errorf("resolving active spec: %w", err)
	}
	if beadID == "" {
		beadID, err = resolveActiveBeadFn(root, specID)
		if err != nil {
			// Fallback: check focus for activeBead
			if focus, ferr := state.ReadFocus(localRoot); ferr == nil && focus != nil && focus.ActiveBead != "" {
				beadID = focus.ActiveBead
			} else {
				return nil, fmt.Errorf("resolving active bead: %w", err)
			}
		}
	}
	if beadID == "" {
		// Final fallback: check focus for activeBead
		if focus, ferr := state.ReadFocus(localRoot); ferr == nil && focus != nil && focus.ActiveBead != "" {
			beadID = focus.ActiveBead
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
	// Auto-commit .mindspec/ state files so they don't block the clean-tree check.
	if err := autoCommitStateFiles(checkPath); err != nil {
		return nil, fmt.Errorf("auto-committing state files: %w", err)
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

	// 6.5. Write focus with next state (per-worktree: write to spec worktree or local root).
	specWtPath := state.SpecWorktreePath(root, specID)
	mc := &state.Focus{
		Mode:       nextMode,
		ActiveSpec: specID,
		ActiveBead: nextBead,
		SpecBranch: specBranch,
	}
	focusRoot := specWtPath
	if nextMode == state.ModeIdle {
		result.NextSpec = ""
		mc.ActiveSpec = ""
		mc.SpecBranch = ""
		mc.ActiveWorktree = ""
		focusRoot = localRoot // idle → write to wherever we are
	} else {
		mc.ActiveWorktree = specWtPath
		// Fall back to localRoot if spec worktree .mindspec doesn't exist.
		if _, err := os.Stat(filepath.Join(focusRoot, ".mindspec")); err != nil {
			focusRoot = localRoot
		}
	}
	if err := state.WriteFocus(focusRoot, mc); err != nil {
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
		sb.WriteString("Review implementation against acceptance criteria, then use `/ms:impl-approve` to accept.\n")
	default:
		sb.WriteString("All beads complete. Mode: idle\n")
	}
	return sb.String()
}

// autoCommitStateFiles stages and commits any dirty .mindspec/ state files
// so they don't block the clean-worktree check. These files are written by
// `mindspec next` and are not user content.
func autoCommitStateFiles(path string) error {
	// Stage .mindspec/ files
	add := execCommandFn("git", "-C", path, "add", ".mindspec/")
	if err := add.Run(); err != nil {
		return nil // .mindspec/ may not exist — not an error
	}
	// Check if anything was staged
	diff := execCommandFn("git", "-C", path, "diff", "--cached", "--quiet")
	if diff.Run() == nil {
		return nil // nothing staged
	}
	// Commit the staged state files
	commit := execCommandFn("git", "-C", path, "commit", "-m", "mindspec: state checkpoint")
	if out, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("committing state files: %s", strings.TrimSpace(string(out)))
	}
	return nil
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

	// Read epic_id from lifecycle.yaml (SpecDir is worktree-aware per ADR-0022).
	specDir := workspace.SpecDir(root, specID)
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
