package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/approve"
	"github.com/mindspec/mindspec/internal/explore"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/validate"
	"github.com/mindspec/mindspec/internal/workspace"
)

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

	// Check focus
	mc, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("assertState: reading focus: %v", err)
	}
	if mc == nil {
		if expectedMode != "" && expectedMode != state.ModeIdle {
			t.Errorf("assertState: focus is nil, want mode=%q", expectedMode)
		}
	} else if mc.Mode != expectedMode {
		t.Errorf("assertState: focus mode = %q, want %q", mc.Mode, expectedMode)
	}

	// Check lifecycle phase (skip if no specID or phase expected)
	if specID == "" || expectedPhase == "" {
		return
	}
	specDir := workspace.SpecDir(root, specID)
	lc, err := state.ReadLifecycle(specDir)
	if err != nil {
		t.Fatalf("assertState: reading lifecycle for %s: %v", specID, err)
	}
	if lc == nil {
		t.Fatalf("assertState: lifecycle is nil for %s", specID)
	}
	if lc.Phase != expectedPhase {
		t.Errorf("assertState: lifecycle phase = %q, want %q", lc.Phase, expectedPhase)
	}
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
	writeFile(t, root, filepath.Join(".mindspec/docs/specs", specID, "lifecycle.yaml"), "phase: spec\n")

	// Write focus
	must(t, state.WriteFocus(root, &state.Focus{
		Mode:       state.ModeSpec,
		ActiveSpec: specID,
		SpecBranch: state.SpecBranch(specID),
	}))

	// Commit so git is clean for downstream operations
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", fmt.Sprintf("spec-init %s", specID))
}

// simulateNext writes focus to implement mode with an active bead.
func simulateNext(t *testing.T, root, specID, beadID string) {
	t.Helper()

	must(t, state.WriteFocus(root, &state.Focus{
		Mode:       state.ModeImplement,
		ActiveSpec: specID,
		ActiveBead: beadID,
		SpecBranch: state.SpecBranch(specID),
	}))
}

// simulateComplete transitions from implement to review mode.
func simulateComplete(t *testing.T, root, specID string) {
	t.Helper()

	// Update lifecycle to review
	specDir := workspace.SpecDir(root, specID)
	lc, err := state.ReadLifecycle(specDir)
	if err != nil || lc == nil {
		lc = &state.Lifecycle{}
	}
	lc.Phase = state.ModeReview
	must(t, state.WriteLifecycle(specDir, lc))

	// Update focus
	must(t, state.WriteFocus(root, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: specID,
		SpecBranch: state.SpecBranch(specID),
	}))
}

// simulateApproveImpl transitions from review to idle (done).
func simulateApproveImpl(t *testing.T, root, specID string) {
	t.Helper()

	// Update lifecycle to done
	specDir := workspace.SpecDir(root, specID)
	lc, err := state.ReadLifecycle(specDir)
	if err != nil || lc == nil {
		lc = &state.Lifecycle{}
	}
	lc.Phase = "done"
	must(t, state.WriteLifecycle(specDir, lc))

	// Update focus to idle
	must(t, state.WriteFocus(root, &state.Focus{
		Mode: state.ModeIdle,
	}))
}

// ---------------------------------------------------------------------------
// Scenario Tests
// ---------------------------------------------------------------------------

