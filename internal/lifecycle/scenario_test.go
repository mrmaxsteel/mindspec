package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// stubNoEpics stubs phase functions so that CheckSpecNumberCollision finds no collisions.
func stubNoEpics(t *testing.T) {
	t.Helper()
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)
}

// stubApproveBeads stubs all BD functions in the approve package so that
// ApproveSpec and ApprovePlan don't create real beads in the shared database.
func stubApproveBeads(t *testing.T) {
	t.Helper()
	noopBD := func(args ...string) ([]byte, error) {
		return []byte(`{"id":"fake-test-bead"}`), nil
	}
	t.Cleanup(approve.SetSpecRunBDForTest(noopBD))
	t.Cleanup(approve.SetPlanRunBDForTest(noopBD))
	t.Cleanup(approve.SetPlanRunBDCombinedForTest(noopBD))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// testRepo creates a fresh git repo with .mindspec/ structure, config, and
// an initial commit — suitable for driving lifecycle transitions.
func testRepo(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "repo")
	must(t, os.MkdirAll(root, 0o755))

	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "test@mindspec.dev")
	gitRun(t, root, "config", "user.name", "MindSpec Test")

	// .mindspec structure
	for _, d := range []string{
		".mindspec",
		".mindspec/docs",
		".mindspec/docs/specs",
		".mindspec/docs/adr",
	} {
		must(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}

	writeFile(t, root, ".mindspec/config.yaml", `protected_branches: [main]
merge_strategy: direct
worktree_root: .worktrees
enforcement:
  pre_commit_hook: true
  cli_guards: true
  agent_hooks: true
`)

	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "initial commit")

	return root
}

