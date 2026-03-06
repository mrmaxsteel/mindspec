package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
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
	ctx, err := phase.ResolveContext(root)
	if err != nil || ctx == nil {
		return &guardState{}, nil
	}
	// WorktreeMain path in ResolveContext doesn't populate BeadID.
	// Query for an active bead so the redirect points to the bead
	// worktree (not just the spec worktree).
	if ctx.BeadID == "" && ctx.EpicID != "" {
		ctx.BeadID = phase.FindActiveBeadForEpic(ctx.EpicID)
	}
	gs := &guardState{
		ActiveSpec: ctx.SpecID,
	}
	// Derive worktree path from context.
	// Validate existence at each level: prefer bead worktree > spec worktree.
	// If neither exists on disk (both deleted during crash/cleanup),
	// leave ActiveWorktree empty so no redirect fires.
	if ctx.SpecID != "" {
		specWt := state.SpecWorktreePath(root, ctx.SpecID)
		if ctx.BeadID != "" {
			beadWt := state.BeadWorktreePath(specWt, ctx.BeadID)
			if dirExists(beadWt) {
				gs.ActiveWorktree = beadWt
			} else if dirExists(specWt) {
				gs.ActiveWorktree = specWt
			}
		} else if dirExists(specWt) {
			gs.ActiveWorktree = specWt
		}
	}
	return gs, nil
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

// ActiveWorktreePath returns the active worktree path from beads context, or empty string.
func ActiveWorktreePath(root string) string {
	gs, err := readGuardStateFn(root)
	if err != nil {
		return ""
	}
	return gs.ActiveWorktree
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
