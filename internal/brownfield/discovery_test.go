package brownfield

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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

func TestRun_ReportArtifactsAreDeterministic(t *testing.T) {
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
		// report-only should succeed
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
