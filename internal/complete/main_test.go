package complete

import (
	"os"
	"testing"
)

// TestMain defaults completeWorkDestructionPreflightFn to a permissive
// stub for every test in this package, before any test runs.
//
// Spec 127 R4(a): completeWorkDestructionPreflightFn
// (lifecycle.EvaluateWorkDestructionPreflight -> gitutil.
// EvaluateWorkDestruction) performs REAL git I/O UNCONDITIONALLY on
// every non-reconcile Run call — unlike evaluateOrphanHintFn, which only
// fires when an orphan scan actually finds one, this new §1 check always
// runs. Most of this package's Run-calling tests drive complete.Run
// against a fabricated root/beadHead/specBranch that was never a real
// git repo (they exercise other gates via MockExecutor/fake seams), and
// not every one of them calls saveAndRestore (which also defaults this
// seam — see complete_test.go), so defaulting it here too, at the
// process level, keeps every test in the package hermetic regardless of
// which helper it uses. Any test asserting AC-5's own §1 refusal or the
// --allow-net-deletion override overrides this default per-test and
// restores it via t.Cleanup — same convention as internal/approve's own
// TestMain (main_test.go there) for the identical seam shape.
func TestMain(m *testing.M) {
	completeWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		return nil
	}
	os.Exit(m.Run())
}
