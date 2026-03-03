package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/guard"
	"github.com/mindspec/mindspec/internal/instruct"
	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/state"
)

// emitInstruct derives state from beads and prints mode-appropriate guidance.
// This is the "instruct-tail" convention: every state-changing command
// (approve, next, complete) calls this after transitioning to emit
// guidance for the new mode.
//
// root is the main repo root (for spec dirs and guard).
func emitInstruct(root string) error {
	// ADR-0023: derive state from beads, not focus files.
	ctx, _ := phase.ResolveContext(root)
	var mc *state.Focus
	if ctx != nil && ctx.Phase != "" {
		mc = &state.Focus{
			Mode:       ctx.Phase,
			ActiveSpec: ctx.SpecID,
			ActiveBead: ctx.BeadID,
		}
		if ctx.WorktreePath != "" {
			mc.ActiveWorktree = ctx.WorktreePath
		}
	}
	if mc == nil {
		mc = &state.Focus{Mode: state.ModeIdle}
	}

	// CWD redirect: if on main with active worktree, emit redirect only.
	if wtPath := guard.ActiveWorktreePath(root); wtPath != "" && guard.IsMainCWD(root) {
		fmt.Fprintf(os.Stdout, "\n## Worktree Redirect\n\nYou are in the main worktree. Switch to:\n\n  cd %s\n\nThen run `mindspec instruct` for mode-appropriate guidance.\n", wtPath)
		return nil
	}

	iCtx := instruct.BuildContext(root, mc)

	// Add worktree check when an active worktree is set.
	if mc.ActiveWorktree != "" {
		if warning := instruct.CheckWorktree(mc.ActiveWorktree); warning != "" {
			iCtx.Warnings = append(iCtx.Warnings, "[worktree] "+warning)
		}
	}

	output, err := instruct.Render(iCtx)
	if err != nil {
		return fmt.Errorf("rendering guidance: %w", err)
	}

	fmt.Fprint(os.Stdout, output)
	return nil
}
