package brownfield

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestResolveLLMConfig_ExplicitOff(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	cfg := ResolveLLMConfig()
	if cfg.Provider != "off" {
		t.Fatalf("provider = %q, want off", cfg.Provider)
	}
	if cfg.Model != "default" {
		t.Fatalf("model = %q, want default", cfg.Model)
	}
	if cfg.Available {
		t.Fatal("expected unavailable provider")
	}
}

func TestResolveLLMConfig_AutoDetectClaudeCLI(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "")
	t.Setenv("MINDSPEC_LLM_MODEL", "claude-sonnet")

	oldProbe := claudeProbeFn
	claudeProbeFn = func(model string) bool {
		return model == "claude-sonnet"
	}
	defer func() { claudeProbeFn = oldProbe }()

	cfg := ResolveLLMConfig()
	if cfg.Provider != "claude-cli" {
		t.Fatalf("provider = %q, want claude-cli", cfg.Provider)
	}
	if cfg.Model != "claude-sonnet" {
		t.Fatalf("model = %q, want claude-sonnet", cfg.Model)
	}
	if !cfg.Available {
		t.Fatal("expected available provider")
	}
}

func TestResolveLLMConfig_AutoDetectUnavailable(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	oldProbe := claudeProbeFn
	claudeProbeFn = func(model string) bool { return false }
	defer func() { claudeProbeFn = oldProbe }()

	cfg := ResolveLLMConfig()
	if cfg.Provider != "off" {
		t.Fatalf("provider = %q, want off", cfg.Provider)
	}
	if cfg.Model != "default" {
		t.Fatalf("model = %q, want default", cfg.Model)
	}
	if cfg.Available {
		t.Fatal("expected unavailable provider")
	}
}

func TestResolveLLMConfig_ExplicitProviderSkipsProbe(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "openai")
	t.Setenv("MINDSPEC_LLM_MODEL", "gpt-test")

	oldProbe := claudeProbeFn
	called := false
	claudeProbeFn = func(model string) bool {
		called = true
		return false
	}
	defer func() { claudeProbeFn = oldProbe }()

	cfg := ResolveLLMConfig()
	if called {
		t.Fatal("expected explicit provider to skip claude probe")
	}
	if cfg.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", cfg.Provider)
	}
	if cfg.Model != "gpt-test" {
		t.Fatalf("model = %q, want gpt-test", cfg.Model)
	}
	if !cfg.Available {
		t.Fatal("expected explicit provider to be marked available")
	}
}

