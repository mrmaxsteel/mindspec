package workspace

import (
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

// Naming conventions for branches and worktree directories.
//
// These constants are the single source of truth for the strings that
// used to be duplicated as inline literals across the codebase.
// Together with config.WorktreeRoot (default ".worktrees"), they
// identify every worktree MindSpec manages.
//
// Note: the bead worktree directory prefix is "worktree-" (no "bead-"
// infix). The bead ID itself disambiguates it from a spec worktree —
// SpecWorktreePrefix sits inside the same namespace ("worktree-spec-...").
const (
	SpecBranchPrefix   = "spec/"
	BeadBranchPrefix   = "bead/"
	SpecWorktreePrefix = "worktree-spec-"
	BeadWorktreePrefix = "worktree-"

	// FinalizeBranchPrefix names bug wu7t's protected-main finalize
	// carrier: when a spec branch is already merged into main before
	// `impl approve` runs (the common already-merged-implementation-PR
	// case), the epic-close JSONL export commit rides a FRESH from-main
	// branch instead of the dead spec branch. See
	// MindspecExecutor.FinalizeEpic.
	FinalizeBranchPrefix = "chore/finalize-"
	// FinalizeWorktreePrefix names the TEMPORARY worktree used to commit
	// onto FinalizeBranchPrefix; removed before FinalizeEpic returns.
	FinalizeWorktreePrefix = "worktree-finalize-"
)

// SpecBranch returns the canonical branch name for a spec.
// Pure naming convention — no config dependency. Validates specID via
// idvalidate.SpecID and fails closed (the SpecDir SEC-1 precedent, ADR-0042):
// no caller can obtain a composed branch name from an invalid ID.
func SpecBranch(specID string) (string, error) {
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	return SpecBranchPrefix + specID, nil
}

// BeadBranch returns the canonical branch name for a bead.
// Pure naming convention — no config dependency. Validates beadID via
// idvalidate.BeadID and fails closed (ADR-0042 composition waist).
func BeadBranch(beadID string) (string, error) {
	if err := idvalidate.BeadID(beadID); err != nil {
		return "", err
	}
	return BeadBranchPrefix + beadID, nil
}

// SpecWorktreeName returns the directory basename for a spec worktree
// (e.g. "worktree-spec-053-foo"). Pure naming convention. Validates
// specID via idvalidate.SpecID and fails closed (ADR-0042).
func SpecWorktreeName(specID string) (string, error) {
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	return SpecWorktreePrefix + specID, nil
}

// BeadWorktreeName returns the directory basename for a bead worktree
// (e.g. "worktree-mindspec-c8q0"). Pure naming convention. Validates
// beadID via idvalidate.BeadID and fails closed (ADR-0042).
func BeadWorktreeName(beadID string) (string, error) {
	if err := idvalidate.BeadID(beadID); err != nil {
		return "", err
	}
	return BeadWorktreePrefix + beadID, nil
}

// FinalizeBranch returns bug wu7t's from-main finalize-carrier branch name
// for a spec (e.g. "chore/finalize-053-foo"). Pure naming convention.
// Validates specID via idvalidate.SpecID and fails closed (ADR-0042).
func FinalizeBranch(specID string) (string, error) {
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	return FinalizeBranchPrefix + specID, nil
}

// FinalizeWorktreeName returns the directory basename for bug wu7t's
// TEMPORARY finalize worktree (e.g. "worktree-finalize-053-foo"). Pure
// naming convention. Validates specID via idvalidate.SpecID and fails
// closed (ADR-0042).
func FinalizeWorktreeName(specID string) (string, error) {
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	return FinalizeWorktreePrefix + specID, nil
}

// WorktreesDir returns the absolute worktrees-root directory path under
// root, honoring cfg.WorktreeRoot. If cfg is nil the default config is
// used so callers can use this helper before loading config explicitly.
func WorktreesDir(root string, cfg *config.Config) string {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return filepath.Join(root, cfg.WorktreeRoot)
}

// DefaultWorktreesDir returns the absolute worktrees-root directory
// path under root using the default config. Convenience wrapper for
// test helpers and other call sites that have no *config.Config in
// scope.
func DefaultWorktreesDir(root string) string {
	return WorktreesDir(root, nil)
}

// SpecWorktreePath returns the absolute spec worktree path under root,
// honoring cfg.WorktreeRoot. If cfg is nil the default config is used.
// Validates specID via idvalidate.SpecID (through SpecWorktreeName) and
// fails closed (ADR-0042).
func SpecWorktreePath(root string, cfg *config.Config, specID string) (string, error) {
	name, err := SpecWorktreeName(specID)
	if err != nil {
		return "", err
	}
	return filepath.Join(WorktreesDir(root, cfg), name), nil
}

// BeadWorktreePath returns the absolute bead worktree path nested
// under its spec worktree. cfg.WorktreeRoot controls the nested
// worktrees-root directory name. If cfg is nil the default config is
// used. Validates beadID via idvalidate.BeadID (through BeadWorktreeName)
// and fails closed (ADR-0042).
func BeadWorktreePath(specWorktree string, cfg *config.Config, beadID string) (string, error) {
	name, err := BeadWorktreeName(beadID)
	if err != nil {
		return "", err
	}
	return filepath.Join(WorktreesDir(specWorktree, cfg), name), nil
}

// FinalizeWorktreePath returns the absolute path to bug wu7t's TEMPORARY
// finalize worktree under root, honoring cfg.WorktreeRoot. If cfg is nil
// the default config is used. Validates specID via idvalidate.SpecID
// (through FinalizeWorktreeName) and fails closed (ADR-0042).
func FinalizeWorktreePath(root string, cfg *config.Config, specID string) (string, error) {
	name, err := FinalizeWorktreeName(specID)
	if err != nil {
		return "", err
	}
	return filepath.Join(WorktreesDir(root, cfg), name), nil
}
