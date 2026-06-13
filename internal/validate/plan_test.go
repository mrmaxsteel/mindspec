package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/config"
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
	// Note: 005-next is a pre-080 plan without per-bead AC (grandfathered)
	// Note: 005-next is a pre-087 plan whose Superseded ADR-0005 chain
	// head (ADR-0015) is not cited, so the new semantic-coverage gate
	// (Rev 4 fixup removed the Approved-plan skip) surfaces
	// `adr-coverage-missing` for the `core` domain. Grandfathered for the
	// same reason as `bead-acceptance-criteria` — the test asserts the
	// validator's structural shape, not domain-graph completeness on
	// pre-existing plans.
	allowedErrors := map[string]bool{
		"bead-id-missing":            true,
		"bead-acceptance-criteria":   true,
		"adr-coverage-missing":       true,
		"adr-cite-irrelevant":        true,
		"adr-supersede-chain-broken": true,
	}
	for _, issue := range r.Issues {
		if issue.Severity == SevError && !allowedErrors[issue.Name] {
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
	// ZFC-4 (mindspec-d78q): StepsCount < 3 is now a WARNING, not an error.
	// A 1-step bead validates (no failure) but surfaces a bead-steps warning.
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## ADR Fitness\n\nNone.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n## Bead 999-A: Test\n\n**Scope**: Do something\n\n**Acceptance Criteria**:\n- [ ] It works\n\n**Steps**:\n1. Only one step\n\n**Verification**:\n- [ ] `go test` passes\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	r := ValidatePlan(tmp, "999-test")
	if r.HasFailures() {
		t.Errorf("expected no failure for 1-step bead (warning only), got failures: %v", r.Issues)
	}

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "bead-steps" {
			if issue.Severity != SevWarning {
				t.Errorf("expected bead-steps to be WARNING, got %s", issue.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected bead-steps warning for 1-step bead")
	}
}

func TestValidatePlan_BeadOneStep_Passes(t *testing.T) {
	// ZFC-4 (mindspec-d78q): a 1-step bead, otherwise complete (verification,
	// AC, depends on) must validate without failures.
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## ADR Fitness\n\nNone.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n## Bead 999-A: Test\n\n**Scope**: Do something\n\n**Acceptance Criteria**:\n- [ ] It works\n\n**Steps**:\n1. Only one step\n\n**Verification**:\n- [ ] `go test` passes\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	r := ValidatePlan(tmp, "999-test")
	if r.HasFailures() {
		t.Errorf("expected 1-step bead to validate without failures, got: %v", r.Issues)
	}
}

func TestValidatePlan_BeadZeroSteps_Errors(t *testing.T) {
	// ZFC-4 (mindspec-d78q): newly-added structural floor — a **Steps**
	// heading with zero numbered items is malformed and must error.
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	plan := "---\nstatus: Draft\nspec_id: \"999-test\"\nversion: \"1.0\"\n---\n\n# Plan\n\n## ADR Fitness\n\nNone.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n## Bead 999-A: Test\n\n**Scope**: Do something\n\n**Acceptance Criteria**:\n- [ ] It works\n\n**Steps**:\n\n**Verification**:\n- [ ] `go test` passes\n\n**Depends on**: nothing\n"
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0644)

	r := ValidatePlan(tmp, "999-test")
	if !r.HasFailures() {
		t.Errorf("expected failure for 0-step bead, got: %v", r.Issues)
	}

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "bead-steps" {
			if issue.Severity != SevError {
				t.Errorf("expected bead-steps to be ERROR for 0 steps, got %s", issue.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected bead-steps error for 0-step bead")
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

func TestParsePlanFrontmatter_ScalarADRCitations(t *testing.T) {
	content := "---\nstatus: Draft\nspec_id: \"005\"\nversion: \"1.0\"\nadr_citations:\n  - ADR-0001\n  - ADR-0002\n---\n\n# Plan\n"

	fm, err := parsePlanFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.ADRCitations) != 2 {
		t.Fatalf("expected 2 ADR citations, got %d", len(fm.ADRCitations))
	}
	if fm.ADRCitations[0].ID != "ADR-0001" || fm.ADRCitations[1].ID != "ADR-0002" {
		t.Errorf("unexpected citation IDs: %+v", fm.ADRCitations)
	}
}

func TestParsePlanFrontmatter_MixedADRCitations(t *testing.T) {
	content := "---\nstatus: Draft\nspec_id: \"005\"\nversion: \"1.0\"\nadr_citations:\n  - ADR-0001\n  - id: ADR-0002\n    sections: [\"CLI\"]\n---\n\n# Plan\n"

	fm, err := parsePlanFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.ADRCitations) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(fm.ADRCitations))
	}
	if fm.ADRCitations[0].ID != "ADR-0001" || len(fm.ADRCitations[0].Sections) != 0 {
		t.Errorf("scalar citation not decoded: %+v", fm.ADRCitations[0])
	}
	if fm.ADRCitations[1].ID != "ADR-0002" || len(fm.ADRCitations[1].Sections) != 1 {
		t.Errorf("map citation not decoded: %+v", fm.ADRCitations[1])
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

// --- Bead verification testability (Spec 039, removed as ZFC violation) ---
//
// The deterministic `bead-verification-testability` check was removed: it
// encoded a framework keyword allowlist (`go test`, `pytest`, `_test.go`, …),
// which is the exact "keyword-based routing" that Yegge flags as a Zero
// Framework Cognition violation. Quality judgement of verification items
// (is it concrete? is it testable?) now lives in the plan-approve AI review
// and in the plan instruct template — the validator only enforces the
// structural requirement that the **Verification** section exists with at
// least one checkbox item (covered by TestValidatePlan_BeadMissingVerification).
//
// A regression test pins this behaviour: a plan with vague verification
// ("Confirm it works correctly") must still pass structural validation —
// failing it would reintroduce the cognitive heuristic.

func TestValidatePlan_VerificationTestability_IsNotEnforced(t *testing.T) {
	cases := []struct {
		name       string
		verifyLine string
	}{
		{"vague prose", "- [ ] Confirm it works correctly"},
		{"rust", "- [ ] `cargo test --package foo` passes"},
		{"ruby", "- [ ] `bundle exec rspec spec/foo_spec.rb` green"},
		{"elixir", "- [ ] `mix test test/foo_test.exs` passes"},
		{"swift", "- [ ] `swift test --filter FooTests` green"},
		{"dotnet", "- [ ] `dotnet test --filter FullyQualifiedName~FooTests` green"},
		{"bazel", "- [ ] `bazel test //pkg/foo:all` passes"},
		{"http no backticks", "- [ ] The GET /healthz endpoint returns 200"},
		{"plain path", "- [ ] New file src/foo/bar.rb exists and is imported"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			makePlanWithSections(t, tmp, "", true, true, true, "", tc.verifyLine)

			r := ValidatePlan(tmp, "999-test")

			for _, issue := range r.Issues {
				if issue.Name == "bead-verification-testability" {
					t.Errorf("unexpected testability error for %s: the validator must not judge verification quality (ZFC). Got: %s", tc.name, issue.Message)
				}
			}
		})
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
	checkDecompositionQuality(r, sections, nil, config.DefaultConfig().Decomposition)

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
	checkDecompositionQuality(r, sections, nil, config.DefaultConfig().Decomposition)

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
	checkDecompositionQuality(r, sections, nil, config.DefaultConfig().Decomposition)

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
	chunks := []WorkChunk{
		{ID: 1},
		{ID: 2, DependsOn: []int{1}},
		{ID: 3, DependsOn: []int{2}},
		{ID: 4, DependsOn: []int{3}},
	}
	checkDecompositionQuality(r, sections, chunks, config.DefaultConfig().Decomposition)

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
	chunks := []WorkChunk{
		{ID: 1},
		{ID: 2, DependsOn: []int{1}},
		{ID: 3, DependsOn: []int{1}},
		{ID: 4, DependsOn: []int{2}},
		{ID: 5, DependsOn: []int{3}},
	}
	checkDecompositionQuality(r, sections, chunks, config.DefaultConfig().Decomposition)

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
	chunks := []WorkChunk{
		{ID: 1},
		{ID: 2},
		{ID: 3, DependsOn: []int{1}},
	}
	checkDecompositionQuality(r, sections, chunks, config.DefaultConfig().Decomposition)

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

// TestDecompositionQuality_ConfigOverride confirms the thresholds are honored
// from config: raising MaxBeads silences a warning that defaults would fire,
// and lowering MaxChainDepth fires a warning that defaults would silence.
func TestDecompositionQuality_ConfigOverride(t *testing.T) {
	t.Run("raise_max_beads_silences_warning", func(t *testing.T) {
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
		cfg := config.DefaultConfig().Decomposition
		cfg.MaxBeads = 100
		checkDecompositionQuality(r, sections, nil, cfg)

		for _, issue := range r.Issues {
			if issue.Name == "decomposition-bead-count" {
				t.Errorf("did not expect decomposition-bead-count warning with MaxBeads=100, got: %s", issue.Message)
			}
		}
	})

	t.Run("lower_max_chain_depth_triggers_warning", func(t *testing.T) {
		r := &Result{}
		// Chain depth 2: Bead 1 → Bead 2. Defaults (MaxChainDepth=3) would
		// not warn; lowering to 1 should.
		sections := []BeadSection{
			{Heading: "Bead 1: A", HasDependsOn: true, DependsOn: "None"},
			{Heading: "Bead 2: B", HasDependsOn: true, DependsOn: "Bead 1"},
		}
		chunks := []WorkChunk{
			{ID: 1},
			{ID: 2, DependsOn: []int{1}},
		}
		cfg := config.DefaultConfig().Decomposition
		cfg.MaxChainDepth = 1
		checkDecompositionQuality(r, sections, chunks, cfg)

		found := false
		for _, issue := range r.Issues {
			if issue.Name == "decomposition-chain-depth" {
				found = true
				if !strings.Contains(issue.Message, "threshold 1") {
					t.Errorf("expected threshold 1 in message, got: %s", issue.Message)
				}
			}
		}
		if !found {
			t.Error("expected decomposition-chain-depth warning with MaxChainDepth=1")
		}
	})
}

// TestDecompositionQuality_ConsumesWorkChunks proves the dependency graph is
// built from the structured `work_chunks` deps, not the prose "Depends on Bead
// N" text (spec 097 R3). The sections declare a deep prose chain (Bead 1 → 2 →
// 3 → 4) but NO work_chunks are passed, so the chain-depth warning must NOT
// fire. RED on revert: the retired `beadDepRe` prose scrape would read the
// prose chain and warn at depth 4.
func TestDecompositionQuality_ConsumesWorkChunks(t *testing.T) {
	r := &Result{}
	sections := []BeadSection{
		{Heading: "Bead 1: A", HasDependsOn: true, DependsOn: "None"},
		{Heading: "Bead 2: B", HasDependsOn: true, DependsOn: "Bead 1"},
		{Heading: "Bead 3: C", HasDependsOn: true, DependsOn: "Bead 2"},
		{Heading: "Bead 4: D", HasDependsOn: true, DependsOn: "Bead 3"},
	}
	// No structured work_chunks → no dependency edges → no deep chain.
	checkDecompositionQuality(r, sections, nil, config.DefaultConfig().Decomposition)

	for _, issue := range r.Issues {
		if issue.Name == "decomposition-chain-depth" {
			t.Errorf("chain-depth warning fired from prose deps; the check must consume work_chunks only: %s", issue.Message)
		}
	}
}

// TestValidateWorkChunkAlignment exercises the spec 097 R3 alignment guard that
// protects the positional bead_ids[N-1] wiring from misaligned/out-of-range
// chunk ids.
func TestValidateWorkChunkAlignment(t *testing.T) {
	tests := []struct {
		name        string
		chunks      []WorkChunk
		numSections int
		wantErr     bool
	}{
		{"empty is trivially aligned", nil, 3, false},
		{"contiguous aligned", []WorkChunk{{ID: 1}, {ID: 2, DependsOn: []int{1}}, {ID: 3, DependsOn: []int{1, 2}}}, 3, false},
		{"count mismatch", []WorkChunk{{ID: 1}, {ID: 2}}, 3, true},
		{"non-contiguous gap", []WorkChunk{{ID: 1}, {ID: 3}, {ID: 4}}, 3, true},
		{"duplicate id", []WorkChunk{{ID: 1}, {ID: 1}, {ID: 3}}, 3, true},
		{"id out of range", []WorkChunk{{ID: 0}, {ID: 1}, {ID: 2}}, 3, true},
		{"depends_on out of range", []WorkChunk{{ID: 1}, {ID: 2, DependsOn: []int{9}}, {ID: 3}}, 3, true},
		{"self dependency", []WorkChunk{{ID: 1}, {ID: 2, DependsOn: []int{2}}, {ID: 3}}, 3, true},
		{"two-node cycle", []WorkChunk{{ID: 1, DependsOn: []int{2}}, {ID: 2, DependsOn: []int{1}}}, 2, true},
		{"three-node cycle", []WorkChunk{{ID: 1, DependsOn: []int{3}}, {ID: 2, DependsOn: []int{1}}, {ID: 3, DependsOn: []int{2}}}, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkChunkAlignment(tt.chunks, tt.numSections)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateWorkChunkAlignment() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateWorkChunkAlignment_CycleMessage proves the approve-side guard
// rejects a cyclic work_chunks graph with a clear, path-bearing error so a
// cyclic plan is caught BEFORE any `bd dep add` wires it (spec 097 R3,
// bc5-edge). 1 depends_on [2], 2 depends_on [1] is the canonical 2-cycle.
func TestValidateWorkChunkAlignment_CycleMessage(t *testing.T) {
	chunks := []WorkChunk{
		{ID: 1, DependsOn: []int{2}},
		{ID: 2, DependsOn: []int{1}},
	}
	err := ValidateWorkChunkAlignment(chunks, 2)
	if err == nil {
		t.Fatal("expected a cycle error for 1<->2, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected a 'cycle' error, got: %v", err)
	}
	// The message renders the closed loop, e.g. "1 -> 2 -> 1".
	if !strings.Contains(err.Error(), "->") {
		t.Errorf("expected the cycle path to render with '->', got: %v", err)
	}
}

// TestCheckDecompositionQuality_CyclicGraphNoStackOverflow proves the
// validate-side path is cycle-SAFE (spec 097 R3, bc5-edge): a cyclic
// work_chunks graph reaches checkDecompositionQuality (which does NOT call
// ValidateWorkChunkAlignment first), and the longest-path walk must return
// gracefully — emitting a clean advisory finding — instead of recursing
// forever and stack-overflowing. The recover() guard fails loudly if the
// walk ever panics.
func TestCheckDecompositionQuality_CyclicGraphNoStackOverflow(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("checkDecompositionQuality panicked on a cyclic graph: %v", rec)
		}
	}()

	sections := []BeadSection{
		{StepLines: []string{"do a"}, VerifyLines: []string{"check a"}},
		{StepLines: []string{"do b"}, VerifyLines: []string{"check b"}},
		{StepLines: []string{"do c"}, VerifyLines: []string{"check c"}},
	}
	// 1 -> 2 -> 3 -> 1 : a 3-cycle that would stack-overflow an unguarded
	// recursive longest-path walk.
	chunks := []WorkChunk{
		{ID: 1, DependsOn: []int{3}},
		{ID: 2, DependsOn: []int{1}},
		{ID: 3, DependsOn: []int{2}},
	}

	r := &Result{}
	checkDecompositionQuality(r, sections, chunks, config.DefaultConfig().Decomposition)

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "decomposition-dep-cycle" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a 'decomposition-dep-cycle' advisory finding, got issues: %v", r.Issues)
	}
}

// TestComputeChainDepth_CycleSafe is a direct, bounded unit test on the
// recursion itself: a back-edge must be detected (hasCycle == true) and the
// call must return rather than overflow the stack.
func TestComputeChainDepth_CycleSafe(t *testing.T) {
	// 0 -> 1 -> 2 -> 0 (a 3-cycle in 0-indexed adjacency form).
	adj := map[int][]int{
		0: {1},
		1: {2},
		2: {0},
	}
	depth, hasCycle := computeChainDepth(adj, 3)
	if !hasCycle {
		t.Errorf("expected hasCycle = true for a cyclic graph, got false (depth=%d)", depth)
	}
}

// --- Spec 087 Bead 1: ADR semantic gates ---

// writeTestADRWithDomains writes an ADR with a custom Domain(s) value.
// Used by Spec 087 tests where the cite-relevant / coverage checks
// depend on the cited ADR's declared domains.
func writeTestADRWithDomains(t *testing.T, root, id, status, domains, supersededBy string) {
	t.Helper()
	adrDir := filepath.Join(root, "docs", "adr")
	os.MkdirAll(adrDir, 0o755)

	supBy := "n/a"
	if supersededBy != "" {
		supBy = supersededBy
	}
	content := "# " + id + ": Test\n\n- **Status**: " + status + "\n- **Domain(s)**: " + domains + "\n- **Supersedes**: n/a\n- **Superseded-by**: " + supBy + "\n\n## Decision\nSome decision.\n"
	os.WriteFile(filepath.Join(adrDir, id+".md"), []byte(content), 0o644)
}

// writeTestSpec writes a minimal spec.md with the given impacted-domains.
// Spec 087 plan-time checks consult spec.md via contextpack.ParseSpec to
// resolve the impacted-domains set.
func writeTestSpec(t *testing.T, root string, impactedDomains []string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0o755)

	var b strings.Builder
	b.WriteString("# Spec 999-test\n\n## Goal\n\nTest spec.\n\n## Impacted Domains\n\n")
	for _, d := range impactedDomains {
		b.WriteString("- " + d + ": touched by tests\n")
	}
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(b.String()), 0o644)
}

func TestPlanRejectsIrrelevantADRCitation(t *testing.T) {
	// Spec impacted=[payments], ADR Domains=[search]. The intersection
	// is empty, so the cite-relevant check must surface adr-cite-irrelevant.
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "search", "")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-cite-irrelevant" && issue.Severity == SevError {
			found = true
			if !strings.Contains(issue.Message, "ADR-0001") {
				t.Errorf("expected message to mention ADR-0001, got: %s", issue.Message)
			}
			if !strings.Contains(issue.Message, "payments") {
				t.Errorf("expected message to mention impacted domain payments, got: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected adr-cite-irrelevant error, got issues: %v", r.Issues)
	}
}

func TestPlanRejectsUncoveredDomain(t *testing.T) {
	// Spec impacted=[payments], cited ADR covers [search] — payments
	// has no covering Accepted ADR, must error with the create-domain hint.
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "search", "")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-coverage-missing" && issue.Severity == SevError {
			found = true
			if !strings.Contains(issue.Message, "payments") {
				t.Errorf("expected message to mention payments, got: %s", issue.Message)
			}
			if !strings.Contains(issue.Message, "mindspec adr create --domain payments") {
				t.Errorf("expected message to include actionable hint `mindspec adr create --domain payments`, got: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected adr-coverage-missing error, got issues: %v", r.Issues)
	}
}

func TestPlanSpecWorktreeADRVisible(t *testing.T) {
	// mindspec-ew79: an ADR that exists ONLY on the spec branch (inside
	// the spec worktree at root/.worktrees/worktree-spec-<id>/) must be
	// visible to plan-time citation + coverage checks run from the
	// primary checkout. Before the overlay store this fired spurious
	// adr-cite-missing / adr-coverage-missing because the validator
	// always read ADRs from the primary tree.
	tmp := t.TempDir()
	wtTree := filepath.Join(tmp, ".worktrees", "worktree-spec-999-test")
	// Reuse the standard fixture helpers rooted at the worktree's
	// .mindspec dir: they write to <arg>/docs/specs/999-test and
	// <arg>/docs/adr, which lands at the canonical worktree layout
	// .worktrees/worktree-spec-999-test/.mindspec/docs/... that
	// workspace.SpecDir resolves first (ADR-0022).
	wtMindspec := filepath.Join(wtTree, ".mindspec")
	writeTestSpec(t, wtMindspec, []string{"payments"})
	writeTestADRWithDomains(t, wtMindspec, "ADR-0001", "Accepted", "payments", "")
	makePlanWithCitations(t, wtMindspec, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "adr-cite-missing" {
			t.Errorf("unexpected adr-cite-missing for spec-branch ADR: %s", issue.Message)
		}
		if issue.Name == "adr-coverage-missing" {
			t.Errorf("unexpected adr-coverage-missing for spec-branch ADR: %s", issue.Message)
		}
	}
}

func TestPlanSpecWorktreeADRMissingEverywhereStillFails(t *testing.T) {
	// Acceptance criterion companion: an ADR cited from a worktree-
	// resolved plan that exists in NEITHER tree must still fail.
	tmp := t.TempDir()
	wtMindspec := filepath.Join(tmp, ".worktrees", "worktree-spec-999-test", ".mindspec")
	writeTestSpec(t, wtMindspec, []string{"payments"})
	makePlanWithCitations(t, wtMindspec, "  - id: ADR-0042\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-cite-missing" && strings.Contains(issue.Message, "ADR-0042") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected adr-cite-missing for ADR absent from both trees, got: %v", r.Issues)
	}
}

func TestPlanCoverageAcceptsQualifiedAcceptedStatus(t *testing.T) {
	// mindspec-f115: an ADR whose Status line carries a provenance
	// qualifier — the live ADR-0029 case "Accepted (Finalized in spec
	// 090 Bead 1)" — must still satisfy coverage. Before parse-time
	// normalization this fired a spurious adr-coverage-missing.
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted (Finalized in spec 090 Bead 1)", "payments", "")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "adr-coverage-missing" {
			t.Errorf("unexpected adr-coverage-missing for qualified Accepted status: %s", issue.Message)
		}
	}
}

