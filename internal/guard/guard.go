package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
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

// degradeConfigOnError is the R5 never-block degrade helper (ADR-0042 §4,
// AC-13 companion subtest), extracted from defaultReadGuardStateWithCache
// so it is directly unit-testable without driving the full phase-
// resolution pipeline: a config load failure — e.g. an invalid
// worktree_root that failed containment.ValidateWorktreeRoot at ingress
// — must never block this best-effort, ambient guard-state read. It
// returns the safe default config and ONE escaped warning line (R4:
// cfgErr.Error() may embed the agent-writable raw worktree_root value, so
// it is routed through termsafe.Escape before it ever reaches a
// terminal) rather than silently swallowing the error with no signal at
// all. cfgErr must be non-nil; callers write the returned warning to
// stderr.
func degradeConfigOnError(cfgErr error) (cfg *config.Config, warning string) {
	cfg = config.DefaultConfig()
	warning = fmt.Sprintf("warning: could not load config (falling back to worktree_root=%s): %s\n", cfg.WorktreeRoot, termsafe.Escape(cfgErr.Error()))
	return cfg, warning
}

func defaultReadGuardState(root string) (*guardState, error) {
	return defaultReadGuardStateWithCache(nil, root)
}

func defaultReadGuardStateWithCache(c *phase.Cache, root string) (*guardState, error) {
	ctx, err := phase.ResolveContextWithCache(c, root)
	if err != nil || ctx == nil {
		return &guardState{}, nil
	}
	// WorktreeMain path in ResolveContext doesn't populate BeadID.
	// Query for an active bead so the redirect points to the bead
	// worktree (not just the spec worktree).
	if ctx.BeadID == "" && ctx.EpicID != "" {
		ctx.BeadID = phase.FindActiveBeadForEpicWithCache(c, ctx.EpicID)
	}
	gs := &guardState{
		ActiveSpec: ctx.SpecID,
	}
	cfg, cfgErr := loadConfigFn(root)
	if cfgErr != nil {
		degradedCfg, warning := degradeConfigOnError(cfgErr)
		cfg = degradedCfg
		fmt.Fprint(os.Stderr, warning)
	}
	// Derive worktree path from context.
	// Validate existence at each level: prefer bead worktree > spec worktree.
	// If neither exists on disk (both deleted during crash/cleanup),
	// leave ActiveWorktree empty so no redirect fires.
	if ctx.SpecID != "" {
		specWt := workspace.SpecWorktreePath(root, cfg, ctx.SpecID)
		if ctx.BeadID != "" {
			beadWt := workspace.BeadWorktreePath(specWt, cfg, ctx.BeadID)
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
	return checkCWDWithCache(nil, root)
}

// CheckCWDWithCache is the cache-aware variant of CheckCWD.
func CheckCWDWithCache(c *phase.Cache, root string) error {
	return checkCWDWithCache(c, root)
}

func checkCWDWithCache(c *phase.Cache, root string) error {
	cfg, err := loadConfigFn(root)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	if !cfg.Enforcement.CLIGuards {
		return nil
	}

	var gs *guardState
	if c != nil {
		gs, err = defaultReadGuardStateWithCache(c, root)
	} else {
		gs, err = readGuardStateFn(root)
	}
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
		specWtName := workspace.SpecWorktreeName(gs.ActiveSpec)
		specWtAbs, _ := filepath.Abs(filepath.Join(root, cfg.WorktreeRoot, specWtName))
		if strings.HasPrefix(cwdAbs, specWtAbs) {
			return nil
		}
	}

	// If CWD is under the main repo root (not the worktree), block.
	rootAbs, _ := filepath.Abs(root)
	if strings.HasPrefix(cwdAbs, rootAbs) {
		// Req 12 (spec 092): guard failures end with a `recovery:` line.
		return NewFailure(
			fmt.Sprintf("mindspec: CWD is the main worktree; the active worktree is %s", gs.ActiveWorktree),
			containment.EmitCd(gs.ActiveWorktree),
		)
	}

	return nil
}

// IsMainCWD returns true if CWD is the main worktree and a worktree is active.
func IsMainCWD(root string) bool {
	return CheckCWD(root) != nil
}

// IsMainCWDWithCache is the cache-aware variant of IsMainCWD.
func IsMainCWDWithCache(c *phase.Cache, root string) bool {
	return CheckCWDWithCache(c, root) != nil
}

// ActiveWorktreePath returns the active worktree path from beads context, or empty string.
// Constructs a fresh cache; cache-aware callers should use ActiveWorktreePathWithCache.
func ActiveWorktreePath(root string) string {
	gs, err := readGuardStateFn(root)
	if err != nil {
		return ""
	}
	return gs.ActiveWorktree
}

// ActiveWorktreePathWithCache is the cache-aware variant of ActiveWorktreePath.
// PERF-1: lets cmd/mindspec/instruct.go share its phase.Cache with the
// guard.ActiveWorktreePath call that precedes spec resolution, so the warm
// path stays within the ≤3 bd-call budget instead of paying an extra
// uncached `bd list --type=epic`.
func ActiveWorktreePathWithCache(c *phase.Cache, root string) string {
	gs, err := defaultReadGuardStateWithCache(c, root)
	if err != nil {
		return ""
	}
	return gs.ActiveWorktree
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
