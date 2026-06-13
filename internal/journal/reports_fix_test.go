package journal

// reports_fix_test.go — spec 094 Bead 3 (6-panel fix): regression tests for
// the panel's demonstrated repros and reconciliation decisions:
//
//   - occurrence-order first/last consolidation (codex-consolidation #1);
//   - cross-process resolve preservation (codex-consolidation #2);
//   - source-side resolve-version normalization / shell-metachar rejection
//     (R1 / Req 7 / HC-4);
//   - status model {open, regression, stale} — no dead StatusResolved
//     (codex-completeness #1);
//   - fingerprint = H(identity) collision safety: distinct identities →
//     distinct fingerprints (codex-completeness #2 / DQ5).

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/redact"
)

// appendVersionedIdentity appends a friction event for a chosen identity at a
// chosen version + ts (deterministic ordering via the nowRFC3339 seam).
func appendVersionedIdentity(t *testing.T, ev Event, ver, ts string) {
	t.Helper()
	orig := nowRFC3339
	nowRFC3339 = func() string { return ts }
	defer func() { nowRFC3339 = orig }()
	ev.Version = ver
	if err := AppendSuccessEvent(ev); err != nil {
		t.Fatalf("AppendSuccessEvent: %v", err)
	}
}

// TestReviewConsolidate_UsesEarliestAndLatestEvent is the codex-consolidation
// #1 repro: events appended NEWEST-first (out of semver order) must still yield
// first_version from the chronologically EARLIEST event and last_version from
// the LATEST — derived by OCCURRENCE ORDER (ts), NOT by semver extrema. The
// version and its paired *_seen_ts move together.
//
// RED-before: the old min/max derivation returned first_version="1.0.0" (semver
// min) instead of the earliest event's version "2.0.0".
func TestReviewConsolidate_UsesEarliestAndLatestEvent(t *testing.T) {
	stateDir(t)
	// Append out of semver order: the EARLIEST event (by ts) is at 2.0.0, a
	// LATER event is at 1.0.0 (a downgrade build). Semver min/max would pick
	// 1.0.0 as "first" — wrong; occurrence order says first = 2.0.0.
	appendVersioned(t, "2.0.0", "2026-01-01T00:00:00Z") // earliest occurrence
	appendVersioned(t, "3.0.0", "2026-02-01T00:00:00Z")
	appendVersioned(t, "1.0.0", "2026-03-01T00:00:00Z") // latest occurrence

	reports, err := Consolidate()
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("want 1 report, got %d", len(reports))
	}
	r := reports[0]
	if r.FirstVersion != "2.0.0" {
		t.Errorf("first_version: want earliest event's version 2.0.0, got %q", r.FirstVersion)
	}
	if r.FirstSeenTS != "2026-01-01T00:00:00Z" {
		t.Errorf("first_seen_ts: want earliest event's ts, got %q", r.FirstSeenTS)
	}
	if r.LastVersion != "1.0.0" {
		t.Errorf("last_version: want latest event's version 1.0.0, got %q", r.LastVersion)
	}
	if r.LastSeenTS != "2026-03-01T00:00:00Z" {
		t.Errorf("last_seen_ts: want latest event's ts, got %q", r.LastSeenTS)
	}
}

// TestConsolidate_OrdersByTSNotFileOrder asserts that when the journal is read
// in file order but timestamps are out of file order, consolidation orders by
// ts (stable). The append-only journal is normally oldest-first, but a stable
// ts-sort defends a clock-jittered / interleaved stream.
func TestConsolidate_OrdersByTSNotFileOrder(t *testing.T) {
	stateDir(t)
	// Append file-order A,B but with B's ts BEFORE A's ts.
	appendVersioned(t, "5.0.0", "2026-05-01T00:00:00Z") // file-first, ts-LATER
	appendVersioned(t, "4.0.0", "2026-04-01T00:00:00Z") // file-second, ts-EARLIER

	reports, err := Consolidate()
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	r := reports[0]
	if r.FirstVersion != "4.0.0" {
		t.Errorf("first_version: want ts-earliest 4.0.0, got %q", r.FirstVersion)
	}
	if r.LastVersion != "5.0.0" {
		t.Errorf("last_version: want ts-latest 5.0.0, got %q", r.LastVersion)
	}
}

