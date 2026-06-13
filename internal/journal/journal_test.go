package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/redact"
)

// resetSession clears the per-process storm counters so each test starts
// from a clean session (the counters persist for the process lifetime in
// production).
func resetSession(t *testing.T) {
	t.Helper()
	mu.Lock()
	sessionCounts = map[string]int{}
	mu.Unlock()
}

// stateDir points the journal at a hermetic temp dir for the duration of a
// test (the MINDSPEC_STATE_DIR seam) and returns that dir. It also resets
// the per-process storm counters so the per-session cap is deterministic.
func stateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(StateDirEnv, dir)
	resetSession(t)
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

// TestAppendSuccessEvent_OneLine asserts a single success-path
// escape-hatch event appends exactly ONE append-only record carrying its
// enum fields, fingerprint, identity, version, and a ts.
func TestAppendSuccessEvent_OneLine(t *testing.T) {
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
	if r.Command != "complete" || r.EscapeHatch != "override-adr" {
		t.Errorf("unexpected enum fields: %+v", r)
	}
	if r.Version != "1.4.2" {
		t.Errorf("want version 1.4.2, got %q", r.Version)
	}
	// §Storage Contract requires a ts (rfc3339); assert it is present.
	if r.TS == "" {
		t.Errorf("record is missing the required ts field: %+v", r)
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
// holds ONLY basename(argv0) + enum + fingerprint + version + ts — no
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

// TestAppendSuccessEvent_PerSessionStormCap asserts the per-fingerprint-
// PER-SESSION append cap (Req 8): within ONE process session, firing the
// same fingerprint M < L times appends M lines; firing L+1 times appends
// exactly L lines (the L+1-th and beyond are dropped, NOT collapsed). The
// journal is append-only — each survivor is its own line.
func TestAppendSuccessEvent_PerSessionStormCap(t *testing.T) {
	stateDir(t)

	// Below the cap: M fires within this session → M lines.
	const m = 5
	for i := 0; i < m; i++ {
		if err := AppendSuccessEvent(goodEvent()); err != nil {
			t.Fatalf("AppendSuccessEvent #%d: %v", i, err)
		}
	}
	recs, _ := ListReports()
	if len(recs) != m {
		t.Fatalf("below cap: want %d append-only lines, got %d", m, len(recs))
	}

	// Drive past the cap within the SAME session: total L+10 fires →
	// exactly L lines (appends beyond the per-session cap are dropped).
	for i := m; i < JournalStormCapL+10; i++ {
		if err := AppendSuccessEvent(goodEvent()); err != nil {
			t.Fatalf("AppendSuccessEvent (storm) #%d: %v", i, err)
		}
	}
	recs, _ = ListReports()
	if len(recs) != JournalStormCapL {
		t.Errorf("per-session storm cap: want exactly %d lines (capped), got %d", JournalStormCapL, len(recs))
	}
}

// TestAppendSuccessEvent_StormCapIsPerSession asserts the cap is scoped to
// a SESSION: after the cap is hit, a NEW session (counters reset, mirroring
// a fresh process) can append again for the same fingerprint. Cross-session
// growth is bounded by real usage, not by an in-file lifetime cap.
func TestAppendSuccessEvent_StormCapIsPerSession(t *testing.T) {
	stateDir(t)
	for i := 0; i < JournalStormCapL+5; i++ {
		if err := AppendSuccessEvent(goodEvent()); err != nil {
			t.Fatalf("session 1 append #%d: %v", i, err)
		}
	}
	if recs, _ := ListReports(); len(recs) != JournalStormCapL {
		t.Fatalf("session 1: want %d lines, got %d", JournalStormCapL, len(recs))
	}
	// New session: reset the per-process counters (a fresh invocation).
	resetSession(t)
	if err := AppendSuccessEvent(goodEvent()); err != nil {
		t.Fatalf("session 2 append: %v", err)
	}
	if recs, _ := ListReports(); len(recs) != JournalStormCapL+1 {
		t.Errorf("session 2: a fresh session appends again past the prior cap; want %d lines, got %d", JournalStormCapL+1, len(recs))
	}
}

// TestAppendSuccessEvent_DistinctFingerprintsSeparateLines asserts two
// DIFFERENT friction events each append their own line (append-only — no
// collapse between them).
func TestAppendSuccessEvent_DistinctFingerprintsSeparateLines(t *testing.T) {
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
		t.Fatalf("distinct fingerprints: want 2 append-only lines, got %d: %+v", len(recs), recs)
	}
}

// TestAppendSuccessEvent_PreservesPerEventVersion asserts each line keeps
// its OWN version stamp so Bead 3 can derive first/last-seen version per
// identity from the append-only history (the version-overwrite defect the
// collapse-in-journal design had is structurally gone).
func TestAppendSuccessEvent_PreservesPerEventVersion(t *testing.T) {
	stateDir(t)
	v1 := goodEvent()
	v1.Version = "1.4.2"
	v2 := goodEvent()
	v2.Version = "1.5.0"
	if err := AppendSuccessEvent(v1); err != nil {
		t.Fatal(err)
	}
	if err := AppendSuccessEvent(v2); err != nil {
		t.Fatal(err)
	}
	recs, _ := ListReports()
	if len(recs) != 2 {
		t.Fatalf("want 2 version-stamped lines, got %d", len(recs))
	}
	got := map[string]bool{recs[0].Version: true, recs[1].Version: true}
	if !got["1.4.2"] || !got["1.5.0"] {
		t.Errorf("per-event version history not preserved: got versions %q,%q want 1.4.2 and 1.5.0", recs[0].Version, recs[1].Version)
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
// MINDSPEC_STATE_DIR seam pointed at a clean temp dir, Path() resolves
// there; here we additionally prove the journal path is NOT the repo's
// .beads/issues.jsonl nor under a .git tree.
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

// TestStateDirEnv_GitTreeRejectFailsClosed asserts MINDSPEC_STATE_DIR
// pointed inside a git work tree (e.g. repo/.beads/friction-state) is
// REJECTED and the write FAILS CLOSED — NOTHING is written (HC-3). This is
// the isolation-bypass the codex adversarial/concurrency reviewers flagged.
func TestStateDirEnv_GitTreeRejectFailsClosed(t *testing.T) {
	// Build a fake repo: <repo>/.git plus a .beads/friction-state subdir the
	// override points at.
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	guarded := filepath.Join(repo, ".beads", "friction-state")
	if err := os.MkdirAll(guarded, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(StateDirEnv, guarded)
	resetSession(t)

	// Dir() and Path() must reject the in-tree override.
	if _, err := Dir(); err == nil {
		t.Fatalf("Dir() accepted a MINDSPEC_STATE_DIR inside a git tree (HC-3 bypass)")
	}

	// AppendSuccessEvent must fail closed: an error returned AND nothing on
	// disk anywhere under the repo.
	if err := AppendSuccessEvent(goodEvent()); err == nil {
		t.Errorf("AppendSuccessEvent accepted a guarded override (want fail-closed error)")
	}
	if _, err := os.Stat(filepath.Join(guarded, journalFileName)); !os.IsNotExist(err) {
		t.Errorf("fail-closed violated: journal.jsonl was written under the guarded git tree")
	}
}

// TestStateDirEnv_AcceptedOverrideStill0600 asserts an ACCEPTED override
// (a temp dir outside any git tree) still writes the journal 0600 — the
// guard rejects only in-tree paths, not legitimate explicit seams.
func TestStateDirEnv_AcceptedOverrideStill0600(t *testing.T) {
	dir := stateDir(t) // a clean temp dir, no .git ancestor
	if err := AppendSuccessEvent(goodEvent()); err != nil {
		t.Fatalf("AppendSuccessEvent on an accepted override: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, journalFileName))
	if err != nil {
		t.Fatalf("stat journal: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("accepted override journal perms: want 0600, got %o", got)
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

// TestMarkResolved_DoesNotMutateJournal asserts the Bead-3 MarkResolved
// seam never touches the append-only journal (the journal is immutable
// history; resolution lives on Bead 3's reports layer).
func TestMarkResolved_DoesNotMutateJournal(t *testing.T) {
	stateDir(t)
	if err := AppendSuccessEvent(goodEvent()); err != nil {
		t.Fatal(err)
	}
	before, _ := ListReports()
	if err := MarkResolved("somefp", "2.0.0"); err != nil {
		t.Fatalf("MarkResolved stub returned an error: %v", err)
	}
	after, _ := ListReports()
	if len(before) != len(after) {
		t.Errorf("MarkResolved mutated the append-only journal: %d → %d lines", len(before), len(after))
	}
}
