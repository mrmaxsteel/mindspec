package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/redact"
)

// stateDir points the journal at a hermetic temp dir for the duration of a
// test (the MINDSPEC_STATE_DIR seam) and returns that dir.
func stateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(StateDirEnv, dir)
	return dir
}

// readJournalBytes returns the raw on-disk journal bytes (or "" if absent).
func readJournalBytes(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, journalFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read journal: %v", err)
	}
	return string(data)
}

// goodEvent is a representative enum-valid friction event (a success-path
// `complete --override-adr`). Argv0 carries a home-dir invocation path to
// prove M3 basename reduction.
func goodEvent() Event {
	return Event{
		Argv0:       "/Users/victim/.local/bin/mindspec",
		Command:     "complete",
		EscapeHatch: "override-adr",
		Subcommand:  "",
		Version:     "1.4.2",
		OS:          "darwin",
	}
}

// TestAppendSuccessEvent_OneEntry asserts a single success-path
// escape-hatch event appends exactly ONE collapsed record with count 1.
func TestAppendSuccessEvent_OneEntry(t *testing.T) {
	stateDir(t)
	if err := AppendSuccessEvent(goodEvent()); err != nil {
		t.Fatalf("AppendSuccessEvent: %v", err)
	}
	recs, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("want exactly 1 record, got %d: %+v", len(recs), recs)
	}
	r := recs[0]
	if r.Count != 1 {
		t.Errorf("want count 1, got %d", r.Count)
	}
	if r.Command != "complete" || r.EscapeHatch != "override-adr" {
		t.Errorf("unexpected enum fields: %+v", r)
	}
	if r.Version != "1.4.2" {
		t.Errorf("want version 1.4.2, got %q", r.Version)
	}
	// Fingerprint matches redact.Fingerprint of the identity.
	wantFP := redact.Fingerprint(redact.Identity{Command: "complete", EscapeHatch: "override-adr"})
	if r.Fingerprint != wantFP {
		t.Errorf("fingerprint mismatch: got %q want %q", r.Fingerprint, wantFP)
	}
	// Identity tuple persisted alongside the hash (DQ5 collision safety).
	if r.Identity != (Identity{Command: "complete", EscapeHatch: "override-adr"}) {
		t.Errorf("identity not persisted as the normalized tuple: %+v", r.Identity)
	}
}

// TestAppendSuccessEvent_EnumOnly_NoRawValue asserts the on-disk entry
// holds ONLY basename(argv0) + enum + fingerprint + count + version — no
// raw invocation path, no flag value (M3/M4). A planted home-dir path and
// override reason must be ABSENT from the journal bytes.
func TestAppendSuccessEvent_EnumOnly_NoRawValue(t *testing.T) {
	dir := stateDir(t)
	if err := AppendSuccessEvent(goodEvent()); err != nil {
		t.Fatalf("AppendSuccessEvent: %v", err)
	}
	raw := readJournalBytes(t, dir)

	// M3: the verbatim home-dir invocation path is never stored; only the
	// basename "mindspec" survives.
	if strings.Contains(raw, "/Users/victim") || strings.Contains(raw, ".local/bin") {
		t.Errorf("journal leaked the raw argv0 invocation path:\n%s", raw)
	}
	if !strings.Contains(raw, `"argv0":"mindspec"`) {
		t.Errorf("journal does not carry basename(argv0)=\"mindspec\":\n%s", raw)
	}
}

// TestAppendSuccessEvent_FailClosedDrop asserts a non-classifiable event
// (a tainted Command that is NOT a closed-set token) yields NO on-disk
// entry and never the raw value (HC-7) — RedactEvent drops it and
// AppendSuccessEvent writes nothing.
func TestAppendSuccessEvent_FailClosedDrop(t *testing.T) {
	dir := stateDir(t)
	ev := goodEvent()
	// A path smuggled into the Command enum field — must DROP the whole
	// event (not a CommandTokens member).
	ev.Command = "/Users/victim/.ssh/id_rsa"
	if err := AppendSuccessEvent(ev); err != nil {
		t.Fatalf("AppendSuccessEvent should swallow a drop as nil error, got %v", err)
	}
	if raw := readJournalBytes(t, dir); raw != "" {
		t.Errorf("fail-closed violated: a dropped event left an on-disk entry:\n%s", raw)
	}
	recs, _ := ListReports()
	if len(recs) != 0 {
		t.Errorf("fail-closed violated: %d records after a dropped event", len(recs))
	}
}

