package lifecycle

// Spec 127 bead 1 (fix round, G1-2): the ADR-0030 boundary wrapper pin.
// gitquery.go now declares the seam as an UNEXPORTED package-level `var`
// (evaluateWorkDestructionFn) and exports EvaluateWorkDestruction as an
// immutable `func` that calls through to it. This is a shape change from
// the bead's original delivery, which exported the `var` itself: a
// pointer-equality snapshot only proves the two sides matched at the
// INSTANT the test ran, and exporting a mutable func-valued var let any
// package under the module repoint it — proved both ways (an
// external-package probe successfully reassigned
// lifecycle.EvaluateWorkDestruction, and running under `-race` reported a
// genuine data race between that write and this package's own read).
// Keeping the seam unexported closes the rewiring hole; the
// pointer-equality test below still catches in-package DRIFT (see
// internal/executor/neteffect_probe_test.go's identical pin for
// netEffectLandedFn ≡ gitutil.NetEffectLanded, which predates this one and
// was never exported either).

import (
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

func TestEvaluateWorkDestruction_WrapperPinnedToImplementation(t *testing.T) {
	got := reflect.ValueOf(evaluateWorkDestructionFn).Pointer()
	want := reflect.ValueOf(gitutil.EvaluateWorkDestruction).Pointer()
	if got != want {
		t.Fatalf("lifecycle.evaluateWorkDestructionFn must be pointer-identical to gitutil.EvaluateWorkDestruction (no-second-rewirable-seam); got %v, want %v", got, want)
	}
}

// TestEvaluateWorkDestruction_WrapperPinFailsIfRepointed is the fixture's
// own falsifiability check (never taken on trust): re-pointing the
// in-package seam to ANY other value with the same signature — even one
// that behaves identically — must make the pin above fail. This proves
// the pointer-equality assertion actually discriminates, rather than
// passing vacuously (e.g. because both sides always compare equal for
// unrelated reasons). Re-pointing is only possible from WITHIN this
// package now (evaluateWorkDestructionFn is unexported) — that in-package
// reach is exactly as wide as the unexported functions this seam
// discipline already tolerates elsewhere in the tree, not a new hazard.
func TestEvaluateWorkDestruction_WrapperPinFailsIfRepointed(t *testing.T) {
	orig := evaluateWorkDestructionFn
	t.Cleanup(func() { evaluateWorkDestructionFn = orig })

	// A distinct function value with an IDENTICAL signature and behavior
	// (it simply calls through) — the pin must still distinguish it from
	// the real symbol by POINTER identity, not by behavior.
	evaluateWorkDestructionFn = func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
		return gitutil.EvaluateWorkDestruction(workdir, branch, target)
	}

	got := reflect.ValueOf(evaluateWorkDestructionFn).Pointer()
	want := reflect.ValueOf(gitutil.EvaluateWorkDestruction).Pointer()
	if got == want {
		t.Fatal("re-pointing the seam to a distinct (even behaviorally-identical) function must break the pointer-equality pin, but it still compared equal")
	}
}

// TestEvaluateWorkDestruction_ExportedFuncCallsThroughTheSeam proves the
// EXPORTED func is not a second, independent implementation that happens
// to look right — it genuinely delegates to evaluateWorkDestructionFn.
// Re-pointing the seam to a stub that returns a distinguishable sentinel
// value and calling the EXPORTED EvaluateWorkDestruction must observe
// that sentinel.
func TestEvaluateWorkDestruction_ExportedFuncCallsThroughTheSeam(t *testing.T) {
	orig := evaluateWorkDestructionFn
	t.Cleanup(func() { evaluateWorkDestructionFn = orig })

	sentinelEvidence := gitutil.WorkDestructionEvidence{FailedProbe: "sentinel-from-stubbed-seam"}
	evaluateWorkDestructionFn = func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
		return guard.DestructionEvidenceError, sentinelEvidence, nil
	}

	outcome, evidence, err := EvaluateWorkDestruction("workdir", "branch", "target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError (from the stubbed seam)", outcome)
	}
	if evidence.FailedProbe != sentinelEvidence.FailedProbe {
		t.Errorf("evidence.FailedProbe = %q, want %q — the exported func must call through the seam, not bypass it", evidence.FailedProbe, sentinelEvidence.FailedProbe)
	}
}
