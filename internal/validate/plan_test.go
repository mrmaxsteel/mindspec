package validate

import (
	"fmt"
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

func TestValidatePlan_ApprovedFrontmatterOpenGateWarns(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	spec := `---
molecule_id: mol-1
step_mapping:
  plan-approve: gate-plan
---
# Spec
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644)

	plan := `---
status: Approved
spec_id: "999-test"
version: "1.0"
---

# Plan

## Bead 999-A: Test

**Steps**:
1. Step one
2. Step two
3. Step three

**Verification**:
- [ ] ` + "`go test ./...` passes" + `

**Depends on**: nothing
`
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	orig := runBDGateStatusFn
	defer func() { runBDGateStatusFn = orig }()
	runBDGateStatusFn = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "show" && args[1] == "gate-plan" {
			return []byte(`[{"status":"open"}]`), nil
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}

	r := ValidatePlan(tmp, "999-test")
	found := false
	for _, issue := range r.Issues {
		if issue.Name == "plan-gate-consistency" {
			found = true
			if issue.Severity != SevWarning {
				t.Errorf("expected warning severity, got %s", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected plan-gate-consistency warning")
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
	if len(sections[0].verifyLines) != 2 {
		t.Errorf("bead A: expected 2 verification lines, got %d", len(sections[0].verifyLines))
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

	for _, issue := range r.Issues {
		for _, check := range newChecks {
			if issue.Name == check {
				t.Errorf("approved plan should skip new check %s, but got: [%s] %s", check, issue.Severity, issue.Message)
			}
		}
	}
}
