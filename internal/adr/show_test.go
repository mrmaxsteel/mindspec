package adr

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShow(t *testing.T) {
	root := setupTestADRs(t)

	a, err := Show(root, "ADR-0001")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if a.Title != "Test Decision" {
		t.Errorf("Title = %q, want %q", a.Title, "Test Decision")
	}
	if a.Status != "Accepted" {
		t.Errorf("Status = %q, want Accepted", a.Status)
	}
}

func TestShow_NotFound(t *testing.T) {
	root := setupTestADRs(t)

	_, err := Show(root, "ADR-9999")
	if err == nil {
		t.Error("expected error for nonexistent ADR")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

// TestShow_FullSluggedIDResolves is the AC-9 GUARD (spec 123 R5(c)): show
// must keep resolving a caller-supplied FULL slugged stem
// ("ADR-0001-integrate-at-contracts-not-tools") directly, unaffected by
// the collision-detection path that now applies to bare canonical input.
// This is not a RED fix — it pins that behavior stays working.
func TestShow_FullSluggedIDResolves(t *testing.T) {
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0001-integrate-at-contracts-not-tools.md"), []byte(testADR1), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := Show(root, "ADR-0001-integrate-at-contracts-not-tools")
	if err != nil {
		t.Fatalf("Show with full slugged stem: %v", err)
	}
	if a.ID != "ADR-0001" {
		t.Errorf("ID = %q, want ADR-0001", a.ID)
	}
	if a.Title != "Test Decision" {
		t.Errorf("Title = %q, want %q", a.Title, "Test Decision")
	}
}

// TestShow_BareSluggedCollisionErrors is the AC-10 collision pin (spec
// 123 R5(c)): a directory holding BOTH a bare ADR-0002.md and a slugged
// ADR-0002-foo.md must make `show ADR-0002` error naming both paths with
// an ADR-0035 recovery line, instead of the pre-123 silent short-circuit
// to the exact bare match. RED on revert.
func TestShow_BareSluggedCollisionErrors(t *testing.T) {
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0002.md"), []byte(testADR2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0002-foo.md"), []byte(testADR2), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Show(root, "ADR-0002")
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "ADR-0002.md") || !strings.Contains(err.Error(), "ADR-0002-foo.md") {
		t.Errorf("error must name both paths, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery:") {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}
}

func TestExtractDecision(t *testing.T) {
	content := `# ADR-0001: Test

## Context
Some context.

## Decision
We will use Redis for caching.

This improves performance.

## Consequences
Something.
`
	decision := ExtractDecision(content)
	if !strings.Contains(decision, "Redis for caching") {
		t.Errorf("decision = %q, expected Redis content", decision)
	}
	if strings.Contains(decision, "Consequences") {
		t.Error("decision should not include Consequences section")
	}
}

func TestExtractDecision_NoSection(t *testing.T) {
	content := "# ADR-0001: Test\n\n## Context\nJust context.\n"
	decision := ExtractDecision(content)
	if decision != "" {
		t.Errorf("expected empty decision, got %q", decision)
	}
}

func TestFormatSummary(t *testing.T) {
	root := setupTestADRs(t)
	a, _ := Show(root, "ADR-0001")

	summary := FormatSummary(a)
	if !strings.Contains(summary, "ADR-0001") {
		t.Error("expected ID in summary")
	}
	if !strings.Contains(summary, "Test Decision") {
		t.Error("expected title in summary")
	}
	if !strings.Contains(summary, "Accepted") {
		t.Error("expected status in summary")
	}
	if !strings.Contains(summary, "Decision:") {
		t.Error("expected Decision section in summary")
	}
}

func TestFormatJSON(t *testing.T) {
	root := setupTestADRs(t)
	a, _ := Show(root, "ADR-0001")

	jsonStr, err := FormatJSON(a)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["id"] != "ADR-0001" {
		t.Errorf("id = %v, want ADR-0001", parsed["id"])
	}
	if parsed["status"] != "Accepted" {
		t.Errorf("status = %v, want Accepted", parsed["status"])
	}
	if parsed["title"] != "Test Decision" {
		t.Errorf("title = %v, want Test Decision", parsed["title"])
	}
}

func TestFormatJSON_SupersededADR(t *testing.T) {
	root := setupTestADRs(t)
	a, _ := Show(root, "ADR-0003")

	jsonStr, err := FormatJSON(a)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &parsed)

	if parsed["superseded_by"] != "ADR-0005" {
		t.Errorf("superseded_by = %v, want ADR-0005", parsed["superseded_by"])
	}
}
