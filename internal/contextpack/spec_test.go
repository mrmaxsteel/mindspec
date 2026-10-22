package contextpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "docs", "specs", "001-test")
	os.MkdirAll(specDir, 0o755)

	content := `# Spec 001: Test Spec

## Goal

Build a test feature for validation purposes.

## Impacted Domains

- core: CLI and workspace
- context-system: context pack generation

## Requirements

1. Something
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644)

	meta, err := ParseSpec(specDir)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if meta.SpecID != "001-test" {
		t.Errorf("SpecID = %q, want %q", meta.SpecID, "001-test")
	}

	if meta.Goal != "Build a test feature for validation purposes." {
		t.Errorf("Goal = %q, want %q", meta.Goal, "Build a test feature for validation purposes.")
	}

	if len(meta.Domains) != 2 {
		t.Fatalf("Domains count = %d, want 2", len(meta.Domains))
	}
	if meta.Domains[0] != "core" {
		t.Errorf("Domains[0] = %q, want %q", meta.Domains[0], "core")
	}
	if meta.Domains[1] != "context-system" {
		t.Errorf("Domains[1] = %q, want %q", meta.Domains[1], "context-system")
	}
}

func TestParseSpec_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := ParseSpec(dir)
	if err == nil {
		t.Fatal("expected error for missing spec.md")
	}
}

// TestParseSpec_ImpactedDomainsSubBulletsAndCodeSpans: indented glob
// sub-bullets under a domain bullet are not domains, and a bullet that
// names several code-span domains yields each one. The old parser returned
// the glob paths as domains and joined `a`, `b` into one token.
func TestParseSpec_ImpactedDomainsSubBulletsAndCodeSpans(t *testing.T) {
	dir := t.TempDir()
	body := "# Spec\n\n## Impacted Domains\n\n" +
		"- **workflow**:\n" +
		"  - `internal/panel/**`: pass rule.\n" +
		"  - `internal/setup/**`, `plugins/mindspec/skills/**`: skill text.\n" +
		"- `core`, `execution`: config and executor.\n" +
		"\n## Requirements\n\n- not a domain\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := ParseSpec(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"workflow", "core", "execution"}
	if strings.Join(meta.Domains, ",") != strings.Join(want, ",") {
		t.Fatalf("Domains = %q, want %q", meta.Domains, want)
	}
}

// TestParseSpec_ImpactedDomainsIndentedOnly: a section whose bullets are all
// indented (no top-level bullet first) still yields its domains.
func TestParseSpec_ImpactedDomainsIndentedOnly(t *testing.T) {
	dir := t.TempDir()
	body := "## Impacted Domains\n\n  - workflow: x\n  - core\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := ParseSpec(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(meta.Domains, ",") != "workflow,core" {
		t.Fatalf("Domains = %q", meta.Domains)
	}
}