func TestDiscoverMarkdown_DeterministicAndFiltered(t *testing.T) {
	root := t.TempDir()

	mk := func(rel string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte("# test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mk("README.md")
	mk("docs/a.md")
	mk("docs/z.MD")
	mk("notes/todo.txt")
	mk(".git/ignored.md")
	mk(".beads/internal.md")
	mk("beads/README.md")
	mk("worktree-demo/README.md")
	mk("nested-repo/README.md")
	mk(".claude/commands/spec-init.md")
	mk("internal/instruct/templates/spec.md")
	if err := os.WriteFile(filepath.Join(root, "nested-repo", ".git"), []byte("gitdir: /tmp/nested\n"), 0o644); err != nil {
		t.Fatalf("write nested repo .git marker: %v", err)
	}

	got, err := DiscoverMarkdown(root)
	if err != nil {
		t.Fatalf("DiscoverMarkdown: %v", err)
	}

	want := []string{
		"README.md",
		"docs/a.md",
		"docs/z.MD",
	}
	if !reflect.DeepEqual(got.MarkdownFiles, want) {
		t.Fatalf("markdown files mismatch\ngot:  %#v\nwant: %#v", got.MarkdownFiles, want)
	}
}

func TestDiscoverMarkdown_RespectsGitIgnore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, string(out))
	}

	mk := func(rel string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte("# test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".venv/\nignored.md\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	mk(".venv/docs/hidden.md")
	mk("ignored.md")
	mk("docs/visible.md")

	got, err := DiscoverMarkdown(root)
	if err != nil {
		t.Fatalf("DiscoverMarkdown: %v", err)
	}

	want := []string{"docs/visible.md"}
	if !reflect.DeepEqual(got.MarkdownFiles, want) {
		t.Fatalf("markdown files mismatch\ngot:  %#v\nwant: %#v", got.MarkdownFiles, want)
	}
}

func TestRun_ReportArtifactsAreDeterministic(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "adr"), 0o755); err != nil {
		t.Fatalf("mkdir docs/adr: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "adr", "ADR-0001.md"), []byte("# adr\n"), 0o644); err != nil {
		t.Fatalf("write adr file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	first, err := Run(root, RunOptions{RunID: "run-a"})
	if err == nil {
		// dry-run should succeed
	} else {
		t.Fatalf("run first: %v", err)
	}
	second, err := Run(root, RunOptions{RunID: "run-b"})
	if err != nil {
		t.Fatalf("run second: %v", err)
	}

	if !reflect.DeepEqual(first.Inventory, second.Inventory) {
		t.Fatalf("inventory mismatch across runs")
	}
	if !reflect.DeepEqual(first.Classification, second.Classification) {
		t.Fatalf("classification mismatch across runs")
	}

	for _, runID := range []string{"run-a", "run-b"} {
		for _, name := range []string{"inventory.json", "classification.json", "plan.json", "plan.md", "state.json"} {
			if _, err := os.Stat(filepath.Join(root, ".mindspec", "migrations", runID, name)); err != nil {
				t.Fatalf("expected artifact %s for %s: %v", name, runID, err)
			}
		}
	}
}

func TestRun_PlanUsesLLMClassificationWhenAvailable(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "claude-cli")
	t.Setenv("MINDSPEC_LLM_MODEL", "claude-sonnet")

	oldClassify := llmClassifyFn
	llmClassifyFn = func(root string, report *Report) ([]ClassificationEntry, error) {
		out := make([]ClassificationEntry, len(report.Classification))
		copy(out, report.Classification)
		for i := range out {
			if !out[i].RequiresLLM {
				continue
			}
			out[i].Category = "user-docs"
			out[i].Confidence = 0.91
			out[i].Rule = "llm:test"
			out[i].Rationale = "Document is operational guidance for users."
			out[i].RequiresLLM = false
		}
		return out, nil
	}
	defer func() { llmClassifyFn = oldClassify }()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "misc"), 0o755); err != nil {
		t.Fatalf("mkdir misc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "misc", "notes.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	report, err := Run(root, RunOptions{RunID: "run-llm"})
	if err != nil {
		t.Fatalf("plan run failed: %v", err)
	}
	if len(report.Unresolved) != 0 {
		t.Fatalf("expected unresolved to be cleared by LLM classification, got %d", len(report.Unresolved))
	}

	found := false
	for _, c := range report.Classification {
		if c.Path == "misc/notes.md" {
			found = true
			if c.Rule != "llm:test" {
				t.Fatalf("expected llm rule for misc/notes.md, got %q", c.Rule)
			}
			if c.Rationale == "" {
				t.Fatal("expected llm rationale for misc/notes.md")
			}
			if c.RequiresLLM {
				t.Fatal("expected llm classified entry to clear RequiresLLM")
			}
		}
	}
	if !found {
		t.Fatal("expected classified entry for misc/notes.md")
	}
}

func TestRun_PlanFailsWhenLLMClassifierErrors(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "claude-cli")
	t.Setenv("MINDSPEC_LLM_MODEL", "claude-sonnet")

	oldClassify := llmClassifyFn
	llmClassifyFn = func(root string, report *Report) ([]ClassificationEntry, error) {
		return nil, fmt.Errorf("simulated classifier failure")
	}
	defer func() { llmClassifyFn = oldClassify }()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "misc"), 0o755); err != nil {
		t.Fatalf("mkdir misc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "misc", "notes.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	_, err := Run(root, RunOptions{RunID: "run-llm-fail"})
	if err == nil {
		t.Fatal("expected plan failure when llm classifier errors")
	}
	if !strings.Contains(err.Error(), "migrate plan LLM classification failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_ApplyFailsOnSourceDrift(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mk("docs/core/ARCHITECTURE.md", "# v1\n")
	if _, err := Run(root, RunOptions{RunID: "drift-run"}); err != nil {
		t.Fatalf("plan run: %v", err)
	}

	mk("docs/core/ARCHITECTURE.md", "# v2\n")
	_, err := Run(root, RunOptions{
		Apply:       true,
		ArchiveMode: "copy",
		RunID:       "drift-run",
		Resume:      true,
	})
	if err == nil {
		t.Fatal("expected drift failure")
	}
	if !strings.Contains(err.Error(), "source drift detected") {
		t.Fatalf("unexpected drift error: %v", err)
	}
}

func TestRun_ApplyFailsWithoutLLMWhenUnresolvedExists(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "misc"), 0o755); err != nil {
		t.Fatalf("mkdir misc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "misc", "notes.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	report, err := Run(root, RunOptions{Apply: true, ArchiveMode: "copy", RunID: "run-apply"})
	if err == nil {
		t.Fatal("expected apply failure when LLM unavailable and unresolved docs exist")
	}
	if report == nil {
		t.Fatal("expected report on failure")
	}
	if len(report.Unresolved) == 0 {
		t.Fatal("expected unresolved docs")
	}
	if !strings.Contains(err.Error(), "no provider is configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_ApplyPromotesCanonicalAndArchivesSources(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mk("docs/core/ARCHITECTURE.md", "# arch\n")
	mk("docs/adr/ADR-0001.md", "# adr\n")
	mk("docs/specs/001-demo/spec.md", "# spec\n")
	mk("docs/domains/core/overview.md", "# domain\n")
	mk("docs/context-map.md", "# context map\n")
	mk("GLOSSARY.md", "| Term | Target |\n|:-----|:-------|\n| **Arch** | [docs/core/ARCHITECTURE.md](docs/core/ARCHITECTURE.md) |\n")
	mk("architecture/policies.yml", "policies:\n  - id: x\n    reference: \"docs/core/ARCHITECTURE.md\"\n")

	report, err := Run(root, RunOptions{Apply: true, ArchiveMode: "copy", RunID: "run-ok"})
	if err != nil {
		t.Fatalf("apply run failed: %v", err)
	}
	if report == nil {
		t.Fatal("expected report")
	}

	// Canonical docs promoted
	for _, rel := range []string{
		".mindspec/docs/core/ARCHITECTURE.md",
		".mindspec/docs/adr/ADR-0001.md",
		".mindspec/docs/specs/001-demo/spec.md",
		".mindspec/docs/domains/core/overview.md",
		".mindspec/docs/context-map.md",
		".mindspec/docs/glossary.md",
		".mindspec/policies.yml",
		".mindspec/lineage/manifest.json",
	} {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); statErr != nil {
			t.Fatalf("expected canonical artifact %s: %v", rel, statErr)
		}
	}

	// Legacy markdown docs archived in copy mode.
	for _, rel := range []string{
		"docs/core/ARCHITECTURE.md",
		"docs/adr/ADR-0001.md",
		"docs/specs/001-demo/spec.md",
		"docs/domains/core/overview.md",
		"docs/context-map.md",
		"GLOSSARY.md",
		"architecture/policies.yml",
	} {
		archived := filepath.Join(root, "docs_archive", "run-ok", filepath.FromSlash(rel))
		if _, statErr := os.Stat(archived); statErr != nil {
			t.Fatalf("expected archived source %s: %v", rel, statErr)
		}
		// Copy mode keeps source files in place.
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); statErr != nil {
			t.Fatalf("expected source to remain in copy mode %s: %v", rel, statErr)
		}
	}

	policyBytes, err := os.ReadFile(filepath.Join(root, ".mindspec", "policies.yml"))
	if err != nil {
		t.Fatalf("read canonical policies: %v", err)
	}
	if !strings.Contains(string(policyBytes), "reference: \".mindspec/docs/core/ARCHITECTURE.md\"") {
		t.Fatalf("expected canonicalized policy reference, got:\n%s", string(policyBytes))
	}
	glossaryBytes, err := os.ReadFile(filepath.Join(root, ".mindspec", "docs", "glossary.md"))
	if err != nil {
		t.Fatalf("read canonical glossary: %v", err)
	}
	if !strings.Contains(string(glossaryBytes), "(.mindspec/docs/core/ARCHITECTURE.md)") {
		t.Fatalf("expected canonicalized glossary links, got:\n%s", string(glossaryBytes))
	}

	statePath := filepath.Join(root, ".mindspec", "migrations", "run-ok", "state.json")
	var state struct {
		Stage string `json:"stage"`
	}
	if err := readJSON(statePath, &state); err != nil {
		t.Fatalf("read state checkpoint: %v", err)
	}
	if state.Stage != stageApplied {
		t.Fatalf("expected stage %q, got %q", stageApplied, state.Stage)
	}
}

func TestRun_ApplyPromotesUserDocsCategory(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mk("docs/core/ARCHITECTURE.md", "# arch\n")
	mk("AGENTS.md", "# agent hints\n")
	mk("docs/archive/legacy.md", "# old doc\n")
	mk("docs/templates/plan.md", "# old template\n")

	report, err := Run(root, RunOptions{Apply: true, ArchiveMode: "copy", RunID: "run-user"})
	if err != nil {
		t.Fatalf("apply run failed: %v", err)
	}
	if report == nil {
		t.Fatal("expected report")
	}

	for _, rel := range []string{
		".mindspec/docs/core/ARCHITECTURE.md",
		".mindspec/docs/user/AGENTS.md",
		".mindspec/docs/user/archive/legacy.md",
		".mindspec/docs/user/templates/plan.md",
	} {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); statErr != nil {
			t.Fatalf("expected canonical user-doc artifact %s: %v", rel, statErr)
		}
	}
	for _, rel := range []string{
		"AGENTS.md",
		"docs/archive/legacy.md",
		"docs/templates/plan.md",
	} {
		archived := filepath.Join(root, "docs_archive", "run-user", filepath.FromSlash(rel))
		if _, statErr := os.Stat(archived); statErr != nil {
			t.Fatalf("expected archived user-doc source %s: %v", rel, statErr)
		}
	}
}