func TestPlanCoverageProposedCitedDowngradesToWarning(t *testing.T) {
	// mindspec-53qx: a CITED Proposed ADR covering an impacted domain
	// suppresses adr-coverage-missing and instead emits the advisory
	// adr-coverage-proposed warning. This deliberately reverses the
	// spec-087 "revision 11" Proposed-exclusion (see coverageOf docs):
	// spec-introduced ADRs are legitimately Proposed until post-impl
	// validation, and citing them is the explicit opt-in.
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Proposed", "payments", "")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	var hasProposedWarning, hasCiteProposed bool
	for _, issue := range r.Issues {
		if issue.Name == "adr-coverage-missing" {
			t.Errorf("unexpected adr-coverage-missing for cited Proposed covering ADR: %s", issue.Message)
		}
		if issue.Name == "adr-coverage-proposed" {
			hasProposedWarning = true
			if issue.Severity != SevWarning {
				t.Errorf("adr-coverage-proposed severity = %v, want warning (advisory, never gates)", issue.Severity)
			}
			for _, want := range []string{"payments", "ADR-0001", "flip it to Accepted"} {
				if !strings.Contains(issue.Message, want) {
					t.Errorf("adr-coverage-proposed message missing %q: %s", want, issue.Message)
				}
			}
		}
		if issue.Name == "adr-cite-proposed" {
			hasCiteProposed = true
		}
	}
	if !hasProposedWarning {
		t.Errorf("expected adr-coverage-proposed warning, got: %v", r.Issues)
	}
	// The existing per-citation adr-cite-proposed warning is preserved.
	if !hasCiteProposed {
		t.Errorf("expected adr-cite-proposed warning to be preserved, got: %v", r.Issues)
	}
}

