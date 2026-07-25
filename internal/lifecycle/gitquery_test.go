package lifecycle

// Spec 127 bead 1: the ADR-0030 boundary wrapper pointer-pin. gitquery.go
// declares EvaluateWorkDestruction as a package-level `var` — not a `func`
// — precisely so this test can assert wrapper ≡ implementation by pointer
// equality, joining the spec-121 AC-17 anti-drift consumer set (see
// internal/executor/neteffect_probe_test.go's identical pin for
// netEffectLandedFn ≡ gitutil.NetEffectLanded).

import (
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

func TestEvaluateWorkDestruction_WrapperPinnedToImplementation(t *testing.T) {
	got := reflect.ValueOf(EvaluateWorkDestruction).Pointer()
	want := reflect.ValueOf(gitutil.EvaluateWorkDestruction).Pointer()
	if got != want {
		t.Fatalf("lifecycle.EvaluateWorkDestruction must be pointer-identical to gitutil.EvaluateWorkDestruction (no-second-rewirable-seam); got %v, want %v", got, want)
	}
}

// TestEvaluateWorkDestruction_WrapperPinFailsIfRepointed is the fixture's
// own falsifiability check (never taken on trust): re-pointing the wrapper
// var to ANY other value with the same signature — even one that behaves
// identically — must make the pin above fail. This proves the pointer-
// equality assertion actually discriminates, rather than passing
// vacuously (e.g. because both sides always compare equal for unrelated
// reasons).
func TestEvaluateWorkDestruction_WrapperPinFailsIfRepointed(t *testing.T) {
	orig := EvaluateWorkDestruction
	t.Cleanup(func() { EvaluateWorkDestruction = orig })

	// A distinct function value with an IDENTICAL signature and behavior
	// (it simply calls through) — the pin must still distinguish it from
	// the real symbol by POINTER identity, not by behavior.
	EvaluateWorkDestruction = func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
		return gitutil.EvaluateWorkDestruction(workdir, branch, target)
	}

	got := reflect.ValueOf(EvaluateWorkDestruction).Pointer()
	want := reflect.ValueOf(gitutil.EvaluateWorkDestruction).Pointer()
	if got == want {
		t.Fatal("re-pointing the wrapper to a distinct (even behaviorally-identical) function must break the pointer-equality pin, but it still compared equal")
	}
}