func TestRun_ApplyMoveRemovesLegacyDocsTree(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mk("docs/core/ARCHITECTURE.md", "# arch\n")
	mk("docs/context-map.md", "# map\n")
	mk("docs/specs/001-demo/spec.md", "# spec\n")
	mk("docs/specs/001-demo/recording/manifest.json", "{\"ok\":true}\n")
	mk("GLOSSARY.md", "# glossary\n")
	mk("AGENTS.md", "# agent\n")
	mk("architecture/policies.yml", "policies:\n  - id: x\n    reference: \"docs/core/ARCHITECTURE.md\"\n")

	if _, err := Run(root, RunOptions{Apply: true, ArchiveMode: "move", RunID: "run-move"}); err != nil {
		t.Fatalf("apply move failed: %v", err)
	}

	// Legacy docs were moved out of place.
	for _, rel := range []string{
		"docs/core/ARCHITECTURE.md",
		"docs/context-map.md",
		"docs/specs/001-demo/spec.md",
		"docs/specs/001-demo/recording/manifest.json",
		"GLOSSARY.md",
		"architecture/policies.yml",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("expected moved source %s to be absent, got err=%v", rel, err)
		}
	}

	// Operational root docs are copied, not moved.
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("expected AGENTS.md to remain in place: %v", err)
	}

	// Legacy roots are pruned when emptied.
	if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
		t.Fatalf("expected docs/ root to be pruned, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "architecture")); !os.IsNotExist(err) {
		t.Fatalf("expected architecture/ root to be pruned, got err=%v", err)
	}

	// Archive contains moved and copied sources.
	for _, rel := range []string{
		"docs/core/ARCHITECTURE.md",
		"docs/context-map.md",
		"docs/specs/001-demo/spec.md",
		"docs/specs/001-demo/recording/manifest.json",
		"GLOSSARY.md",
		"AGENTS.md",
		"architecture/policies.yml",
	} {
		archived := filepath.Join(root, "docs_archive", "run-move", filepath.FromSlash(rel))
		if _, err := os.Stat(archived); err != nil {
			t.Fatalf("expected archived source %s: %v", rel, err)
		}
	}

	// Spec residual artifacts are migrated into canonical spec tree.
	if _, err := os.Stat(filepath.Join(root, ".mindspec", "docs", "specs", "001-demo", "recording", "manifest.json")); err != nil {
		t.Fatalf("expected canonical spec recording artifact: %v", err)
	}
}

