package panel

// disposition_migrate_test.go: spec 117 Bead 4 — field-by-field
// round-trip fidelity vs the raw spec-116 seed (AC4/AC7a). Reads the
// REAL absolute seed/archive paths (DefaultSeedPath/DefaultArchiveDir —
// the same fixed local paths the plan's Testing Strategy and the R4
// migration source both name) and migrates into a t.TempDir() target
// (never the live `.mindspec/specs/116-panel-message-escaping` store,
// which is generated and committed separately as the Bead 4 deliverable
// itself).

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// wantGateMapping is the R4 short→canonical gate table pinned as a
// LITERAL in the test — DELIBERATELY not derived from the production
// gateShortToCanonical map (O1-1): asserting the migration's output
// against the very map it consults would let a valid-but-wrong remap
// (e.g. "final" → "spec_approve") stay GREEN. This independent literal
// is the spec/R4 ground truth (spec→spec_approve, plan→plan_approve,
// bead→bead, final→final_review, gapfix→adhoc); if the production map
// ever drifts from it, the round-trip test fails.
var wantGateMapping = map[string]string{
	"spec":   "spec_approve",
	"plan":   "plan_approve",
	"bead":   "bead",
	"final":  "final_review",
	"gapfix": "adhoc",
}

// wantPanelSlotCounts pins the R4/AC7a per-panel coverage-manifest slot
// counts (9/9/8/8/8/8/12/4 = 66), keyed by panel name.
var wantPanelSlotCounts = map[string]int{
	"panel-116-spec":   9,
	"panel-116-plan":   9,
	"panel-116-bead1":  8,
	"panel-116-bead2":  8,
	"panel-116-bead3a": 8,
	"panel-116-bead3b": 8,
	"final-116":        12,
	"gapfix-panel":     4,
}

// migrateForTest runs MigrateSpec116Seed against the real external seed
// + archive paths into dir (a fresh temp target), skipping the test if
// either source path is unavailable in this environment (the migration
// source lives outside the repo, on the maintainer's machine).
func migrateForTest(t *testing.T, dir string) {
	t.Helper()
	if _, err := loadRawSeedRows(DefaultSeedPath); err != nil {
		t.Skipf("spec-116 raw seed unavailable at %s: %v", DefaultSeedPath, err)
	}
	if err := MigrateSpec116Seed(DefaultSeedPath, DefaultArchiveDir, dir); err != nil {
		t.Fatalf("MigrateSpec116Seed: %v", err)
	}
}

// TestSeedMigration_RowAndManifestCounts asserts the migrated store
// holds exactly 21 disposition rows (all spec=116) across the 8 panel
// files, plus exactly one coverage manifest per panel with the pinned
// slot counts, round:1, and backfilled:true throughout (AC4/AC7a).
func TestSeedMigration_RowAndManifestCounts(t *testing.T) {
	dir := t.TempDir()
	migrateForTest(t, dir)

	pattern := filepath.Join(dir, "reviews", "*", "dispositions.jsonl")
	rows, manifests, err := LoadStore(pattern)
	if err != nil {
		t.Fatalf("LoadStore(%s): %v", pattern, err)
	}

	if len(rows) != 21 {
		t.Errorf("migrated store has %d disposition rows, want exactly 21", len(rows))
	}
	for _, r := range rows {
		if r.Spec != Spec116ID {
			t.Errorf("row (panel %s, reviewer %s) has spec %q, want %q", r.Panel, r.Reviewer, r.Spec, Spec116ID)
		}
		if !r.Backfilled {
			t.Errorf("row (panel %s, reviewer %s) has backfilled=false, want true", r.Panel, r.Reviewer)
		}
		if r.Round != nil {
			t.Errorf("row (panel %s, reviewer %s) has round=%v, want nil (migrated rows carry no round)", r.Panel, r.Reviewer, *r.Round)
		}
		if r.CreatedAt != spec116BackfillCreatedAt {
			t.Errorf("row (panel %s, reviewer %s) has created_at %q, want %q", r.Panel, r.Reviewer, r.CreatedAt, spec116BackfillCreatedAt)
		}
	}

	if len(manifests) != len(spec116Panels) {
		t.Fatalf("migrated store has %d coverage manifests, want exactly %d", len(manifests), len(spec116Panels))
	}
	seenPanels := map[string]bool{}
	for _, m := range manifests {
		if seenPanels[m.Panel] {
			t.Errorf("panel %s has more than one coverage manifest", m.Panel)
		}
		seenPanels[m.Panel] = true

		if m.Spec != Spec116ID {
			t.Errorf("manifest (panel %s) has spec %q, want %q", m.Panel, m.Spec, Spec116ID)
		}
		if !m.Backfilled {
			t.Errorf("manifest (panel %s) has backfilled=false, want true", m.Panel)
		}
		if m.Round != spec116MigrationRound {
			t.Errorf("manifest (panel %s) has round=%d, want %d", m.Panel, m.Round, spec116MigrationRound)
		}
		want, ok := wantPanelSlotCounts[m.Panel]
		if !ok {
			t.Errorf("manifest for unexpected panel %s", m.Panel)
			continue
		}
		if len(m.Slots) != want {
			t.Errorf("manifest (panel %s) has %d slots, want %d", m.Panel, len(m.Slots), want)
		}
	}
	for p := range wantPanelSlotCounts {
		if !seenPanels[p] {
			t.Errorf("no coverage manifest found for panel %s", p)
		}
	}
}

