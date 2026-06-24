package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

// Spec 106 Bead 4 (Req 8 / AC14): mindspec doctor detects the docs layout,
// warns when a canonical/legacy tree would flatten, ERRORs on a dual-layout
// duplicate spec id, and its dry-run-migration spec walk is tier-aware.

// TestCheckLayout_ReportsCanonicalAndWouldMigrate: a canonical tree reports
// layout=canonical and emits a would-migrate-layout Warn (no failure).
func TestCheckLayout_ReportsCanonicalAndWouldMigrate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs", "106-x"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkLayout(r, root)

	layout := findCheck(r, "layout")
	if layout == nil || layout.Message != "canonical" {
		t.Errorf("expected layout=canonical; got %+v", r.Checks)
	}
	wm := findCheck(r, "would-migrate-layout")
	if wm == nil || wm.Status != Warn {
		t.Errorf("expected a would-migrate-layout Warn on a canonical tree; got %+v", r.Checks)
	}
	if r.HasFailures() {
		t.Errorf("a clean canonical tree must not have failures; got %+v", r.Checks)
	}
}

// TestCheckLayout_FlatNoWouldMigrate: a flat tree reports layout=flat and emits
// NO would-migrate-layout warning (nothing to flatten).
func TestCheckLayout_FlatNoWouldMigrate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs", "106-x"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkLayout(r, root)

	layout := findCheck(r, "layout")
	if layout == nil || layout.Message != "flat" {
		t.Errorf("expected layout=flat; got %+v", r.Checks)
	}
	if findCheck(r, "would-migrate-layout") != nil {
		t.Errorf("a flat tree must NOT emit would-migrate-layout; got %+v", r.Checks)
	}
}

// TestCheckLayout_DualLayoutDuplicateIDErrors: the SAME spec id present under
// two layout tiers is an ERROR (the stale-duplicate read hazard).
func TestCheckLayout_DualLayoutDuplicateIDErrors(t *testing.T) {
	root := t.TempDir()
	// Same id under flat AND canonical → mixed tree + duplicate id.
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs", "106-dup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs", "106-dup"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkLayout(r, root)

	dup := findCheck(r, "dual-layout-spec: 106-dup")
	if dup == nil || dup.Status != Error {
		t.Fatalf("expected a dual-layout-spec ERROR for 106-dup; got %+v", r.Checks)
	}
	if !r.HasFailures() {
		t.Error("a dual-layout duplicate id must trip HasFailures()")
	}
}

// TestCheckDryRunMigration_TierAwareFlat: the dry-run reporter walks the FLAT
// spec root via workspace.SpecsDir (not the hardcoded .mindspec/docs/specs), so
// a flat tree's specs are still enumerated.
func TestCheckDryRunMigration_TierAwareFlat(t *testing.T) {
	root := t.TempDir()
	// Flat spec dir — the OLD hardcoded path would miss this entirely.
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs", "106-legacy"), 0o755); err != nil {
		t.Fatal(err)
	}

	epicLegacy := `{"id":"epic-106","title":"[SPEC 106-legacy] Legacy","status":"open","issue_type":"epic","metadata":{"spec_num":106,"spec_title":"legacy"}}`
	restore := stubDryRunFinders(t,
		`[`+epicLegacy+`]`,
		map[string]string{"epic-106": `[]`},
		map[string]string{"epic-106": epicLegacy},
	)
	defer restore()

	r := &Report{}
	checkDryRunMigration(r, root)

	if findCheck(r, "would-migrate: spec=106-legacy") == nil {
		t.Errorf("tier-aware dry-run must enumerate the FLAT spec 106-legacy; got %+v", r.Checks)
	}
}

// TestRunWithOptions_RegistersLayoutCheck: the normal doctor run includes the
// layout detector.
func TestRunWithOptions_RegistersLayoutCheck(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "domains"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := RunWithOptions(root, Options{})
	if findCheck(r, "layout") == nil {
		t.Errorf("the normal doctor run must register the layout detector; checks=%+v", r.Checks)
	}
}