// TestReviewConcurrentResolvePreservedAcrossProcesses is the
// codex-consolidation #2 repro: a stale `report` (Consolidate→WriteReports)
// running in a SEPARATE process must not clobber a concurrent
// `report list --resolve` and erase resolved_in_version. We simulate the lost
// update deterministically:
//
//  1. parent seeds + consolidates + writes (open report);
//  2. a HELPER process loads a STALE snapshot (the open slice) and PAUSES
//     before writing — modeled here by capturing the open slice;
//  3. parent resolves the fingerprint (writes resolved_in_version to disk);
//  4. the helper now writes its STALE open slice back.
//
// Without the cross-process compare-and-merge in WriteReports, step 4 erases
// the resolution. With it, the non-empty on-disk resolved_in_version WINS.
//
// We exercise the merge directly (the helper's stale WriteReports) plus a real
// separate-process write to prove it holds across an actual os.Exec boundary.
func TestReviewConcurrentResolvePreservedAcrossProcesses(t *testing.T) {
	dir := stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")

	// (1) parent consolidates + writes the open report.
	open, err := Consolidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteReports(open); err != nil {
		t.Fatal(err)
	}
	fp := open[0].Fingerprint

	// (2) capture a STALE open snapshot (what a concurrent `report` already
	// holds in memory before it writes).
	stale := make([]Report, len(open))
	copy(stale, open)

	// (3) a separate process resolves the fingerprint (writes resolved_in to
	// disk). We re-invoke the test binary as a resolve helper so the resolve
	// genuinely happens in another OS process.
	resolveInSeparateProcess(t, dir, fp, "2.0.0")

	persisted, err := ReadReports()
	if err != nil || len(persisted) != 1 || persisted[0].ResolvedInVersion != "2.0.0" {
		t.Fatalf("separate-process resolve did not persist: %+v (err=%v)", persisted, err)
	}

	// (4) the STALE consolidator now writes its open slice back. The
	// compare-and-merge in WriteReports must PRESERVE the newer resolution.
	if err := WriteReports(stale); err != nil {
		t.Fatal(err)
	}

	after, err := ReadReports()
	if err != nil || len(after) != 1 {
		t.Fatalf("ReadReports after stale write: %+v (err=%v)", after, err)
	}
	if after[0].ResolvedInVersion != "2.0.0" {
		t.Errorf("stale cross-process write ERASED the resolve: resolved_in_version=%q, want 2.0.0",
			after[0].ResolvedInVersion)
	}
}

// resolveInSeparateProcess re-execs the test binary in a child mode that calls
// MarkResolved against the SAME store dir, proving the resolution survives an
// actual process boundary (not just the in-process merge path).
func resolveInSeparateProcess(t *testing.T, dir, fp, ver string) {
	t.Helper()
	if os.Getenv("MINDSPEC_TEST_RESOLVE_HELPER") == "1" {
		return // guard: never recurse inside the helper
	}
	cmd := exec.Command(os.Args[0], "-test.run", "TestResolveHelperProcess")
	cmd.Env = append(os.Environ(),
		"MINDSPEC_TEST_RESOLVE_HELPER=1",
		StateDirEnv+"="+dir,
		"MINDSPEC_TEST_RESOLVE_FP="+fp,
		"MINDSPEC_TEST_RESOLVE_VER="+ver,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve helper process failed: %v\n%s", err, out)
	}
}

// TestResolveHelperProcess is the child entry point for
// resolveInSeparateProcess. It is a no-op unless the helper env is set, so it
// never runs as part of the normal suite.
func TestResolveHelperProcess(t *testing.T) {
	if os.Getenv("MINDSPEC_TEST_RESOLVE_HELPER") != "1" {
		t.Skip("not the resolve helper process")
	}
	fp := os.Getenv("MINDSPEC_TEST_RESOLVE_FP")
	ver := os.Getenv("MINDSPEC_TEST_RESOLVE_VER")
	// When MINDSPEC_TEST_RESOLVE_PAUSE_MS is set, this helper PAUSES inside the
	// reports.jsonl read-modify-write window (after the on-disk re-read, before
	// the rename) via the writeRereadHook seam, so a concurrent second-process
	// resolve is given the chance to interleave. This deterministically widens
	// the cross-process lost-update window so the OS lock is what makes the
	// outcome correct, not timing luck.
	if ms := os.Getenv("MINDSPEC_TEST_RESOLVE_PAUSE_MS"); ms != "" {
		if d, err := time.ParseDuration(ms + "ms"); err == nil {
			var once sync.Once
			writeRereadHook = func() { once.Do(func() { time.Sleep(d) }) }
		}
	}
	if err := MarkResolved(fp, ver); err != nil {
		t.Fatalf("helper MarkResolved: %v", err)
	}
}