// TestSeedMigration_FieldByFieldRoundTrip loads the raw seed
// independently and asserts every migrated row's summary/note/severity/
// model/disposition/spec/panel is byte-identical to its source row,
// gate is mapped ONLY per the R4 table, and reviewer/convergent_with
// are byte-identical EXCEPT the two documented C1 canonicalizations
// (AC4).
func TestSeedMigration_FieldByFieldRoundTrip(t *testing.T) {
	if _, err := loadRawSeedRows(DefaultSeedPath); err != nil {
		t.Skipf("spec-116 raw seed unavailable at %s: %v", DefaultSeedPath, err)
	}
	rawRows, err := loadRawSeedRows(DefaultSeedPath)
	if err != nil {
		t.Fatalf("loadRawSeedRows: %v", err)
	}
	if len(rawRows) != 21 {
		t.Fatalf("raw seed has %d rows, want exactly 21 (has the seed file changed?)", len(rawRows))
	}

	dir := t.TempDir()
	migrateForTest(t, dir)
	pattern := filepath.Join(dir, "reviews", "*", "dispositions.jsonl")
	rows, _, err := LoadStore(pattern)
	if err != nil {
		t.Fatalf("LoadStore(%s): %v", pattern, err)
	}
	if len(rows) != len(rawRows) {
		t.Fatalf("migrated %d rows, raw seed has %d — cannot pair for round-trip", len(rows), len(rawRows))
	}

	// Pair each migrated row to its source raw row by (panel, summary):
	// summary is free, reviewer-authored prose and is unique per row in
	// this 21-row seed, and is asserted byte-identical below anyway.
	byPanelSummary := make(map[string]rawDispositionRow, len(rawRows))
	for _, r := range rawRows {
		byPanelSummary[r.Panel+"\x00"+r.Summary] = r
	}

	canonicalizedCount := 0
	observedGatePairs := map[string]string{}

	for _, mr := range rows {
		raw, ok := byPanelSummary[mr.Panel+"\x00"+mr.Summary]
		if !ok {
			t.Errorf("migrated row (panel %s, reviewer %s, summary %q) has no matching raw seed row", mr.Panel, mr.Reviewer, mr.Summary)
			continue
		}

		if mr.Spec != raw.Spec {
			t.Errorf("panel %s summary %q: spec = %q, want byte-identical %q", mr.Panel, mr.Summary, mr.Spec, raw.Spec)
		}
		if mr.Panel != raw.Panel {
			t.Errorf("panel %s summary %q: panel = %q, want byte-identical %q", mr.Panel, mr.Summary, mr.Panel, raw.Panel)
		}
		if mr.Model != raw.Model {
			t.Errorf("panel %s summary %q: model = %q, want byte-identical %q", mr.Panel, mr.Summary, mr.Model, raw.Model)
		}
		if mr.Severity != raw.Severity {
			t.Errorf("panel %s summary %q: severity = %q, want byte-identical %q", mr.Panel, mr.Summary, mr.Severity, raw.Severity)
		}
		if mr.Disposition != raw.Disposition {
			t.Errorf("panel %s summary %q: disposition = %q, want byte-identical %q", mr.Panel, mr.Summary, mr.Disposition, raw.Disposition)
		}
		if mr.Note != raw.Note {
			t.Errorf("panel %s summary %q: note = %q, want byte-identical %q", mr.Panel, mr.Summary, mr.Note, raw.Note)
		}

		wantGate, gateOK := wantGateMapping[raw.Gate]
		if !gateOK {
			t.Errorf("raw row (panel %s) has gate %q outside the R4 mapping table", raw.Panel, raw.Gate)
		} else if mr.Gate != wantGate {
			t.Errorf("panel %s summary %q: gate = %q, want %q (mapped from raw %q per the R4 literal)", mr.Panel, mr.Summary, mr.Gate, wantGate, raw.Gate)
		}
		observedGatePairs[raw.Gate] = mr.Gate

		wantReviewer := canonicalizeSlot(raw.Panel, raw.Reviewer)
		if mr.Reviewer != wantReviewer {
			t.Errorf("panel %s summary %q: reviewer = %q, want %q", mr.Panel, mr.Summary, mr.Reviewer, wantReviewer)
		}
		if wantReviewer != raw.Reviewer {
			canonicalizedCount++
		}

		if len(mr.ConvergentWith) != len(raw.ConvergentWith) {
			t.Errorf("panel %s summary %q: convergent_with has %d entries, want %d", mr.Panel, mr.Summary, len(mr.ConvergentWith), len(raw.ConvergentWith))
			continue
		}
		for i, rawC := range raw.ConvergentWith {
			wantC := canonicalizeSlot(raw.Panel, rawC)
			if mr.ConvergentWith[i] != wantC {
				t.Errorf("panel %s summary %q: convergent_with[%d] = %q, want %q", mr.Panel, mr.Summary, i, mr.ConvergentWith[i], wantC)
			}
			if wantC != rawC {
				canonicalizedCount++
			}
		}
	}

	// R4/AC4 pin EXACTLY two canonicalizations across the whole seed:
	// panel-116-bead3a's "G-codex"->"G1-codex" (a `reviewer`) and
	// gapfix-panel's "Sonnet-tests"->"S-tests" (a `reviewer`). No other
	// reviewer/convergent_with value changes.
	if canonicalizedCount != 2 {
		t.Errorf("observed %d canonicalized reviewer/convergent_with values, want exactly 2 (G-codex->G1-codex, Sonnet-tests->S-tests)", canonicalizedCount)
	}

	// Confirm the gate map was applied EXCLUSIVELY per the R4 LITERAL
	// table: the observed (raw, canonical) pairs must equal wantGateMapping
	// (the seed exercises all five short forms), independent of the
	// production gateShortToCanonical map (O1-1).
	if len(observedGatePairs) != len(wantGateMapping) {
		t.Errorf("observed %d distinct raw gate values in the seed, want exactly %d (%v)", len(observedGatePairs), len(wantGateMapping), wantGateMapping)
	}
	for raw, canon := range observedGatePairs {
		if wantGateMapping[raw] != canon {
			t.Errorf("observed gate mapping %q -> %q is not in the R4 literal table (%v)", raw, canon, wantGateMapping)
		}
	}
}

