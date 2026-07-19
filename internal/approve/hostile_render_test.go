package approve

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
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

// TestApproveHostileRendersEscaped pins AC-15/16/17: the open-child hint
// (formatOpenChildHint) and the panel advisory slot line
// (formatAdvisorySlotLine) both escape agent-writable content and route
// ID-typed positions through idrender, with clean-fixture byte-identity
// preserved. (The per-line IsTreeClean porcelain chain plan.go/spec.go
// wrap via %w is proven at its actual escaping site,
// internal/executor's IsTreeClean and TestConflictFailureBodiesEscapedPerLine
// — approve adds no further transformation of that error, so this is the
// doubly-held pattern used elsewhere in this spec.)
func TestApproveHostileRendersEscaped(t *testing.T) {
	t.Run("formatOpenChildHint hostile title and malformed id", func(t *testing.T) {
		hostile := []phase.ChildInfo{
			{ID: "120-x;evil", Title: "do a thing" + hostileFieldSuffix},
		}
		out := formatOpenChildHint("120-x;evil", hostile)
		assertCleanRender(t, out)
		if !strings.Contains(out, strconv.Quote("120-x;evil")) {
			t.Errorf("expected the malformed id forced-quoted, got: %q", out)
		}

		clean := []phase.ChildInfo{
			{ID: "mindspec-9cyu.1", Title: "Do something"},
		}
		cleanOut := formatOpenChildHint("008b-human-gates", clean)
		if !strings.Contains(cleanOut, "mindspec-9cyu.1 (Do something)") {
			t.Errorf("clean child hint must render byte-identical, got: %q", cleanOut)
		}
		if !strings.Contains(cleanOut, "008b-human-gates") || strings.Contains(cleanOut, strconv.Quote("008b-human-gates")) {
			t.Errorf("clean spec id must render byte-identical (unquoted), got: %q", cleanOut)
		}
	})

	t.Run("formatAdvisorySlotLine hostile slot and malformed bead id", func(t *testing.T) {
		unresolved := []panel.Verdict{{Slot: "R1" + hostileFieldSuffix}}
		out := formatAdvisorySlotLine("120-x;evil", unresolved)
		assertCleanRender(t, out)
		if !strings.Contains(out, strconv.Quote("120-x;evil")) {
			t.Errorf("expected the malformed bead id forced-quoted, got: %q", out)
		}

		cleanOut := formatAdvisorySlotLine("mindspec-9cyu.1", []panel.Verdict{{Slot: "R1"}})
		if !strings.Contains(cleanOut, "mindspec-9cyu.1") || strings.Contains(cleanOut, strconv.Quote("mindspec-9cyu.1")) {
			t.Errorf("clean bead id must render byte-identical (unquoted), got: %q", cleanOut)
		}
		if !strings.Contains(cleanOut, "R1") {
			t.Errorf("clean slot must render byte-identical, got: %q", cleanOut)
		}
	})
}