// TestMarkResolved_RejectsShellMetacharVersion is the R1 slot-escaping fix at
// the SOURCE: a shell-metachar --version value is REJECTED by MarkResolved and
// NEVER persisted, so it can never reach a copy-pasteable rendered field.
func TestMarkResolved_RejectsShellMetacharVersion(t *testing.T) {
	stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	reports, _ := Consolidate()
	if err := WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	fp := reports[0].Fingerprint

	for _, bad := range []string{
		"1.0.0; rm -rf /",
		"$(rm -rf /)",
		"`id`",
		"1.0.0 | cat /etc/passwd",
		"1.0.0 && reboot",
		"not-a-version",
	} {
		if err := MarkResolved(fp, bad); err == nil {
			t.Errorf("MarkResolved(%q) should be rejected, got nil error", bad)
		}
		// And nothing was persisted.
		persisted, _ := ReadReports()
		if len(persisted) == 1 && persisted[0].ResolvedInVersion != "" {
			t.Errorf("rejected version %q leaked into resolved_in_version=%q", bad, persisted[0].ResolvedInVersion)
		}
	}
}

// TestMarkResolved_CanonicalizesSemver asserts an accepted resolve version is
// canonicalized to bare major.minor.patch (a leading `v` / suffix is dropped),
// so only a well-formed value is ever persisted.
func TestMarkResolved_CanonicalizesSemver(t *testing.T) {
	stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	reports, _ := Consolidate()
	if err := WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	fp := reports[0].Fingerprint

	if err := MarkResolved(fp, "v2.3.4-rc1+build"); err != nil {
		t.Fatalf("MarkResolved on a decorated semver: %v", err)
	}
	persisted, _ := ReadReports()
	if persisted[0].ResolvedInVersion != "2.3.4" {
		t.Errorf("resolve version not canonicalized: want 2.3.4, got %q", persisted[0].ResolvedInVersion)
	}
}

// TestMarkResolved_AcceptsDev asserts the explicit dev sentinel is accepted
// (the DQ4 unbounded-newest policy value).
func TestMarkResolved_AcceptsDev(t *testing.T) {
	stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	reports, _ := Consolidate()
	if err := WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	fp := reports[0].Fingerprint
	if err := MarkResolved(fp, "dev"); err != nil {
		t.Fatalf("MarkResolved with dev should be accepted: %v", err)
	}
	persisted, _ := ReadReports()
	if persisted[0].ResolvedInVersion != "dev" {
		t.Errorf("dev resolve not persisted: got %q", persisted[0].ResolvedInVersion)
	}
}

// TestClassify_StatusModelIsThreeState asserts the status model is exactly
// {open, regression, stale} — Classify never returns a "resolved" token
// (codex-completeness #1: StatusResolved was dead). It also pins the value set.
func TestClassify_StatusModelIsThreeState(t *testing.T) {
	cases := []struct {
		last, resolved string
		want           Status
	}{
		{"1.0.0", "", StatusOpen},
		{"2.0.0", "2.0.0", StatusRegression}, // == boundary
		{"2.1.0", "2.0.0", StatusRegression}, // >
		{"1.0.0", "2.0.0", StatusStale},      // < (resolved, no recurrence since)
		{"dev", "2.0.0", StatusRegression},   // dev → unbounded-newest
	}
	for _, tc := range cases {
		r := Report{LastVersion: tc.last, ResolvedInVersion: tc.resolved}
		got := r.Classify()
		if got != tc.want {
			t.Errorf("Classify(last=%q,resolved=%q): want %q, got %q", tc.last, tc.resolved, tc.want, got)
		}
		if got != StatusOpen && got != StatusRegression && got != StatusStale {
			t.Errorf("Classify returned a token outside {open,regression,stale}: %q", got)
		}
	}
}

