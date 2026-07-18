package doctor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// stubScanIntegrity swaps the shared aggregate scan for one test.
func stubScanIntegrity(t *testing.T, fn func(root string, cache *phase.Cache) lifecycle.IntegrityFindings) {
	t.Helper()
	orig := scanIntegrityFindingsFn
	t.Cleanup(func() { scanIntegrityFindingsFn = orig })
	scanIntegrityFindingsFn = fn
}

// A stale-open bead is reported as Error with the recovery line and a
// FixFunc.
func TestCheckLifecycleIntegrity_StaleOpenReported(t *testing.T) {
	root := t.TempDir()

	stubScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{
			StaleOpen: []lifecycle.StaleOpenBead{{BeadID: "one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}},
		}
	})

	r := &Report{}
	checkLifecycleIntegrity(r, root)

	var found *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "stale-open bead") {
			found = &r.Checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a stale-open bead check, got %+v", r.Checks)
	}
	if found.Status != Error {
		t.Errorf("status = %v, want Error", found.Status)
	}
	if !strings.Contains(found.Message, "mindspec complete one") {
		t.Errorf("message must carry the recovery command; got %q", found.Message)
	}
	if found.FixFunc == nil {
		t.Error("a stale-open bead check must carry a FixFunc for --fix")
	}
	if !r.HasFailures() {
		t.Error("a stale-open bead must trip HasFailures (Error status)")
	}
}

// An outstanding chore/finalize-* branch is reported as Error with the
// message + recovery command combined via FinalizeOrphan.FullMessage(),
// and a stale-tracker finding via the same shape.
func TestCheckLifecycleIntegrity_FinalizeOrphansReported(t *testing.T) {
	root := t.TempDir()

	branch := lifecycle.FinalizeOrphan{
		Kind:    "finalize_branch",
		SpecID:  "119-test",
		Branch:  "chore/finalize-119-test",
		Message: "finalize branch chore/finalize-119-test is unmerged",
	}
	tracker := lifecycle.FinalizeOrphan{
		Kind:    "stale_tracker",
		SpecID:  "118-old",
		Message: "epic epic-1 closed but main disagrees",
	}
	stubScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{
			FinalizeBranches: []lifecycle.FinalizeOrphan{branch},
			StaleTrackers:    []lifecycle.FinalizeOrphan{tracker},
		}
	})

	r := &Report{}
	checkLifecycleIntegrity(r, root)

	var sawBranch, sawTracker *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "finalize_branch") {
			sawBranch = &r.Checks[i]
		}
		if strings.Contains(r.Checks[i].Name, "stale_tracker") {
			sawTracker = &r.Checks[i]
		}
	}
	if sawBranch == nil || sawTracker == nil {
		t.Fatalf("expected finalize_branch AND stale_tracker checks, got %+v", r.Checks)
	}
	if sawBranch.Status != Error || sawTracker.Status != Error {
		t.Errorf("both findings must be Error, got %v / %v", sawBranch.Status, sawTracker.Status)
	}
	if !strings.Contains(sawBranch.Message, branch.Message) || !strings.Contains(sawBranch.Message, branch.RecoveryCommand()) {
		t.Errorf("message must be FullMessage() (message + recovery command); got %q", sawBranch.Message)
	}
}

// Healthy: an empty aggregate → no checks (read-only, no false positive).
func TestCheckLifecycleIntegrity_Clean(t *testing.T) {
	root := t.TempDir()
	stubScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{}
	})

	r := &Report{}
	checkLifecycleIntegrity(r, root)
	if len(r.Checks) != 0 {
		t.Errorf("clean repo must report no lifecycle-integrity checks; got %+v", r.Checks)
	}
}

// The FixFunc re-invokes `mindspec complete <id>` for the stale-open bead;
// --fix flips the check to Fixed.
func TestCheckLifecycleIntegrity_FixInvokesComplete(t *testing.T) {
	root := t.TempDir()

	stubScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{
			StaleOpen: []lifecycle.StaleOpenBead{{BeadID: "two", SpecBranch: "spec/119-test", LandedSHA: "cafef00d"}},
		}
	})

	var completed []string
	origRun := runMindspecCompleteFn
	t.Cleanup(func() { runMindspecCompleteFn = origRun })
	runMindspecCompleteFn = func(r, beadID string) error {
		completed = append(completed, beadID)
		return nil
	}

	r := &Report{}
	checkLifecycleIntegrity(r, root)
	r.Fix()

	if len(completed) != 1 || completed[0] != "two" {
		t.Errorf("FixFunc must run `mindspec complete two`; got %v", completed)
	}
	var c *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "stale-open bead") {
			c = &r.Checks[i]
		}
	}
	if c == nil || c.Status != Fixed {
		t.Errorf("after --fix the stale-open check must be Fixed; got %+v", c)
	}
}

// TestCheckLifecycleIntegrity_InvokesSharedAggregate is the AC-12
// anti-drift pin's doctor half, updated for the final-review F1 shape:
// checkLifecycleIntegrity must invoke the exported
// lifecycle.ScanIntegrityFindings aggregate directly — the SAME symbol the
// generated `mindspec instruct` guidance is pinned to (its own anti-drift
// test) — never a private reimplementation or a per-spec-dir fan-out.
func TestCheckLifecycleIntegrity_InvokesSharedAggregate(t *testing.T) {
	if reflect.ValueOf(scanIntegrityFindingsFn).Pointer() != reflect.ValueOf(lifecycle.ScanIntegrityFindings).Pointer() {
		t.Fatal("scanIntegrityFindingsFn must be lifecycle.ScanIntegrityFindings (AC-12 anti-drift: doctor and instruct must invoke the identical exported aggregate)")
	}
}

// TestRunWithOptions_WiresLifecycleIntegrityCheck pins that RunWithOptions
// actually calls checkLifecycleIntegrity (not just that the check works in
// isolation) — genuinely RED-on-revert if the registration line in
// RunWithOptions is removed.
func TestRunWithOptions_WiresLifecycleIntegrityCheck(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{
			FinalizeBranches: []lifecycle.FinalizeOrphan{{Kind: "finalize_branch", SpecID: "119-test", Message: "wired finalize orphan"}},
			StaleOpen:        []lifecycle.StaleOpenBead{{BeadID: "wired-one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}},
		}
	})

	report := RunWithOptions(root, Options{})

	var sawStaleOpen, sawFinalizeOrphan bool
	for _, c := range report.Checks {
		if strings.Contains(c.Name, "stale-open bead: wired-one") {
			sawStaleOpen = true
		}
		if strings.Contains(c.Name, "finalize_branch") {
			sawFinalizeOrphan = true
		}
	}
	if !sawStaleOpen || !sawFinalizeOrphan {
		t.Errorf("RunWithOptions must invoke checkLifecycleIntegrity (stale-open %v, finalize %v); checks = %+v", sawStaleOpen, sawFinalizeOrphan, report.Checks)
	}
}
