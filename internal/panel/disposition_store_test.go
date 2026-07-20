package panel

// disposition_store_test.go: spec 117 Bead 2 plan Step 6 — tests for
// AppendRecord/WriteTerminalManifest/CheckCompleteness: completeness
// (AC2), idempotency (AC7d), gate-before-mutate (AC7c), the
// finding-less-panel manifest (AC7b), and the T1-T4 concurrency proofs
// (AC7e). All fixtures are constructed INLINE in a temp dir (never
// `--spec 116`, which Bead 4 lands) — the AC2 test reuses Bead 1's
// migrated seed fixture FILES verbatim (testdata/disposition/valid/) so
// its content matches the real spec-116 shapes without re-deriving them.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// --- construction helpers -------------------------------------------------

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

// testRow builds a well-formed DispositionRow for spec "999" / gate
// "bead" with the given id/panel/reviewer/convergent_with. convergent
// MUST be a non-nil (possibly empty) slice: Validate rejects a JSON
// `null` convergent_with, and a nil []string marshals to `null`.
func testRow(id, panelName, reviewer string, convergent []string) DispositionRow {
	if convergent == nil {
		convergent = []string{}
	}
	return DispositionRow{
		Record:         RecordDisposition,
		ID:             id,
		Spec:           "999",
		Gate:           "bead",
		Panel:          panelName,
		Reviewer:       reviewer,
		Model:          "sonnet",
		Severity:       "major",
		Summary:        "test finding " + id,
		ConvergentWith: convergent,
		Disposition:    "confirmed-fixed",
		CreatedAt:      "2026-07-20T00:00:00Z",
		Backfilled:     false,
	}
}

// testManifest builds a well-formed CoverageManifest for spec "999" /
// gate "bead".
func testManifest(panelName string, round int, slots []ManifestSlot) CoverageManifest {
	return CoverageManifest{
		Record:     RecordPanelManifest,
		Spec:       "999",
		Gate:       "bead",
		Panel:      panelName,
		Round:      round,
		Slots:      slots,
		Backfilled: false,
	}
}

// countLines returns the number of non-blank lines in path, or 0 if the
// file does not exist.
func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("reading %s: %v", path, err)
	}
	n := 0
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			n++
		}
	}
	return n
}

// loadFixtureLine reads one already-migrated Bead 1 fixture file
// (internal/panel/testdata/disposition/valid/<name>) and returns its
// trimmed single JSONL line, for reuse building an inline Bead 2 store
// fixture with real seed-shaped content.
func loadFixtureLine(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "disposition", "valid", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return bytes.TrimSpace(data)
}

// writeDispositionsFile writes lines (already-terminated-or-not, this
// adds the newlines) as specDir/reviews/<panel>/dispositions.jsonl,
// creating the panel directory if needed. It bypasses AppendRecord
// entirely — used only to seed a store's STARTING state for a test
// (e.g. the AC2 fixture, or the "delete the covering row" mutation)
// so the test can assert on AppendRecord/CheckCompleteness's behavior
// against a known-shape file, not on AppendRecord's own writing.
func writeDispositionsFile(t *testing.T, specDir, panelName string, lines ...[]byte) {
	t.Helper()
	dir := filepath.Join(specDir, "reviews", panelName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	var buf bytes.Buffer
	for _, l := range lines {
		buf.Write(bytes.TrimSpace(l))
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, dispositionsFileName), buf.Bytes(), 0o600); err != nil {
		t.Fatalf("writing dispositions.jsonl under %s: %v", dir, err)
	}
}

// --- AC2: completeness floor over an inline migrated-seed-shaped fixture --

