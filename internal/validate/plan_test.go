package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// Note: 005-next is an approved plan, so new Spec 039 checks are skipped
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

	plan := "---\nstatus: Draft\n---\n\n# Plan\n\n## ADR Fitness\n\nNo relevant ADRs.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n## Bead 999-A: Test\n\n**Steps**:\n1. Step one\n2. Step two\n3. Step three\n\n**Verification**:\n- [ ] `go test ./...` passes\n\n**Depends on**: nothing\n"
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

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## ADR Fitness\n\nNone.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n## Bead 999-A: Test\n\n**Scope**: Do something\n\n**Steps**:\n1. Only one step\n\n**Verification**:\n- [ ] `go test` passes\n\n**Depends on**: nothing\n"
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

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## ADR Fitness\n\nNone.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n## Bead 999-A: Test\n\n**Scope**: Do something\n\n**Steps**:\n1. Step one\n2. Step two\n3. Step three\n\n**Depends on**: nothing\n"
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

// TestValidatePlan_ApprovedFrontmatterPlanPhaseWarns removed:
// ADR-0023 eliminated lifecycle.yaml; plan-gate-consistency check no longer applies.

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

	sections := ParseBeadSections(content)
	if len(sections) != 2 {
		t.Fatalf("expected 2 bead sections, got %d", len(sections))
	}

	if sections[0].StepsCount != 4 {
		t.Errorf("bead A: expected 4 steps, got %d", sections[0].StepsCount)
	}
	if sections[0].VerifyCount != 2 {
		t.Errorf("bead A: expected 2 verification items, got %d", sections[0].VerifyCount)
	}
	if len(sections[0].VerifyLines) != 2 {
		t.Errorf("bead A: expected 2 verification lines, got %d", len(sections[0].VerifyLines))
	}
	if !sections[0].HasDependsOn {
		t.Error("bead A: expected depends-on to be present")
	}

	if sections[1].StepsCount != 3 {
		t.Errorf("bead B: expected 3 steps, got %d", sections[1].StepsCount)
	}
	if !sections[1].HasDependsOn {
		t.Error("bead B: expected depends-on to be present")
	}
}

func TestParseBeadSections_H3Headings(t *testing.T) {
	content := `---
status: Draft
---

# Plan

## Bead 006-A: First

### Scope
Something

### Steps
1. Step one
2. Step two
3. Step three
4. Step four

### Verification
- [ ] Check one
- [ ] Check two

### Depends on
nothing
`

	sections := ParseBeadSections(content)
	if len(sections) != 1 {
		t.Fatalf("expected 1 bead section, got %d", len(sections))
	}

	if sections[0].StepsCount != 4 {
		t.Errorf("expected 4 steps, got %d", sections[0].StepsCount)
	}
	if sections[0].VerifyCount != 2 {
		t.Errorf("expected 2 verification items, got %d", sections[0].VerifyCount)
	}
	if !sections[0].HasVerify {
		t.Error("expected hasVerify to be true")
	}
	if !sections[0].HasDependsOn {
		t.Error("expected hasDependsOn to be true")
	}
}

// --- ADR citation validation tests ---

func makePlanWithSections(t *testing.T, root string, citations string, hasADRFitness bool, hasTestingStrategy bool, hasProvenance bool, status string, verifyLine string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0o755)

	fitnessSection := ""
	if hasADRFitness {
		fitnessSection = "\n## ADR Fitness\n\nAll cited ADRs remain appropriate.\n"
	}

	testingStrategySection := ""
	if hasTestingStrategy {
		testingStrategySection = "\n## Testing Strategy\n\nUnit tests with go test.\n"
	}

	provenanceSection := ""
	if hasProvenance {
		provenanceSection = "\n## Provenance\n\n| AC | Bead |\n|---|---|\n| AC1 | 999-A |\n"
	}

	if verifyLine == "" {
		verifyLine = "- [ ] `go test ./internal/validate/...` passes"
	}

	if status == "" {
		status = "Draft"
	}

	plan := "---\nstatus: " + status + "\nspec_id: \"999-test\"\nversion: \"1.0\"\nadr_citations:\n" + citations + "---\n\n# Plan\n" + fitnessSection + testingStrategySection + provenanceSection + "\n## Bead 999-A: Test\n\n**Steps**:\n1. Step one\n2. Step two\n3. Step three\n\n**Verification**:\n" + verifyLine + "\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0o644)
}

