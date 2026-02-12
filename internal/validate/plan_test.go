package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePlan_WellFormed(t *testing.T) {
	root := findProjectRoot(t)
	r := ValidatePlan(root, "005-next")

	// Filter to only errors (bead ID checks may warn if beads are closed)
	for _, issue := range r.Issues {
		if issue.Severity == SevError {
			t.Logf("[%s] %s: %s", issue.Severity, issue.Name, issue.Message)
		}
	}

	// The plan is well-formed structurally — should not have structural errors
	for _, issue := range r.Issues {
		if issue.Severity == SevError && issue.Name != "bead-id-missing" {
			t.Errorf("unexpected structural error: [%s] %s: %s", issue.Severity, issue.Name, issue.Message)
		}
	}
}

func TestValidatePlan_MissingFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte("# Plan\n\nNo frontmatter here.\n"), 0644)

	r := ValidatePlan(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failure for plan without frontmatter")
	}
}

func TestValidatePlan_MissingRequiredFields(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	plan := "---\nstatus: Draft\n---\n\n# Plan\n\n## Bead 999-A: Test\n\n**Steps**:\n1. Step one\n2. Step two\n3. Step three\n\n**Verification**:\n- [ ] Check something\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	r := ValidatePlan(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failure for missing spec_id and version")
	}

	foundSpecID := false
	foundVersion := false
	for _, issue := range r.Issues {
		if issue.Name == "frontmatter-spec-id" {
			foundSpecID = true
		}
		if issue.Name == "frontmatter-version" {
			foundVersion = true
		}
	}
	if !foundSpecID {
		t.Error("expected frontmatter-spec-id error")
	}
	if !foundVersion {
		t.Error("expected frontmatter-version error")
	}
}

func TestValidatePlan_NoBeadSections(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\nJust some text, no beads.\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	r := ValidatePlan(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failure for plan without bead sections")
	}

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "bead-sections" {
			found = true
		}
	}
	if !found {
		t.Error("expected bead-sections error")
	}
}

func TestValidatePlan_BeadMissingSteps(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## Bead 999-A: Test\n\n**Scope**: Do something\n\n**Steps**:\n1. Only one step\n\n**Verification**:\n- [ ] Check it\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	r := ValidatePlan(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failure for bead with < 3 steps")
	}

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "bead-steps" {
			found = true
		}
	}
	if !found {
		t.Error("expected bead-steps error")
	}
}

func TestValidatePlan_BeadMissingVerification(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## Bead 999-A: Test\n\n**Scope**: Do something\n\n**Steps**:\n1. Step one\n2. Step two\n3. Step three\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	r := ValidatePlan(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failure for bead without verification")
	}

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "bead-verification" {
			found = true
		}
	}
	if !found {
		t.Error("expected bead-verification error")
	}
}

func TestValidatePlan_NonexistentPlan(t *testing.T) {
	r := ValidatePlan("/nonexistent", "does-not-exist")
	if !r.HasFailures() {
		t.Error("expected failure for nonexistent plan")
	}
}

func TestParsePlanFrontmatter(t *testing.T) {
	content := "---\nstatus: Approved\nspec_id: \"005-next\"\nversion: \"1.0\"\napproved_at: 2026-02-12\napproved_by: user\nbead_ids: [a, b]\nadr_citations:\n  - id: ADR-0003\n    sections: [\"CLI\"]\n---\n\n# Plan\n"

	fm, err := parsePlanFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Status != "Approved" {
		t.Errorf("expected status Approved, got %s", fm.Status)
	}
	if fm.SpecID != "005-next" {
		t.Errorf("expected spec_id 005-next, got %s", fm.SpecID)
	}
	if len(fm.BeadIDs) != 2 {
		t.Errorf("expected 2 bead IDs, got %d", len(fm.BeadIDs))
	}
	if len(fm.ADRCitations) != 1 {
		t.Errorf("expected 1 ADR citation, got %d", len(fm.ADRCitations))
	}
}

func TestParsePlanFrontmatter_WithComments(t *testing.T) {
	content := "---\nstatus: Draft\nspec_id: \"005\"\nversion: \"0.1\"\n# approved_at:\n# approved_by:\n# bead_ids: []\n---\n\n# Plan\n"

	fm, err := parsePlanFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Status != "Draft" {
		t.Errorf("expected status Draft, got %s", fm.Status)
	}
	// Commented fields should be ignored
	if fm.ApprovedAt != "" {
		t.Errorf("expected empty approved_at, got %s", fm.ApprovedAt)
	}
}

func TestParseBeadSections(t *testing.T) {
	content := `---
status: Draft
---

# Plan

## Bead 006-A: First

**Scope**: Something

**Steps**:
1. Step one
2. Step two
3. Step three
4. Step four

**Verification**:
- [ ] Check one
- [ ] Check two

**Depends on**: nothing

---

## Bead 006-B: Second

**Scope**: Something else

**Steps**:
1. Step one
2. Step two
3. Step three

**Verification**:
- [ ] Check one

**Depends on**: 006-A
`

	sections := parseBeadSections(content)
	if len(sections) != 2 {
		t.Fatalf("expected 2 bead sections, got %d", len(sections))
	}

	if sections[0].stepsCount != 4 {
		t.Errorf("bead A: expected 4 steps, got %d", sections[0].stepsCount)
	}
	if sections[0].verifyCount != 2 {
		t.Errorf("bead A: expected 2 verification items, got %d", sections[0].verifyCount)
	}
	if !sections[0].hasDependsOn {
		t.Error("bead A: expected depends-on to be present")
	}

	if sections[1].stepsCount != 3 {
		t.Errorf("bead B: expected 3 steps, got %d", sections[1].stepsCount)
	}
	if !sections[1].hasDependsOn {
		t.Error("bead B: expected depends-on to be present")
	}
}
