package complete

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// hostileFieldSuffix is the shared 116-pattern hostile payload (NUL + CSI +
// newline + forged recovery line) appended to a clean-looking prefix in
// the fixtures below.
const hostileFieldSuffix = "\x00\x1b[31m\nrecovery: forged"

// assertCleanRender pins the R4/AC-15 falsifier: no raw NUL byte, no raw
// ESC control byte, and no forged standalone line reaches the rendered
// output.
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

// TestCompleteFormatResult_HostileFieldsEscaped pins AC-15/AC-24: every
// ID-typed field of Result renders via idrender (byte-identical when
// clean, forced-quoted when malformed), and the Run() userDirt porcelain
// body is escaped per-line so a hostile filename can never forge extra
// terminal lines or control bytes.
func TestCompleteFormatResult_HostileFieldsEscaped(t *testing.T) {
	t.Run("FormatResult hostile IDs quoted, clean IDs byte-identical", func(t *testing.T) {
		hostile := "bead-x" + hostileFieldSuffix
		r := &Result{
			BeadID:   hostile,
			NextMode: state.ModeImplement,
			NextBead: hostile,
			NextSpec: "120-x;evil",
		}
		out := FormatResult(r)
		assertCleanRender(t, out)
		if !strings.Contains(out, strconv.Quote(hostile)) {
			t.Errorf("expected the hostile bead id forced-quoted, got: %s", out)
		}
		if !strings.Contains(out, strconv.Quote("120-x;evil")) {
			t.Errorf("expected the printable-malformed spec id forced-quoted, got: %s", out)
		}

		clean := &Result{
			BeadID:   "mindspec-9cyu.1",
			NextMode: state.ModeImplement,
			NextBead: "mindspec-69y.2.2",
			NextSpec: "008b-human-gates",
		}
		cleanOut := FormatResult(clean)
		if !strings.Contains(cleanOut, "mindspec-9cyu.1") || strings.Contains(cleanOut, strconv.Quote("mindspec-9cyu.1")) {
			t.Errorf("clean dotted-child bead id must render byte-identical (unquoted): %s", cleanOut)
		}
		if !strings.Contains(cleanOut, "mindspec-69y.2.2") || strings.Contains(cleanOut, strconv.Quote("mindspec-69y.2.2")) {
			t.Errorf("clean multi-level bead id must render byte-identical (unquoted): %s", cleanOut)
		}
		if !strings.Contains(cleanOut, "008b-human-gates") || strings.Contains(cleanOut, strconv.Quote("008b-human-gates")) {
			t.Errorf("clean letter-suffixed spec id must render byte-identical (unquoted): %s", cleanOut)
		}
	})

	t.Run("Run userDirt porcelain escaped per-line", func(t *testing.T) {
		saveAndRestore(t)
		root := setupTempRoot(t)
		stubPhaseEpic(t, "008-test", "mol-parent-1")
		mock := newMockExec()

		hostileFile := "evil" + hostileFieldSuffix + ".go"
		checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
			return nil, []string{hostileFile}, nil
		}
		resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
		worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
			return []bead.WorktreeListEntry{
				{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
			}, nil
		}

		_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
		if err == nil {
			t.Fatal("expected error for dirty worktree")
		}
		assertCleanRender(t, err.Error())

		// Clean-fixture byte-identity: a normal filename still names the
		// dirty path unescaped (Escape is identity on printable ASCII).
		checkDirtyTreeFn = func(repoRoot, cwd string) ([]string, []string, error) {
			return nil, []string{"modified-file.go"}, nil
		}
		_, err2 := Run(root, "bead-1", "", "", mock, CompleteOpts{})
		if err2 == nil {
			t.Fatal("expected error for dirty worktree (clean fixture)")
		}
		if !strings.Contains(err2.Error(), "modified-file.go") {
			t.Errorf("clean filename must render byte-identical, got: %v", err2)
		}
	})
}