// TestSeedMigration_C1Canonicalizations pins the two documented
// canonicalizations by name and location, and confirms every
// already-matching name (e.g. gapfix-panel's own "G-codex") is left
// verbatim (R1/R4 C1).
func TestSeedMigration_C1Canonicalizations(t *testing.T) {
	cases := []struct {
		panel string
		raw   string
		want  string
	}{
		{"panel-116-bead3a", "G-codex", "G1-codex"},
		{"gapfix-panel", "Sonnet-tests", "S-tests"},
		// Already-matching names: left verbatim.
		{"gapfix-panel", "G-codex", "G-codex"},
		{"panel-116-bead1", "Fable", "Fable"},
		{"panel-116-bead1", "O3", "O3"},
		{"panel-116-spec", "F1", "F1"},
	}
	for _, c := range cases {
		got := canonicalizeSlot(c.panel, c.raw)
		if got != c.want {
			t.Errorf("canonicalizeSlot(%q, %q) = %q, want %q", c.panel, c.raw, got, c.want)
		}
	}
}

// TestSeedMigration_CoverageManifestSlotsSorted asserts each migrated
// manifest's slots are in a deterministic (sorted) order and every slot
// token/model/verdict is well-formed, over the real archive.
func TestSeedMigration_CoverageManifestSlotsSorted(t *testing.T) {
	dir := t.TempDir()
	migrateForTest(t, dir)

	pattern := filepath.Join(dir, "reviews", "*", "dispositions.jsonl")
	_, manifests, err := LoadStore(pattern)
	if err != nil {
		t.Fatalf("LoadStore(%s): %v", pattern, err)
	}
	for _, m := range manifests {
		tokens := make([]string, len(m.Slots))
		for i, s := range m.Slots {
			tokens[i] = s.Slot
			if s.Model == "" {
				t.Errorf("panel %s slot %s has empty model", m.Panel, s.Slot)
			}
			if !isValidVerdict(s.Verdict) {
				t.Errorf("panel %s slot %s has out-of-enum verdict %q", m.Panel, s.Slot, s.Verdict)
			}
		}
		if !sort.StringsAreSorted(tokens) {
			t.Errorf("panel %s manifest slots are not sorted: %v", m.Panel, tokens)
		}
	}
}

