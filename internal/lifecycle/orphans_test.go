package lifecycle

import (
	"errors"
	"strconv"
	"strings"
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

// --- Spec 115 Bead 1: ScanOrphanedClosedBeads, the error-preserving core ---
//
// TestScanOrphanedClosedBeads_ErrorPreserving pins that the core PROPAGATES
// each of the three cleanly-signaled infra errors (epic-lookup, bd-list,
// ancestry) while the fail-open FindOrphanedClosedBeads wrapper stays
// byte-identical for its existing consumers (complete/next/doctor) — pinned
// on a MIXED list, not merely an all-error list, per AC1's lifecycle half.
func TestScanOrphanedClosedBeads_ErrorPreserving(t *testing.T) {
	t.Run("epic-lookup error propagates", func(t *testing.T) {
		origEpic := findEpicBySpecIDFn
		t.Cleanup(func() { findEpicBySpecIDFn = origEpic })
		findEpicBySpecIDFn = func(specID string) (string, error) {
			return "", errors.New("bd show epic: transient failure")
		}

		orphans, err := ScanOrphanedClosedBeads("008-test", ".", "")
		if err == nil {
			t.Fatal("expected the epic-lookup error to propagate")
		}
		if len(orphans) != 0 {
			t.Errorf("expected no orphans alongside the epic-lookup error, got %+v", orphans)
		}
		if got := FindOrphanedClosedBeads("008-test", ".", ""); len(got) != 0 {
			t.Errorf("fail-open wrapper must still yield no orphans, got %+v", got)
		}
	})

	t.Run("bd-list error propagates", func(t *testing.T) {
		origEpic := findEpicBySpecIDFn
		origList := listClosedBeadsFn
		t.Cleanup(func() {
			findEpicBySpecIDFn = origEpic
			listClosedBeadsFn = origList
		})
		findEpicBySpecIDFn = func(specID string) (string, error) { return "epic-1", nil }
		listClosedBeadsFn = func(epicID string) ([]bead.BeadInfo, error) {
			return nil, errors.New("bd list: transient failure")
		}

		orphans, err := ScanOrphanedClosedBeads("008-test", ".", "")
		if err == nil {
			t.Fatal("expected the bd-list error to propagate")
		}
		if len(orphans) != 0 {
			t.Errorf("expected no orphans alongside the bd-list error, got %+v", orphans)
		}
		if got := FindOrphanedClosedBeads("008-test", ".", ""); len(got) != 0 {
			t.Errorf("fail-open wrapper must still yield no orphans, got %+v", got)
		}
	})

	t.Run("single-bead ancestry error propagates and wrapper stays fail-open", func(t *testing.T) {
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

		orphans, err := ScanOrphanedClosedBeads("008-test", ".", "")
		if err == nil {
			t.Fatal("expected the ancestry error to propagate")
		}
		if len(orphans) != 0 {
			t.Errorf("expected no orphans alongside a single-bead ancestry error, got %+v", orphans)
		}
		if got := FindOrphanedClosedBeads("008-test", ".", ""); len(got) != 0 {
			t.Errorf("fail-open wrapper must still yield no orphans (ancestry-error skip), got %+v", got)
		}
	})

	// MIXED-list parity (round-3 G2): bead-A's ancestry check errors, but
	// bead-B (LATER in the list) is a provable orphan. The core must not
	// abandon the scan on A's error — it returns bead-B's orphan alongside
	// the propagated error, so the fail-open wrapper (which discards the
	// error) reproduces bead-B byte-identically, exactly as it did before
	// this refactor.
	t.Run("mixed list: later provable orphan survives an earlier ancestry error", func(t *testing.T) {
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

		findEpicBySpecIDFn = func(specID string) (string, error) { return "epic-1", nil }
		listClosedBeadsFn = func(epicID string) ([]bead.BeadInfo, error) {
			return []bead.BeadInfo{{ID: "bead-a", Status: "closed"}, {ID: "bead-b", Status: "closed"}}, nil
		}
		branchExistsFn = func(name string) bool {
			return name == "bead/bead-a" || name == "bead/bead-b"
		}
		isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
			if ancestor == "bead/bead-a" {
				return false, errors.New("transient git failure on bead-a")
			}
			// bead-b: a provable orphan (branch exists, not an ancestor).
			return false, nil
		}

		orphans, err := ScanOrphanedClosedBeads("008-test", ".", "")
		if err == nil {
			t.Fatal("expected bead-a's ancestry error to propagate from the core")
		}
		if len(orphans) != 1 || orphans[0].BeadID != "bead-b" {
			t.Fatalf("expected the core to still surface bead-b's orphan alongside the error, got %+v", orphans)
		}

		got := FindOrphanedClosedBeads("008-test", ".", "")
		if len(got) != 1 || got[0].BeadID != "bead-b" {
			t.Fatalf("fail-open wrapper must return bead-b byte-identically (ignoring bead-a's error), got %+v", got)
		}
	})
}

