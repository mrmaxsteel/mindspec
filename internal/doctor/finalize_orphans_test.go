package doctor

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

func stubFinalizeOrphanDoctorSeams(t *testing.T,
	branches []lifecycle.FinalizeOrphan, branchesErr error,
	findEpic func(specID string) (string, error),
	epicStatus func(epicID string) (string, error),
	staleTracker func(workdir, specID, epicID string, liveClosed bool) (*lifecycle.FinalizeOrphan, error),
) {
	t.Helper()
	origBranches := findOutstandingFinalizeBranchesFn
	origFindEpic := findEpicForFinalizeCheckFn
	origEpicStatus := findEpicStatusFn
	origStaleTracker := staleTrackerOnMainFn
	t.Cleanup(func() {
		findOutstandingFinalizeBranchesFn = origBranches
		findEpicForFinalizeCheckFn = origFindEpic
		findEpicStatusFn = origEpicStatus
		staleTrackerOnMainFn = origStaleTracker
	})
	findOutstandingFinalizeBranchesFn = func(workdir string) ([]lifecycle.FinalizeOrphan, error) {
		return branches, branchesErr
	}
	if findEpic != nil {
		findEpicForFinalizeCheckFn = findEpic
	}
	if epicStatus != nil {
		findEpicStatusFn = epicStatus
	}
	if staleTracker != nil {
		staleTrackerOnMainFn = staleTracker
	}
}

// An outstanding chore/finalize-* branch is reported as Error with the
// message + recovery command combined via FinalizeOrphan.FullMessage().
func TestCheckFinalizeOrphans_FinalizeBranchReported(t *testing.T) {
	root := t.TempDir()

	o := lifecycle.FinalizeOrphan{
		Kind:    "finalize_branch",
		SpecID:  "119-test",
		Branch:  "chore/finalize-119-test",
		Message: "finalize branch chore/finalize-119-test is unmerged",
	}
	stubFinalizeOrphanDoctorSeams(t,
		[]lifecycle.FinalizeOrphan{o}, nil,
		func(specID string) (string, error) { return "", nil }, // no specs dir walked in this fixture
		nil, nil,
	)

	r := &Report{}
	checkFinalizeOrphans(r, root)

	var found *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "finalize_branch") {
			found = &r.Checks[i]
		}
	}
	if found == nil {
		t.Fatalf("expected a finalize_branch check, got %+v", r.Checks)
	}
	if found.Status != Error {
		t.Errorf("status = %v, want Error", found.Status)
	}
	if !strings.Contains(found.Message, o.Message) || !strings.Contains(found.Message, o.RecoveryCommand()) {
		t.Errorf("message must be FullMessage() (message + recovery command); got %q", found.Message)
	}
}

// A stale-tracker finding is surfaced per spec, driven by the epic's live
// bd status.
func TestCheckFinalizeOrphans_StaleTrackerReported(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFinalizeOrphanDoctorSeams(t,
		nil, nil,
		func(specID string) (string, error) {
			if specID == "119-test" {
				return "epic-1", nil
			}
			return "", nil
		},
		func(epicID string) (string, error) { return "closed", nil },
		func(workdir, specID, epicID string, liveClosed bool) (*lifecycle.FinalizeOrphan, error) {
			if !liveClosed {
				t.Fatalf("expected liveClosed=true from a closed epic status")
			}
			return &lifecycle.FinalizeOrphan{
				Kind:    "stale_tracker",
				SpecID:  specID,
				Message: "epic " + epicID + " closed but main disagrees",
			}, nil
		},
	)

	r := &Report{}
	checkFinalizeOrphans(r, root)

	var found *Check
	for i := range r.Checks {
		if strings.Contains(r.Checks[i].Name, "stale_tracker") {
			found = &r.Checks[i]
		}
	}
	if found == nil {
		t.Fatalf("expected a stale_tracker check, got %+v", r.Checks)
	}
	if found.Status != Error {
		t.Errorf("status = %v, want Error", found.Status)
	}
}

// Healthy: no branches, no stale trackers → no findings.
func TestCheckFinalizeOrphans_Clean(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFinalizeOrphanDoctorSeams(t,
		nil, nil,
		func(specID string) (string, error) { return "epic-1", nil },
		func(epicID string) (string, error) { return "open", nil },
		func(workdir, specID, epicID string, liveClosed bool) (*lifecycle.FinalizeOrphan, error) {
			return nil, nil
		},
	)

	r := &Report{}
	checkFinalizeOrphans(r, root)
	if len(r.Checks) != 0 {
		t.Errorf("expected no findings on a healthy repo, got %+v", r.Checks)
	}
}

// Errors from any seam are best-effort — no panic, no spurious findings.
func TestCheckFinalizeOrphans_ErrorsAreBestEffort(t *testing.T) {
	root := t.TempDir()
	makeSpecDir(t, root, "119-test")

	stubFinalizeOrphanDoctorSeams(t,
		nil, errors.New("simulated branch-list failure"),
		func(specID string) (string, error) { return "", errors.New("simulated epic lookup failure") },
		func(epicID string) (string, error) { return "", errors.New("simulated status failure") },
		func(workdir, specID, epicID string, liveClosed bool) (*lifecycle.FinalizeOrphan, error) {
			return nil, errors.New("simulated stale-tracker failure")
		},
	)

	r := &Report{}
	checkFinalizeOrphans(r, root)
	if len(r.Checks) != 0 {
		t.Errorf("errors must be best-effort with zero findings, got %+v", r.Checks)
	}
}

// No specs dir → the finalize-branch scan still runs; the stale-tracker
// per-spec walk is a no-op.
func TestCheckFinalizeOrphans_NoSpecsDir(t *testing.T) {
	root := t.TempDir()
	o := lifecycle.FinalizeOrphan{Kind: "finalize_branch", SpecID: "x", Message: "m"}
	stubFinalizeOrphanDoctorSeams(t, []lifecycle.FinalizeOrphan{o}, nil, nil, nil, nil)

	r := &Report{}
	checkFinalizeOrphans(r, root)
	if len(r.Checks) != 1 {
		t.Errorf("expected exactly the finalize-branch finding, got %+v", r.Checks)
	}
}

// TestCheckFinalizeOrphans_InvokesExportedPredicates is the AC-12
// anti-drift pin's doctor half for the finalize-orphan checks.
func TestCheckFinalizeOrphans_InvokesExportedPredicates(t *testing.T) {
	if reflect.ValueOf(findOutstandingFinalizeBranchesFn).Pointer() != reflect.ValueOf(lifecycle.FindOutstandingFinalizeBranches).Pointer() {
		t.Fatal("findOutstandingFinalizeBranchesFn must be lifecycle.FindOutstandingFinalizeBranches (AC-12 anti-drift)")
	}
	if reflect.ValueOf(staleTrackerOnMainFn).Pointer() != reflect.ValueOf(lifecycle.StaleTrackerOnMain).Pointer() {
		t.Fatal("staleTrackerOnMainFn must be lifecycle.StaleTrackerOnMain (AC-12 anti-drift)")
	}
}
