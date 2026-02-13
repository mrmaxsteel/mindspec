package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdatePlanApproval_UpdatesFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Draft
spec_id: test
version: "0.1"
work_chunks:
  - id: 1
    title: "test"
    scope: "test.go"
    verify: []
    depends_on: []
---

# Plan body

Details here.
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	err := updatePlanApproval(planPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(planPath)
	content := string(data)

	// Should have approved status
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected status: Approved in frontmatter")
	}

	// Should have approved_at
	if !strings.Contains(content, "approved_at:") {
		t.Error("expected approved_at in frontmatter")
	}

	// Should have approved_by
	if !strings.Contains(content, "approved_by: user") {
		t.Error("expected approved_by: user in frontmatter")
	}

	// Body preserved
	if !strings.Contains(content, "# Plan body") {
		t.Error("plan body lost")
	}
	if !strings.Contains(content, "Details here.") {
		t.Error("plan details lost")
	}

	// Original fields preserved
	if !strings.Contains(content, "spec_id: test") {
		t.Error("spec_id field lost")
	}
}

func TestUpdatePlanApproval_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte("# No frontmatter\n"), 0644)

	err := updatePlanApproval(planPath)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestUpdatePlanApproval_PreservesGeneratedBlock(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Draft
spec_id: test
version: "0.1"
generated:
  mol_parent_id: mol-123
  bead_ids:
    "1": bead-abc
work_chunks:
  - id: 1
    title: "test"
    scope: "test.go"
    verify: []
    depends_on: []
---

# Plan
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	err := updatePlanApproval(planPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(planPath)
	content := string(data)

	// Generated block should be preserved
	if !strings.Contains(content, "mol_parent_id") {
		t.Error("generated.mol_parent_id lost")
	}
	if !strings.Contains(content, "bead-abc") {
		t.Error("generated.bead_ids lost")
	}

	// Approval fields set
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected status: Approved")
	}
}
