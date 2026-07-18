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
	os.Exit(m.Run())
}
