package phase

// derivation_failed_test.go — spec 128 (lifecycle-verb-state-honesty)
// bead 5, round-1 panel G1/G3: at least one internal/phase-level unit
// test pinning a genuine Context.DerivationFailed case directly against
// ResolveContextFromDirWithCache, for each reachable failure site
// (round-1 panel found ZERO direct test references — every setting site
// was covered only transitively through internal/instruct/cmd tests).
//
// DerivePhaseWithCache/DerivePhaseWithStatusWithCache never itself
// returns a non-nil error (every internal helper swallows a lookup
// failure into "" and falls through to a derived value), so the
// WorktreeBead/WorktreeSpec "phaseErr" branches are defensive-only under
// today's implementation — genuinely reachable failures instead come
// from FindEpicForBeadWithCache (a real `bd show` failure, distinct from
// the ErrNoEpicLineage "genuinely no lineage" sentinel), the
// AllEpics-health-probe (a real `bd list --type=epic` failure), and
// DiscoverActiveSpecsWithCache's own epic list query. This file exercises
// those three reachable sites.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

// requireBeadsDir creates root/.beads so BeadsDirMissing(root) is false —
// the fixture precondition for every test below that wants a genuine bd
// failure classified as DerivationFailed rather than the honest
// "workspace never initialized" idle/spec fallback.
func requireBeadsDir(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
}

// TestResolveContextFromDir_BeadWorktree_LookupFailure_SetsDerivationFailed
// pins the WorktreeBead failure site: a genuine `bd show <beadID>`
// failure (not the ErrNoEpicLineage "bead not found" sentinel) sets
// DerivationFailed + DerivationError. EpicID stays unresolved on this
// path, so derivePhaseForSpecWithCache(c, "") legitimately (and
// unchanged from pre-bead-5 behavior) returns ModeSpec — honest
// consumers check DerivationFailed BEFORE Phase, so this fallback value
// is never presented as a derived state.
func TestResolveContextFromDir_BeadWorktree_LookupFailure_SetsDerivationFailed(t *testing.T) {
	root := t.TempDir()
	requireBeadsDir(t, root)

	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return nil, errors.New("bd: dolt daemon unreachable")
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)

	dir := filepath.Join(root, ".worktrees", "worktree-mindspec-abc123")
	ctx, err := ResolveContextFromDir(root, dir)
	if err != nil {
		t.Fatalf("ResolveContextFromDir returned an error (it never should): %v", err)
	}
	if !ctx.DerivationFailed {
		t.Fatal("expected DerivationFailed=true on a genuine bd show failure")
	}
	if ctx.DerivationError == "" {
		t.Error("expected a non-empty DerivationError")
	}
	if ctx.Phase != state.ModeSpec {
		t.Errorf("Phase = %q, want %q (unresolved EpicID falls through to derivePhaseForSpecWithCache(\"\") — unchanged pre-bead-5 behavior)", ctx.Phase, state.ModeSpec)
	}
}

// TestResolveContextFromDir_SpecWorktree_HealthProbeFailure_SetsDerivationFailedButBackfillsModeSpec
// pins the WorktreeSpec failure site (round-1 panel G1/G3's hard block):
// when the epic lookup fails AND the AllEpics health probe ALSO fails
// (with .beads present, so this is genuinely a bd health failure, never
// "workspace never initialized"), DerivationFailed is set — AND Phase is
// STILL unconditionally backfilled to ModeSpec, exactly like
// WorktreeBead's ModeImplement fallback. This is the field's own
// "additive, values unchanged" contract: gating the backfill on
// `!DerivationFailed` (the round-1 hard-block bug) left Phase=="" here,
// which silently flipped internal/cleanup's active-spec refusal
// (`ctx.Phase != ""`) from fail-closed to fail-open under a bd outage.
func TestResolveContextFromDir_SpecWorktree_HealthProbeFailure_SetsDerivationFailedButBackfillsModeSpec(t *testing.T) {
	root := t.TempDir()
	requireBeadsDir(t, root)

	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, errors.New("bd: dolt daemon unreachable")
	})
	t.Cleanup(restoreList)
	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	specID := "128-derivation-failed-test"
	dir := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	ctx, err := ResolveContextFromDir(root, dir)
	if err != nil {
		t.Fatalf("ResolveContextFromDir returned an error (it never should): %v", err)
	}
	if !ctx.DerivationFailed {
		t.Fatal("expected DerivationFailed=true when both the epic lookup and the AllEpics health probe fail")
	}
	if ctx.DerivationError == "" {
		t.Error("expected a non-empty DerivationError")
	}
	if ctx.Phase != state.ModeSpec {
		t.Errorf("Phase = %q, want %q — the WorktreeSpec backfill must be UNCONDITIONAL (round-1 panel G1/G3), never gated on DerivationFailed", ctx.Phase, state.ModeSpec)
	}
	if ctx.SpecID != specID {
		t.Errorf("SpecID = %q, want %q", ctx.SpecID, specID)
	}
}

// TestResolveContextFromDir_MainWorktree_DiscoverActiveSpecsFailure_SetsDerivationFailed
// pins the WorktreeMain failure site: a genuine AllEpics/ActiveEpics
// failure (with .beads present) sets DerivationFailed. Unlike
// WorktreeBead/WorktreeSpec, WorktreeMain defines no fallback Phase value
// for this arm (pre-bead-5 behavior: Phase stayed "" on any main-worktree
// error) — callers distinguish this honestly-empty Phase from a real
// idle via DerivationFailed, never by treating "" as idle.
func TestResolveContextFromDir_MainWorktree_DiscoverActiveSpecsFailure_SetsDerivationFailed(t *testing.T) {
	root := t.TempDir()
	requireBeadsDir(t, root)

	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, errors.New("bd: dolt daemon unreachable")
	})
	t.Cleanup(restoreList)
	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	ctx, err := ResolveContextFromDir(root, root)
	if err != nil {
		t.Fatalf("ResolveContextFromDir returned an error (it never should): %v", err)
	}
	if !ctx.DerivationFailed {
		t.Fatal("expected DerivationFailed=true on a genuine DiscoverActiveSpecsWithCache failure")
	}
	if ctx.DerivationError == "" {
		t.Error("expected a non-empty DerivationError")
	}
	if ctx.Phase != "" {
		t.Errorf("Phase = %q, want empty — WorktreeMain defines no fallback value on this arm (pre-bead-5 behavior, unchanged)", ctx.Phase)
	}
}
