package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanSourceMarkdown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create source files that should be found
	for _, rel := range []string{
		"docs/guide.md",
		"docs/adr/ADR-0001.md",
		"README.md",
		"CONTRIBUTING.md",
	} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("# "+rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create files that should be skipped
	for _, rel := range []string{
		".mindspec/docs/core/USAGE.md",
		".mindspec/migrations/run-1/plan.md",
		".git/config.md",
		".beads/issues.md",
	} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("skip\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := scanSourceMarkdown(root)
	if err != nil {
		t.Fatalf("scanSourceMarkdown: %v", err)
	}

	expected := map[string]bool{
		"CONTRIBUTING.md":      false,
		"README.md":            false,
		"docs/adr/ADR-0001.md": false,
		"docs/guide.md":        false,
	}
	for _, f := range files {
		if _, ok := expected[f]; ok {
			expected[f] = true
		} else {
			t.Errorf("unexpected file: %s", f)
		}
	}
	for f, found := range expected {
		if !found {
			t.Errorf("missing expected file: %s", f)
		}
	}
}

func TestScanCanonicalDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	docsDir := filepath.Join(root, ".mindspec", "docs")
	for _, rel := range []string{
		"core/USAGE.md",
		"glossary.md",
	} {
		abs := filepath.Join(docsDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("canonical\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := scanCanonicalDocs(root)
	if err != nil {
		t.Fatalf("scanCanonicalDocs: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 canonical files, got %d: %v", len(files), files)
	}
}

func TestScanCanonicalDocs_NoDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files, err := scanCanonicalDocs(root)
	if err != nil {
		t.Fatalf("scanCanonicalDocs: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestBuildMigratePrompt_ContainsRequiredSections(t *testing.T) {
	t.Parallel()

	prompt := buildMigratePrompt(
		[]string{"docs/guide.md", "README.md"},
		[]string{".mindspec/docs/core/USAGE.md"},
	)

	required := []string{
		"Phase 1",
		"Phase 2",
		"Phase 3",
		"Phase 4",
		"Phase 5",
		"Canonical Structure",
		"Category Rubric",
		"adr",
		"spec",
		"domain",
		"core",
		"context-map",
		"user-docs",
		"agent",
		"mindspec domain add",
		"Source Files to Classify",
		"docs/guide.md",
		"README.md",
		"Existing Canonical Docs",
		".mindspec/docs/core/USAGE.md",
		"Instructions",
		"mindspec doctor",
	}

	for _, r := range required {
		if !strings.Contains(prompt, r) {
			t.Errorf("prompt missing required content: %q", r)
		}
	}
}

func TestMigrateJSON_ValidOutput(t *testing.T) {
	t.Parallel()

	inv := MigrateInventory{
		SourceFiles:    []string{"docs/guide.md"},
		CanonicalFiles: []string{".mindspec/docs/core/USAGE.md"},
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed MigrateInventory
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.SourceFiles) != 1 || parsed.SourceFiles[0] != "docs/guide.md" {
		t.Errorf("unexpected source_files: %v", parsed.SourceFiles)
	}
	if len(parsed.CanonicalFiles) != 1 || parsed.CanonicalFiles[0] != ".mindspec/docs/core/USAGE.md" {
		t.Errorf("unexpected canonical_files: %v", parsed.CanonicalFiles)
	}
}
