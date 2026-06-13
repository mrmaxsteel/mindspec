package journal

// reports_test.go — spec 094 Bead 3: the consolidation + regression/stale
// loop + MarkResolved persistence over reports.jsonl.

import (
	"os"
	"path/filepath"
	"testing"
)

// appendVersioned appends one friction event stamped at a chosen version + ts
// (overriding the nowRFC3339 seam so first/last-seen ordering is
// deterministic). It uses the goodEvent identity unless overridden.
func appendVersioned(t *testing.T, ver, ts string) {
	t.Helper()
	orig := nowRFC3339
	nowRFC3339 = func() string { return ts }
	defer func() { nowRFC3339 = orig }()
	ev := goodEvent()
	ev.Version = ver
	if err := AppendSuccessEvent(ev); err != nil {
		t.Fatalf("AppendSuccessEvent(%s): %v", ver, err)
	}
}

// TestConsolidate_CountAndFirstVersion asserts `report` consolidation
// collapses the journal by fingerprint with the correct count, first_version
// (earliest), and last_version (latest) (Req 4).
func TestConsolidate_CountAndFirstVersion(t *testing.T) {
	stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	appendVersioned(t, "1.2.0", "2026-02-01T00:00:00Z")
	appendVersioned(t, "1.1.0", "2026-01-15T00:00:00Z")

	reports, err := Consolidate()
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("want 1 report (one fingerprint), got %d", len(reports))
	}
	r := reports[0]
	if r.Count != 3 {
		t.Errorf("count: want 3, got %d", r.Count)
	}
	if r.FirstVersion != "1.0.0" {
		t.Errorf("first_version: want 1.0.0 (earliest), got %q", r.FirstVersion)
	}
	if r.LastVersion != "1.2.0" {
		t.Errorf("last_version: want 1.2.0 (latest), got %q", r.LastVersion)
	}
	if r.Command != "complete" || r.EscapeHatch != "override-adr" {
		t.Errorf("identity not carried: %+v", r.Identity)
	}
}

// TestConsolidate_Empty asserts no journal → empty consolidation, no error
// (the clean empty case the report command renders as a message).
func TestConsolidate_Empty(t *testing.T) {
	stateDir(t)
	reports, err := Consolidate()
	if err != nil {
		t.Fatalf("Consolidate on empty journal: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("want 0 reports on empty journal, got %d", len(reports))
	}
}

// TestWriteReports_Persists0600 asserts WriteReports writes reports.jsonl in
// the isolated store dir at 0600 (HC-8) and ReadReports round-trips it.
func TestWriteReports_Persists0600(t *testing.T) {
	dir := stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	reports, err := Consolidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteReports(reports); err != nil {
		t.Fatalf("WriteReports: %v", err)
	}
	path := filepath.Join(dir, reportsFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat reports.jsonl: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("reports.jsonl perms: want 0600, got %o", info.Mode().Perm())
	}
	back, err := ReadReports()
	if err != nil || len(back) != 1 {
		t.Fatalf("ReadReports round-trip: want 1 report, got %d (err=%v)", len(back), err)
	}
}

// TestMarkResolved_PersistsAndClassifies is the Req 3 / Req 5 loop: a report
// MarkResolved'd at v2 then re-occurring is REGRESSION at == v2 and > v2, and
// stale at < v2; a dev recurrence is REGRESSION (unbounded-newest, DQ4).
func TestMarkResolved_PersistsAndClassifies(t *testing.T) {
	cases := []struct {
		name        string
		lastVersion string
		resolvedIn  string
		want        Status
	}{
		{"recur at == resolution (≥ boundary)", "2.0.0", "2.0.0", StatusRegression},
		{"recur after resolution", "2.1.0", "2.0.0", StatusRegression},
		{"recur before resolution (stale)", "1.9.0", "2.0.0", StatusStale},
		{"dev recurrence (unbounded-newest)", "dev", "2.0.0", StatusRegression},
		{"never resolved → open", "1.0.0", "", StatusOpen},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateDir(t)
			appendVersioned(t, tc.lastVersion, "2026-01-01T00:00:00Z")

			reports, err := Consolidate()
			if err != nil || len(reports) != 1 {
				t.Fatalf("Consolidate: want 1, got %d (err=%v)", len(reports), err)
			}
			fp := reports[0].Fingerprint

			if tc.resolvedIn != "" {
				if err := MarkResolved(fp, tc.resolvedIn); err != nil {
					t.Fatalf("MarkResolved: %v", err)
				}
				// Re-read from disk to prove the resolution PERSISTED.
				persisted, err := ReadReports()
				if err != nil || len(persisted) != 1 {
					t.Fatalf("ReadReports after resolve: %d (err=%v)", len(persisted), err)
				}
				if persisted[0].ResolvedInVersion != tc.resolvedIn {
					t.Errorf("resolved_in not persisted: want %q, got %q", tc.resolvedIn, persisted[0].ResolvedInVersion)
				}
				if got := persisted[0].Classify(); got != tc.want {
					t.Errorf("status: want %q, got %q", tc.want, got)
				}
			} else {
				if got := reports[0].Classify(); got != tc.want {
					t.Errorf("status: want %q, got %q", tc.want, got)
				}
			}
		})
	}
}