// Legacy helper preserved for existing tests
func makePlanWithCitations(t *testing.T, root string, citations string, hasADRFitness bool) {
	t.Helper()
	// Include all required sections so legacy tests focus on their specific check
	makePlanWithSections(t, root, citations, hasADRFitness, true, true, "", "")
}

func writeTestADR(t *testing.T, root, id, status string) {
	t.Helper()
	adrDir := filepath.Join(root, "docs", "adr")
	os.MkdirAll(adrDir, 0o755)

	content := "# " + id + ": Test\n\n- **Status**: " + status + "\n- **Domain(s)**: core\n- **Supersedes**: n/a\n- **Superseded-by**: n/a\n\n## Decision\nSome decision.\n"
	os.WriteFile(filepath.Join(adrDir, id+".md"), []byte(content), 0o644)
}

func TestValidatePlan_ADRCiteMissing(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithCitations(t, tmp, "  - id: ADR-9999\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-cite-missing" {
			found = true
		}
	}
	if !found {
		t.Error("expected adr-cite-missing error for nonexistent ADR")
	}
}

func TestValidatePlan_ADRCiteSuperseded(t *testing.T) {
	tmp := t.TempDir()
	writeTestADR(t, tmp, "ADR-0001", "Superseded")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-cite-superseded" {
			found = true
		}
	}
	if !found {
		t.Error("expected adr-cite-superseded warning for Superseded ADR")
	}
}

func TestValidatePlan_ADRCiteProposed(t *testing.T) {
	tmp := t.TempDir()
	writeTestADR(t, tmp, "ADR-0001", "Proposed")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-cite-proposed" {
			found = true
		}
	}
	if !found {
		t.Error("expected adr-cite-proposed warning for Proposed ADR")
	}
}

// --- Spec 039: ADR Fitness promoted to error ---

func TestValidatePlan_ADRFitnessMissing_IsError(t *testing.T) {
	tmp := t.TempDir()
	writeTestADR(t, tmp, "ADR-0001", "Accepted")
	makePlanWithSections(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", false, true, true, "", "")

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-fitness-missing" {
			if issue.Severity != SevError {
				t.Errorf("expected adr-fitness-missing to be ERROR, got %s", issue.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected adr-fitness-missing error when ## ADR Fitness section is absent")
	}
}

func TestValidatePlan_ADRFitnessPresent(t *testing.T) {
	tmp := t.TempDir()
	writeTestADR(t, tmp, "ADR-0001", "Accepted")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "adr-fitness-missing" {
			t.Error("unexpected adr-fitness-missing when ## ADR Fitness section is present")
		}
	}
}

// --- Spec 039: Conditional ADR citations ---

func TestValidatePlan_EmptyCitations_WithFitness_IsWarning(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "")

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-citations" {
			if issue.Severity != SevWarning {
				t.Errorf("expected adr-citations to be WARNING when ADR Fitness is present, got %s", issue.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected adr-citations warning when citations empty but ADR Fitness present")
	}
}

func TestValidatePlan_EmptyCitations_WithoutFitness_IsError(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", false, true, true, "", "")

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-citations" {
			if issue.Severity != SevError {
				t.Errorf("expected adr-citations to be ERROR when both empty, got %s", issue.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected adr-citations error when both citations and ADR Fitness are missing")
	}
}

// --- Spec 039: Testing Strategy section ---

func TestValidatePlan_TestingStrategyMissing_IsError(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, false, true, "", "")

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "testing-strategy-missing" {
			if issue.Severity != SevError {
				t.Errorf("expected testing-strategy-missing to be ERROR, got %s", issue.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected testing-strategy-missing error")
	}
}

func TestValidatePlan_TestingStrategyPresent(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "")

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "testing-strategy-missing" {
			t.Error("unexpected testing-strategy-missing when section is present")
		}
	}
}

// --- Spec 039: Provenance section ---

func TestValidatePlan_ProvenanceMissing_IsWarning(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, false, "", "")

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "provenance-missing" {
			if issue.Severity != SevWarning {
				t.Errorf("expected provenance-missing to be WARNING, got %s", issue.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected provenance-missing warning")
	}
}

func TestValidatePlan_ProvenancePresent(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "")

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "provenance-missing" {
			t.Error("unexpected provenance-missing when section is present")
		}
	}
}

