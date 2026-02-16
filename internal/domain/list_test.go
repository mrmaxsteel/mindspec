package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBoundedContexts(t *testing.T) {
	root := setupTestRoot(t)
	cmPath := filepath.Join(root, "docs", "context-map.md")

	contexts, err := ParseBoundedContexts(cmPath)
	if err != nil {
		t.Fatalf("ParseBoundedContexts() error: %v", err)
	}

	if len(contexts) != 1 {
		t.Fatalf("expected 1 bounded context, got %d", len(contexts))
	}

	if contexts[0].Name != "Core" {
		t.Errorf("expected name=Core, got %q", contexts[0].Name)
	}
	if !strings.Contains(contexts[0].Owns, "CLI entry point") {
		t.Errorf("expected Owns to contain 'CLI entry point', got %q", contexts[0].Owns)
	}
}

func TestParseBoundedContextsMultiple(t *testing.T) {
	tmp := t.TempDir()
	cm := `# Context Map

## Bounded Contexts

### Core

**Owns**: CLI entry point.

### Workflow

**Owns**: Mode system, spec lifecycle.

### Context-System

**Owns**: Glossary, context packs.

---

## Relationships
`
	os.WriteFile(filepath.Join(tmp, "context-map.md"), []byte(cm), 0644)

	contexts, err := ParseBoundedContexts(filepath.Join(tmp, "context-map.md"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(contexts) != 3 {
		t.Fatalf("expected 3 contexts, got %d", len(contexts))
	}
}

func TestListWithDomains(t *testing.T) {
	root := setupTestRoot(t)

	// Create some domain directories
	os.MkdirAll(filepath.Join(root, "docs", "domains", "core"), 0755)
	os.MkdirAll(filepath.Join(root, "docs", "domains", "workflow"), 0755)

	entries, err := List(root)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Should be sorted alphabetically
	if entries[0].Name != "core" {
		t.Errorf("expected first entry=core, got %q", entries[0].Name)
	}
	if entries[1].Name != "workflow" {
		t.Errorf("expected second entry=workflow, got %q", entries[1].Name)
	}

	// Core should have Owns from context map
	if !strings.Contains(entries[0].Owns, "CLI entry point") {
		t.Errorf("expected core Owns to contain 'CLI entry point', got %q", entries[0].Owns)
	}
}

func TestListEmpty(t *testing.T) {
	root := setupTestRoot(t)

	// Remove all domain dirs (setupTestRoot doesn't create any under docs/domains/)
	entries, err := List(root)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d entries", len(entries))
	}
}

func TestListNoDomainDir(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".mindspec"), 0755)
	// No docs/domains/ directory at all

	entries, err := List(root)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if entries != nil {
		t.Errorf("expected nil for missing domains dir, got %v", entries)
	}
}

func TestFormatTableEmpty(t *testing.T) {
	result := FormatTable(nil)
	if !strings.Contains(result, "No domains found.") {
		t.Errorf("expected 'No domains found.', got: %s", result)
	}
}

func TestFormatTableWithEntries(t *testing.T) {
	entries := []DomainEntry{
		{Name: "core", Owns: "CLI, workspace", Relationships: []string{"→ Context-System (upstream)"}},
		{Name: "workflow", Owns: "Modes, specs", Relationships: nil},
	}

	result := FormatTable(entries)
	if !strings.Contains(result, "core") {
		t.Error("table missing 'core'")
	}
	if !strings.Contains(result, "workflow") {
		t.Error("table missing 'workflow'")
	}
	if !strings.Contains(result, "Domain") {
		t.Error("table missing header 'Domain'")
	}
}
