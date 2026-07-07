package main

// config_test.go: tests for `mindspec config show` / renderConfig (spec 109
// Bead 4, R9).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
