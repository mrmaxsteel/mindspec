package doctor

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// stubFindStaleOpen swaps the shared predicate for the duration of a test.
func stubFindStaleOpen(t *testing.T, fn func(specID, workdir string) ([]lifecycle.StaleOpenBead, error)) {
	t.Helper()
	orig := findStaleOpenBeadsFn
	t.Cleanup(func() { findStaleOpenBeadsFn = orig })
	findStaleOpenBeadsFn = fn
}

// A stale-open bead is reported as Error with the recovery line.
func TestCheckStaleOpenBeads_ReportsError(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFindStaleOpen(t, func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) {
		if specID != "119-test" {
			return nil, nil
		}
		return []lifecycle.StaleOpenBead{{BeadID: "one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}}, nil
	})

	r := &Report{}
	checkStaleOpenBeads(r, root)

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

// No stale-open findings → no check (read-only, no false-positive) —
// covers both the healthy-agreement and fresh-claim negative cases, whose
// underlying predicate behavior is pinned in internal/lifecycle.
func TestCheckStaleOpenBeads_Clean(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFindStaleOpen(t, func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) { return nil, nil })

	r := &Report{}
	checkStaleOpenBeads(r, root)
	for _, c := range r.Checks {
		if strings.Contains(c.Name, "stale-open bead") {
			t.Errorf("clean repo must report no stale-open check; got %+v", c)
		}
	}
}

// A predicate error must not abort the scan or panic; it is treated as
// best-effort (mirroring checkOrphanedBeads' fail-open contract).
func TestCheckStaleOpenBeads_PredicateErrorIsBestEffort(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFindStaleOpen(t, func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) {
		return nil, errors.New("simulated predicate failure")
	})

	r := &Report{}
	checkStaleOpenBeads(r, root)
	if len(r.Checks) != 0 {
		t.Errorf("a predicate error must yield zero findings, not a panic or spurious check; got %+v", r.Checks)
	}
}

// The FixFunc re-invokes `mindspec complete <id>` for the stale-open bead;
// --fix flips the check to Fixed.
func TestCheckStaleOpenBeads_FixInvokesComplete(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFindStaleOpen(t, func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) {
		return []lifecycle.StaleOpenBead{{BeadID: "two", SpecBranch: "spec/119-test", LandedSHA: "cafef00d"}}, nil
	})

	var completed []string
	origRun := runMindspecCompleteFn
	t.Cleanup(func() { runMindspecCompleteFn = origRun })
	runMindspecCompleteFn = func(r, beadID string) error {
		completed = append(completed, beadID)
		return nil
	}

	r := &Report{}
	checkStaleOpenBeads(r, root)
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

// No specs dir → no-op, no panic.
func TestCheckStaleOpenBeads_NoSpecsDir(t *testing.T) {
	root := t.TempDir()
	r := &Report{}
	checkStaleOpenBeads(r, root)
	if len(r.Checks) != 0 {
		t.Errorf("missing specs dir must be a no-op; got %+v", r.Checks)
	}
}

// TestCheckStaleOpenBeads_InvokesExportedPredicate is the AC-12 anti-drift
// pin's doctor half: checkStaleOpenBeads must invoke the exported
// lifecycle.FindStaleOpenBeads symbol directly, never a private
// reimplementation, so it cannot drift from the generated `mindspec
// instruct` guidance (which is pinned to invoke the SAME symbol in
// internal/instruct's own anti-drift test).
func TestCheckStaleOpenBeads_InvokesExportedPredicate(t *testing.T) {
	if reflect.ValueOf(findStaleOpenBeadsFn).Pointer() != reflect.ValueOf(lifecycle.FindStaleOpenBeads).Pointer() {
		t.Fatal("findStaleOpenBeadsFn must be lifecycle.FindStaleOpenBeads (AC-12 anti-drift: doctor and instruct must invoke the identical exported predicate)")
	}
}

// TestRunWithOptions_WiresStaleOpenAndFinalizeOrphanChecks pins that
// RunWithOptions actually calls checkStaleOpenBeads and checkFinalizeOrphans
// (not just that the check functions work in isolation) — genuinely
// RED-on-revert if either registration line in RunWithOptions is removed.
func TestRunWithOptions_WiresStaleOpenAndFinalizeOrphanChecks(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFindStaleOpen(t, func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) {
		return []lifecycle.StaleOpenBead{{BeadID: "wired-one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}}, nil
	})
	t.Cleanup(SetFindEpicForFinalizeCheckForTest(func(specID string) (string, error) { return "", nil }))
	origBranches := findOutstandingFinalizeBranchesFn
	t.Cleanup(func() { findOutstandingFinalizeBranchesFn = origBranches })
	findOutstandingFinalizeBranchesFn = func(workdir string) ([]lifecycle.FinalizeOrphan, error) {
		return []lifecycle.FinalizeOrphan{{Kind: "finalize_branch", SpecID: "119-test", Message: "wired finalize orphan"}}, nil
	}

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
	if !sawStaleOpen {
		t.Errorf("RunWithOptions must invoke checkStaleOpenBeads; checks = %+v", report.Checks)
	}
	if !sawFinalizeOrphan {
		t.Errorf("RunWithOptions must invoke checkFinalizeOrphans; checks = %+v", report.Checks)
	}
}