// --- Spec 039: Bead verification testability ---

func TestValidatePlan_VerificationTestable_GoTest(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "- [ ] `go test ./internal/validate/...` passes")

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "bead-verification-testability" {
			t.Error("unexpected testability error when verification references go test")
		}
	}
}

func TestValidatePlan_VerificationTestable_TestFile(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "- [ ] New tests in `plan_test.go` pass")

	r := ValidatePlan(tmp, "999-test")

	// _test.go is the pattern, plan_test.go doesn't contain it literally
	// but let's check — "plan_test.go" does NOT contain "_test.go" substring... actually it does: plan_test.go
	for _, issue := range r.Issues {
		if issue.Name == "bead-verification-testability" {
			t.Error("unexpected testability error when verification references _test.go file")
		}
	}
}

func TestValidatePlan_VerificationTestable_MakeTest(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "- [ ] `make test` passes with no regressions")

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "bead-verification-testability" {
			t.Error("unexpected testability error when verification references make test")
		}
	}
}

func TestValidatePlan_VerificationTestable_MindspecValidate(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "- [ ] `mindspec validate plan 039` passes")

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "bead-verification-testability" {
			t.Error("unexpected testability error when verification references mindspec validate")
		}
	}
}

func TestValidatePlan_VerificationNotTestable(t *testing.T) {
	tmp := t.TempDir()
	makePlanWithSections(t, tmp, "", true, true, true, "", "- [ ] Confirm it works correctly")

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "bead-verification-testability" {
			found = true
		}
	}
	if !found {
		t.Error("expected bead-verification-testability error for vague verification")
	}
}

func TestValidatePlan_VerificationMixed_OneTestable(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0o755)

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## ADR Fitness\n\nNone.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n## Bead 999-A: Test\n\n**Steps**:\n1. Step one\n2. Step two\n3. Step three\n\n**Verification**:\n- [ ] Confirm it looks right\n- [ ] `go test ./...` passes\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0o644)

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "bead-verification-testability" {
			t.Error("unexpected testability error when at least one verification item is testable")
		}
	}
}

// --- Spec 076: ParseBeadSections StepLines ---

func TestParseBeadSections_StepLines(t *testing.T) {
	content := `---
status: Draft
---

# Plan

## Bead 1: Widget

**Steps**
1. Create internal/widget/widget.go with Widget function
2. Add tests in internal/widget/widget_test.go
3. Wire into cmd/mindspec/root.go

**Verification**
- [ ] ` + "`go test ./internal/widget/...`" + ` passes

**Depends on**
None
`

	sections := ParseBeadSections(content)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if len(sections[0].StepLines) != 3 {
		t.Fatalf("expected 3 step lines, got %d", len(sections[0].StepLines))
	}
	if sections[0].StepLines[0] != "1. Create internal/widget/widget.go with Widget function" {
		t.Errorf("unexpected step line 0: %s", sections[0].StepLines[0])
	}
}

// --- Spec 078: Per-bead acceptance criteria ---

func TestParseBeadSections_AcceptanceCriteria(t *testing.T) {
	content := `---
status: Draft
---

# Plan

## Bead 1: Widget

**Steps**
1. Create widget
2. Add tests
3. Wire up

**Verification**
- [ ] ` + "`go test ./internal/widget/...`" + ` passes

**Acceptance Criteria**
- [ ] Widget frobs correctly
- [ ] Widget handles nil input

**Depends on**
None

## Bead 2: Gadget

**Steps**
1. Create gadget
2. Add tests
3. Wire up

**Verification**
- [ ] ` + "`go test ./internal/gadget/...`" + ` passes

**Depends on**
Bead 1
`

	sections := ParseBeadSections(content)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}

	// Bead 1 has AC
	if sections[0].AcceptanceCriteria == "" {
		t.Error("expected bead 1 to have acceptance criteria")
	}
	if sections[0].AcceptanceCriteria != "- [ ] Widget frobs correctly\n- [ ] Widget handles nil input" {
		t.Errorf("unexpected AC for bead 1: %q", sections[0].AcceptanceCriteria)
	}

	// Bead 2 has no AC
	if sections[1].AcceptanceCriteria != "" {
		t.Errorf("expected bead 2 to have empty acceptance criteria, got %q", sections[1].AcceptanceCriteria)
	}
}