// TestAppendSuccessEvent_StormCap asserts the per-fingerprint storm cap
// (Req 8): firing the same fingerprint M < L times yields one entry with
// count == M; firing L+1 times yields one entry whose count caps at
// JournalStormCapL (count == L, not L+1).
func TestAppendSuccessEvent_StormCap(t *testing.T) {
	stateDir(t)

	// Below the cap: M fires → count == M, one entry.
	const m = 5
	for i := 0; i < m; i++ {
		if err := AppendSuccessEvent(goodEvent()); err != nil {
			t.Fatalf("AppendSuccessEvent #%d: %v", i, err)
		}
	}
	recs, _ := ListReports()
	if len(recs) != 1 {
		t.Fatalf("want 1 collapsed entry, got %d", len(recs))
	}
	if recs[0].Count != m {
		t.Errorf("below cap: want count %d, got %d", m, recs[0].Count)
	}

	// Drive past the cap: L+1 total fires → count == L, still one entry.
	for i := recs[0].Count; i < JournalStormCapL+1; i++ {
		if err := AppendSuccessEvent(goodEvent()); err != nil {
			t.Fatalf("AppendSuccessEvent (storm) #%d: %v", i, err)
		}
	}
	recs, _ = ListReports()
	if len(recs) != 1 {
		t.Fatalf("storm cap: want 1 collapsed entry, got %d", len(recs))
	}
	if recs[0].Count != JournalStormCapL {
		t.Errorf("storm cap: want count == %d (capped), got %d", JournalStormCapL, recs[0].Count)
	}
}

// TestAppendSuccessEvent_DistinctFingerprintsSeparateEntries asserts two
// DIFFERENT friction events do not collapse into one another — each gets
// its own counted entry.
func TestAppendSuccessEvent_DistinctFingerprintsSeparateEntries(t *testing.T) {
	stateDir(t)
	a := goodEvent() // complete / override-adr
	b := goodEvent()
	b.EscapeHatch = "allow-doc-skew" // different escape hatch → different fp
	if err := AppendSuccessEvent(a); err != nil {
		t.Fatal(err)
	}
	if err := AppendSuccessEvent(b); err != nil {
		t.Fatal(err)
	}
	recs, _ := ListReports()
	if len(recs) != 2 {
		t.Fatalf("distinct fingerprints must not collapse: want 2 entries, got %d: %+v", len(recs), recs)
	}
}

// TestJournalPerms0600 asserts the journal file is created 0600 under the
// non-project state dir (HC-8).
func TestJournalPerms0600(t *testing.T) {
	dir := stateDir(t)
	if err := AppendSuccessEvent(goodEvent()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, journalFileName))
	if err != nil {
		t.Fatalf("stat journal: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("journal perms: want 0600, got %o", got)
	}
}

// TestJournalDir_NotUnderProjectTree asserts the resolved store dir is not
// inside any git/bd/dolt project tree (HC-3 store isolation). With the
// MINDSPEC_STATE_DIR seam unset, Dir() resolves to config.GlobalConfigDir,
// which is git-tree-guarded; here we additionally prove the journal path
// is NOT the repo's .beads/issues.jsonl nor under a .git tree.
func TestJournalDir_NotUnderProjectTree(t *testing.T) {
	dir := stateDir(t)
	// The seam dir is a fresh t.TempDir with no .git / .beads — prove the
	// journal lands there and not in any committable tree.
	p, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if filepath.Dir(p) != dir {
		t.Errorf("journal not under the state dir: %q not in %q", p, dir)
	}
	if strings.Contains(p, ".beads") {
		t.Errorf("journal path is under the beads tracker: %q", p)
	}
	// Walk up from the journal path: no enclosing .git (the temp dir is
	// outside any repo).
	cur := filepath.Dir(p)
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			t.Fatalf("journal store is under a git work tree at %q (HC-3 violation)", cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
}

// TestListReports_Empty asserts a missing journal reads as zero records,
// no error.
func TestListReports_Empty(t *testing.T) {
	stateDir(t)
	recs, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports on empty journal: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("want 0 records on a fresh state dir, got %d", len(recs))
	}
}

// TestReadRecords_SkipsMalformedLine asserts a partial/corrupt line (from a
// cross-process append race) is skipped, not fatal — the rest survive.
func TestReadRecords_SkipsMalformedLine(t *testing.T) {
	dir := stateDir(t)
	if err := AppendSuccessEvent(goodEvent()); err != nil {
		t.Fatal(err)
	}
	// Append a torn line directly.
	f, err := os.OpenFile(filepath.Join(dir, journalFileName), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{not valid json\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	recs, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports tolerating a torn line: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("want 1 valid record (torn line skipped), got %d", len(recs))
	}
}
