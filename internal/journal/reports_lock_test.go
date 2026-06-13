package journal

// reports_lock_test.go — spec 094 Bead 3 RE-PANEL (rp-codex-consolidation
// #1/#2): the cross-process reports.jsonl read-modify-write must be serialized
// by an OS-visible lock and must UNION (never delete) rows, so:
//
//   - TestReviewerConcurrentDifferentResolves_UnionKeepsBoth — two stale writers
//     resolving DIFFERENT fingerprints both survive (no lost update);
//   - TestReviewerStaleWrite_DropsOnDiskOnlyResolvedFingerprint — a stale
//     snapshot that is missing an on-disk-only (newly resolved) row does NOT
//     collapse it away on rewrite.
//
// RED-before: WriteReports merged resolved-state ONLY onto in-memory-present
// fingerprints and rewrote wholesale under the in-process mutex only, so a
// stale snapshot dropped on-disk-only rows and a cross-process resolve was
// clobbered. GREEN-after: withReportsLock + the on-disk-only carry-forward.

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// startResolveFPProcess LAUNCHES (does not wait on) a genuine separate-process
// MarkResolved(fp, ver) against the SAME store dir. When pauseMS > 0 the child
// PAUSES inside its read-modify-write window (writeRereadHook seam), widening
// the cross-process lost-update window so the OS lock is what makes the outcome
// correct. The returned func waits for completion.
func startResolveFPProcess(t *testing.T, dir, fp, ver string, pauseMS int) func() {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run", "TestResolveHelperProcess")
	env := append(os.Environ(),
		"MINDSPEC_TEST_RESOLVE_HELPER=1",
		StateDirEnv+"="+dir,
		"MINDSPEC_TEST_RESOLVE_FP="+fp,
		"MINDSPEC_TEST_RESOLVE_VER="+ver,
	)
	if pauseMS > 0 {
		env = append(env, "MINDSPEC_TEST_RESOLVE_PAUSE_MS="+itoaTest(pauseMS))
	}
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatalf("start resolve helper (fp=%s): %v", fp, err)
	}
	return func() {
		if err := cmd.Wait(); err != nil {
			t.Errorf("resolve helper (fp=%s) failed: %v", fp, err)
		}
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestReviewerConcurrentDifferentResolves_UnionKeepsBoth is the
// rp-codex-consolidation #1 repro: two SEPARATE PROCESSES each resolve a
// DIFFERENT fingerprint concurrently. Both resolutions must survive — neither
// process may read a stale snapshot and clobber the other's resolve.
//
// The lost-update is a narrow cross-process read→rename race. We make it
// DETERMINISTIC: writer-1's process PAUSES inside its read-modify-write window
// (after the on-disk re-read, before the rename — the writeRereadHook seam),
// while writer-2's process runs without pause. With the OS lock
// (withReportsLock), writer-2 BLOCKS at lock acquisition until writer-1's whole
// RMW completes, then unions writer-1's resolve and writes both → GREEN.
// Without the lock (the pre-fix in-process-mutex-only code), writer-2 races in
// during the pause; both write, and the later rename overwrites the earlier
// resolve — one is LOST (the RED the reviewer's harness demonstrated).
func TestReviewerConcurrentDifferentResolves_UnionKeepsBoth(t *testing.T) {
	dir := stateDir(t)

	// Seed TWO distinct identities → two fingerprints, consolidated to disk.
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
		t.Fatalf("want 2 reports, got %d", len(reports))
	}
	if err := WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	var fpA, fpB string
	for _, r := range reports {
		if r.EscapeHatch == "override-adr" {
			fpA = r.Fingerprint
		} else {
			fpB = r.Fingerprint
		}
	}

	// writer-1 pauses 200ms inside its RMW window; writer-2 (started ~30ms
	// later, no pause) tries to write during that pause.
	w1 := startResolveFPProcess(t, dir, fpA, "2.0.0", 200)
	time.Sleep(30 * time.Millisecond)
	w2 := startResolveFPProcess(t, dir, fpB, "3.0.0", 0)
	w1()
	w2()

	persisted, err := ReadReports()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range persisted {
		got[r.Fingerprint] = r.ResolvedInVersion
	}
	if got[fpA] != "2.0.0" {
		t.Errorf("fpA resolve lost (lost update): resolved_in_version=%q, want 2.0.0 (full map: %v)", got[fpA], got)
	}
	if got[fpB] != "3.0.0" {
		t.Errorf("fpB resolve lost (lost update): resolved_in_version=%q, want 3.0.0 (full map: %v)", got[fpB], got)
	}
}

// TestReviewerStaleWrite_DropsOnDiskOnlyResolvedFingerprint is the
// rp-codex-consolidation #2 repro: three reports are on disk (one freshly
// resolved). A stale writer holds a TWO-row snapshot (taken before the third
// row existed) and writes it back. The on-disk-only third row must be CARRIED
// FORWARD, not collapsed away.
//
// RED-before: WriteReports iterated only the incoming slice, so the on-disk-only
// row vanished (3 rows → 2).
func TestReviewerStaleWrite_DropsOnDiskOnlyResolvedFingerprint(t *testing.T) {
	stateDir(t)

	// Three distinct identities → three fingerprints.
	a := goodEvent() // complete / override-adr
	b := goodEvent()
	b.EscapeHatch = "allow-doc-skew"
	c := goodEvent()
	c.EscapeHatch = "supersede-adr"
	appendVersionedIdentity(t, a, "1.0.0", "2026-01-01T00:00:00Z")
	appendVersionedIdentity(t, b, "1.0.0", "2026-01-02T00:00:00Z")
	appendVersionedIdentity(t, c, "1.0.0", "2026-01-03T00:00:00Z")

	all, err := Consolidate()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 reports, got %d", len(all))
	}

	// A STALE writer's snapshot has only the FIRST TWO rows (the third was added
	// to the store after this writer read). Capture it BEFORE the third lands.
	stale := []Report{all[0], all[1]}

	// Meanwhile the third fingerprint is written + resolved on disk (a separate
	// process consolidated all three then resolved the third).
	if err := WriteReports(all); err != nil {
		t.Fatal(err)
	}
	fpC := all[2].Fingerprint
	if err := MarkResolved(fpC, "2.0.0"); err != nil {
		t.Fatal(err)
	}

	// Now the STALE 2-row writer rewrites. The on-disk-only resolved third row
	// must NOT be collapsed away.
	if err := WriteReports(stale); err != nil {
		t.Fatal(err)
	}

	after, err := ReadReports()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 3 {
		t.Fatalf("stale write COLLAPSED an on-disk-only row: want 3 reports, got %d", len(after))
	}
	found := false
	for _, r := range after {
		if r.Fingerprint == fpC {
			found = true
			if r.ResolvedInVersion != "2.0.0" {
				t.Errorf("on-disk-only row's resolution lost: resolved_in_version=%q, want 2.0.0", r.ResolvedInVersion)
			}
		}
	}
	if !found {
		t.Errorf("on-disk-only resolved fingerprint %s was DELETED by the stale write", fpC)
	}
}
