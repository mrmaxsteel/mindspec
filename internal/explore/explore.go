package explore

import (
	"fmt"

	"github.com/mindspec/mindspec/internal/specinit"
	"github.com/mindspec/mindspec/internal/state"
)

// Enter validates the current state is idle (or absent) and transitions to explore mode.
func Enter(root, description string) error {
	mc, err := state.ReadFocus(root)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if mc == nil {
		// No focus file means idle — OK to proceed
		mc = &state.Focus{Mode: state.ModeIdle}
	}

	if mc.Mode != state.ModeIdle && mc.Mode != "" {
		return fmt.Errorf("cannot enter explore mode: currently in %q mode (must be idle)", mc.Mode)
	}

	return state.WriteFocus(root, &state.Focus{Mode: state.ModeExplore})
}

// Dismiss validates the current state is explore and transitions to idle.
func Dismiss(root string) error {
	mc, err := state.ReadFocus(root)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}

	if mc.Mode != state.ModeExplore {
		return fmt.Errorf("cannot dismiss: not in explore mode (currently %q)", mc.Mode)
	}

	return state.WriteFocus(root, &state.Focus{Mode: state.ModeIdle})
}

// Promote validates the current state is explore and delegates to spec-init.
// specinit.Run handles the state transition to spec mode and molecule creation.
func Promote(root, specID, title string) error {
	mc, err := state.ReadFocus(root)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}

	if mc.Mode != state.ModeExplore {
		return fmt.Errorf("cannot promote: not in explore mode (currently %q)", mc.Mode)
	}

	_, err = specinit.Run(root, specID, title)
	return err
}
