package lifecycle

import (
	"errors"
	"testing"
)

// stubFinalizeOrphanSeams installs the finalize-orphan predicate's
// injectable seams for a single test and restores them on cleanup.
func stubFinalizeOrphanSeams(t *testing.T,
	branches []string, branchesErr error,
	commitCount int, commitCountErr error,
	diffStat string, diffStatErr error,
	fileAtRef []byte, fileAtRefErr error,
) {
	t.Helper()
	origBranches := localBranchRefsFn
	origCommitCount := finalizeOrphanCommitCountFn
	origDiffStat := finalizeOrphanDiffStatFn
	origFileAtRef := fileAtRefFn
	t.Cleanup(func() {
		localBranchRefsFn = origBranches
		finalizeOrphanCommitCountFn = origCommitCount
		finalizeOrphanDiffStatFn = origDiffStat
		fileAtRefFn = origFileAtRef
	})

	localBranchRefsFn = func(workdir string) ([]string, error) { return branches, branchesErr }
	finalizeOrphanCommitCountFn = func(workdir, base, head string) (int, error) { return commitCount, commitCountErr }
	finalizeOrphanDiffStatFn = func(workdir, base, head string) (string, error) { return diffStat, diffStatErr }
	fileAtRefFn = func(workdir, ref, path string) ([]byte, error) { return fileAtRef, fileAtRefErr }
}

// (a) an outstanding chore/finalize-<specID> branch is flagged, with stats
// computed against origin/main (never local main — the seams below prove
// the CALL args, not just the values).
func TestFindOutstandingFinalizeBranches_Flagged(t *testing.T) {
	var gotCountBase, gotDiffBase string
	stubFinalizeOrphanSeams(t,
		[]string{"main", "spec/010-test", "chore/finalize-010-test"}, nil,
		3, nil,
		"2 files changed", nil,
		nil, nil,
	)
	// Wrap the count/diff seams to capture the base arg actually passed.
	origCount := finalizeOrphanCommitCountFn
	finalizeOrphanCommitCountFn = func(workdir, base, head string) (int, error) {
		gotCountBase = base
		return origCount(workdir, base, head)
	}
	origDiff := finalizeOrphanDiffStatFn
	finalizeOrphanDiffStatFn = func(workdir, base, head string) (string, error) {
		gotDiffBase = base
		return origDiff(workdir, base, head)
	}

	orphans, err := FindOutstandingFinalizeBranches(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %+v", len(orphans), orphans)
	}
	o := orphans[0]
	if o.Kind != "finalize_branch" {
		t.Errorf("Kind = %q, want finalize_branch", o.Kind)
	}
	if o.SpecID != "010-test" {
		t.Errorf("SpecID = %q, want 010-test", o.SpecID)
	}
	if o.Branch != "chore/finalize-010-test" {
		t.Errorf("Branch = %q, want chore/finalize-010-test", o.Branch)
	}
	if o.CommitCount != 3 {
		t.Errorf("CommitCount = %d, want 3", o.CommitCount)
	}
	if o.DiffStat != "2 files changed" {
		t.Errorf("DiffStat = %q, want %q", o.DiffStat, "2 files changed")
	}
	if gotCountBase != "origin/main" {
		t.Errorf("CommitCount base = %q, want origin/main (never local main)", gotCountBase)
	}
	if gotDiffBase != "origin/main" {
		t.Errorf("DiffStat base = %q, want origin/main (never local main)", gotDiffBase)
	}
	if o.RecoveryCommand() == "" {
		t.Error("RecoveryCommand must not be empty")
	}
}

// (b) no chore/finalize-* branch present → no findings.
func TestFindOutstandingFinalizeBranches_Healthy(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		[]string{"main", "spec/010-test", "bead/mindspec-x.1"}, nil,
		0, nil, "", nil, nil, nil,
	)
	orphans, err := FindOutstandingFinalizeBranches(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("expected no orphans, got %+v", orphans)
	}
}

// (c) a local-branch enumeration failure propagates as an error.
func TestFindOutstandingFinalizeBranches_PropagatesListError(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, errors.New("simulated git failure"),
		0, nil, "", nil, nil, nil,
	)
	if _, err := FindOutstandingFinalizeBranches("."); err == nil {
		t.Fatal("expected a propagated error on branch-listing failure, got nil")
	}
}

// (d) live-closed epic + main's committed export still shows it open →
// flagged, naming the divergence.
func TestStaleTrackerOnMain_Flagged(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-1","status":"open"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected a finding, got nil")
	}
	if o.Kind != "stale_tracker" {
		t.Errorf("Kind = %q, want stale_tracker", o.Kind)
	}
	if o.SpecID != "010-test" {
		t.Errorf("SpecID = %q, want 010-test", o.SpecID)
	}
	if o.RecoveryCommand() != "mindspec impl approve 010-test" {
		t.Errorf("RecoveryCommand = %q, want %q", o.RecoveryCommand(), "mindspec impl approve 010-test")
	}
}

// (e) agreement (main's export already shows closed) → no finding.
func TestStaleTrackerOnMain_HealthyAgreement(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-1","status":"closed"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != nil {
		t.Errorf("expected no finding on agreement, got %+v", o)
	}
}

// (f) live NOT closed → never a finding, regardless of main's content
// (this predicate only ever fires on the "reverted close" signature).
func TestStaleTrackerOnMain_LiveNotClosed(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-1","status":"open"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != nil {
		t.Errorf("expected no finding when live epic is not closed, got %+v", o)
	}
}

// (g) epic absent from main's committed export → no finding (not this
// predicate's concern; e.g. a brand-new epic never yet exported to main).
func TestStaleTrackerOnMain_EpicAbsentFromMain(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		[]byte(`{"id":"epic-other","status":"open"}`+"\n"), nil,
	)
	o, err := StaleTrackerOnMain(".", "010-test", "epic-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != nil {
		t.Errorf("expected no finding when epic is absent from main's export, got %+v", o)
	}
}

// (h) a genuine git-read failure propagates (distinguished from "no
// finding").
func TestStaleTrackerOnMain_PropagatesReadError(t *testing.T) {
	stubFinalizeOrphanSeams(t,
		nil, nil, 0, nil, "", nil,
		nil, errors.New("simulated git show failure"),
	)
	if _, err := StaleTrackerOnMain(".", "010-test", "epic-1", true); err == nil {
		t.Fatal("expected a propagated error on git-read failure, got nil")
	}
}
