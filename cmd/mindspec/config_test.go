package main

// config_test.go: tests for `mindspec config show` / renderConfig (spec 109
// Bead 4, R9).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// TestConfigShow_EmitsPanelModelsLoop asserts that renderConfig(DefaultConfig())
// (a pure function — no fs, no process) surfaces the panel reviewers, the raw
// approve_threshold expression, the empty models block, loop.enabled=false,
// runner: claude-code-skills, and the "declared, not yet enforced" annotation
// on each of the three inert blocks (spec 109 AC6).
func TestConfigShow_EmitsPanelModelsLoop(t *testing.T) {
	out, err := renderConfig(config.DefaultConfig())
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}

	// panel reviewers (3+3 default mix) and the raw threshold expression.
	if !strings.Contains(out, "family: claude") || !strings.Contains(out, "count: 3") {
		t.Errorf("expected the claude reviewer entry, got:\n%s", out)
	}
	if !strings.Contains(out, "family: codex") {
		t.Errorf("expected the codex reviewer entry, got:\n%s", out)
	}
	if !strings.Contains(out, "approve_threshold: n-1") {
		t.Errorf("expected the raw approve_threshold expression, got:\n%s", out)
	}

	// models: empty by default, INERT.
	if !strings.Contains(out, "models: {}") {
		t.Errorf("expected an empty models block, got:\n%s", out)
	}

	// loop: enabled=false by default, INERT.
	if !strings.Contains(out, "loop:") || !strings.Contains(out, "enabled: false") {
		t.Errorf("expected loop with enabled: false, got:\n%s", out)
	}

	// runner: default, INERT.
	if !strings.Contains(out, "runner: claude-code-skills") {
		t.Errorf("expected runner: claude-code-skills, got:\n%s", out)
	}

	// The three inert blocks (models/loop/runner) are each annotated; panel
	// (which DOES drive behavior today) is not one of them.
	if got := strings.Count(out, "declared, not yet enforced"); got < 3 {
		t.Errorf("expected the \"declared, not yet enforced\" annotation on all three inert blocks (models/loop/runner), got %d occurrences:\n%s", got, out)
	}
}

// TestConfigShow_ReviewerCountNoteWhenPanelDiffers exercises the full `config
// show` command (through rootCmd) against a repo with a registered panel
// whose recorded expected_reviewers differs from the config default: the
// output contains the caller-side panel.ReviewerCountNote advisory (spec 109
// R8), and the command still exits 0 and mutates no file (read-only, R9).
func TestConfigShow_ReviewerCountNoteWhenPanelDiffers(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	writeConfigShowPanel(t, root, "109-bd04", panel.Panel{
		Spec: "109", Round: 1, ExpectedReviewers: 4,
	})
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"config", "show"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mindspec config show: %v\nstderr=%s", err, stderr.String())
	}
	out := stdout.String() + stderr.String()

	// Default config is 3+3=6 expected reviewers; the panel recorded 4.
	if !strings.Contains(out, "recorded 4") || !strings.Contains(out, "config default is 6") {
		t.Errorf("expected a reviewer-count note (recorded 4 vs default 6), got:\n%s", out)
	}

	// Read-only: mutates no file under root.
	entries, err := os.ReadDir(filepath.Join(root, "review", "109-bd04"))
	if err != nil {
		t.Fatalf("read panel dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != panel.FileName {
		t.Errorf("config show must write nothing beyond the fixture panel.json, got: %v", entries)
	}
}

