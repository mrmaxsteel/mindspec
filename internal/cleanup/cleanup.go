package cleanup

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitops"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
		return forceCleanup(result, specWtName, specBranch)
	}

	// Derive context from beads to check if spec is still active (ADR-0023).
	localRoot := root
	if lr, err := findLocalRootFn(); err == nil {
		localRoot = lr
	}
	ctx, _ := phase.ResolveContextFromDir(root, localRoot)
	if ctx != nil && ctx.SpecID == specID && ctx.Phase != state.ModeIdle && ctx.Phase != "" {
		return nil, fmt.Errorf("spec %s is still active (phase: %s). Run `mindspec impl approve %s` first", specID, ctx.Phase, specID)
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
func forceCleanup(result *Result, wtName, branch string) (*Result, error) {
	if err := worktreeRemoveFn(wtName); err != nil {
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove worktree: %v", err))
		}
	} else {
		result.WorktreeRemoved = true
	}

	if gitops.BranchExists(branch) {
		if err := deleteBranchFn(branch); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not delete branch: %v", err))
		} else {
			result.BranchDeleted = true
		}
	}

	return result, nil
}
