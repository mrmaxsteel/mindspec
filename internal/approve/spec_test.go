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

// TestScaffoldPlanEmitsADRCitations (Spec 100 R4 AC1; strengthened by Spec
// 122 R5/AC-12b): the generated plan.md skeleton names the exact
// `adr_citations` frontmatter key the gate reads, so the author sees it up
// front. The key must appear inside the YAML frontmatter region (between
// the opening and closing `---`).
//
// Spec 122 AC-12b additionally pins the commented REMEDY-GUIDANCE sentence
// beside that key ("cite the Accepted ADRs whose Domain(s) cover" —
// `internal/approve/spec.go`'s `scaffoldPlan`) so the documented-key remedy
// text cannot silently drop; this half is red if that guidance sentence is
// removed even though the bare `adr_citations` key remains (a mutation
// probe recorded in review evidence: dropping the sentence while keeping
// the key turns this assertion, and only this assertion, red).
func TestScaffoldPlanEmitsADRCitations(t *testing.T) {
	out := scaffoldPlan("100-x")

	if !strings.Contains(out, "adr_citations") {
		t.Errorf("expected scaffoldPlan output to contain the literal adr_citations key, got:\n%s", out)
	}

	// Confirm the key lands inside the frontmatter region, not the body.
	const fence = "---"
	first := strings.Index(out, fence)
	if first < 0 {
		t.Fatalf("expected opening frontmatter fence, got:\n%s", out)
	}
	second := strings.Index(out[first+len(fence):], fence)
	if second < 0 {
		t.Fatalf("expected closing frontmatter fence, got:\n%s", out)
	}
	frontmatter := out[first : first+len(fence)+second]
	if !strings.Contains(frontmatter, "adr_citations") {
		t.Errorf("expected adr_citations within the frontmatter region, got frontmatter:\n%s", frontmatter)
	}

	// Spec 122 AC-12b: the commented remedy-guidance sentence beside the
	// key — the actual working remedy ("cite the Accepted ADRs whose
	// Domain(s) cover" this plan's impacted domains), not just the bare key
	// name.
	const guidanceSentence = "cite the Accepted ADRs whose Domain(s) cover"
	if !strings.Contains(frontmatter, guidanceSentence) {
		t.Errorf("expected the adr_citations remedy-guidance sentence %q within the frontmatter region, got frontmatter:\n%s", guidanceSentence, frontmatter)
	}
}