// TestCheckCompleteness_MigratedSeedShapedFixture_AC2 builds three panel
// files directly from Bead 1's migrated fixture lines (the real
// panel-116-bead2 / panel-116-bead3a / gapfix-panel shapes, including
// the two C1 slot-token canonicalizations: G1-codex and S-tests),
// asserts the floor PASSES on every one of them, then deletes the sole
// covering row for panel-116-bead2's REQUEST_CHANGES slot S1 and asserts
// the floor now FAILS, naming that panel and that slot.
func TestCheckCompleteness_MigratedSeedShapedFixture_AC2(t *testing.T) {
	specDir := t.TempDir()

	writeDispositionsFile(t, specDir, "panel-116-bead2",
		loadFixtureLine(t, "manifest-panel-116-bead2.jsonl"),
		loadFixtureLine(t, "row-10-panel-116-bead2-S1.jsonl"),
	)
	// panel-116-bead3a's manifest marks THREE slots REQUEST_CHANGES
	// (G1-codex, O1, S1); all three covering rows are required for the
	// floor to pass here, including the G1-codex canonicalized coverage.
	writeDispositionsFile(t, specDir, "panel-116-bead3a",
		loadFixtureLine(t, "manifest-panel-116-bead3a.jsonl"),
		loadFixtureLine(t, "row-12-panel-116-bead3a-O1.jsonl"),
		loadFixtureLine(t, "row-13-panel-116-bead3a-S1.jsonl"),
		loadFixtureLine(t, "row-14-panel-116-bead3a-G1-codex.jsonl"),
	)
	// gapfix-panel's manifest marks G-codex and S-tests REQUEST_CHANGES;
	// row-21's `reviewer` is the S-tests canonicalized coverage.
	writeDispositionsFile(t, specDir, "gapfix-panel",
		loadFixtureLine(t, "manifest-gapfix-panel.jsonl"),
		loadFixtureLine(t, "row-20-gapfix-panel-G-codex.jsonl"),
		loadFixtureLine(t, "row-21-gapfix-panel-S-tests.jsonl"),
	)

	for _, p := range []string{"panel-116-bead2", "panel-116-bead3a", "gapfix-panel"} {
		if err := CheckCompleteness(specDir, p); err != nil {
			t.Errorf("CheckCompleteness(%s) = %v, want nil (floor satisfied incl. C1-canonicalized coverages)", p, err)
		}
	}

	// No raw verdict file exists anywhere under specDir. CheckCompleteness
	// passing above IS the proof it never consulted one — it read only
	// each panel's own dispositions.jsonl (readDispositionLines' single
	// os.ReadFile call per panel). Assert the fixture tree itself never
	// grew a verdict-*.json file, so a future edit that (mistakenly)
	// threads one in would be caught here.
	walkErr := filepath.Walk(specDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(filepath.Base(path), "verdict-") {
			t.Errorf("fixture unexpectedly contains a raw verdict file %s; CheckCompleteness must never need one", path)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking %s: %v", specDir, walkErr)
	}

	// Delete the sole covering row for bead-2's S1 REQUEST_CHANGES slot.
	writeDispositionsFile(t, specDir, "panel-116-bead2",
		loadFixtureLine(t, "manifest-panel-116-bead2.jsonl"),
	)
	err := CheckCompleteness(specDir, "panel-116-bead2")
	if err == nil {
		t.Fatal("CheckCompleteness after deleting the S1 covering row = nil, want an error naming panel-116-bead2 + slot S1")
	}
	if !strings.Contains(err.Error(), "panel-116-bead2") || !strings.Contains(err.Error(), "S1") {
		t.Errorf("error %q does not name panel-116-bead2 + slot S1", err.Error())
	}
}

func TestCheckCompleteness_NoManifest_Errors(t *testing.T) {
	specDir := t.TempDir()
	if err := CheckCompleteness(specDir, "no-such-panel"); err == nil {
		t.Fatal("CheckCompleteness on a panel with no dispositions.jsonl = nil, want an error")
	}
}

// --- AC7b: finding-less all-APPROVE panel still gets its manifest --------

func TestWriteTerminalManifest_FindingLessPanel(t *testing.T) {
	specDir := t.TempDir()
	manifest := testManifest("panel-approve-only", 1, []ManifestSlot{
		{Slot: "F1", Model: "fable", Verdict: "APPROVE"},
		{Slot: "O1", Model: "opus", Verdict: "APPROVE"},
	})

	if err := WriteTerminalManifest(specDir, "panel-approve-only", manifest); err != nil {
		t.Fatalf("WriteTerminalManifest: %v", err)
	}

	path := dispositionsPath(specDir, "panel-approve-only")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("finding-less panel: got %d line(s), want exactly 1 (the manifest, zero disposition rows)", len(lines))
	}
	var m map[string]interface{}
	if err := json.Unmarshal(lines[0], &m); err != nil {
		t.Fatalf("unmarshal manifest line: %v", err)
	}
	if m["record"] != RecordPanelManifest {
		t.Errorf("record = %v, want %q", m["record"], RecordPanelManifest)
	}

	// A finding-less (all-APPROVE) panel vacuously satisfies the floor:
	// no slot is REQUEST_CHANGES/REJECT, so nothing needs coverage.
	if err := CheckCompleteness(specDir, "panel-approve-only"); err != nil {
		t.Errorf("CheckCompleteness on an all-APPROVE finding-less panel = %v, want nil", err)
	}
}