// TestSeedMigration_Idempotent replays MigrateSpec116Seed against an
// already-migrated target and asserts the row/manifest counts are
// unchanged (AC7d, over the migration path specifically).
func TestSeedMigration_Idempotent(t *testing.T) {
	dir := t.TempDir()
	migrateForTest(t, dir)
	migrateForTest(t, dir) // replay

	pattern := filepath.Join(dir, "reviews", "*", "dispositions.jsonl")
	rows, manifests, err := LoadStore(pattern)
	if err != nil {
		t.Fatalf("LoadStore(%s): %v", pattern, err)
	}
	if len(rows) != 21 {
		t.Errorf("after replaying the migration, store has %d rows, want 21 (replay must be a no-op)", len(rows))
	}
	if len(manifests) != len(spec116Panels) {
		t.Errorf("after replaying the migration, store has %d manifests, want %d (replay must be a no-op)", len(manifests), len(spec116Panels))
	}
}

// TestSeedMigration_CheckCompletenessOverMigratedStore integration-tests
// Bead 2's CheckCompleteness against the freshly migrated store (a
// stand-in for the `mindspec panel disposition check --spec
// 116-panel-message-escaping` integration proof over the LANDED store):
// every panel's R1(b) floor holds after migration.
func TestSeedMigration_CheckCompletenessOverMigratedStore(t *testing.T) {
	dir := t.TempDir()
	migrateForTest(t, dir)

	for _, p := range spec116Panels {
		if err := CheckCompleteness(dir, p); err != nil {
			t.Errorf("CheckCompleteness(%s) = %v, want nil (the migrated store must satisfy the R1(b) floor)", p, err)
		}
	}
}

// TestSplitJSONLLines_OversizedLineFailsClosed proves the S3 fail-closed
// fix: a JSONL blob whose single line exceeds splitJSONLLinesBufCap must
// make splitJSONLLines return a NON-NIL error, never a silent partial /
// empty success. bufio.Scanner otherwise stops SILENTLY on such a line
// (dropping it and every subsequent line), which would let
// MigrateSpec116Seed "succeed" writing fewer rows than the seed holds.
func TestSplitJSONLLines_OversizedLineFailsClosed(t *testing.T) {
	// One line just past the cap, then a second line that a
	// silently-truncating scanner would drop. Any error at all is the
	// pass condition; the point is that Scan's early stop is surfaced.
	oversized := bytes.Repeat([]byte("x"), splitJSONLLinesBufCap+1)
	data := append(oversized, []byte("\n{\"record\":\"disposition\"}\n")...)

	lines, err := splitJSONLLines(data)
	if err == nil {
		t.Fatalf("splitJSONLLines(oversized) = %d lines, nil error; want a non-nil error (fail-closed on Scanner over-cap, not a silent drop)", len(lines))
	}
	if lines != nil {
		t.Errorf("splitJSONLLines(oversized) returned non-nil lines alongside the error; want nil lines")
	}

	// And a well-formed small blob still succeeds — the guard does not
	// reject legitimate input.
	small := []byte("{\"a\":1}\n\n{\"b\":2}\n")
	got, err := splitJSONLLines(small)
	if err != nil {
		t.Fatalf("splitJSONLLines(small) unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("splitJSONLLines(small) = %d lines, want 2 (blank line skipped)", len(got))
	}
}

// repoRootFromMigrateTest resolves the repository root from this test
// file's own location (internal/panel/), with NO absolute machine paths
// and NO external tools — so it runs in CI. thisFile is
// <repo>/internal/panel/disposition_migrate_test.go, so the repo root is
// three directories up.
func repoRootFromMigrateTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to resolve this test file's path")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolved repo root %s has no go.mod: %v", root, err)
	}
	return root
}

