package approve

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

// TestPlanLintDoubleAssignedFiles_UnitLevel pins planLintDoubleAssignedFiles
// directly against validate.ParseBeadSections output — the spec-118 plan
// panel's exact real-world case (a helper assigned to both Bead 1 and
// Bead 2's steps) that motivated mindspec-jli8's plan-lint leg.
func TestPlanLintDoubleAssignedFiles_UnitLevel(t *testing.T) {
	content := `## Bead 1: First thing

**Steps**
1. Add ` + "`internal/shared/helper.go`" + ` with the new logic.
2. Wire it into ` + "`internal/approve/plan.go`" + `.

**Verification**
- [ ] tests pass

**Acceptance Criteria**
- Works

## Bead 2: Second thing

**Steps**
1. Consume ` + "`internal/shared/helper.go`" + ` from the executor.
2. Update ` + "`internal/executor/mindspec_executor.go`" + ` only.

**Verification**
- [ ] tests pass

**Acceptance Criteria**
- Works
`
	sections := validate.ParseBeadSections(content)
	if len(sections) != 2 {
		t.Fatalf("expected 2 parsed bead sections, got %d", len(sections))
	}

	warnings := planLintDoubleAssignedFiles(sections)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "internal/shared/helper.go") &&
			strings.Contains(w, "Bead 1") && strings.Contains(w, "Bead 2") {
			found = true
		}
		// The single-bead files must NOT be flagged.
		if strings.Contains(w, "internal/approve/plan.go") || strings.Contains(w, "internal/executor/mindspec_executor.go") {
			t.Errorf("single-bead file wrongly flagged as double-assigned: %q", w)
		}
	}
	if !found {
		t.Errorf("expected a plan-lint finding naming internal/shared/helper.go and both Bead 1/Bead 2, got: %v", warnings)
	}
}

// TestPlanLintDoubleAssignedFiles_NoFalsePositive verifies a plan where
// every file is assigned to exactly one bead's steps produces no finding
// at all — the check must not flag legitimate, disjoint per-bead scope.
func TestPlanLintDoubleAssignedFiles_NoFalsePositive(t *testing.T) {
	content := `## Bead 1: First thing

**Steps**
1. Add ` + "`internal/foo/a.go`" + `.

**Acceptance Criteria**
- Works

## Bead 2: Second thing

**Steps**
1. Add ` + "`internal/foo/b.go`" + `.

**Acceptance Criteria**
- Works
`
	sections := validate.ParseBeadSections(content)
	warnings := planLintDoubleAssignedFiles(sections)
	if len(warnings) != 0 {
		t.Errorf("expected no plan-lint findings for disjoint per-bead files, got: %v", warnings)
	}
}

// TestPlanLintDoubleAssignedFiles_TermsafeEscaped pins the R11/spec-116
// requirement directly at the unit level: a hostile bead HEADING carrying
// control bytes (e.g. an ESC/CSI sequence attempting to forge a fake
// terminal line) must reach the plan-lint warning only via
// internal/termsafe.Escape (a single-line, double-quoted Go string
// literal) — never as a raw, un-escaped control byte — mirroring
// internal/complete/bead_scope.go's identical hostile-input test.
func TestPlanLintDoubleAssignedFiles_TermsafeEscaped(t *testing.T) {
	hostileHeading := "Bead 1: \x1b[2K\rFAKE: gate passed"
	sections := []validate.BeadSection{
		{
			Heading:   hostileHeading,
			StepLines: []string{"1. Add `internal/shared/helper.go` with the new logic."},
		},
		{
			Heading:   "Bead 2: Second thing",
			StepLines: []string{"1. Consume `internal/shared/helper.go` from the executor."},
		},
	}

	warnings := planLintDoubleAssignedFiles(sections)
	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 warning, got %d: %v", len(warnings), warnings)
	}
	out := warnings[0]

	if strings.Contains(out, "\x1b") {
		t.Errorf("raw ESC control byte leaked into plan-lint warning (must be termsafe-escaped): %q", out)
	}
	quoted := strconv.Quote(hostileHeading)
	if !strings.Contains(out, quoted) {
		t.Errorf("expected the termsafe-escaped (strconv.Quote) heading %q in output, got:\n%s", quoted, out)
	}
}

