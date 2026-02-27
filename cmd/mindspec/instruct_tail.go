package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/guard"
	"github.com/mindspec/mindspec/internal/instruct"
	"github.com/mindspec/mindspec/internal/state"
)

// emitInstruct reads mode-cache and prints mode-appropriate guidance.
// This is the "instruct-tail" convention: every state-changing command
// (approve, next, complete) calls this after transitioning to emit
// guidance for the new mode.
func emitInstruct(root string) error {
	mc, err := state.ReadModeCache(root)
	if err != nil {
		mc = &state.ModeCache{Mode: state.ModeIdle}
	}

	// CWD redirect: if on main with active worktree, emit redirect only.
	if wtPath := guard.ActiveWorktreePath(root); wtPath != "" && guard.IsMainCWD(root) {
		fmt.Fprintf(os.Stdout, "\n## Worktree Redirect\n\nYou are in the main worktree. Switch to:\n\n  cd %s\n\nThen run `mindspec instruct` for mode-appropriate guidance.\n", wtPath)
		return nil
	}

	// Build a State from mode-cache for instruct.BuildContext compatibility.
	s := &state.State{
		Mode:       mc.Mode,
		ActiveSpec: mc.ActiveSpec,
		ActiveBead: mc.ActiveBead,
	}
	ctx := instruct.BuildContext(root, s)

	// Add worktree check when an active worktree is set.
	if mc.ActiveWorktree != "" {
		if warning := instruct.CheckWorktree(mc.ActiveWorktree); warning != "" {
			ctx.Warnings = append(ctx.Warnings, "[worktree] "+warning)
		}
	}

	output, err := instruct.Render(ctx)
	if err != nil {
		return fmt.Errorf("rendering guidance: %w", err)
	}

	fmt.Fprint(os.Stdout, output)
	return nil
}
