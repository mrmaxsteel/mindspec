package lifecycle

import (
	"errors"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// stubPredicate installs the four injectable seams for a single test and
// restores them on cleanup. closedIDs are the closed sibling bead ids returned
// for the epic; existingBranches and ancestorBranches drive BranchExists and
// IsAncestor by branch name (bead/<id>).
func stubPredicate(t *testing.T, epicID string, closedIDs []string, existingBranches, ancestorBranches map[string]bool) {
	t.Helper()
	origEpic := findEpicBySpecIDFn
	origList := listClosedBeadsFn
	origExists := branchExistsFn
	origAnc := isAncestorFn
	t.Cleanup(func() {
		findEpicBySpecIDFn = origEpic
		listClosedBeadsFn = origList
		branchExistsFn = origExists
		isAncestorFn = origAnc
	})

	findEpicBySpecIDFn = func(specID string) (string, error) { return epicID, nil }
	listClosedBeadsFn = func(id string) ([]bead.BeadInfo, error) {
		var items []bead.BeadInfo
		for _, cid := range closedIDs {
			items = append(items, bead.BeadInfo{ID: cid, Status: "closed"})
		}
		return items, nil
	}
	branchExistsFn = func(name string) bool { return existingBranches[name] }
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
		return ancestorBranches[ancestor], nil
	}
}

// (a) orphaned: closed + branch exists + NOT ancestor → flagged.
func TestFindOrphanedClosedBeads_Orphaned(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{"bead/bead-1": true},
		map[string]bool{}, // not an ancestor
	)

	orphans := FindOrphanedClosedBeads("008-test", ".", "")
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %+v", len(orphans), orphans)
	}
	o := orphans[0]
	if o.BeadID != "bead-1" {
		t.Errorf("BeadID = %q, want bead-1", o.BeadID)
	}
	if o.BeadBranch != "bead/bead-1" {
		t.Errorf("BeadBranch = %q, want bead/bead-1", o.BeadBranch)
	}
	if o.SpecBranch != "spec/008-test" {
		t.Errorf("SpecBranch = %q, want spec/008-test", o.SpecBranch)
	}
	if o.RecoveryCommand() != "mindspec complete bead-1" {
		t.Errorf("RecoveryCommand = %q, want mindspec complete bead-1", o.RecoveryCommand())
	}
}

// (b) benign: closed + branch exists + IS ancestor → NOT flagged
// (merged-but-branch-undeleted).
func TestFindOrphanedClosedBeads_BenignMergedUndeleted(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{"bead/bead-1": true},
		map[string]bool{"bead/bead-1": true}, // IS an ancestor of the spec branch
	)

	if orphans := FindOrphanedClosedBeads("008-test", ".", ""); len(orphans) != 0 {
		t.Fatalf("merged-but-undeleted branch must NOT be flagged; got %+v", orphans)
	}
}

// (c) cleanly completed: branch gone → NOT flagged.
func TestFindOrphanedClosedBeads_CleanlyCompleted(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{}, // branch deleted by complete
		map[string]bool{},
	)

	if orphans := FindOrphanedClosedBeads("008-test", ".", ""); len(orphans) != 0 {
		t.Fatalf("cleanly-completed bead (no branch) must NOT be flagged; got %+v", orphans)
	}
}

// excludeBeadID skips the very bead being completed (chicken-and-egg) while
// still flagging an orphaned sibling.
func TestFindOrphanedClosedBeads_ExcludesSelf(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1", "bead-2"},
		map[string]bool{"bead/bead-1": true, "bead/bead-2": true},
		map[string]bool{}, // neither is an ancestor
	)

	orphans := FindOrphanedClosedBeads("008-test", ".", "bead-1")
	if len(orphans) != 1 {
		t.Fatalf("expected only the non-excluded sibling, got %+v", orphans)
	}
	if orphans[0].BeadID != "bead-2" {
		t.Errorf("flagged %q, want bead-2 (bead-1 excluded as self)", orphans[0].BeadID)
	}
}

// An ancestry-check error means "cannot confirm orphaned" → skip (read-only,
// never false-block on a transient git error).
func TestFindOrphanedClosedBeads_AncestryErrorSkips(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{"bead/bead-1": true},
		map[string]bool{},
	)
	origAnc := isAncestorFn
	t.Cleanup(func() { isAncestorFn = origAnc })
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
		return false, errors.New("transient git failure")
	}

	if orphans := FindOrphanedClosedBeads("008-test", ".", ""); len(orphans) != 0 {
		t.Fatalf("an ancestry-check error must skip (never false-block); got %+v", orphans)
	}
}

// No epic for the spec → no orphans (best-effort, read-only).
func TestFindOrphanedClosedBeads_NoEpic(t *testing.T) {
	origEpic := findEpicBySpecIDFn
	t.Cleanup(func() { findEpicBySpecIDFn = origEpic })
	findEpicBySpecIDFn = func(specID string) (string, error) { return "", nil }

	if orphans := FindOrphanedClosedBeads("008-test", ".", ""); len(orphans) != 0 {
		t.Fatalf("no epic must yield no orphans; got %+v", orphans)
	}
}