func TestPlanCoverageProposedUncitedStillMissing(t *testing.T) {
	// mindspec-53qx companion: an UNCITED Proposed ADR covering the
	// domain does NOT satisfy coverage — citation is the opt-in.
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments", "search"})
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "search", "")
	writeTestADRWithDomains(t, tmp, "ADR-0002", "Proposed", "payments", "")
	// Only the search ADR is cited; the Proposed payments ADR is not.
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	foundMissing := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-coverage-missing" && strings.Contains(issue.Message, "payments") {
			foundMissing = true
		}
		if issue.Name == "adr-coverage-proposed" {
			t.Errorf("unexpected adr-coverage-proposed for uncited Proposed ADR: %s", issue.Message)
		}
	}
	if !foundMissing {
		t.Errorf("expected adr-coverage-missing for payments (Proposed ADR uncited), got: %v", r.Issues)
	}
}

func TestPlanCoverageAcceptedUpgradesOverProposed(t *testing.T) {
	// When BOTH a Proposed and an Accepted cited ADR cover the domain,
	// the Accepted one wins: no adr-coverage-proposed noise.
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Proposed", "payments", "")
	writeTestADRWithDomains(t, tmp, "ADR-0002", "Accepted", "payments", "")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n  - id: ADR-0002\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	for _, issue := range r.Issues {
		if issue.Name == "adr-coverage-missing" || issue.Name == "adr-coverage-proposed" {
			t.Errorf("unexpected %s when an Accepted ADR also covers: %s", issue.Name, issue.Message)
		}
	}
}

