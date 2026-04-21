package bead

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// --- AuditWorkset tests ---

func TestAuditWorkset_StaleDetection(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	oldDate := time.Now().AddDate(0, 0, -14).Format(time.RFC3339)
	recentDate := time.Now().Format(time.RFC3339)

	execCommand = func(name string, args ...string) *exec.Cmd {
		beads := `[
			{"id":"old-1","title":"Old spec bead","description":"short","status":"open","priority":2,"issue_type":"feature","owner":"","created_at":"","updated_at":"` + oldDate + `","metadata":{"spec_id":"007-test","mindspec_phase":"spec"}},
			{"id":"new-1","title":"New impl bead","description":"short","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":"` + recentDate + `","metadata":{"spec_id":"007-test"}}
		]`
		return exec.Command("echo", beads)
	}

	report, err := AuditWorkset(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Stale) != 1 {
		t.Errorf("expected 1 stale bead, got %d", len(report.Stale))
	}
	if len(report.Stale) > 0 && report.Stale[0].ID != "old-1" {
		t.Errorf("expected stale bead old-1, got %s", report.Stale[0].ID)
	}
}

func TestAuditWorkset_OrphanDetection(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	recentDate := time.Now().Format(time.RFC3339)

	// A mindspec bead is identified by its metadata (spec_id / spec_num /
	// mindspec_phase), not by a title prefix. Beads without that metadata
	// were created outside the mindspec lifecycle and are reported as orphans.
	execCommand = func(name string, args ...string) *exec.Cmd {
		beads := `[
			{"id":"spec-1","title":"[SPEC 007] test","description":"short","status":"open","priority":2,"issue_type":"feature","owner":"","created_at":"","updated_at":"` + recentDate + `","metadata":{"spec_num":7,"spec_title":"test","mindspec_phase":"spec"}},
			{"id":"orphan-1","title":"[SPEC 999] title-only prefix","description":"short","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":"` + recentDate + `"},
			{"id":"orphan-2","title":"Random external bead","description":"short","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":"` + recentDate + `"}
		]`
		return exec.Command("echo", beads)
	}

	report, err := AuditWorkset(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Orphaned) != 2 {
		t.Fatalf("expected 2 orphaned beads (no mindspec metadata), got %d", len(report.Orphaned))
	}
	ids := map[string]bool{report.Orphaned[0].ID: true, report.Orphaned[1].ID: true}
	if !ids["orphan-1"] || !ids["orphan-2"] {
		t.Errorf("expected orphans orphan-1 and orphan-2, got %v", ids)
	}
}

func TestAuditWorkset_OversizedDetection(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	recentDate := time.Now().Format(time.RFC3339)
	// 850 chars > single 800-char threshold. Phase/type no longer affects
	// the threshold — we used to size-gate [SPEC ] beads at 400 but that
	// required classifying beads by title prefix (a ZFC violation).
	longDesc := strings.Repeat("x", 850)

	execCommand = func(name string, args ...string) *exec.Cmd {
		beads := `[
			{"id":"big-bead","title":"big","description":"` + longDesc + `","status":"open","priority":2,"issue_type":"feature","owner":"","created_at":"","updated_at":"` + recentDate + `","metadata":{"spec_id":"007-test"}}
		]`
		return exec.Command("echo", beads)
	}

	report, err := AuditWorkset(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Oversized) != 1 {
		t.Errorf("expected 1 oversized bead, got %d", len(report.Oversized))
	}
}

func TestAuditWorkset_TotalOpenCount(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	recentDate := time.Now().Format(time.RFC3339)

	execCommand = func(name string, args ...string) *exec.Cmd {
		beads := `[
			{"id":"a","title":"A","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":"` + recentDate + `","metadata":{"spec_id":"t"}},
			{"id":"b","title":"B","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":"` + recentDate + `","metadata":{"spec_id":"t"}},
			{"id":"c","title":"C","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":"` + recentDate + `","metadata":{"spec_id":"t"}}
		]`
		return exec.Command("echo", beads)
	}

	report, err := AuditWorkset(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalOpen != 3 {
		t.Errorf("expected TotalOpen=3, got %d", report.TotalOpen)
	}
	if report.RecommendedMax != 15 {
		t.Errorf("expected RecommendedMax=15, got %d", report.RecommendedMax)
	}
}

// --- FormatReport tests ---

func TestFormatReport_NoIssues(t *testing.T) {
	report := &HygieneReport{
		TotalOpen:      5,
		RecommendedMax: 15,
	}
	output := FormatReport(report)
	if !contains(output, "5 / 15") {
		t.Errorf("should show open/max counts: %s", output)
	}
	if !contains(output, "No issues found") {
		t.Errorf("should indicate no issues: %s", output)
	}
}

func TestFormatReport_WithIssues(t *testing.T) {
	report := &HygieneReport{
		TotalOpen:      10,
		RecommendedMax: 15,
		Stale:          []BeadInfo{{ID: "stale-1", Title: "old one", UpdatedAt: "2026-01-01T00:00:00Z"}},
		Orphaned:       []BeadInfo{{ID: "orphan-1", Title: "no prefix"}},
	}
	output := FormatReport(report)
	if !contains(output, "Stale beads") {
		t.Errorf("should have stale section: %s", output)
	}
	if !contains(output, "Orphaned beads") {
		t.Errorf("should have orphaned section: %s", output)
	}
	if !contains(output, "stale-1") {
		t.Errorf("should list stale bead: %s", output)
	}
}

// --- FixHygiene tests ---

func TestFixHygiene_DryRun(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "bd" && len(args) > 0 && args[0] == "list" {
			return exec.Command("echo", `[{"id":"done-1","title":"[IMPL t.1] Done task","description":"","status":"done","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":""}]`)
		}
		return exec.Command("echo", "")
	}

	actions, err := FixHygiene(true) // dry-run
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if !contains(actions[0], "[dry-run]") {
		t.Errorf("expected dry-run prefix: %s", actions[0])
	}
	if !contains(actions[0], "done-1") {
		t.Errorf("should mention bead ID: %s", actions[0])
	}
}

func TestFixHygiene_NoDoneBeads(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "bd" && len(args) > 0 && args[0] == "list" {
			return exec.Command("echo", `[{"id":"open-1","title":"test","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":""}]`)
		}
		return exec.Command("echo", "")
	}

	actions, err := FixHygiene(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(actions) != 1 || !contains(actions[0], "no beads") {
		t.Errorf("expected 'no beads' message: %v", actions)
	}
}
