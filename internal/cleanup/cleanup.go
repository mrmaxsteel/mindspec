package cleanup

import (
	"fmt"
	"os"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Package-level function variables for testability.
var (
	worktreeRemoveFn = bead.WorktreeRemove
	deleteBranchFn   = gitops.DeleteBranch
	findLocalRootFn  = defaultFindLocalRoot
)

func defaultFindLocalRoot() (string, error) {
	return workspace.FindLocalRoot(".")
}

// Result holds the output of a cleanup operation.
type Result struct {
	SpecID          string
	WorktreeRemoved bool
	BranchDeleted   bool
	Warnings        []string
}

// Run cleans up worktree and branch resources for a completed spec.
// It removes the worktree and branch when the spec lifecycle is done.
func Run(root, specID string, force bool) (*Result, error) {
	result := &Result{SpecID: specID}

	// Determine the spec branch and worktree name from conventions.
	specBranch := state.SpecBranch(specID)
	specWtName := "worktree-spec-" + specID

	if force {
		return forceCleanup(root, result, specWtName, specBranch)
	}

	// If focus still has activeSpec matching, check mode (per-worktree focus).
	localRoot := root
	if lr, err := findLocalRootFn(); err == nil {
		localRoot = lr
	}
	mc, _ := state.ReadFocus(localRoot)
	if mc != nil && mc.ActiveSpec == specID && mc.Mode != state.ModeIdle {
		return nil, fmt.Errorf("spec %s is still active (mode: %s). Run `mindspec impl approve %s` first", specID, mc.Mode, specID)
	}

	// Check if branch still exists locally.
	if !gitops.BranchExists(specBranch) {
		fmt.Fprintf(os.Stderr, "Branch %s already deleted.\n", specBranch)
	}

	// Remove worktree (best-effort).
	if err := worktreeRemoveFn(specWtName); err != nil {
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove worktree: %v", err))
		}
	} else {
		result.WorktreeRemoved = true
	}

	// Clear focus if it still points to this spec (prevent stale worktree deadlock).
	clearFocusIfStale(localRoot, specID)

	// Delete branch (best-effort).
	if gitops.BranchExists(specBranch) {
		if err := deleteBranchFn(specBranch); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not delete branch: %v", err))
		} else {
			result.BranchDeleted = true
		}
	}

	return result, nil
}

// forceCleanup removes worktree and branch without checking state.
func forceCleanup(root string, result *Result, wtName, branch string) (*Result, error) {
	if err := worktreeRemoveFn(wtName); err != nil {
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove worktree: %v", err))
		}
	} else {
		result.WorktreeRemoved = true
	}

	// Clear focus if it still points to this spec (per-worktree).
	forceLocalRoot := root
	if lr, err := findLocalRootFn(); err == nil {
		forceLocalRoot = lr
	}
	clearFocusIfStale(forceLocalRoot, result.SpecID)

	if gitops.BranchExists(branch) {
		if err := deleteBranchFn(branch); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not delete branch: %v", err))
		} else {
			result.BranchDeleted = true
		}
	}

	return result, nil
}

// clearFocusIfStale resets focus to idle if it still references the given spec.
func clearFocusIfStale(root, specID string) {
	f, err := state.ReadFocus(root)
	if err != nil || f == nil {
		return
	}
	if f.ActiveSpec == specID {
		_ = state.WriteFocus(root, &state.Focus{Mode: state.ModeIdle})
	}
}