// TestCommittedSpec116Store_ValidAndComplete is the AC-global
// "the committed store is valid" guarantee, and — unlike the round-trip
// tests above — it uses NO absolute path and NO external tool, so it
// runs GREEN in CI (F1-2). It loads the COMMITTED landed store at
// .mindspec/specs/116-panel-message-escaping/reviews/*/dispositions.jsonl
// (relative to the repo root resolved from this file's own location) and
// asserts: exactly 21 disposition rows + 8 coverage manifests; every
// record passes Validate + HygienePredicate; and CheckCompleteness holds
// for all 8 panels. If a future edit corrupts, drops, or mis-migrates a
// committed file, THIS test — not just the skippable round-trip — fails.
func TestCommittedSpec116Store_ValidAndComplete(t *testing.T) {
	root := repoRootFromMigrateTest(t)
	specDir := filepath.Join(root, ".mindspec", "specs", Spec116DirName)
	pattern := filepath.Join(specDir, "reviews", "*", "dispositions.jsonl")

	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(files) != len(spec116Panels) {
		t.Fatalf("committed store has %d panel files, want exactly %d", len(files), len(spec116Panels))
	}

	// LoadStore runs Validate on every line internally; do it here too,
	// explicitly, plus HygienePredicate (which LoadStore does not apply)
	// so the R5 hygiene guarantee over the COMMITTED bytes is covered.
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading committed store file %s: %v", f, err)
		}
		for i, line := range bytes.Split(raw, []byte("\n")) {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) == 0 {
				continue
			}
			if err := Validate(trimmed); err != nil {
				t.Errorf("%s line %d: Validate failed: %v", filepath.Base(filepath.Dir(f)), i+1, err)
			}
			if err := HygienePredicate(trimmed); err != nil {
				t.Errorf("%s line %d: HygienePredicate failed: %v", filepath.Base(filepath.Dir(f)), i+1, err)
			}
		}
	}

	rows, manifests, err := LoadStore(pattern)
	if err != nil {
		t.Fatalf("LoadStore(%s): %v", pattern, err)
	}
	if len(rows) != 21 {
		t.Errorf("committed store has %d disposition rows, want exactly 21", len(rows))
	}
	if len(manifests) != len(spec116Panels) {
		t.Errorf("committed store has %d coverage manifests, want exactly %d", len(manifests), len(spec116Panels))
	}
	for _, r := range rows {
		if r.Spec != Spec116ID {
			t.Errorf("committed row (panel %s, reviewer %s) has spec %q, want %q", r.Panel, r.Reviewer, r.Spec, Spec116ID)
		}
		if !r.Backfilled {
			t.Errorf("committed row (panel %s, reviewer %s) has backfilled=false, want true", r.Panel, r.Reviewer)
		}
	}
	for _, m := range manifests {
		if !m.Backfilled || m.Round != spec116MigrationRound {
			t.Errorf("committed manifest (panel %s) has backfilled=%v round=%d, want true/%d", m.Panel, m.Backfilled, m.Round, spec116MigrationRound)
		}
		if want := wantPanelSlotCounts[m.Panel]; len(m.Slots) != want {
			t.Errorf("committed manifest (panel %s) has %d slots, want %d", m.Panel, len(m.Slots), want)
		}
	}

	// Resolve each committed panel dir from the glob (independent of the
	// spec116Panels enumeration) and assert the R1(b) floor holds for
	// each, reading only the durable dispositions.jsonl.
	var panelNames []string
	for _, f := range files {
		panelNames = append(panelNames, filepath.Base(filepath.Dir(f)))
	}
	sort.Strings(panelNames)
	for _, p := range panelNames {
		if err := CheckCompleteness(specDir, p); err != nil {
			t.Errorf("CheckCompleteness(committed %s) = %v, want nil", p, err)
		}
	}
	// And the committed panel set is exactly the 8 spec-116 panels.
	wantSet := map[string]bool{}
	for _, p := range spec116Panels {
		wantSet[p] = true
	}
	for _, p := range panelNames {
		if !wantSet[p] {
			t.Errorf("committed store has unexpected panel %q", p)
		}
		delete(wantSet, p)
	}
	for p := range wantSet {
		t.Errorf("committed store is missing panel %q", p)
	}
}