func TestScenario_HappyPath(t *testing.T) {
	root := testRepo(t)
	specID := "001-test-feature"

	// Phase 1: Explore → Spec
	// Enter explore mode
	if err := explore.Enter(root, "testing lifecycle"); err != nil {
		t.Fatalf("explore.Enter: %v", err)
	}
	assertState(t, root, "", state.ModeExplore, "")

	// Phase 2: Spec Init (simulated — specinit.Run needs bd + worktree mocks)
	simulateSpecInit(t, root, specID)
	assertState(t, root, specID, state.ModeSpec, "spec")

	// Phase 3: Approve Spec (called directly — no bd needed)
	result, err := approve.ApproveSpec(root, specID, "test-user")
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

	// Phase 4: Approve Plan (called directly — no epic_id means bead creation skipped)
	planResult, err := approve.ApprovePlan(root, specID, "test-user")
	if err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}
	if planResult.SpecID != specID {
		t.Errorf("ApprovePlan result specID = %q, want %q", planResult.SpecID, specID)
	}
	// Lifecycle should be implement now, but focus stays at plan (plan approved,
	// user needs to run `mindspec next` to move to implement)
	assertState(t, root, specID, state.ModePlan, state.ModeImplement)

	// Phase 5: Next / Claim bead (simulated — needs bd mocking)
	simulateNext(t, root, specID, "test-bead-001")
	assertState(t, root, specID, state.ModeImplement, state.ModeImplement)

	// Phase 6: Complete (simulated — needs bd mocking)
	simulateComplete(t, root, specID)
	assertState(t, root, specID, state.ModeReview, state.ModeReview)

	// Phase 7: Approve Impl (simulated — needs bd + git mocking)
	simulateApproveImpl(t, root, specID)
	assertState(t, root, specID, state.ModeIdle, "done")
}

func TestScenario_Abandon(t *testing.T) {
	root := testRepo(t)

	// Enter explore mode
	if err := explore.Enter(root, "abandoned idea"); err != nil {
		t.Fatalf("explore.Enter: %v", err)
	}
	assertState(t, root, "", state.ModeExplore, "")

	// Dismiss — return to idle
	if err := explore.Dismiss(root); err != nil {
		t.Fatalf("explore.Dismiss: %v", err)
	}
	assertState(t, root, "", state.ModeIdle, "")
}

func TestScenario_AbandonRejectsDoubleEnter(t *testing.T) {
	root := testRepo(t)

	if err := explore.Enter(root, "first"); err != nil {
		t.Fatalf("explore.Enter: %v", err)
	}

	// Attempting to enter explore again should fail
	err := explore.Enter(root, "second")
	if err == nil {
		t.Fatal("expected error entering explore while already in explore")
	}
	if !strings.Contains(err.Error(), "explore") {
		t.Errorf("error should mention explore mode, got: %v", err)
	}
}

func TestScenario_InterruptForBug(t *testing.T) {
	root := testRepo(t)
	specID := "002-main-feature"
	bugSpecID := "003-hotfix-bug"

	// Set up: in the middle of implementing spec 002
	simulateSpecInit(t, root, specID)
	// Approve spec
	_, err := approve.ApproveSpec(root, specID, "test-user")
	if err != nil {
		t.Fatalf("ApproveSpec: %v", err)
	}

	// Write and approve plan
	writeFile(t, root, filepath.Join(".mindspec/docs/specs", specID, "plan.md"), validPlanMD(specID))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "add plan")

	_, err = approve.ApprovePlan(root, specID, "test-user")
	if err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}

	// Simulate claiming a bead and starting implementation
	simulateNext(t, root, specID, "feature-bead-001")
	assertState(t, root, specID, state.ModeImplement, state.ModeImplement)

	// --- INTERRUPT: Bug discovered ---
	// Save current state for later resume
	savedFocus, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("reading focus before interrupt: %v", err)
	}

	// Set up bug fix spec (simulate the entire bug-fix lifecycle quickly)
	simulateSpecInit(t, root, bugSpecID)
	assertState(t, root, bugSpecID, state.ModeSpec, "spec")

	// Approve bug spec
	_, err = approve.ApproveSpec(root, bugSpecID, "test-user")
	if err != nil {
		t.Fatalf("ApproveSpec(bug): %v", err)
	}

	// Write and approve bug plan
	writeFile(t, root, filepath.Join(".mindspec/docs/specs", bugSpecID, "plan.md"), validPlanMD(bugSpecID))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "add bug plan")

	_, err = approve.ApprovePlan(root, bugSpecID, "test-user")
	if err != nil {
		t.Fatalf("ApprovePlan(bug): %v", err)
	}

	// Complete the bug fix lifecycle
	simulateNext(t, root, bugSpecID, "bug-bead-001")
	simulateComplete(t, root, bugSpecID)
	simulateApproveImpl(t, root, bugSpecID)
	assertState(t, root, bugSpecID, state.ModeIdle, "done")

	// --- RESUME original work ---
	// Restore focus to the original spec's implement mode
	must(t, state.WriteFocus(root, savedFocus))
	assertState(t, root, specID, state.ModeImplement, state.ModeImplement)

	// The original spec's lifecycle should still be in implement phase
	specDir := workspace.SpecDir(root, specID)
	lc, err := state.ReadLifecycle(specDir)
	if err != nil {
		t.Fatalf("reading lifecycle after resume: %v", err)
	}
	if lc.Phase != state.ModeImplement {
		t.Errorf("original spec lifecycle phase = %q after resume, want %q", lc.Phase, state.ModeImplement)
	}
}

