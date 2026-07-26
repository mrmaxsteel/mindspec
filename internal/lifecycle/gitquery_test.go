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
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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

// TestFetchRemoteBranchIn_WrapperPinnedToImplementation mirrors
// TestEvaluateWorkDestruction_WrapperPinnedToImplementation above for
// spec 127 bead 3's second wrapper: fetchRemoteBranchInFn must be
// pointer-identical to gitutil.FetchRemoteBranchIn.
func TestFetchRemoteBranchIn_WrapperPinnedToImplementation(t *testing.T) {
	got := reflect.ValueOf(fetchRemoteBranchInFn).Pointer()
	want := reflect.ValueOf(gitutil.FetchRemoteBranchIn).Pointer()
	if got != want {
		t.Fatalf("lifecycle.fetchRemoteBranchInFn must be pointer-identical to gitutil.FetchRemoteBranchIn (no-second-rewirable-seam); got %v, want %v", got, want)
	}
}

// TestFetchRemoteBranchIn_WrapperPinFailsIfRepointed is the falsifiability
// check for the pin above — see
// TestEvaluateWorkDestruction_WrapperPinFailsIfRepointed's identical
// rationale.
func TestFetchRemoteBranchIn_WrapperPinFailsIfRepointed(t *testing.T) {
	orig := fetchRemoteBranchInFn
	t.Cleanup(func() { fetchRemoteBranchInFn = orig })

	fetchRemoteBranchInFn = func(workdir, remote, branch string) (string, error) {
		return gitutil.FetchRemoteBranchIn(workdir, remote, branch)
	}

	got := reflect.ValueOf(fetchRemoteBranchInFn).Pointer()
	want := reflect.ValueOf(gitutil.FetchRemoteBranchIn).Pointer()
	if got == want {
		t.Fatal("re-pointing the seam to a distinct (even behaviorally-identical) function must break the pointer-equality pin, but it still compared equal")
	}
}

// TestFetchRemoteBranchIn_ExportedFuncCallsThroughTheSeam proves the
// exported func genuinely delegates to fetchRemoteBranchInFn rather than
// being a second, independent implementation.
func TestFetchRemoteBranchIn_ExportedFuncCallsThroughTheSeam(t *testing.T) {
	orig := fetchRemoteBranchInFn
	t.Cleanup(func() { fetchRemoteBranchInFn = orig })

	sentinel := errors.New("sentinel-from-stubbed-fetch-seam")
	fetchRemoteBranchInFn = func(workdir, remote, branch string) (string, error) {
		return "", sentinel
	}

	if _, err := FetchRemoteBranchIn("workdir", "origin", "branch"); err != sentinel {
		t.Errorf("FetchRemoteBranchIn must call through the seam, not bypass it; got %v, want %v", err, sentinel)
	}
}

// TestFetchRemoteBranchIn_ReturnsFetchedSHAThroughTheSeam is S1-1's
// wrapper-level proof that the SHA FetchRemoteBranchIn returns is
// genuinely propagated (not dropped) by the exported wrapper.
func TestFetchRemoteBranchIn_ReturnsFetchedSHAThroughTheSeam(t *testing.T) {
	orig := fetchRemoteBranchInFn
	t.Cleanup(func() { fetchRemoteBranchInFn = orig })

	const sentinelSHA = "deadbeefcafef00dfacade0123456789abcdef0"
	fetchRemoteBranchInFn = func(workdir, remote, branch string) (string, error) {
		return sentinelSHA, nil
	}

	sha, err := FetchRemoteBranchIn("workdir", "origin", "branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != sentinelSHA {
		t.Errorf("FetchRemoteBranchIn sha = %q, want %q (from the stubbed seam)", sha, sentinelSHA)
	}
}

// repoRoot returns the absolute path to the mindspec repo root by walking
// up from this test's runtime working directory (which `go test` sets to
// the package directory) until go.mod is found — needed below as `go
// build`'s cmd.Dir, so its module resolution finds the real module
// rather than treating the probe as an ad-hoc, moduleless file.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		info, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil && !info.IsDir() {
			return dir
		}
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("stat %s: %v", filepath.Join(dir, "go.mod"), statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find repo root (go.mod) walking up from the test's working directory")
	return ""
}

// TestEvaluateWorkDestruction_ExternalPackageCannotReassign is G1-2's
// compile-time proof — the FOURTH of the four original BLOCKING classes
// (spec 127 bead-1 fix round 2, ruling 3: "for each of the four original
// BLOCKING classes ... re-inject the original defect and show the test
// that goes RED"). testdata/g1rewireprobe/main.go attempts
// `lifecycle.EvaluateWorkDestruction = someFunc` — the exact reassignment
// G1 proved possible against the pre-fix exported `var`. Against the
// CURRENT (immutable func) shape, that must fail to COMPILE, not merely
// disagree with a runtime pointer-equality snapshot (which only proves
// the two sides matched at the instant the test ran).
//
// Red-on-revert, performed manually and reported rather than repeated by
// this test (which would otherwise have to ship the pre-fix source
// permanently): reverting gitquery.go to ab5aca11's shape (`var
// EvaluateWorkDestruction = gitutil.EvaluateWorkDestruction`) via `go
// build -overlay` and re-running this exact probe makes `go build` exit
// 0 — confirmed empirically before this test was written.
func TestEvaluateWorkDestruction_ExternalPackageCannotReassign(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("go", "build", "./internal/lifecycle/testdata/g1rewireprobe/")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected the external-package reassignment probe to FAIL to compile (lifecycle.EvaluateWorkDestruction must be an immutable func, not an assignable var); it built successfully")
	}
	if !strings.Contains(string(out), "cannot assign to lifecycle.EvaluateWorkDestruction") {
		t.Errorf("expected a 'cannot assign to lifecycle.EvaluateWorkDestruction' compile error, got:\n%s", out)
	}
}