// TestMarkResolved_UnknownFingerprint asserts resolving a fingerprint that was
// never observed is an error (you cannot resolve a report that does not
// exist) and NEVER mutates the append-only journal.
func TestMarkResolved_UnknownFingerprint(t *testing.T) {
	stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	before, _ := ListReports()
	if err := MarkResolved("deadbeef-not-a-real-fingerprint", "2.0.0"); err == nil {
		t.Error("MarkResolved on an unknown fingerprint should error")
	}
	after, _ := ListReports()
	if len(before) != len(after) {
		t.Errorf("MarkResolved mutated the append-only journal: %d → %d", len(before), len(after))
	}
}

// TestMarkResolved_SurvivesReconsolidate asserts a prior resolved_in mark is
// PRESERVED when `report` consolidates again (Consolidate must not erase a
// resolution).
func TestMarkResolved_SurvivesReconsolidate(t *testing.T) {
	stateDir(t)
	appendVersioned(t, "1.0.0", "2026-01-01T00:00:00Z")
	reports, _ := Consolidate()
	fp := reports[0].Fingerprint
	if err := MarkResolved(fp, "2.0.0"); err != nil {
		t.Fatal(err)
	}
	// A new occurrence lands, then `report` re-consolidates.
	appendVersioned(t, "1.5.0", "2026-03-01T00:00:00Z")
	reconsolidated, err := Consolidate()
	if err != nil || len(reconsolidated) != 1 {
		t.Fatalf("re-Consolidate: %d (err=%v)", len(reconsolidated), err)
	}
	if reconsolidated[0].ResolvedInVersion != "2.0.0" {
		t.Errorf("reconsolidation erased the resolution: got %q", reconsolidated[0].ResolvedInVersion)
	}
}

// TestReportsStore_IsolatedFromJournalAndGit asserts reports.jsonl lives in
// the SAME isolated store dir as the journal — never under a git/bd/dolt
// tree (HC-3 / HC-8). The MINDSPEC_STATE_DIR git-tree guard is shared.
func TestReportsStore_IsolatedFromJournalAndGit(t *testing.T) {
	dir := stateDir(t)
	jp, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	rp, err := ReportsPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(jp) != filepath.Dir(rp) {
		t.Errorf("reports + journal must share the isolated store dir: %q vs %q", filepath.Dir(jp), filepath.Dir(rp))
	}
	// Dir() canonicalizes the MINDSPEC_STATE_DIR override (Abs+EvalSymlinks)
	// for the HC-3 git-tree guard, so compare against the same canonical form
	// (on macOS /var → /private/var).
	wantDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		wantDir = dir
	}
	if filepath.Dir(rp) != wantDir {
		t.Errorf("reports store dir: want %q, got %q", wantDir, filepath.Dir(rp))
	}
}