func TestSupersededADRDoesNotSatisfyCoverage(t *testing.T) {
	// ADR-0001 (Superseded by ADR-0002) is cited, but ADR-0002 is NOT
	// cited — coverage must NOT be satisfied. ADR-0002 itself exists
	// and is Accepted+covering, but the lack of citation breaks the
	// transitive coverage rule per IsDomainCovered.
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Superseded", "payments", "ADR-0002")
	writeTestADRWithDomains(t, tmp, "ADR-0002", "Accepted", "payments", "")
	// Only cite ADR-0001 (the superseded one).
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-coverage-missing" && strings.Contains(issue.Message, "payments") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected adr-coverage-missing error when only the superseded ADR is cited, got: %v", r.Issues)
	}
}

func TestIsDomainCovered_SupersededWithCitedHeadIsCovered(t *testing.T) {
	// Companion to TestSupersededADRDoesNotSatisfyCoverage: when BOTH
	// the superseded ADR and the Accepted chain head are cited,
	// IsDomainCovered returns true. This pins the transitive coverage
	// rule for Bead 2's divergence consumer.
	tmp := t.TempDir()
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Superseded", "payments", "ADR-0002")
	writeTestADRWithDomains(t, tmp, "ADR-0002", "Accepted", "payments", "")

	store := adr.NewFileStore(tmp)
	citations := []ADRCitation{{ID: "ADR-0001"}, {ID: "ADR-0002"}}
	if !IsDomainCovered(store, citations, "payments") {
		t.Error("expected payments to be covered when superseded ADR and chain head are both cited")
	}

	// Conversely, citing only the superseded ADR must NOT satisfy.
	citations = []ADRCitation{{ID: "ADR-0001"}}
	if IsDomainCovered(store, citations, "payments") {
		t.Error("expected payments NOT to be covered when only the superseded ADR is cited")
	}
}