func TestScenario_ResumeAfterCrash(t *testing.T) {
	root := testRepo(t)
	specID := "004-crash-resume"
	beadID := "crash-bead-001"

	// Set up state as if a session died mid-implementation:
	// - lifecycle.yaml says implement
	// - focus says implement with an active bead
	// - spec directory exists with artifacts
	simulateSpecInit(t, root, specID)

	// Skip through to implement state
	specDir := workspace.SpecDir(root, specID)
	must(t, state.WriteLifecycle(specDir, &state.Lifecycle{Phase: state.ModeImplement}))
	must(t, state.WriteFocus(root, &state.Focus{
		Mode:       state.ModeImplement,
		ActiveSpec: specID,
		ActiveBead: beadID,
		SpecBranch: state.SpecBranch(specID),
	}))

	// Verify state is consistent after "crash"
	assertState(t, root, specID, state.ModeImplement, state.ModeImplement)

	// Verify focus has the correct active bead
	mc, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("ReadFocus: %v", err)
	}
	if mc.ActiveBead != beadID {
		t.Errorf("ActiveBead = %q, want %q", mc.ActiveBead, beadID)
	}
	if mc.ActiveSpec != specID {
		t.Errorf("ActiveSpec = %q, want %q", mc.ActiveSpec, specID)
	}

	// A recovered session would call mindspec next, which detects the
	// existing in-progress bead and resumes it. Since next.* requires
	// bd mocking, we verify the precondition: focus + lifecycle agree
	// on implement mode with the correct spec and bead.
	if mc.SpecBranch != state.SpecBranch(specID) {
		t.Errorf("SpecBranch = %q, want %q", mc.SpecBranch, state.SpecBranch(specID))
	}
}

func TestScenario_InvalidTransition(t *testing.T) {
	root := testRepo(t)

	// Attempting to dismiss when not in explore mode should fail, even
	// when focus is absent (implicit idle).
	err := explore.Dismiss(root)
	if err == nil {
		t.Fatal("expected error dismissing when not in explore mode")
	}

	// Attempting approve-spec when no spec exists should fail
	_, err = approve.ApproveSpec(root, "999-nonexistent", "test-user")
	if err == nil {
		t.Fatal("expected error approving nonexistent spec")
	}
}

func TestScenario_SpecApprovalUpdatesArtifacts(t *testing.T) {
	root := testRepo(t)
	specID := "005-artifact-check"

	simulateSpecInit(t, root, specID)

	_, err := approve.ApproveSpec(root, specID, "artifact-tester")
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

	// Verify lifecycle.yaml transitioned to plan
	specDir := workspace.SpecDir(root, specID)
	lc, err := state.ReadLifecycle(specDir)
	if err != nil {
		t.Fatalf("reading lifecycle: %v", err)
	}
	if lc.Phase != state.ModePlan {
		t.Errorf("lifecycle phase = %q, want %q", lc.Phase, state.ModePlan)
	}
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
	must(t, os.WriteFile(filepath.Join(wtSpecDir, "lifecycle.yaml"),
		[]byte("phase: spec\n"), 0o644))

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
	root := testRepo(t)
	specID := "006-plan-artifact"

	simulateSpecInit(t, root, specID)

	_, err := approve.ApproveSpec(root, specID, "test-user")
	if err != nil {
		t.Fatalf("ApproveSpec: %v", err)
	}

	writeFile(t, root, filepath.Join(".mindspec/docs/specs", specID, "plan.md"), validPlanMD(specID))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "add plan")

	_, err = approve.ApprovePlan(root, specID, "test-user")
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
