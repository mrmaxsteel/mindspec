package approve

import (
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

// TestApprovePlan_DoubleAssignedFile_LintWarns is the Spec 119 AC-23
// end-to-end proof: `plan approve` on a plan whose TWO beads both
// reference the same file in their **Steps** lists surfaces a
// plan-lint finding (advisory — approve still succeeds) naming the
// file and both beads.
func TestApprovePlan_DoubleAssignedFile_LintWarns(t *testing.T) {
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