func TestParseBeadSections_AcceptanceCriteria_H3(t *testing.T) {
	content := `---
status: Draft
---

# Plan

## Bead 1: Widget

**Steps**
1. Create widget
2. Add tests
3. Wire up

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

### Acceptance Criteria
- [ ] Widget works

**Depends on**
None
`

	sections := ParseBeadSections(content)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].AcceptanceCriteria != "- [ ] Widget works" {
		t.Errorf("unexpected AC: %q", sections[0].AcceptanceCriteria)
	}
}

// --- Spec 076: ExtractPathRefs ---

func TestExtractPathRefs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "go file path",
			input:    "Create internal/widget/widget.go with Widget function",
			expected: []string{"internal/widget/widget.go"},
		},
		{
			name:     "multiple paths",
			input:    "Modify cmd/mindspec/root.go and internal/validate/plan.go",
			expected: []string{"cmd/mindspec/root.go", "internal/validate/plan.go"},
		},
		{
			name:     "package path with dots",
			input:    "`go test ./internal/validate/...` passes",
			expected: []string{"./internal/validate/..."},
		},
		{
			name:     "dotted prefix",
			input:    "Edit ./internal/foo/bar.go",
			expected: []string{"./internal/foo/bar.go"},
		},
		{
			name:     "deduplication",
			input:    "internal/foo/bar.go and then internal/foo/bar.go again",
			expected: []string{"internal/foo/bar.go"},
		},
		{
			name:     "no paths",
			input:    "Just some plain text with no file references",
			expected: nil,
		},
		{
			name:     "package path no extension",
			input:    "Update internal/validate/plan module",
			expected: []string{"internal/validate/plan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPathRefs(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d paths, got %d: %v", len(tt.expected), len(got), got)
			}
			for i, exp := range tt.expected {
				if got[i] != exp {
					t.Errorf("path[%d]: expected %q, got %q", i, exp, got[i])
				}
			}
		})
	}
}

// --- Spec 039: Backwards compatibility ---

func TestValidatePlan_ApprovedPlan_SkipsNewChecks(t *testing.T) {
	tmp := t.TempDir()
	// Approved plan with no ADR Fitness, no Testing Strategy, no Provenance, vague verification
	makePlanWithSections(t, tmp, "", false, false, false, "Approved", "- [ ] Confirm it works")

	r := ValidatePlan(tmp, "999-test")

	newChecks := []string{
		"adr-fitness-missing",
		"adr-citations",
		"testing-strategy-missing",
		"provenance-missing",
		"bead-verification-testability",
	}

	// Include decomposition checks in the skip list
	newChecks = append(newChecks,
		"decomposition-bead-count",
		"decomposition-scope-redundancy",
		"decomposition-chain-depth",
		"decomposition-parallelism",
	)

	for _, issue := range r.Issues {
		for _, check := range newChecks {
			if issue.Name == check {
				t.Errorf("approved plan should skip new check %s, but got: [%s] %s", check, issue.Severity, issue.Message)
			}
		}
	}
}

// --- Spec 076: Decomposition quality checks ---

func TestDecompositionQuality_HighBeadCount(t *testing.T) {
	r := &Result{}
	sections := make([]BeadSection, 7)
	for i := range sections {
		sections[i] = BeadSection{
			Heading:      fmt.Sprintf("Bead %d: Task", i+1),
			StepsCount:   3,
			HasDependsOn: true,
			DependsOn:    "None",
		}
	}
	checkDecompositionQuality(r, sections)

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "decomposition-bead-count" {
			found = true
		}
	}
	if !found {
		t.Error("expected decomposition-bead-count warning for 7 beads")
	}
}

