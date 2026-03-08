package cleanup

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Package-level function variables for testability.
var (
	findLocalRootFn = defaultFindLocalRoot
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
func Run(root, specID string, force bool, exec executor.Executor) (*Result, error) {
	result := &Result{SpecID: specID}

	if force {
		if err := exec.Cleanup(specID, true); err != nil {
			return nil, err
		}
		result.WorktreeRemoved = true
		result.BranchDeleted = true
		return result, nil
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

	// Delegate worktree removal and branch deletion to the Executor.
	if err := exec.Cleanup(specID, false); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cleanup: %v\n", err)
	} else {
		result.WorktreeRemoved = true
		result.BranchDeleted = true
	}

	return result, nil
}
