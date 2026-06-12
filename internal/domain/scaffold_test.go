package domain

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Silence the populate prompt Add prints (spec 091 Req 9) so
	// unrelated tests stay quiet; capturePopulatePrompt re-overrides
	// this for the tests that assert on the prompt.
	prev := populatePromptWriter
	populatePromptWriter = io.Discard
	t.Cleanup(func() { populatePromptWriter = prev })

	// Create marker
	os.Mkdir(filepath.Join(root, ".mindspec"), 0755)

	// Create domains dir
	os.MkdirAll(filepath.Join(root, "docs", "domains"), 0755)

	// Create context map
	cm := `# MindSpec Context Map

## Bounded Contexts

### Core

**Owns**: CLI entry point, workspace resolution.

**Domain docs**: [` + "`" + `docs/domains/core/` + "`" + `](domains/core/overview.md)

---

## Relationships
`
	os.WriteFile(filepath.Join(root, "docs", "context-map.md"), []byte(cm), 0644)

	return root
}

func TestAddCreatesTemplateFiles(t *testing.T) {
	root := setupTestRoot(t)

	err := Add(root, "payments")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	domainDir := filepath.Join(root, "docs", "domains", "payments")
	files := []string{"overview.md", "architecture.md", "interfaces.md", "runbook.md"}

	for _, f := range files {
		path := filepath.Join(domainDir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s not created: %v", f, err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, "# Payments Domain") {
			t.Errorf("%s missing title, got:\n%s", f, content)
		}
	}
}

func TestAddOverviewHasCorrectSections(t *testing.T) {
	root := setupTestRoot(t)

	if err := Add(root, "billing"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "docs", "domains", "billing", "overview.md"))
	content := string(data)

	sections := []string{
		"## What This Domain Owns",
		"## Boundaries",
		"## Key Files",
		"## Current State",
	}
	for _, s := range sections {
		if !strings.Contains(content, s) {
			t.Errorf("overview.md missing section %q", s)
		}
	}
}

func TestAddIdempotencyGuard(t *testing.T) {
	root := setupTestRoot(t)

	// First add succeeds
	if err := Add(root, "payments"); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}

	// Second add fails
	err := Add(root, "payments")
	if err == nil {
		t.Fatal("expected error for existing domain, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddInvalidName(t *testing.T) {
	root := setupTestRoot(t)

	tests := []string{"123bad", "Bad-Name", "has spaces", "-leading-dash", "UPPER"}
	for _, name := range tests {
		err := Add(root, name)
		if err == nil {
			t.Errorf("expected error for name %q, got nil", name)
		}
		if !strings.Contains(err.Error(), "invalid domain name") {
			t.Errorf("expected 'invalid domain name' error for %q, got: %v", name, err)
		}
	}
}

func TestAddUpdatesContextMap(t *testing.T) {
	root := setupTestRoot(t)

	if err := Add(root, "payments"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "context-map.md"))
	if err != nil {
		t.Fatalf("reading context map: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "### Payments") {
		t.Error("context map missing ### Payments heading")
	}
	if !strings.Contains(content, "**Owns**: _(fill in)_") {
		t.Error("context map missing Owns placeholder")
	}
	if !strings.Contains(content, "docs/domains/payments/") {
		t.Error("context map missing domain docs link")
	}

	// Entry should appear before the --- separator
	sepIdx := strings.Index(content, "## Relationships")
	paymentsIdx := strings.Index(content, "### Payments")
	if sepIdx >= 0 && paymentsIdx >= sepIdx {
		t.Error("payments entry should appear before ## Relationships")
	}
}

func TestAddHyphenatedName(t *testing.T) {
	root := setupTestRoot(t)

	if err := Add(root, "my-cool-domain"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "docs", "domains", "my-cool-domain", "overview.md"))
	if !strings.Contains(string(data), "# My-Cool-Domain Domain") {
		t.Errorf("expected title-cased hyphenated name, got:\n%s", string(data))
	}

	cm, _ := os.ReadFile(filepath.Join(root, "docs", "context-map.md"))
	if !strings.Contains(string(cm), "### My-Cool-Domain") {
		t.Error("context map missing title-cased heading for hyphenated domain")
	}
}

// capturePopulatePrompt redirects the populate-prompt writer to a
// buffer for the duration of the test.
func capturePopulatePrompt(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := populatePromptWriter
	populatePromptWriter = &buf
	t.Cleanup(func() { populatePromptWriter = prev })
	return &buf
}

func TestAddScaffoldsOwnershipStub(t *testing.T) {
	root := setupTestRoot(t)
	capturePopulatePrompt(t)

	if err := Add(root, "payments"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "domains", "payments", "OWNERSHIP.yaml"))
	if err != nil {
		t.Fatalf("OWNERSHIP.yaml not created: %v", err)
	}
	content := string(data)

	// Req 8 stub: empty paths body, `domain add` comment variant.
	if !strings.Contains(content, "paths: []") {
		t.Errorf("OWNERSHIP.yaml missing empty `paths: []` stub:\n%s", content)
	}
	if !strings.Contains(content, "# Auto-generated by mindspec domain add payments on ") {
		t.Errorf("OWNERSHIP.yaml comment missing `domain add` command variant:\n%s", content)
	}
	// ZFC: the framework must not pre-fill a claim for the new domain.
	if strings.Contains(content, "internal/payments/**") {
		t.Errorf("OWNERSHIP.yaml contains framework-proposed glob (ZFC violation):\n%s", content)
	}
}

func TestAddPrintsPopulatePrompt(t *testing.T) {
	root := setupTestRoot(t)
	buf := capturePopulatePrompt(t)

	if err := Add(root, "payments"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, ".mindspec/docs/domains/payments/OWNERSHIP.yaml") {
		t.Errorf("populate prompt not printed (missing manifest path):\n%s", out)
	}
	if !strings.Contains(out, "The framework deliberately provides no pattern hints") {
		t.Errorf("populate prompt not printed (missing ZFC sentence):\n%s", out)
	}
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"payments", "Payments"},
		{"my-domain", "My-Domain"},
		{"a-b-c", "A-B-C"},
		{"context-system", "Context-System"},
	}
	for _, tt := range tests {
		got := titleCase(tt.input)
		if got != tt.want {
			t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