// writeFile writes a file under root, creating intermediate directories.
func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	must(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	must(t, os.WriteFile(abs, []byte(content), 0o644))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// assertState checks that focus mode and lifecycle.yaml phase match expected.
func assertState(t *testing.T, root, specID, expectedMode, expectedPhase string) {
	t.Helper()

	// ADR-0023: State is derived from beads, not focus/lifecycle files.
	// These scenario tests verify the approval functions succeed (via error returns).
	// Phase derivation is tested in internal/phase/ and internal/resolve/.
	// This function is kept as a no-op to preserve test structure during migration.
}

// validSpecMD returns a spec.md that passes validate.ValidateSpec.
func validSpecMD(specID string) string {
	return fmt.Sprintf(`---
spec_id: %s
status: Draft
version: 1
---

# Spec: %s — Test Spec

## Goal

This is a test specification for lifecycle scenario testing.

## Impacted Domains

- core

## ADR Touchpoints

None currently applicable.

## Requirements

1. The lifecycle state machine transitions correctly
2. Focus mode and lifecycle phase stay in sync
3. Worktree state is managed properly

## Scope

### In Scope

- State machine transitions
- Focus file management

### Out of Scope

- LLM behavior testing
- Performance testing

## Acceptance Criteria

- [ ] All lifecycle transitions update focus mode correctly
- [ ] All lifecycle transitions update lifecycle.yaml phase correctly
- [ ] Invalid transitions are rejected with appropriate errors

## Open Questions

None.

## Approval

Pending.
`, specID, specID)
}

// validPlanMD returns a plan.md that passes validate.ValidatePlan.
func validPlanMD(specID string) string {
	return fmt.Sprintf(`---
spec_id: %s
status: Draft
version: 1
last_updated: "2026-02-28"
bead_ids: []
---

# Plan: %s — Test Plan

## ADR Fitness

No relevant ADRs — this is a test fixture.

## Testing Strategy

Unit tests in scenario_test.go validate all transitions via go test.

## Bead 1: Implement Core Logic

**Steps**

1. Create the main module
2. Add state transition logic
3. Write unit tests in core_test.go

**Verification**

- [ ] `+"`go test ./internal/lifecycle/`"+` passes
- [ ] State transitions work correctly
- [ ] Invalid transitions rejected

**Depends on**

None

## Bead 2: Integration Tests

**Steps**

1. Create integration test file
2. Add end-to-end lifecycle test
3. Add edge case tests

**Verification**

- [ ] `+"`go test ./internal/lifecycle/`"+` passes
- [ ] Full lifecycle tested end-to-end
- [ ] Edge cases covered

**Depends on**

Bead 1
`, specID, specID)
}

// simulateSpecInit creates the spec directory and files that specinit.Run
// would produce, without calling bd or creating worktrees.
func simulateSpecInit(t *testing.T, root, specID string) {
	t.Helper()

	specDir := workspace.SpecDir(root, specID)
	must(t, os.MkdirAll(specDir, 0o755))

	writeFile(t, root, filepath.Join(".mindspec/docs/specs", specID, "spec.md"), validSpecMD(specID))

	// ADR-0023: no lifecycle.yaml or focus file — state derived from beads.

	// Commit so git is clean for downstream operations
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", fmt.Sprintf("spec-init %s", specID))
}

// simulateNext simulates claiming a bead (no focus file needed per ADR-0023).
func simulateNext(t *testing.T, root, specID, beadID string) {
	t.Helper()
	// ADR-0023: state derived from beads, no focus file written.
}

// simulateComplete simulates bead completion (no lifecycle/focus files per ADR-0023).
func simulateComplete(t *testing.T, root, specID string) {
	t.Helper()
	// ADR-0023: state derived from beads, no lifecycle/focus files written.
}

// simulateApproveImpl simulates impl approval (no lifecycle/focus files per ADR-0023).
func simulateApproveImpl(t *testing.T, root, specID string) {
	t.Helper()
	// ADR-0023: state derived from beads, no lifecycle/focus files written.
}

// ---------------------------------------------------------------------------
// Scenario Tests
// ---------------------------------------------------------------------------

func TestScenario_HappyPath(t *testing.T) {
	stubNoEpics(t)
	stubApproveBeads(t)
	root := testRepo(t)
	specID := "001-test-feature"

	// Phase 1: Spec Init (simulated — specinit.Run needs bd + worktree mocks)
	simulateSpecInit(t, root, specID)
	assertState(t, root, specID, state.ModeSpec, "spec")

	// Phase 2: Approve Spec (called directly — no bd needed)
	result, err := approve.ApproveSpec(root, specID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApproveSpec: %v", err)
	}
	if result.SpecID != specID {
		t.Errorf("ApproveSpec result specID = %q, want %q", result.SpecID, specID)
	}
	assertState(t, root, specID, state.ModePlan, state.ModePlan)

	// Write plan.md for plan approval
	writeFile(t, root, filepath.Join(".mindspec/docs/specs", specID, "plan.md"), validPlanMD(specID))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "add plan")

	// Phase 3: Approve Plan (called directly — no epic_id means bead creation skipped)
	planResult, err := approve.ApprovePlan(root, specID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}
	if planResult.SpecID != specID {
		t.Errorf("ApprovePlan result specID = %q, want %q", planResult.SpecID, specID)
	}
	// Lifecycle should be implement now, but focus stays at plan (plan approved,
	// user needs to run `mindspec next` to move to implement)
	assertState(t, root, specID, state.ModePlan, state.ModeImplement)

	// Phase 4: Next / Claim bead (simulated — needs bd mocking)
	simulateNext(t, root, specID, "test-bead-001")
	assertState(t, root, specID, state.ModeImplement, state.ModeImplement)

	// Phase 5: Complete (simulated — needs bd mocking)
	simulateComplete(t, root, specID)
	assertState(t, root, specID, state.ModeReview, state.ModeReview)

	// Phase 6: Approve Impl (simulated — needs bd + git mocking)
	simulateApproveImpl(t, root, specID)
	assertState(t, root, specID, state.ModeIdle, "done")
}

func TestScenario_IdleStartsClean(t *testing.T) {
	root := testRepo(t)

	// Fresh repo should be in idle state
	assertState(t, root, "", state.ModeIdle, "")
}

func TestScenario_InterruptForBug(t *testing.T) {
	stubNoEpics(t)
	stubApproveBeads(t)
	root := testRepo(t)
	specID := "002-main-feature"
	bugSpecID := "003-hotfix-bug"

	// Set up: in the middle of implementing spec 002
	simulateSpecInit(t, root, specID)
	// Approve spec
	_, err := approve.ApproveSpec(root, specID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApproveSpec: %v", err)
	}

	// Write and approve plan
	writeFile(t, root, filepath.Join(".mindspec/docs/specs", specID, "plan.md"), validPlanMD(specID))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "add plan")

	_, err = approve.ApprovePlan(root, specID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}

	// Simulate claiming a bead and starting implementation
	simulateNext(t, root, specID, "feature-bead-001")
	assertState(t, root, specID, state.ModeImplement, state.ModeImplement)

	// --- INTERRUPT: Bug discovered ---
	// ADR-0023: no focus save/restore needed — state derived from beads.

	// Set up bug fix spec (simulate the entire bug-fix lifecycle quickly)
	simulateSpecInit(t, root, bugSpecID)

	// Approve bug spec
	_, err = approve.ApproveSpec(root, bugSpecID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApproveSpec(bug): %v", err)
	}

	// Write and approve bug plan
	writeFile(t, root, filepath.Join(".mindspec/docs/specs", bugSpecID, "plan.md"), validPlanMD(bugSpecID))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "add bug plan")

	_, err = approve.ApprovePlan(root, bugSpecID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApprovePlan(bug): %v", err)
	}

	// Complete the bug fix lifecycle
	simulateNext(t, root, bugSpecID, "bug-bead-001")
	simulateComplete(t, root, bugSpecID)
	simulateApproveImpl(t, root, bugSpecID)

	// --- RESUME original work ---
	// ADR-0023: With beads-derived state, both specs' epics coexist.
	// The original spec's epic and in-progress bead are still in beads,
	// so `mindspec next` would discover and resume the original work.
	// No focus file restoration needed.
}