func TestSupersedeChainCycleDetected(t *testing.T) {
	// ADR-A.SupersededBy = ADR-B, ADR-B.SupersededBy = ADR-A. The walker
	// must terminate and surface an adr-supersede-cycle error.
	tmp := t.TempDir()
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Superseded", "core", "ADR-0002")
	writeTestADRWithDomains(t, tmp, "ADR-0002", "Superseded", "core", "ADR-0001")

	store := adr.NewFileStore(tmp)
	_, err := walkSupersededChain(store, "ADR-0001")
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "adr-supersede-cycle") {
		t.Errorf("expected error to mention adr-supersede-cycle, got: %v", err)
	}
}

// --- Spec 087 Bead 1 (Rev 1 fixup): walker errors surface through ValidatePlan ---

// TestPlanReportsSupersedeChainCycle exercises the integration: when a
// plan cites a Superseded ADR whose chain CYCLES, ValidatePlan must
// surface the `adr-supersede-cycle` error on the Result (not just
// `adr-coverage-missing`). Pins Rev 1 of the bead-zy4u.1 fixup.
func TestPlanReportsSupersedeChainCycle(t *testing.T) {
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})
	// A → B → A: cycle.
	writeTestADRWithDomains(t, tmp, "ADR-0001", "Superseded", "payments", "ADR-0002")
	writeTestADRWithDomains(t, tmp, "ADR-0002", "Superseded", "payments", "ADR-0001")
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	foundCycle := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-supersede-cycle" && issue.Severity == SevError {
			foundCycle = true
			if !strings.Contains(issue.Message, "ADR-0001") && !strings.Contains(issue.Message, "ADR-0002") {
				t.Errorf("expected cycle message to mention an ADR in the chain, got: %s", issue.Message)
			}
		}
	}
	if !foundCycle {
		t.Errorf("expected adr-supersede-cycle error on Result, got issues: %v", r.Issues)
	}
}