// TestApprovePlan_DoubleAssignedFile_LintWarns is the Spec 119 AC-23
// end-to-end proof: `plan approve` on a plan whose TWO beads both
// reference the same file in their **Steps** lists surfaces a
// plan-lint finding (advisory — approve still succeeds) naming the
// file and both beads.
func TestApprovePlan_DoubleAssignedFile_LintWarns(t *testing.T) {
	noopStubPhaseMergeMetadataForTest(t)
	specID := "042-test"
	planContent := `---
status: Draft
spec_id: "042-test"
version: "1.0"
work_chunks:
  - id: 1
    depends_on: []
  - id: 2
    depends_on: [1]
---

# Plan

## ADR Fitness

No ADRs are relevant to this work.

## Testing Strategy

Unit tests will verify the implementation.

## Bead 1: First thing

**Steps**
1. Add ` + "`internal/shared/helper.go`" + ` with the new logic.
2. Wire it into the caller.
3. Add tests.

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- First bead works

**Depends on**
None

## Bead 2: Second thing

**Steps**
1. Consume ` + "`internal/shared/helper.go`" + ` from the executor.
2. Wire the executor caller.
3. Add tests.

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- Second bead works

**Depends on**
Bead 1
`
	root, _, _ := setupPreflightPlan(t, specID, planContent)
	stubApprovePlanEpic(t, "epic-42", specID)

	origBD := planRunBDFn
	defer func() { planRunBDFn = origBD }()
	beadCounter := 0
	planRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			beadCounter++
			return []byte(`{"id":"bead-` + string(rune('0'+beadCounter)) + `"}`), nil
		}
		if len(args) > 0 && args[0] == "dep" {
			return nil, nil
		}
		return []byte(`[]`), nil
	}

	mockExec := &executor.MockExecutor{}
	result, err := ApprovePlan(root, specID, "tester", mockExec)
	if err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "plan-lint") &&
			strings.Contains(w, "internal/shared/helper.go") &&
			strings.Contains(w, "Bead 1") && strings.Contains(w, "Bead 2") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a plan-lint warning naming internal/shared/helper.go and both Bead 1/Bead 2, got: %v", result.Warnings)
	}
}

// TestApprovePlan_SurfacesUnlistedPlanPathWarning pins that plan approve
// passes the validator's unlisted-plan-path warning through to its own
// warnings, and that the warning does not refuse the approval.
func TestApprovePlan_SurfacesUnlistedPlanPathWarning(t *testing.T) {
	const specID, epicID = "042-test", "epic-42"
	plan := strings.Replace(validSingleBeadPlan, "    depends_on: []\n", "    depends_on: []\n    key_file_paths:\n      - internal/listed/a.go\n", 1)
	plan = strings.Replace(plan, "1. Step one\n", "1. Edit `internal/listed/a.go` and add `internal/unlisted/a_test.go`\n", 1)
	root, _, _ := setupPreflightPlan(t, specID, plan)

	wirePlanEpicSeams(t, specID, epicID, func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	restoreBD := SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte(`{"id":"bead-1"}`), nil
		}
		return []byte(`[]`), nil
	})
	defer restoreBD()
	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) { return nil, nil })
	defer restoreCombined()
	restoreMerge := SetPlanMergeMetadataForTest(func(id string, updates map[string]interface{}) error { return nil })
	defer restoreMerge()

	result, err := ApprovePlan(root, specID, "tester", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("the warning must not refuse approval, got: %v", err)
	}
	var hits []string
	for _, w := range result.Warnings {
		if strings.Contains(w, "in no bead's key_file_paths") {
			hits = append(hits, w)
		}
	}
	if len(hits) != 1 || !strings.Contains(hits[0], "internal/unlisted/a_test.go") || strings.Contains(hits[0], "internal/listed/a.go") {
		t.Fatalf("expected one warning naming only internal/unlisted/a_test.go, got: %v", result.Warnings)
	}
}
