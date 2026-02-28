package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSpecApproval_UpdatesSection(t *testing.T) {
	tmp := t.TempDir()
	specContent := `# Spec 010: Test Feature

## Goal

Build a test feature.

## Impacted Domains

- **core**: test

## ADR Touchpoints

- ADR-0001

## Requirements

1. Requirement one
2. Requirement two

## Scope

### In Scope
- Things

### Out of Scope
- Other things

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Open Questions

None

## Approval

- **Status**: DRAFT
- **Approved By**: -
`
	specPath := filepath.Join(tmp, "spec.md")
	os.WriteFile(specPath, []byte(specContent), 0644)

	err := updateSpecApproval(specPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(specPath)
	content := string(data)

	if !strings.Contains(content, "status: Approved") {
		t.Error("expected status: Approved in frontmatter")
	}
	if !strings.Contains(content, "approved_at:") {
		t.Error("expected approved_at in frontmatter")
	}
	if !strings.Contains(content, "approved_by: user") {
		t.Error("expected approved_by in frontmatter")
	}
	if !strings.Contains(content, "**Status**: APPROVED") {
		t.Error("expected Status: APPROVED in output")
	}
	if !strings.Contains(content, "**Approved By**: user") {
		t.Error("expected Approved By: user in output")
	}
	if !strings.Contains(content, "**Approval Date**:") {
		t.Error("expected Approval Date in output")
	}
	// Original content preserved
	if !strings.Contains(content, "## Goal") {
		t.Error("Goal section lost")
	}
	if !strings.Contains(content, "Build a test feature.") {
		t.Error("Goal content lost")
	}
	// DRAFT should be gone
	if strings.Contains(content, "DRAFT") {
		t.Error("DRAFT should have been replaced")
	}
}

func TestUpdateSpecApproval_AddsApprovalSectionWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	specContent := "# Spec\n\n## Goal\n\nSomething.\n"
	specPath := filepath.Join(tmp, "spec.md")
	os.WriteFile(specPath, []byte(specContent), 0644)

	err := updateSpecApproval(specPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(specPath)
	content := string(data)
	if !strings.Contains(content, "## Approval") {
		t.Fatal("expected Approval section to be added")
	}
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected approved status frontmatter")
	}
}

func TestUpdateSpecApproval_ApprovalNotLast(t *testing.T) {
	tmp := t.TempDir()
	specContent := `# Spec

## Approval

- **Status**: DRAFT

## Appendix

Extra stuff here.
`
	specPath := filepath.Join(tmp, "spec.md")
	os.WriteFile(specPath, []byte(specContent), 0644)

	err := updateSpecApproval(specPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(specPath)
	content := string(data)

	// Appendix should be preserved
	if !strings.Contains(content, "## Appendix") {
		t.Error("Appendix section lost")
	}
	if !strings.Contains(content, "Extra stuff here.") {
		t.Error("Appendix content lost")
	}
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected approved status frontmatter")
	}
	if !strings.Contains(content, "APPROVED") {
		t.Error("expected APPROVED in output")
	}
}
