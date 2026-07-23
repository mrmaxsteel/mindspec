package adr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeOverlayADR writes a parseable ADR under root/docs/adr/ with a
// distinguishing title so tests can tell which store a result came from.
func writeOverlayADR(t *testing.T, root, id, title, status, domains string) {
	t.Helper()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "# " + id + ": " + title + "\n\n- **Date**: 2026-06-01\n- **Status**: " + status + "\n- **Domain(s)**: " + domains + "\n\n## Decision\nX.\n"
	if err := os.WriteFile(filepath.Join(adrDir, id+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func newTestOverlay(t *testing.T) (overlay *OverlayStore, branchRoot, primaryRoot string) {
	t.Helper()
	branchRoot = t.TempDir()
	primaryRoot = t.TempDir()
	// ADR-0001: primary only. ADR-0002: branch only. ADR-0003: both
	// (branch wins).
	writeOverlayADR(t, primaryRoot, "ADR-0001", "Primary Only", "Accepted", "core")
	writeOverlayADR(t, branchRoot, "ADR-0002", "Branch Only", "Proposed", "payments")
	writeOverlayADR(t, primaryRoot, "ADR-0003", "Primary Version", "Accepted", "core")
	writeOverlayADR(t, branchRoot, "ADR-0003", "Branch Version", "Accepted", "core")
	overlay = NewOverlayStore(NewFileStore(branchRoot), NewFileStore(primaryRoot))
	return overlay, branchRoot, primaryRoot
}

func TestOverlayStore_Get(t *testing.T) {
	s, _, _ := newTestOverlay(t)

	// Branch-only ADR resolves (the mindspec-ew79 scenario: an ADR
	// committed only on the spec branch must be visible).
	a, err := s.Get("ADR-0002")
	if err != nil {
		t.Fatalf("Get branch-only: %v", err)
	}
	if a.Title != "Branch Only" {
		t.Errorf("Title = %q, want Branch Only", a.Title)
	}

	// Primary-only ADR falls through.
	a, err = s.Get("ADR-0001")
	if err != nil {
		t.Fatalf("Get primary-only: %v", err)
	}
	if a.Title != "Primary Only" {
		t.Errorf("Title = %q, want Primary Only", a.Title)
	}

	// Present in both: branch wins.
	a, err = s.Get("ADR-0003")
	if err != nil {
		t.Fatalf("Get dual: %v", err)
	}
	if a.Title != "Branch Version" {
		t.Errorf("Title = %q, want Branch Version", a.Title)
	}

	// Present in neither: error.
	if _, err := s.Get("ADR-0099"); err == nil {
		t.Error("expected error for ADR missing from both stores")
	}
}

// TestOverlayStore_Get_BranchCollisionPropagates is the G2 final-review
// pin: a branch-local canonical-number collision (two same-numbered ADRs
// in the BRANCH store, e.g. "ADR-0005.md" and "ADR-0005-slug.md") must
// PROPAGATE from OverlayStore.Get, not be masked by a fallback to the
// primary store. Falling back on every branch error — the pre-fix bug —
// would silently show whatever ADR-0005 happens to resolve to in the
// primary store (or a spurious not-found if there is none), hiding the
// real collision from a caller like `adr show`.
func TestOverlayStore_Get_BranchCollisionPropagates(t *testing.T) {
	branchRoot := t.TempDir()
	primaryRoot := t.TempDir()

	// Branch store: a genuine collision — both a bare and a slugged file
	// for ADR-0005.
	writeOverlayADR(t, branchRoot, "ADR-0005", "Bare", "Accepted", "core")
	adrDir := filepath.Join(branchRoot, "docs", "adr")
	content := "# ADR-0005: Slugged\n\n- **Date**: 2026-06-01\n- **Status**: Accepted\n- **Domain(s)**: core\n\n## Decision\nY.\n"
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0005-slugged.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Primary store: an unrelated, cleanly-resolvable ADR-0005 that a
	// wrongful fallback would silently surface instead of the collision.
	writeOverlayADR(t, primaryRoot, "ADR-0005", "Primary Version", "Accepted", "core")

	overlay := NewOverlayStore(NewFileStore(branchRoot), NewFileStore(primaryRoot))

	_, err := overlay.Get("ADR-0005")
	if err == nil {
		t.Fatal("expected a collision error from OverlayStore.Get, got nil (silently fell back to primary)")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("expected a collision error, got a not-found error: %v", err)
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected the branch collision error to propagate (mentioning 'ambiguous'), got: %v", err)
	}
}

func TestOverlayStore_List(t *testing.T) {
	s, _, _ := newTestOverlay(t)

	adrs, err := s.List(ListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(adrs) != 3 {
		t.Fatalf("got %d ADRs, want 3 (union-dedup by ID): %+v", len(adrs), adrs)
	}
	// Sorted by ID, ADR-0003 is the branch version.
	if adrs[0].ID != "ADR-0001" || adrs[1].ID != "ADR-0002" || adrs[2].ID != "ADR-0003" {
		t.Errorf("IDs = %s,%s,%s — want sorted ADR-0001,0002,0003", adrs[0].ID, adrs[1].ID, adrs[2].ID)
	}
	if adrs[2].Title != "Branch Version" {
		t.Errorf("dedup winner Title = %q, want Branch Version", adrs[2].Title)
	}
}

func TestOverlayStore_ListStatusFilter(t *testing.T) {
	s, _, _ := newTestOverlay(t)

	adrs, err := s.List(ListOpts{Status: "Proposed"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(adrs) != 1 || adrs[0].ID != "ADR-0002" {
		t.Errorf("Proposed filter = %+v, want just ADR-0002", adrs)
	}
}

func TestOverlayStore_Search(t *testing.T) {
	s, _, _ := newTestOverlay(t)

	adrs, err := s.Search("Version")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(adrs) != 1 || adrs[0].ID != "ADR-0003" {
		t.Fatalf("Search = %+v, want just ADR-0003", adrs)
	}
	if adrs[0].Title != "Branch Version" {
		t.Errorf("Title = %q, want branch version to win dedup", adrs[0].Title)
	}
}