// --- AC7d: idempotency -----------------------------------------------------

func TestAppendRecord_DuplicateRowID_NoOp(t *testing.T) {
	specDir := t.TempDir()
	row := mustMarshal(t, testRow("d-dup-row", "panel-dup", "S1", nil))

	if err := AppendRecord(specDir, "panel-dup", row); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := AppendRecord(specDir, "panel-dup", row); err != nil {
		t.Fatalf("second append (duplicate id): %v", err)
	}

	path := dispositionsPath(specDir, "panel-dup")
	if got := countLines(t, path); got != 1 {
		t.Errorf("got %d line(s) after appending the same id twice, want 1 (idempotent)", got)
	}
}

func TestWriteTerminalManifest_SameKeyNoOp_DifferentRoundPersists(t *testing.T) {
	specDir := t.TempDir()
	m1 := testManifest("panel-manifest-idem", 1, []ManifestSlot{{Slot: "S1", Model: "sonnet", Verdict: "APPROVE"}})

	if err := WriteTerminalManifest(specDir, "panel-manifest-idem", m1); err != nil {
		t.Fatalf("first manifest write: %v", err)
	}
	if err := WriteTerminalManifest(specDir, "panel-manifest-idem", m1); err != nil {
		t.Fatalf("second manifest write (same {spec,panel,round}): %v", err)
	}
	path := dispositionsPath(specDir, "panel-manifest-idem")
	if got := countLines(t, path); got != 1 {
		t.Errorf("got %d manifest line(s) for the same key written twice, want 1 (no-op-if-exists)", got)
	}

	// A DIFFERENT round is a DISTINCT key (a re-panel round bump) and
	// must persist as its own second line, never overwriting the first.
	m2 := testManifest("panel-manifest-idem", 2, []ManifestSlot{{Slot: "S1", Model: "sonnet", Verdict: "REQUEST_CHANGES"}})
	if err := WriteTerminalManifest(specDir, "panel-manifest-idem", m2); err != nil {
		t.Fatalf("second-round manifest write: %v", err)
	}
	if got := countLines(t, path); got != 2 {
		t.Errorf("got %d line(s) after a second DISTINCT round, want 2", got)
	}
}

// --- AC7c: gate-before-mutate ----------------------------------------------