// TestPlanReportsSupersedeChainTooLong is the length-cap companion: a
// 12-hop chain triggers `adr-supersede-chain-too-long` and that error
// must surface through ValidatePlan, not be swallowed by the
// IsDomainCovered predicate. Pins Rev 1 of the bead-zy4u.1 fixup.
func TestPlanReportsSupersedeChainTooLong(t *testing.T) {
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments"})

	const total = 12
	for i := 1; i <= total; i++ {
		id := fmt.Sprintf("ADR-%04d", i)
		next := ""
		status := "Superseded"
		if i < total {
			next = fmt.Sprintf("ADR-%04d", i+1)
		} else {
			status = "Accepted"
		}
		writeTestADRWithDomains(t, tmp, id, status, "payments", next)
	}
	// Cite only the chain start.
	makePlanWithCitations(t, tmp, "  - id: ADR-0001\n    sections: [\"CLI\"]\n", true)

	r := ValidatePlan(tmp, "999-test")

	foundTooLong := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-supersede-chain-too-long" && issue.Severity == SevError {
			foundTooLong = true
		}
	}
	if !foundTooLong {
		t.Errorf("expected adr-supersede-chain-too-long error on Result, got issues: %v", r.Issues)
	}
}

// --- Spec 087 Bead 1 (Rev 2 fixup): coverage runs on empty citations ---

