package main

import (
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/instruct"
	"github.com/mindspec/mindspec/internal/state"
)

// emitInstruct reads current state and prints mode-appropriate guidance.
// This is the "instruct-tail" convention: every state-changing command
// (approve, next, complete) calls this after transitioning to emit
// guidance for the new mode.
func emitInstruct(root string) error {
	s, err := state.Read(root)
	if err != nil {
		if err == state.ErrNoState {
			s = &state.State{Mode: state.ModeIdle}
		} else {
			return fmt.Errorf("reading state: %w", err)
		}
	}

	ctx := instruct.BuildContext(root, s)

	if s.Mode == state.ModeImplement {
		if warning := instruct.CheckWorktree(s.ActiveBead); warning != "" {
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