// --- Spec 120 R4 (converging pass): RecoveryCommand hostile-BeadID pin ---
//
// Orphan.BeadID is populated straight from `bd list --status=closed --json`
// (ClosedEpicBeadIDs / listClosedBeadsFn) and is NEVER passed through
// idvalidate.BeadID before landing in the struct — a sibling bd-writing
// agent (or a compromised one) can shape it into anything printable. Every
// consumer that renders o.BeadID or o.RecoveryCommand() (checkUnmergedBeads
// in cmd/mindspec/next.go, implOrphanRefusal in internal/approve/impl.go,
// mindspec doctor) must never let a malformed-but-printable value pass
// through looking like a real ID. RecoveryCommand routes through
// idrender.Bead, which pins that here directly rather than relying on the
// generic idrender package test alone to catch a regression at THIS call
// site.
func TestOrphan_RecoveryCommand_HostileBeadIDForcedQuoted(t *testing.T) {
	hostileIDs := []string{
		"mindspec-1\n--help", // newline-injected extra "line"
		"mindspec-1;evil",    // printable metacharacter injection
		"mindspec-1 --force; rm -rf /",
	}
	for _, id := range hostileIDs {
		o := Orphan{BeadID: id, BeadBranch: "bead/" + id, SpecBranch: "spec/008-test"}
		got := o.RecoveryCommand()
		wantQuoted := "mindspec complete " + strconv.Quote(id)
		if got != wantQuoted {
			t.Errorf("RecoveryCommand() for hostile id %q = %q, want forced-quoted %q", id, got, wantQuoted)
		}
		// The forced-quote must fold any embedded newline into the
		// escaped `\n` two-character sequence — no raw newline can
		// reach the rendered command and forge an extra terminal line.
		if strings.ContainsRune(got, '\n') {
			t.Errorf("RecoveryCommand() for hostile id %q still contains a raw newline: %q", id, got)
		}
	}
}

// TestOrphan_RecoveryCommand_CleanBeadIDByteIdentical is the clean-fixture
// counterpart (F3 discipline): a genuine, waist-valid bead ID must still
// render byte-identically — the hostile-input fix must not degrade the
// happy path.
func TestOrphan_RecoveryCommand_CleanBeadIDByteIdentical(t *testing.T) {
	const clean = "mindspec-9cyu.1"
	o := Orphan{BeadID: clean, BeadBranch: "bead/" + clean, SpecBranch: "spec/008-test"}
	want := "mindspec complete " + clean
	if got := o.RecoveryCommand(); got != want {
		t.Errorf("RecoveryCommand() for clean id %q = %q, want byte-identical %q", clean, got, want)
	}
}

// --- Spec 121 R6(a) (mindspec-tpjn, ADR-0041 §2(i)): IsBeadSelfOrphaned ---

// TestIsBeadSelfOrphaned_Orphaned mirrors
// TestFindOrphanedClosedBeads_Orphaned's trigger+confirmation shape
// (closed + branch exists + NOT ancestor) but as a single-bead answer:
// true.
func TestIsBeadSelfOrphaned_Orphaned(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{"bead/bead-1": true},
		map[string]bool{}, // not an ancestor
	)

	got, err := IsBeadSelfOrphaned("008-test", ".", "bead-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected bead-1 to be reported self-orphaned")
	}
}

// TestIsBeadSelfOrphaned_NotClosed: a bead absent from the epic's
// closed-bead list (the normal in-progress case) is never self-orphaned,
// regardless of what its branch looks like.
func TestIsBeadSelfOrphaned_NotClosed(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-other"}, // bead-1 is NOT in the closed list
		map[string]bool{"bead/bead-1": true},
		map[string]bool{},
	)

	got, err := IsBeadSelfOrphaned("008-test", ".", "bead-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected a bead absent from the closed list to be reported NOT self-orphaned")
	}
}

// TestIsBeadSelfOrphaned_BenignMergedUndeleted mirrors
// TestFindOrphanedClosedBeads_BenignMergedUndeleted: closed + branch
// exists + IS an ancestor is the benign merged-but-undeleted state, not
// self-orphaned.
func TestIsBeadSelfOrphaned_BenignMergedUndeleted(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{"bead/bead-1": true},
		map[string]bool{"bead/bead-1": true}, // IS an ancestor
	)

	got, err := IsBeadSelfOrphaned("008-test", ".", "bead-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected the benign merged-but-undeleted state to be reported NOT self-orphaned")
	}
}

// TestIsBeadSelfOrphaned_NoBranch: closed but with no surviving branch at
// all is cleanly completed, not an orphan.
func TestIsBeadSelfOrphaned_NoBranch(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{}, // branch does not exist
		map[string]bool{},
	)

	got, err := IsBeadSelfOrphaned("008-test", ".", "bead-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected a closed bead with no surviving branch to be reported NOT self-orphaned")
	}
}

// TestIsBeadSelfOrphaned_AncestryErrorPropagates: an infra/ancestry-check
// error is PROPAGATED, never swallowed to false — the caller (complete's
// step-1.6 preflight) is the one that must treat a propagated error as
// NOT self-orphaned and retain its refusal; this predicate itself must
// never silently guess.
func TestIsBeadSelfOrphaned_AncestryErrorPropagates(t *testing.T) {
	stubPredicate(t, "epic-1",
		[]string{"bead-1"},
		map[string]bool{"bead/bead-1": true},
		map[string]bool{},
	)
	origAnc := isAncestorFn
	t.Cleanup(func() { isAncestorFn = origAnc })
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
		return false, errors.New("simulated ancestry-check infra failure")
	}

	_, err := IsBeadSelfOrphaned("008-test", ".", "bead-1")
	if err == nil {
		t.Fatal("expected the ancestry-check error to propagate, not be swallowed to false")
	}
}

// TestIsBeadSelfOrphaned_EpicLookupErrorPropagates: an epic/bd-list
// failure (surfaced through ClosedEpicBeadIDs) also propagates.
func TestIsBeadSelfOrphaned_EpicLookupErrorPropagates(t *testing.T) {
	origEpic := findEpicBySpecIDFn
	t.Cleanup(func() { findEpicBySpecIDFn = origEpic })
	findEpicBySpecIDFn = func(specID string) (string, error) {
		return "", errors.New("simulated epic lookup failure")
	}

	_, err := IsBeadSelfOrphaned("008-test", ".", "bead-1")
	if err == nil {
		t.Fatal("expected the epic-lookup error to propagate")
	}
}