// TestConfigShow_EscapesHostileControlBytes covers the final-review G2 fix:
// a hostile .mindspec/config.yaml whose panel.reviewers[].family and a
// models map key/value all carry ESC (\x1b), BEL (\x07), and an embedded
// newline followed by text shaped like a forged config key
// ("injected_key: forged_value", G2's own reproduction) must never reach
// `mindspec config show`'s stdout as raw control bytes, and must never
// forge a new display line that looks like a legitimate config entry. The
// hostile value is round-tripped through real YAML (double-quoted hex
// escapes, verified to decode to the exact raw bytes below) and through the
// full `mindspec config show` command — not just the pure renderConfig
// helper — so this exercises the same path G2 drove.
func TestConfigShow_EscapesHostileControlBytes(t *testing.T) {
	hostileFamily := "claude\x1b[31mESC\x07BEL\ninjected_key: forged_value"
	hostileModelsKey := "phase\x1bkey\x07\ninjected_key: forged_map_key"
	hostileModelsValue := "modelname\x1bwith\x07control\ninjected_key: forged_value"

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	hostileYAML := "panel:\n" +
		"  reviewers:\n" +
		"    - family: \"claude\\x1B[31mESC\\x07BEL\\ninjected_key: forged_value\"\n" +
		"      count: 3\n" +
		"models:\n" +
		"  \"phase\\x1Bkey\\x07\\ninjected_key: forged_map_key\": \"modelname\\x1Bwith\\x07control\\ninjected_key: forged_value\"\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(hostileYAML), 0o644); err != nil {
		t.Fatalf("write hostile config.yaml: %v", err)
	}

	// Sanity: confirm the YAML source actually decodes to the exact raw
	// bytes this test targets, so a future change to the fixture text can't
	// silently stop exercising the hostile path.
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load(hostile fixture): %v", err)
	}
	if got := cfg.Panel.Reviewers[0].Family; got != hostileFamily {
		t.Fatalf("fixture family did not decode to the expected raw bytes: got %q, want %q", got, hostileFamily)
	}
	if got, ok := cfg.Models[hostileModelsKey]; !ok || got != hostileModelsValue {
		t.Fatalf("fixture models entry did not decode to the expected raw bytes: got %q (ok=%v), want key %q value %q", got, ok, hostileModelsKey, hostileModelsValue)
	}

	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"config", "show"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mindspec config show: %v\nstderr=%s", err, stderr.String())
	}
	outBytes := stdout.Bytes()
	out := stdout.String()

	// Zero raw control bytes on stdout: ESC and BEL must never appear as
	// literal bytes, only inside an escaped (\x1b / \a) textual form.
	if bytes.IndexByte(outBytes, 0x1b) != -1 {
		t.Errorf("raw ESC byte (0x1b) reached stdout:\n%q", out)
	}
	if bytes.IndexByte(outBytes, 0x07) != -1 {
		t.Errorf("raw BEL byte (0x07) reached stdout:\n%q", out)
	}

	// No forged line: the embedded newline must never produce a physical
	// output line that starts with the attacker's fake key (G2's
	// "injected_key" shape). If the newline were rendered raw, the output
	// would contain a line reading exactly "injected_key: forged_value" (or
	// "...forged_map_key", indented like a real config line).
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "injected_key:") {
			t.Errorf("hostile newline forged a display line starting a fake key: %q\nfull output:\n%s", line, out)
		}
	}

	// The hostile values must still be RENDERED (quoted/escaped), not
	// silently dropped or truncated — assert the exact escaped literal
	// (computed independently via strconv.Quote on the raw bytes) appears
	// verbatim, on one line, in stdout.
	wantFamily := strconv.Quote(hostileFamily)
	if !strings.Contains(out, wantFamily) {
		t.Errorf("expected the escaped family literal %s in stdout, got:\n%s", wantFamily, out)
	}
	wantModelsKey := strconv.Quote(hostileModelsKey)
	wantModelsValue := strconv.Quote(hostileModelsValue)
	if !strings.Contains(out, wantModelsKey) {
		t.Errorf("expected the escaped models key literal %s in stdout, got:\n%s", wantModelsKey, out)
	}
	if !strings.Contains(out, wantModelsValue) {
		t.Errorf("expected the escaped models value literal %s in stdout, got:\n%s", wantModelsValue, out)
	}
}

// writeConfigShowPanel writes root/review/<slug>/panel.json for the
// repo-root review/ convention panel.Scan checks.
func writeConfigShowPanel(t *testing.T, root, slug string, p panel.Panel) {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal panel: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, panel.FileName), data, 0o644); err != nil {
		t.Fatalf("write panel.json: %v", err)
	}
}
