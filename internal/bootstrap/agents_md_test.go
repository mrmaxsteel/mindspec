package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agents_md_test.go — spec 123 R7(a)/(b): init's AGENTS.md content is
// config-sourced, never mindspec-the-framework's own identity/build
// (AC-13's init half; AC-14's FR-3 asymmetry guard — init must be
// config-sourced too, not only setup's).

// TestRun_AgentsMDNoFrameworkLeak_GreenfieldNoConfig pins AC-13's init
// half: a fresh init with NO .mindspec/config.yaml (so Commands is
// unset) produces an AGENTS.md with no mindspec-repo-specific build
// commands or title, and no Build & Test section at all. RED on
// pre-spec-123 main: starterAgentsMD hardcoded "MindSpec Project" and
// `make build`/`make test` unconditionally.
func TestRun_AgentsMDNoFrameworkLeak_GreenfieldNoConfig(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	content := string(data)

	for _, forbidden := range []string{"make build", "make test", "MindSpec Project"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("AGENTS.md leaks framework fact %q:\n%s", forbidden, content)
		}
	}
	if strings.Contains(content, "Build & Test") {
		t.Errorf("AGENTS.md must omit the Build & Test section entirely when commands: is unset:\n%s", content)
	}
	if !strings.HasPrefix(content, "# AGENTS.md\n") {
		t.Errorf("AGENTS.md title must be neutral \"# AGENTS.md\", got:\n%s", content)
	}
}

// TestRun_AgentsMDRendersDeclaredCommands pins AC-14's init half: when
// .mindspec/config.yaml already declares commands: BEFORE init runs (the
// R7(d) "init reads config best-effort" plan choice), the fresh AGENTS.md
// renders the declared Build & Test section instead of omitting it —
// with no mindspec-repo-specific commands anywhere.
func TestRun_AgentsMDRendersDeclaredCommands(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "commands:\n  build: npm run build\n  test: npm test\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "## Build & Test") {
		t.Errorf("AGENTS.md must render the declared Build & Test section:\n%s", content)
	}
	if !strings.Contains(content, "npm run build") || !strings.Contains(content, "npm test") {
		t.Errorf("AGENTS.md must render the CONSUMER's declared commands:\n%s", content)
	}
	for _, forbidden := range []string{"make build", "make test", "MindSpec Project"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("AGENTS.md leaks framework fact %q:\n%s", forbidden, content)
		}
	}
}

// TestRun_AgentsMDAppendPath_RendersDeclaredCommands covers the
// append-block path (an existing AGENTS.md with no MindSpec marker yet):
// renderAppendAgentsBlock is ALSO config-sourced, at the nested (H3)
// heading depth.
func TestRun_AgentsMDAppendPath_RendersDeclaredCommands(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "commands:\n  build: cargo build\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	preexisting := "# My Project\n\nSome existing content.\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(preexisting), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, preexisting) {
		t.Errorf("pre-existing AGENTS.md content must be preserved at the head:\n%s", content)
	}
	if !strings.Contains(content, "### Build & Test") {
		t.Errorf("appended block must render the nested (H3) Build & Test section:\n%s", content)
	}
	if !strings.Contains(content, "cargo build") {
		t.Errorf("appended block must render the declared command:\n%s", content)
	}
	if strings.Contains(content, "make build") || strings.Contains(content, "make test") {
		t.Errorf("appended block must not leak mindspec's own build:\n%s", content)
	}
}

// TestRun_MalformedConfigFailsLoudly pins spec 123 FX-1 (bootstrap half):
// a genuinely corrupt/invalid .mindspec/config.yaml (an unrelated
// malformed key) makes `init` FAIL LOUDLY rather than silently rendering
// AGENTS.md from a DefaultConfig fallback. init is create-or-append-only
// (no data-loss path), but failing loudly keeps its generated content
// honest and consistent with setup + every other mindspec command. The
// ordinary greenfield case (config ABSENT) is unaffected — config.Load
// returns DefaultConfig with a nil error there, proven by the other
// AGENTS.md tests in this file.
func TestRun_MalformedConfigFailsLoudly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	badCfg := "runner: not-a-real-adapter\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(badCfg), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Run(root, false)
	if err == nil {
		t.Fatal("expected init to fail loudly on a malformed config, got nil error")
	}
	if !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("error should name the config as the actionable cause, got: %v", err)
	}
}