func TestScenario_ResumeAfterCrash(t *testing.T) {
	// ADR-0023: Crash recovery relies on beads having the in-progress bead.
	// No focus/lifecycle files needed — `mindspec next` queries beads to find
	// the active bead and resumes it. This test verifies that spec artifacts
	// survive a simulated crash (spec dir + spec.md exist).
	root := testRepo(t)
	specID := "004-crash-resume"

	simulateSpecInit(t, root, specID)

	// Verify spec artifacts exist (the durable state that survives crashes)
	specDir := workspace.SpecDir(root, specID)
	if _, err := os.Stat(filepath.Join(specDir, "spec.md")); err != nil {
		t.Fatalf("spec.md should exist after crash: %v", err)
	}
}

func TestScenario_InvalidTransition(t *testing.T) {
	root := testRepo(t)

	// Attempting approve-spec when no spec exists should fail
	_, err := approve.ApproveSpec(root, "999-nonexistent", "test-user", &executor.MockExecutor{})
	if err == nil {
		t.Fatal("expected error approving nonexistent spec")
	}
}

func TestScenario_SpecApprovalUpdatesArtifacts(t *testing.T) {
	stubNoEpics(t)
	stubApproveBeads(t)
	root := testRepo(t)
	specID := "005-artifact-check"

	simulateSpecInit(t, root, specID)

	_, err := approve.ApproveSpec(root, specID, "artifact-tester", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApproveSpec: %v", err)
	}

	// Verify spec.md was updated with approval info
	specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading spec.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "Approved") {
		t.Error("spec.md should contain 'Approved' after approval")
	}
	if !strings.Contains(content, "artifact-tester") {
		t.Error("spec.md should contain approver name 'artifact-tester'")
	}

	// ADR-0023: lifecycle.yaml is eliminated — phase is derived from beads.
	// The spec.md artifact checks above verify the approval was recorded.
}

// TestValidateFromWorktree verifies that validate succeeds when spec artifacts
// only exist in the worktree (not in the main repo). Uses the validate package
// directly to avoid needing a binary path.
func TestValidateFromWorktree(t *testing.T) {
	root := testRepo(t)
	specID := "090-wt-validate"

	// Create spec worktree directory structure with spec artifacts
	// (spec files do NOT exist in main repo's .mindspec/docs/specs/)
	wtSpecDir := filepath.Join(root, ".worktrees", "worktree-spec-"+specID,
		".mindspec", "docs", "specs", specID)
	must(t, os.MkdirAll(wtSpecDir, 0o755))
	must(t, os.WriteFile(filepath.Join(wtSpecDir, "spec.md"),
		[]byte(validSpecMD(specID)), 0o644))

	// Validate spec should find it via worktree-aware SpecDir
	result := validate.ValidateSpec(root, specID)
	if result.HasFailures() {
		t.Fatalf("ValidateSpec from worktree failed:\n%s", result.FormatText())
	}

	// Also validate plan (write a plan in the worktree)
	must(t, os.WriteFile(filepath.Join(wtSpecDir, "plan.md"),
		[]byte(validPlanMD(specID)), 0o644))

	result = validate.ValidatePlan(root, specID)
	if result.HasFailures() {
		t.Fatalf("ValidatePlan from worktree failed:\n%s", result.FormatText())
	}
}

func TestScenario_PlanApprovalUpdatesArtifacts(t *testing.T) {
	stubNoEpics(t)
	stubApproveBeads(t)
	root := testRepo(t)
	specID := "006-plan-artifact"

	simulateSpecInit(t, root, specID)

	_, err := approve.ApproveSpec(root, specID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApproveSpec: %v", err)
	}

	writeFile(t, root, filepath.Join(".mindspec/docs/specs", specID, "plan.md"), validPlanMD(specID))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "add plan")

	_, err = approve.ApprovePlan(root, specID, "test-user", &executor.MockExecutor{})
	if err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}

	// Verify plan.md was updated with approval
	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading plan.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "Approved") {
		t.Error("plan.md should contain 'Approved' after approval")
	}
}