func TestRun_ResumeMissingArtifactsFails(t *testing.T) {
	root := t.TempDir()

	_, err := Run(root, RunOptions{
		RunID:  "missing",
		Resume: true,
	})
	if err == nil {
		t.Fatal("expected resume error for missing artifacts")
	}
	if !strings.Contains(err.Error(), "state.json is missing") {
		t.Fatalf("unexpected resume error: %v", err)
	}
}

func TestRun_ResumeReusesCheckpoint(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mk("docs/core/ARCHITECTURE.md", "# arch\n")
	initial, err := Run(root, RunOptions{RunID: "resume-run"})
	if err != nil {
		t.Fatalf("initial run: %v", err)
	}

	// Add a new markdown file after checkpoint creation; resume should ignore it.
	mk("docs/adr/ADR-0999.md", "# late adr\n")
	resumed, err := Run(root, RunOptions{
		Apply:       true,
		ArchiveMode: "copy",
		RunID:       "resume-run",
		Resume:      true,
	})
	if err != nil {
		t.Fatalf("resume apply failed: %v", err)
	}
	if !reflect.DeepEqual(initial.Inventory, resumed.Inventory) {
		t.Fatalf("resume should reuse checkpoint inventory")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".mindspec", "docs", "core", "ARCHITECTURE.md")); statErr != nil {
		t.Fatalf("expected resumed apply output: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".mindspec", "docs", "adr", "ADR-0999.md")); !os.IsNotExist(statErr) {
		t.Fatalf("expected late-added file to be excluded from resumed apply, got err=%v", statErr)
	}

	statePath := filepath.Join(root, ".mindspec", "migrations", "resume-run", "state.json")
	var state struct {
		Stage   string `json:"stage"`
		Resumed bool   `json:"resumed"`
	}
	if err := readJSON(statePath, &state); err != nil {
		t.Fatalf("read resumed state: %v", err)
	}
	if state.Stage != stageApplied {
		t.Fatalf("expected applied stage after resume, got %q", state.Stage)
	}
	if !state.Resumed {
		t.Fatal("expected resumed state marker")
	}
}

func TestRun_ApplyIdempotentCanonicalOutput(t *testing.T) {
	t.Setenv("MINDSPEC_LLM_PROVIDER", "off")
	t.Setenv("MINDSPEC_LLM_MODEL", "")

	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mk("docs/core/ARCHITECTURE.md", "# arch\n")
	mk("docs/context-map.md", "# context\n")
	mk("GLOSSARY.md", "# glossary\n")

	if _, err := Run(root, RunOptions{Apply: true, ArchiveMode: "copy", RunID: "first"}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	firstHash := treeHash(t, filepath.Join(root, ".mindspec", "docs"))

	if _, err := Run(root, RunOptions{Apply: true, ArchiveMode: "copy", RunID: "second"}); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	secondHash := treeHash(t, filepath.Join(root, ".mindspec", "docs"))

	if firstHash != secondHash {
		t.Fatalf("canonical docs hash mismatch across unchanged apply runs\nfirst=%s\nsecond=%s", firstHash, secondHash)
	}
}

func treeHash(t *testing.T, root string) string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatalf("walk tree: %v", err)
	}
	sort.Strings(files)

	h := sha256.New()
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		h.Write([]byte(rel))
		h.Write([]byte{'\n'})
		h.Write(data)
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
