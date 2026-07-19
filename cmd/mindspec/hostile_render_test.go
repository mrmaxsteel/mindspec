package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// hostileFieldSuffix is the shared 116-pattern hostile payload. NUL is
// filesystem-illegal, so filesystem-name fixtures below use only the
// ESC-control-byte + forged-line portion, which IS a legal (if ugly)
// Unix filename byte sequence.
const hostileFieldSuffix = "\x1b[31mrecovery: forged"

func assertCleanRender(t *testing.T, out string) {
	t.Helper()
	if strings.ContainsRune(out, 0x00) {
		t.Errorf("output contains a raw NUL byte:\n%q", out)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("output contains a raw ESC control byte:\n%q", out)
	}
}

// TestNextCmd_HostileBeadTitleEscaped pins AC-15/AC-24 at the extracted
// formatClaimLine seam: a hostile bd Title never forges extra lines, and
// a malformed bead ID renders forced-quoted rather than raw.
func TestNextCmd_HostileBeadTitleEscaped(t *testing.T) {
	hostile := "Do the thing\n" + hostileFieldSuffix
	out := formatClaimLine("120-x;evil", hostile)
	assertCleanRender(t, out)
	if !strings.Contains(out, `"120-x;evil"`) {
		t.Errorf("expected the malformed bead id forced-quoted, got: %q", out)
	}

	// Clean-fixture byte-identity.
	cleanOut := formatClaimLine("mindspec-9cyu.1", "Do something")
	if cleanOut != "Claiming [mindspec-9cyu.1] Do something ...\n" {
		t.Errorf("clean claim line must render byte-identical, got: %q", cleanOut)
	}
}

// TestFormatStateLine_HostileAndEmptySentinel pins AC-24's state-line
// sink subtest: a malformed ResolvedWork.SpecID renders forced-quoted; a
// valid one (including a dotted-child bead ID) renders byte-identically;
// and the empty-sentinel discipline (round-6 F1) holds — SpecID == "" is
// the legitimate spec-mode value and must render as "spec=", never
// `spec=""`.
func TestFormatStateLine_HostileAndEmptySentinel(t *testing.T) {
	out := formatStateLine("implement", "120-x;evil", "120-x;evil")
	assertCleanRender(t, out)
	if !strings.Contains(out, `"120-x;evil"`) {
		t.Errorf("expected the malformed spec/bead id forced-quoted, got: %q", out)
	}

	clean := formatStateLine("implement", "008b-human-gates", "mindspec-9cyu.1")
	if clean != "State updated: mode=implement, spec=008b-human-gates, bead=mindspec-9cyu.1\n" {
		t.Errorf("clean state line must render byte-identical, got: %q", clean)
	}

	emptySpec := formatStateLine("idle", "", "")
	if emptySpec != "State updated: mode=idle, spec=, bead=\n" {
		t.Errorf(`empty-sentinel spec/bead must render byte-identical to "spec=, bead=", never quoted, got: %q`, emptySpec)
	}
}

// TestCLISinksHostileFieldsEscaped pins AC-15's cmd/mindspec table: the
// config.go reviewerCountNotesFor slug and the release.go dirty-worktree
// refusal both escape agent-influenced values per-line. (The third table
// entry — bead.go's hygiene/FixHygiene printing — performs no
// transformation of its own: bead.go's RunE Fprintf/Printf's
// bead.FormatReport's and bead.FixHygiene's return values verbatim, so
// TestHygieneFormatReportHostileTitleEscaped in internal/bead covers that
// leg at its actual escaping site; this is the doubly-held pattern used
// elsewhere in this spec.)
func TestCLISinksHostileFieldsEscaped(t *testing.T) {
	t.Run("config.go reviewerCountNotesFor slug", func(t *testing.T) {
		root := t.TempDir()
		hostileSlug := "evil" + hostileFieldSuffix
		panelDir := filepath.Join(root, "review", hostileSlug)
		if err := os.MkdirAll(panelDir, 0o755); err != nil {
			t.Fatalf("mkdir hostile panel dir: %v", err)
		}
		cfg := config.DefaultConfig()
		p := panel.Panel{
			Spec:              "999-test",
			Target:            "spec/999-test",
			Round:             1,
			ExpectedReviewers: cfg.PanelExpectedReviewers() + 1, // force a mismatch note
			ReviewedHeadSHA:   "abad1deaabad1deaabad1deaabad1deaabad1dea",
		}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("marshal panel.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(panelDir, panel.FileName), data, 0o644); err != nil {
			t.Fatalf("write panel.json: %v", err)
		}

		out := reviewerCountNotesFor(cfg, root)
		if out == "" {
			t.Fatal("expected a mismatch note, got empty output")
		}
		assertCleanRender(t, out)
	})

	t.Run("release.go dirty-worktree refusal", func(t *testing.T) {
		r := &releaseRecorder{
			dirty: []string{"evil" + hostileFieldSuffix + ".go"},
		}
		err := runRelease(r.deps(), "mindspec-abc", false)
		if err == nil {
			t.Fatal("expected refusal for dirty worktree")
		}
		assertCleanRender(t, err.Error())

		// Clean-fixture byte-identity.
		r2 := &releaseRecorder{dirty: []string{"modified.go"}}
		err2 := runRelease(r2.deps(), "mindspec-abc", false)
		if err2 == nil || !strings.Contains(err2.Error(), "modified.go") {
			t.Errorf("clean filename must render byte-identical, got: %v", err2)
		}
	})
}