// TestFingerprint_DistinctIdentitiesDistinctFingerprints is the
// codex-completeness #2 / DQ5 collision-safety pin: because the fingerprint is
// a strong hash over the FULL normalized identity, two DISTINCT identities
// yield DISTINCT fingerprints — so fingerprint-keying IS identity-keying by
// construction and consolidation/MarkResolved keying by fingerprint alone is
// collision-safe.
func TestFingerprint_DistinctIdentitiesDistinctFingerprints(t *testing.T) {
	ids := []redact.Identity{
		{Command: "complete", EscapeHatch: "override-adr", Subcommand: ""},
		{Command: "complete", EscapeHatch: "allow-doc-skew", Subcommand: ""},
		{Command: "plan", EscapeHatch: "override-adr", Subcommand: ""},
		{Command: "complete", EscapeHatch: "override-adr", Subcommand: "impl"},
		// The cross-field-aliasing pair the NUL framing must keep distinct.
		{Command: "complete-", EscapeHatch: "override", Subcommand: "-adr"},
		{Command: "complete", EscapeHatch: "override-adr", Subcommand: ""},
	}
	seen := map[string]redact.Identity{}
	for _, id := range ids {
		fp := redact.Fingerprint(id)
		if prev, ok := seen[fp]; ok && prev != id {
			t.Errorf("distinct identities %+v and %+v collided on fingerprint %s", prev, id, fp)
		}
		seen[fp] = id
	}
	// And the two identical identities at index 0 and 5 share a fingerprint.
	if redact.Fingerprint(ids[0]) != redact.Fingerprint(ids[5]) {
		t.Errorf("identical identities must share a fingerprint")
	}
}

// TestConsolidate_AndResolve_KeyedByFingerprintIsIdentityKeyed proves two
// DISTINCT identities consolidate to TWO reports and resolving one does NOT
// resolve the other (fingerprint-keying = identity-keying).
func TestConsolidate_AndResolve_KeyedByFingerprintIsIdentityKeyed(t *testing.T) {
	stateDir(t)
	a := goodEvent() // complete / override-adr
	b := goodEvent()
	b.EscapeHatch = "allow-doc-skew" // distinct identity
	appendVersionedIdentity(t, a, "1.0.0", "2026-01-01T00:00:00Z")
	appendVersionedIdentity(t, b, "1.0.0", "2026-01-02T00:00:00Z")

	reports, err := Consolidate()
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatalf("want 2 distinct reports, got %d", len(reports))
	}
	if err := WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	// Resolve only A.
	var fpA, fpB string
	for _, r := range reports {
		if r.EscapeHatch == "override-adr" {
			fpA = r.Fingerprint
		} else {
			fpB = r.Fingerprint
		}
	}
	if fpA == fpB {
		t.Fatal("distinct identities must have distinct fingerprints")
	}
	if err := MarkResolved(fpA, "2.0.0"); err != nil {
		t.Fatal(err)
	}
	persisted, _ := ReadReports()
	for _, r := range persisted {
		if r.Fingerprint == fpA && r.ResolvedInVersion != "2.0.0" {
			t.Errorf("A not resolved: %q", r.ResolvedInVersion)
		}
		if r.Fingerprint == fpB && r.ResolvedInVersion != "" {
			t.Errorf("resolving A leaked onto B: %q", r.ResolvedInVersion)
		}
	}
}

// TestWriteReports_OverwriteNotAppend asserts reports.jsonl is rewritten
// WHOLESALE across re-consolidation (no stale duplicate lines), the
// §Storage-Contract 2-file design.
func TestWriteReports_OverwriteNotAppend(t *testing.T) {
	dir := stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	r1, _ := Consolidate()
	if err := WriteReports(r1); err != nil {
		t.Fatal(err)
	}
	appendVersioned(t, "1.1.0", "2026-02-01T00:00:00Z")
	r2, _ := Consolidate()
	if err := WriteReports(r2); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, reportsFileName))
	if err != nil {
		t.Fatal(err)
	}
	// One fingerprint → exactly one non-empty line (overwrite, not append).
	lines := 0
	for _, l := range splitLines(data) {
		if len(l) > 0 {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("reports.jsonl should hold 1 line after re-consolidate (overwrite), got %d", lines)
	}
}