func TestAppendRecord_GateBeforeMutate_SchemaInvalid(t *testing.T) {
	specDir := t.TempDir()
	invalid := []byte(`{"record":"disposition"}`) // missing every other required field

	path := dispositionsPath(specDir, "panel-gbm")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("precondition failed: %s already exists", path)
	}

	if err := AppendRecord(specDir, "panel-gbm", invalid); err == nil {
		t.Fatal("AppendRecord(schema-invalid) = nil, want an error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("dispositions.jsonl was created (O_CREATE'd) by a schema-invalid refusal; gate-before-mutate requires it stay absent")
	}

	// Seed one valid row, then confirm a SUBSEQUENT invalid append leaves
	// the file byte-for-byte unchanged.
	validRow := mustMarshal(t, testRow("d-gbm-pre", "panel-gbm", "S1", nil))
	if err := AppendRecord(specDir, "panel-gbm", validRow); err != nil {
		t.Fatalf("seeding valid row: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading seeded file: %v", err)
	}
	if err := AppendRecord(specDir, "panel-gbm", invalid); err == nil {
		t.Fatal("AppendRecord(schema-invalid) over an existing file = nil, want an error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file post-refusal: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("dispositions.jsonl changed after a schema-invalid refusal:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestAppendRecord_GateBeforeMutate_HygieneViolation(t *testing.T) {
	specDir := t.TempDir()
	row := testRow("d-gbm-hygiene", "panel-gbm-hyg", "S1", nil)
	row.Note = "raw verdict lived at /Users/Max/replit/mindspec-panel-verdicts/spec-116/final-116/verdict-O1.json"
	hygieneBad := mustMarshal(t, row)

	path := dispositionsPath(specDir, "panel-gbm-hyg")
	if err := AppendRecord(specDir, "panel-gbm-hyg", hygieneBad); err == nil {
		t.Fatal("AppendRecord(hygiene-violating) = nil, want an error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("dispositions.jsonl was created by a hygiene refusal; gate-before-mutate requires it stay absent")
	}
}

// --- AC7e: concurrency (-race) T1/T2/T3 (in-process) + T4 (cross-process) --

// TestAppendConcurrent_T1_SameIDCollapsesToOneRow: N goroutines append
// the SAME disposition row (same id) concurrently -> exactly one row
// persists (no loss, no duplication).
func TestAppendConcurrent_T1_SameIDCollapsesToOneRow(t *testing.T) {
	specDir := t.TempDir()
	row := mustMarshal(t, testRow("d-t1-same", "panel-t1", "S1", nil))

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendRecord(specDir, "panel-t1", row); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("AppendRecord: %v", err)
	}

	path := dispositionsPath(specDir, "panel-t1")
	if got := countLines(t, path); got != 1 {
		t.Errorf("after %d concurrent same-id appends, got %d line(s), want 1", n, got)
	}
}

// TestAppendConcurrent_T2_SameManifestKeyCollapsesToOne: N goroutines
// write the SAME terminal manifest ({spec,panel,round}) concurrently ->
// exactly one manifest line persists.
func TestAppendConcurrent_T2_SameManifestKeyCollapsesToOne(t *testing.T) {
	specDir := t.TempDir()
	manifest := testManifest("panel-t2", 1, []ManifestSlot{{Slot: "S1", Model: "sonnet", Verdict: "APPROVE"}})

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := WriteTerminalManifest(specDir, "panel-t2", manifest); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("WriteTerminalManifest: %v", err)
	}

	path := dispositionsPath(specDir, "panel-t2")
	if got := countLines(t, path); got != 1 {
		t.Errorf("after %d concurrent same-key manifest writes, got %d line(s), want 1", n, got)
	}
}

// TestAppendConcurrent_T3_DistinctRecordsAllPersist: N goroutines each
// append a DISTINCT disposition row concurrently -> all N persist, every
// line is valid JSON with a distinct id, no interleave/corruption.
func TestAppendConcurrent_T3_DistinctRecordsAllPersist(t *testing.T) {
	specDir := t.TempDir()
	const n = 30

	rows := make([][]byte, n)
	for i := 0; i < n; i++ {
		rows[i] = mustMarshal(t, testRow(fmt.Sprintf("d-t3-%02d", i), "panel-t3", fmt.Sprintf("S%02d", i), nil))
	}

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendRecord(specDir, "panel-t3", rows[i]); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("AppendRecord: %v", err)
	}

	path := dispositionsPath(specDir, "panel-t3")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) != n {
		t.Fatalf("got %d line(s), want %d (no loss)", len(lines), n)
	}
	seen := map[string]bool{}
	for _, l := range lines {
		if !json.Valid(l) {
			t.Errorf("corrupted / non-JSON line: %q", l)
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(l, &m); err != nil {
			t.Fatalf("unmarshal line %q: %v", l, err)
		}
		id, _ := m["id"].(string)
		if seen[id] {
			t.Errorf("duplicate id %s among distinct-record appends", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("got %d distinct id(s), want %d", len(seen), n)
	}
}

// repoRootForDispositionTest walks up from the test's working directory
// to find the mindspec module's go.mod (mirrors
// cmd/mindspec/testhelpers_test.go's repoRootFromTestDir, duplicated
// here since that helper lives in an unimportable `package main`).
func repoRootForDispositionTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		gm := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(gm); err == nil {
			if strings.Contains(string(data), "module github.com/mrmaxsteel/mindspec") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find mindspec go.mod walking up from %s", wd)
		}
		dir = parent
	}
}

// TestAppendConcurrent_T4_CrossProcessLockSerializes builds the real
// mindspec binary and races two SUBPROCESSES of `panel disposition
// append` against the same lockfile — proving the lock serializes
// across PROCESSES, not merely goroutines (flock binds to the open file
// description, so an in-process-only proof would not exercise this).
// Two distinct records must both persist; two identical-id records must
// collapse to one.
func TestAppendConcurrent_T4_CrossProcessLockSerializes(t *testing.T) {
	if testing.Short() {
		t.Skip("builds+runs subprocesses; skipped in -short")
	}

	repoRoot := repoRootForDispositionTest(t)
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "mindspec-t4")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/mindspec")
	buildCmd.Dir = repoRoot
	var buildErr bytes.Buffer
	buildCmd.Stderr = &buildErr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("go build ./cmd/mindspec: %v\nstderr: %s", err, buildErr.String())
	}

	workRoot := t.TempDir()
	const specID = "999-t4race"
	const panelName = "panel-t4"
	if err := os.MkdirAll(filepath.Join(workRoot, ".mindspec", "specs", specID), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}

	// Two DISTINCT records racing the same lockfile: both must persist.
	row1 := mustMarshal(t, testRow("d-t4-distinct-a", panelName, "S1", nil))
	row2 := mustMarshal(t, testRow("d-t4-distinct-b", panelName, "S2", nil))

	c1 := buildAppendCmd(binPath, workRoot, specID, panelName, row1)
	c2 := buildAppendCmd(binPath, workRoot, specID, panelName, row2)
	var out1, out2, errBuf1, errBuf2 bytes.Buffer
	c1.Stdout, c1.Stderr = &out1, &errBuf1
	c2.Stdout, c2.Stderr = &out2, &errBuf2

	if err := c1.Start(); err != nil {
		t.Fatalf("start subprocess 1: %v", err)
	}
	if err := c2.Start(); err != nil {
		t.Fatalf("start subprocess 2: %v", err)
	}
	err1 := c1.Wait()
	err2 := c2.Wait()
	if err1 != nil {
		t.Fatalf("subprocess 1 append failed: %v\nstderr: %s", err1, errBuf1.String())
	}
	if err2 != nil {
		t.Fatalf("subprocess 2 append failed: %v\nstderr: %s", err2, errBuf2.String())
	}

	path := filepath.Join(workRoot, ".mindspec", "specs", specID, "reviews", panelName, dispositionsFileName)
	if got := countLines(t, path); got != 2 {
		t.Fatalf("cross-process concurrent DISTINCT appends: got %d line(s), want 2 (both must persist)", got)
	}

	// Two IDENTICAL-id records racing the same lockfile: exactly one
	// persists (idempotent no-op), proving the lock — not luck — governs
	// which subprocess's write survives.
	const dupPanel = "panel-t4-dup"
	dupRow := mustMarshal(t, testRow("d-t4-dup", dupPanel, "S1", nil))
	d1 := buildAppendCmd(binPath, workRoot, specID, dupPanel, dupRow)
	d2 := buildAppendCmd(binPath, workRoot, specID, dupPanel, dupRow)
	var dout1, dout2, derr1, derr2 bytes.Buffer
	d1.Stdout, d1.Stderr = &dout1, &derr1
	d2.Stdout, d2.Stderr = &dout2, &derr2

	if err := d1.Start(); err != nil {
		t.Fatalf("start dup subprocess 1: %v", err)
	}
	if err := d2.Start(); err != nil {
		t.Fatalf("start dup subprocess 2: %v", err)
	}
	if err := d1.Wait(); err != nil {
		t.Fatalf("dup subprocess 1 append failed: %v\nstderr: %s", err, derr1.String())
	}
	if err := d2.Wait(); err != nil {
		t.Fatalf("dup subprocess 2 append failed: %v\nstderr: %s", err, derr2.String())
	}

	dupPath := filepath.Join(workRoot, ".mindspec", "specs", specID, "reviews", dupPanel, dispositionsFileName)
	if got := countLines(t, dupPath); got != 1 {
		t.Fatalf("cross-process concurrent SAME-id appends: got %d line(s), want 1 (idempotent)", got)
	}
}

// buildAppendCmd constructs an unstarted `mindspec panel disposition
// append` subprocess command against the given binary/workspace root.
func buildAppendCmd(binPath, workRoot, specID, panelName string, data []byte) *exec.Cmd {
	c := exec.Command(binPath, "panel", "disposition", "append", "--spec", specID, "--panel", panelName, "--data", string(data))
	c.Dir = workRoot
	return c
}
