package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/state"
)

// guardState holds the subset of state that guards need.
type guardState struct {
	ActiveWorktree string
	ActiveSpec     string
}

// Package-level function variables for testability.
var (
	readGuardStateFn = defaultReadGuardState
	loadConfigFn     = config.Load
	getwdFn          = os.Getwd
)

func defaultReadGuardState(root string) (*guardState, error) {
	mc, err := state.ReadFocus(root)
	if err != nil {
		return nil, err
	}
	if mc == nil {
		return &guardState{}, nil
	}
	return &guardState{
		ActiveWorktree: mc.ActiveWorktree,
		ActiveSpec:     mc.ActiveSpec,
	}, nil
}

// CheckCWD verifies the current working directory matches the active worktree.
// Returns an error if CWD is the main worktree when a worktree is active.
// Returns nil if no worktree is active, guards are disabled, or CWD is correct.
func CheckCWD(root string) error {
	cfg, err := loadConfigFn(root)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	if !cfg.Enforcement.CLIGuards {
		return nil
	}

	gs, err := readGuardStateFn(root)
	if err != nil || gs.ActiveWorktree == "" {
		return nil
	}

	cwd, err := getwdFn()
	if err != nil {
		return nil
	}

	// Normalize paths for comparison.
	cwdAbs, _ := filepath.Abs(cwd)
	wtAbs, _ := filepath.Abs(gs.ActiveWorktree)

	// If CWD is under the active worktree, it's fine.
	if strings.HasPrefix(cwdAbs, wtAbs) {
		return nil
	}

	// Also allow the spec worktree — lifecycle commands (complete, impl-approve)
	// need to run there after all beads are done.
	if gs.ActiveSpec != "" {
		specWtName := "worktree-spec-" + gs.ActiveSpec
		specWtAbs, _ := filepath.Abs(filepath.Join(root, cfg.WorktreeRoot, specWtName))
		if strings.HasPrefix(cwdAbs, specWtAbs) {
			return nil
		}
	}

	// If CWD is under the main repo root (not the worktree), block.
	rootAbs, _ := filepath.Abs(root)
	if strings.HasPrefix(cwdAbs, rootAbs) {
		return fmt.Errorf("mindspec: CWD is the main worktree. Switch to:\n  cd %s", gs.ActiveWorktree)
	}

	return nil
}

// IsMainCWD returns true if CWD is the main worktree and a worktree is active.
func IsMainCWD(root string) bool {
	return CheckCWD(root) != nil
}

// ActiveWorktreePath returns the active worktree path from focus, or empty string.
func ActiveWorktreePath(root string) string {
	gs, err := readGuardStateFn(root)
	if err != nil {
		return ""
	}
	return gs.ActiveWorktree
}
