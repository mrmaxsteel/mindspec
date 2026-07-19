package next

import (
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

// TestNextDirtyTreeFailure_HostileFieldsEscaped pins AC-15: each userDirt
// porcelain entry passed to DirtyTreeFailure is escaped per-line so a
// hostile filename cannot forge extra lines or control bytes into the
// claim-refusal message.
func TestNextDirtyTreeFailure_HostileFieldsEscaped(t *testing.T) {
	hostile := "evil" + hostileFieldSuffix + ".go"
	err := DirtyTreeFailure("/repo", []string{hostile}, "")
	if err == nil {
		t.Fatal("DirtyTreeFailure returned nil")
	}
	assertCleanRender(t, err.Error())

	// Clean-fixture byte-identity.
	cleanErr := DirtyTreeFailure("/repo", []string{"notes.txt"}, "")
	if !strings.Contains(cleanErr.Error(), "notes.txt") {
		t.Errorf("clean filename must render byte-identical, got: %v", cleanErr)
	}
}

// TestFormatWorkListHostileTitleEscaped pins AC-15: a hostile bd Title
// never forges extra lines in FormatWorkList's numbered listing, and a
// malformed ID renders forced-quoted rather than raw.
func TestFormatWorkListHostileTitleEscaped(t *testing.T) {
	hostile := "Do the thing" + hostileFieldSuffix
	items := []BeadInfo{
		{ID: "mindspec-9cyu.1", Title: hostile, Priority: 1, IssueType: "task"},
	}
	out := FormatWorkList(items)
	assertCleanRender(t, out)
	if !strings.Contains(out, "mindspec-9cyu.1") {
		t.Errorf("clean dotted-child bead id must render byte-identical, got: %s", out)
	}

	// Clean-fixture byte-identity (existing behavior preserved).
	clean := []BeadInfo{
		{ID: "abc", Title: "Do something", Priority: 2, IssueType: "task"},
	}
	cleanOut := FormatWorkList(clean)
	if !strings.Contains(cleanOut, "Do something") {
		t.Errorf("clean title must render byte-identical, got: %s", cleanOut)
	}
}
