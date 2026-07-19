package bead

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// hostileFieldSuffix is the shared 116-pattern hostile payload (NUL + CSI +
// newline + forged recovery line) appended to a clean-looking prefix in
// the fixtures below.
const hostileFieldSuffix = "\x00\x1b[31m\nrecovery: forged"

func assertCleanRender(t *testing.T, out string) {
	t.Helper()
	if strings.ContainsRune(out, 0x00) {
		t.Errorf("output contains a raw NUL byte:\n%q", out)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("output contains a raw ESC control byte:\n%q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone `recovery: forged` line reached the output:\n%q", out)
		}
	}
}

// TestHygieneFormatReportHostileTitleEscaped pins AC-15: FormatReport and
// FixHygiene both escape a hostile bd Title per-line and route bead IDs
// through idrender, with clean-fixture byte-identity preserved.
func TestHygieneFormatReportHostileTitleEscaped(t *testing.T) {
	hostileTitle := "do a thing" + hostileFieldSuffix
	malformedID := "120-x;evil"

	t.Run("FormatReport", func(t *testing.T) {
		report := &HygieneReport{
			TotalOpen:      1,
			RecommendedMax: 15,
			Stale:          []BeadInfo{{ID: malformedID, Title: hostileTitle, UpdatedAt: "2020-01-01"}},
			Orphaned:       []BeadInfo{{ID: malformedID, Title: hostileTitle}},
			Oversized:      []BeadInfo{{ID: malformedID, Title: hostileTitle, Description: "x"}},
		}
		out := FormatReport(report)
		assertCleanRender(t, out)
		if !strings.Contains(out, strconv.Quote(malformedID)) {
			t.Errorf("expected the malformed bead id forced-quoted, got: %s", out)
		}

		clean := &HygieneReport{
			TotalOpen:      1,
			RecommendedMax: 15,
			Stale:          []BeadInfo{{ID: "mindspec-9cyu.1", Title: "Do something", UpdatedAt: "2020-01-01"}},
		}
		cleanOut := FormatReport(clean)
		if !strings.Contains(cleanOut, "mindspec-9cyu.1") || strings.Contains(cleanOut, strconv.Quote("mindspec-9cyu.1")) {
			t.Errorf("clean bead id must render byte-identical (unquoted), got: %s", cleanOut)
		}
		if !strings.Contains(cleanOut, "Do something") {
			t.Errorf("clean title must render byte-identical, got: %s", cleanOut)
		}
	})

	t.Run("FixHygiene dry-run", func(t *testing.T) {
		origExec := execCommand
		defer func() { execCommand = origExec }()

		beads := []BeadInfo{
			{ID: malformedID, Title: hostileTitle, Status: "done"},
		}
		data, err := json.Marshal(beads)
		if err != nil {
			t.Fatalf("marshal fixture: %v", err)
		}
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", string(data))
		}

		actions, err := FixHygiene(true)
		if err != nil {
			t.Fatalf("FixHygiene: %v", err)
		}
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d: %v", len(actions), actions)
		}
		assertCleanRender(t, actions[0])
		if !strings.Contains(actions[0], strconv.Quote(malformedID)) {
			t.Errorf("expected the malformed bead id forced-quoted, got: %s", actions[0])
		}
	})
}