// TestPlanCoverageRunsOnEmptyCitations: a plan with non-empty impacted
// domains but ZERO ADR citations must emit `adr-coverage-missing` for
// every impacted domain. Previously the gate sat inside the
// `len(citations) != 0` branch and silently passed.
func TestPlanCoverageRunsOnEmptyCitations(t *testing.T) {
	tmp := t.TempDir()
	writeTestSpec(t, tmp, []string{"payments", "search"})
	// Empty citations, ADR Fitness present so the citations check itself only warns.
	makePlanWithCitations(t, tmp, "", true)

	r := ValidatePlan(tmp, "999-test")

	gotPayments := false
	gotSearch := false
	for _, issue := range r.Issues {
		if issue.Name == "adr-coverage-missing" {
			if strings.Contains(issue.Message, "payments") {
				gotPayments = true
			}
			if strings.Contains(issue.Message, "search") {
				gotSearch = true
			}
		}
	}
	if !gotPayments || !gotSearch {
		t.Errorf("expected adr-coverage-missing for both payments and search; got payments=%v search=%v, issues=%v", gotPayments, gotSearch, r.Issues)
	}
}

func TestSupersedeChainTooLong(t *testing.T) {
	// Chain of 12 ADRs (11 SupersededBy hops) — exceeds the 10-hop cap
	// and must surface adr-supersede-chain-too-long. The walker visits
	// at most maxLen+1 nodes before giving up.
	tmp := t.TempDir()
	const total = 12
	for i := 1; i <= total; i++ {
		id := fmt.Sprintf("ADR-%04d", i)
		next := ""
		status := "Superseded"
		if i < total {
			next = fmt.Sprintf("ADR-%04d", i+1)
		} else {
			// terminal link is Accepted with no successor
			status = "Accepted"
		}
		writeTestADRWithDomains(t, tmp, id, status, "core", next)
	}

	store := adr.NewFileStore(tmp)
	_, err := walkSupersededChain(store, "ADR-0001")
	if err == nil {
		t.Fatal("expected too-long error, got nil")
	}
	if !strings.Contains(err.Error(), "adr-supersede-chain-too-long") {
		t.Errorf("expected error to mention adr-supersede-chain-too-long, got: %v", err)
	}
}