func TestDecompositionQuality_HighScopeRedundancy(t *testing.T) {
	r := &Result{}
	// All 3 beads reference the same files → R=1.0
	sections := []BeadSection{
		{
			Heading:      "Bead 1: A",
			StepLines:    []string{"1. Modify internal/validate/plan.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
		{
			Heading:      "Bead 2: B",
			StepLines:    []string{"1. Modify internal/validate/plan.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
		{
			Heading:      "Bead 3: C",
			StepLines:    []string{"1. Modify internal/validate/plan.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
	}
	checkDecompositionQuality(r, sections)

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "decomposition-scope-redundancy" && strings.Contains(issue.Message, "exceeds threshold 0.50") {
			found = true
		}
	}
	if !found {
		t.Error("expected scope redundancy warning for R > 0.50")
	}
}

func TestDecompositionQuality_LowScopeRedundancy(t *testing.T) {
	r := &Result{}
	// 3 beads, each with unique files → R=0.0
	sections := []BeadSection{
		{
			Heading:      "Bead 1: A",
			StepLines:    []string{"1. Create internal/foo/foo.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
		{
			Heading:      "Bead 2: B",
			StepLines:    []string{"1. Create internal/bar/bar.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
		{
			Heading:      "Bead 3: C",
			StepLines:    []string{"1. Create internal/baz/baz.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
	}
	checkDecompositionQuality(r, sections)

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "decomposition-scope-redundancy" && strings.Contains(issue.Message, "below threshold 0.15") {
			found = true
		}
	}
	if !found {
		t.Error("expected scope redundancy warning for R < 0.15 with >2 beads")
	}
}

func TestDecompositionQuality_DeepChain(t *testing.T) {
	r := &Result{}
	// Chain: Bead 1 → Bead 2 → Bead 3 → Bead 4 (depth 4)
	sections := []BeadSection{
		{Heading: "Bead 1: A", HasDependsOn: true, DependsOn: "None"},
		{Heading: "Bead 2: B", HasDependsOn: true, DependsOn: "Bead 1"},
		{Heading: "Bead 3: C", HasDependsOn: true, DependsOn: "Bead 2"},
		{Heading: "Bead 4: D", HasDependsOn: true, DependsOn: "Bead 3"},
	}
	checkDecompositionQuality(r, sections)

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "decomposition-chain-depth" {
			found = true
		}
	}
	if !found {
		t.Error("expected decomposition-chain-depth warning for chain depth > 3")
	}
}

func TestDecompositionQuality_LowParallelism(t *testing.T) {
	r := &Result{}
	// 4 beads, only 1 has zero inbound deps → parallelism = 0.25
	// Actually 0.25 is exactly at threshold, need < 0.25
	// 5 beads, only 1 root → 0.20
	sections := []BeadSection{
		{Heading: "Bead 1: A", HasDependsOn: true, DependsOn: "None"},
		{Heading: "Bead 2: B", HasDependsOn: true, DependsOn: "Bead 1"},
		{Heading: "Bead 3: C", HasDependsOn: true, DependsOn: "Bead 1"},
		{Heading: "Bead 4: D", HasDependsOn: true, DependsOn: "Bead 2"},
		{Heading: "Bead 5: E", HasDependsOn: true, DependsOn: "Bead 3"},
	}
	checkDecompositionQuality(r, sections)

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "decomposition-parallelism" {
			found = true
		}
	}
	if !found {
		t.Error("expected decomposition-parallelism warning for parallelism ratio < 0.25")
	}
}

func TestDecompositionQuality_NoWarnings(t *testing.T) {
	r := &Result{}
	// 3 beads, moderate overlap, shallow deps, good parallelism
	sections := []BeadSection{
		{
			Heading:      "Bead 1: A",
			StepLines:    []string{"1. Create internal/foo/foo.go", "2. Update internal/shared/util.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
		{
			Heading:      "Bead 2: B",
			StepLines:    []string{"1. Create internal/bar/bar.go", "2. Update internal/shared/util.go"},
			HasDependsOn: true,
			DependsOn:    "None",
		},
		{
			Heading:      "Bead 3: C",
			StepLines:    []string{"1. Wire cmd/mindspec/root.go", "2. Update internal/shared/util.go"},
			HasDependsOn: true,
			DependsOn:    "Bead 1",
		},
	}
	checkDecompositionQuality(r, sections)

	decompositionChecks := []string{
		"decomposition-bead-count",
		"decomposition-scope-redundancy",
		"decomposition-chain-depth",
		"decomposition-parallelism",
	}
	for _, issue := range r.Issues {
		for _, check := range decompositionChecks {
			if issue.Name == check {
				t.Errorf("unexpected decomposition warning: %s: %s", issue.Name, issue.Message)
			}
		}
	}
}
