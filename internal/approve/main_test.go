package approve

import (
	"os"
	"testing"
)

// TestMain defaults planListJSONFn to a bd-independent stub (an empty
// child set) for every test in this package, before any test runs.
//
// queryExistingChildren (plan.go) — the preflight the fail-closed spec 119
// R1/P9 rework routes through — shells to a REAL `bd` via planListJSONFn
// unless a test opts out. Most of this package's createImplementationBeads
// tests only stub planRunBDFn (the `bd create` calls) and never touch
// planListJSONFn, because historically queryExistingChildren's failure was
// tolerated (fail-open: "can't query, proceed"). Now that it fails closed,
// an unstubbed real bd call is load-bearing — and CI has no `bd` on PATH,
// so every one of those tests errored on "exec: bd: executable file not
// found in $PATH" instead of exercising what it meant to.
//
// Defaulting the seam here (rather than patching each test) keeps the
// package bd-independent by construction: any NEW test gets the same safe
// default unless it explicitly calls SetPlanListJSONForTest to assert
// something about queryExistingChildren's own behavior (as
// plan_fault_test.go's p0a/p0b/p1/p2/p3 fixtures already do) — those calls
// override this default per-test and restore it via t.Cleanup, so they are
// unaffected by this change.
func TestMain(m *testing.M) {
	planListJSONFn = func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}
	// Spec 127 R4(a): implWorkDestructionPreflightFn performs REAL git I/O
	// (lifecycle.EvaluateWorkDestructionPreflight ->
	// gitutil.EvaluateWorkDestruction) unconditionally on every ApproveImpl
	// call — unlike implEvaluateOrphanHintFn, which only fires when an
	// orphan is actually found, this new §1 check runs for EVERY spec, so
	// defaulting it here (rather than patching every existing test) keeps
	// the package's pre-127 tests running against a fabricated root/
	// specBranch that was never a real git repo. Any test asserting AC-5
	// (the stale-recreated-spec-branch refusal) or the override escape
	// overrides this default per-test and restores it via t.Cleanup — same
	// convention as planListJSONFn above.
	implWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		return nil
	}
	// Bead-6 fix round 1 (O1-1/O3-1): implHasRemoteFn now gates whether
	// the §1 preflight above is even consulted (it applies only to the
	// no-remote DIRECT spec→main leg). Defaulting it to false here
	// preserves every pre-existing test's behavior exactly as it was
	// before this gate existed — the preflight above still runs
	// (permissively) for every test unless a test explicitly overrides
	// this seam to exercise the PR-routed skip itself.
	implHasRemoteFn = func() bool { return false }
	os.Exit(m.Run())
}
