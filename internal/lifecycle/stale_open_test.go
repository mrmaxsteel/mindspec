package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// stubStaleOpenSeams installs FindStaleOpenBeads' injectable seams for a
// single test and restores them on cleanup.
func stubStaleOpenSeams(t *testing.T, epicID string, epicErr error, items []bead.BeadInfo, itemsErr error) {
	t.Helper()
	origEpic := findEpicBySpecIDFn
	origList := listOpenBeadsFn
	t.Cleanup(func() {
		findEpicBySpecIDFn = origEpic
		listOpenBeadsFn = origList
	})
	findEpicBySpecIDFn = func(specID string) (string, error) { return epicID, epicErr }
	listOpenBeadsFn = func(gotEpicID string) ([]bead.BeadInfo, error) {
		if gotEpicID != epicID {
			t.Fatalf("listOpenBeadsFn called with epicID=%q, want %q", gotEpicID, epicID)
		}
		return items, itemsErr
	}
}

// TestFindStaleOpenBeads_Flagged_BranchSurvives pins the AC-10 positive
// case: a bead merged --no-ff into the spec branch, still OPEN in the
// tracker, with its bead/<id> branch surviving (benign merged-but-
// undeleted) is flagged.
func TestFindStaleOpenBeads_Flagged_BranchSurvives(t *testing.T) {
	// R4: RecoveryCommand()/Message() now idrender.Bead the BeadID field
	// (spec 120 Bead 5 fix-up) — a valid, idvalidate.BeadID-conformant id
	// is required here so the byte-identical render path is exercised
	// rather than the forced-quote path a bare placeholder like "one"
	// would trigger.
	const beadID = "mindspec-9if1"
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, beadID, "spec/119-test")
	stubStaleOpenSeams(t, "epic-1", nil, []bead.BeadInfo{{ID: beadID, Status: "open"}}, nil)

	found, err := FindStaleOpenBeads("119-test", dir)
	if err != nil {
		t.Fatalf("FindStaleOpenBeads: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 stale-open finding, got %d: %+v", len(found), found)
	}
	if found[0].BeadID != beadID {
		t.Errorf("BeadID = %q, want %s", found[0].BeadID, beadID)
	}
	if found[0].LandedSHA == "" {
		t.Error("expected a populated LandedSHA")
	}
	if want := "mindspec complete " + beadID; found[0].RecoveryCommand() != want {
		t.Errorf("RecoveryCommand = %q, want %q", found[0].RecoveryCommand(), want)
	}
	if found[0].Message() == "" {
		t.Error("expected a non-empty Message")
	}
}

// TestFindStaleOpenBeads_Flagged_BranchDeleted: the merge landed and the
// bead branch was subsequently deleted (mindspec complete's best-effort
// cleanup) — still flagged from the spec branch's own history alone.
func TestFindStaleOpenBeads_Flagged_BranchDeleted(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	run("branch", "-D", "bead/bead-one")
	stubStaleOpenSeams(t, "epic-1", nil, []bead.BeadInfo{{ID: "bead-one", Status: "in_progress"}}, nil)

	found, err := FindStaleOpenBeads("119-test", dir)
	if err != nil {
		t.Fatalf("FindStaleOpenBeads: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 stale-open finding, got %d: %+v", len(found), found)
	}
}

// TestFindStaleOpenBeads_FreshClaimNegative is the AC-10 load-bearing
// negative: a freshly claimed bead branch with ZERO own commits must NOT
// be flagged, even though the spec branch has since advanced past the fork
// point via OTHER beads' --no-ff merges present in its history (the round-1
// false-flag scenario this predicate must never reproduce).
func TestFindStaleOpenBeads_FreshClaimNegative(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	// Fresh bead branch claimed FIRST, off the spec branch's CURRENT tip:
	// zero own commits from the moment it's created.
	run("checkout", "-b", "bead/fresh")
	run("checkout", "spec/119-test")
	// THEN the spec branch advances PAST that fork point via an unrelated
	// bead's --no-ff merge — the genuine fork-before-spec-advance topology
	// this negative claims to guard (the round-1 false-flag scenario).
	mergeBead(t, run, dir, "other", "spec/119-test")

	stubStaleOpenSeams(t, "epic-1", nil, []bead.BeadInfo{
		{ID: "other", Status: "closed"}, // not queried (closed, wouldn't be in open/in_progress list anyway)
		{ID: "fresh", Status: "open"},
	}, nil)

	found, err := FindStaleOpenBeads("119-test", dir)
	if err != nil {
		t.Fatalf("FindStaleOpenBeads: %v", err)
	}
	for _, f := range found {
		if f.BeadID == "fresh" {
			t.Fatalf("fresh zero-own-commit bead must never be flagged stale-open, got %+v", found)
		}
	}
}

// TestFindStaleOpenBeads_HealthyAgreement: a bead genuinely still
// in_progress (its branch carries new, unlanded commits on top of an
// earlier landed merge) must not be flagged.
func TestFindStaleOpenBeads_HealthyAgreement(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "bead-one", "spec/119-test")
	run("checkout", "bead/bead-one")
	writeAndCommit(t, run, dir, "more.txt", "more work not yet merged")
	run("checkout", "spec/119-test")

	stubStaleOpenSeams(t, "epic-1", nil, []bead.BeadInfo{{ID: "bead-one", Status: "in_progress"}}, nil)

	found, err := FindStaleOpenBeads("119-test", dir)
	if err != nil {
		t.Fatalf("FindStaleOpenBeads: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected no findings for a genuinely in-progress bead, got %+v", found)
	}
}

// TestFindStaleOpenBeads_NoEpic: an absent epic yields (nil, nil), not an
// error — mirroring FindOrphanedClosedBeads/ClosedEpicBeadIDs.
func TestFindStaleOpenBeads_NoEpic(t *testing.T) {
	stubStaleOpenSeams(t, "", nil, nil, nil)

	found, err := FindStaleOpenBeads("119-test", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil findings for an absent epic, got %+v", found)
	}
}

// TestFindStaleOpenBeads_EpicLookupError propagates the epic-lookup
// failure.
func TestFindStaleOpenBeads_EpicLookupError(t *testing.T) {
	stubStaleOpenSeams(t, "", errors.New("simulated bd failure"), nil, nil)

	if _, err := FindStaleOpenBeads("119-test", "."); err == nil {
		t.Fatal("expected a propagated error on epic-lookup failure")
	}
}

// TestFindStaleOpenBeads_ListError propagates the bd-list failure.
func TestFindStaleOpenBeads_ListError(t *testing.T) {
	stubStaleOpenSeams(t, "epic-1", nil, nil, errors.New("simulated bd list failure"))

	if _, err := FindStaleOpenBeads("119-test", "."); err == nil {
		t.Fatal("expected a propagated error on bd-list failure")
	}
}

// writeAndCommit is a small test helper writing a file and committing it
// on whatever branch is currently checked out.
func writeAndCommit(t *testing.T, run func(args ...string), dir, name, msg string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(msg+"\n"), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	run("add", ".")
	run("commit", "-m", msg)
}
