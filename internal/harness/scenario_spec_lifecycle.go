package harness

import (
	"fmt"
	"strings"
	"testing"
)

// ScenarioSpecToIdle tests the full lifecycle: idle → spec → plan → implement → review → idle.
func ScenarioSpecToIdle() Scenario {
	return Scenario{
		Name:        "spec_to_idle",
		Description: "Full lifecycle from idle through spec to idle",
		MaxTurns:    100,
		TimeoutMin:  15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			// Sandbox comes with a clean repo; agent starts from scratch
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Do NOT ask what I'd like to do. Execute immediately.

You are in a MindSpec project with no active work. Your task: add a simple "greeting" feature — a hello.go program that prints "Hello!". Take it from idea all the way through to a completed implementation using the mindspec workflow.
Finish only when the project is back in idle with cleanup complete.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent may use spec create or spec-init (hidden alias) — both are valid paths.
			assertCommandRanEither(t, events, "mindspec",
				[]string{"spec", "create"}, []string{"spec-init"})
			assertCommandRan(t, events, "mindspec", "complete")
			assertNoPreApproveImplMainMergeOrPR(t, events)

			// All three approve phases must run as distinct commands.
			assertCommandSucceeded(t, events, "mindspec", "approve", "spec")
			assertCommandSucceeded(t, events, "mindspec", "approve", "plan")
			assertCommandSucceeded(t, events, "mindspec", "approve", "impl")

			// `mindspec next` may be skipped if the agent navigates directly
			// to the bead worktree created by plan approve + next in sequence.
			if !commandRanSuccessfully(events, "mindspec", "next") {
				t.Logf("mindspec next was not explicitly called")
			}

			// Git state after full lifecycle
			assertBranchIs(t, sandbox, "main")
			assertNoBranches(t, sandbox, "spec/")
			assertNoBranches(t, sandbox, "bead/")
			assertNoWorktrees(t, sandbox)

			// Agent used worktrees during implementation
			assertEventCWDContains(t, events, ".worktrees/")

			// Final mode should be idle (lifecycle roundtrip complete)
			assertMindspecMode(t, sandbox, "idle")
		},
	}
}

// ScenarioSpecInit tests the /ms-spec-create flow: idle → spec create → spec mode with worktree.
//
// Before: main branch, no worktrees, no spec/ branches, clean tree, idle mode
// After:  main branch (CWD), spec/ branch created, worktree created, spec mode (derived from beads)
func ScenarioSpecInit() Scenario {
	return Scenario{
		Name:        "spec_init",
		Description: "Initialize a new spec from idle mode via guidance discovery",
		MaxTurns:    20,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			// Verify preconditions
			if branch := sandbox.GitBranch(); branch != "main" {
				return fmt.Errorf("precondition: expected main branch, got %q", branch)
			}
			if wts := sandbox.ListWorktrees(); len(wts) != 0 {
				return fmt.Errorf("precondition: expected no worktrees, got %v", wts)
			}
			if branches := sandbox.ListBranches("spec/"); len(branches) != 0 {
				return fmt.Errorf("precondition: expected no spec/ branches, got %v", branches)
			}
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

Start a new specification called "001-calculator" for adding basic arithmetic operations.
Create it through the MindSpec lifecycle. Stop after the spec is initialized.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent discovered and ran mindspec spec create (not raw git)
			assertCommandRanEither(t, events, "mindspec",
				[]string{"spec-init"}, []string{"spec", "create"})

			// Agent should NOT have created spec branch manually with raw git
			for _, e := range events {
				if e.Command != "git" || e.ExitCode != 0 {
					continue
				}
				args := eventArgs(e)
				if containsAll(args, "branch") && containsAll(args, "spec/") {
					t.Error("agent created spec branch with raw git instead of mindspec spec create")
					break
				}
				if containsAll(args, "checkout") && containsAll(args, "-b") && containsAll(args, "spec/") {
					t.Error("agent created spec branch with raw git checkout -b instead of mindspec spec create")
					break
				}
			}

			// Git state: spec branch created
			assertHasBranches(t, sandbox, "spec/")

			// Git state: worktree created
			assertHasWorktrees(t, sandbox)

			// Spec template created in worktree
			if !fileExistsInWorktrees(sandbox.Root, "spec.md") {
				t.Error("spec.md was not created in worktree")
			}

			// Git state: main branch still exists (CWD is main repo root)
			assertBranchIs(t, sandbox, "main")
		},
	}
}

// ScenarioSpecApprove tests the /ms-spec-approve flow: spec mode → approve → plan mode.
//
// Before: spec worktree exists with draft spec, spec/001-calc branch, spec mode, clean tree
//
// After:  approve ran, plan mode, spec/ branch + worktree still exist
func ScenarioSpecApprove() Scenario {
	return Scenario{
		Name:        "spec_approve",
		Description: "Approve a draft spec and transition to plan mode",
		MaxTurns:    15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-calc"

			// Create epic for lifecycle
			_ = sandbox.CreateSpecEpic(specID)

			// Create spec branch + worktree via shared helper
			wt := setupWorktrees(sandbox, specID, "", "spec")

			// Write spec file in the worktree — must pass ValidateSpec
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", `---
title: Calculator Feature
status: Draft
---
# Calculator Feature

## Goal
Add basic arithmetic operations (add, subtract) to the project.

## Impacted Domains
- core arithmetic module

## ADR Touchpoints
None applicable.

## Requirements
1. Implement an add(a, b) function that returns the sum.
2. Implement a subtract(a, b) function that returns the difference.

## Scope

### In Scope
- add and subtract functions
- integer arithmetic

### Out of Scope
- floating point arithmetic
- division and multiplication

## Acceptance Criteria
- [ ] add(2, 3) returns 5
- [ ] subtract(5, 2) returns 3
- [ ] functions handle negative numbers correctly

## Approval
Pending.
`)

			// Commit in the worktree
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "setup: draft spec")

			// Commit setup state (mode derived from beads at runtime)
			sandbox.Commit("setup: spec mode")

			// Verify preconditions
			if branch := sandbox.GitBranch(); branch != "main" {
				return fmt.Errorf("precondition: expected main branch, got %q", branch)
			}
			if !sandbox.GitStatusClean() {
				return fmt.Errorf("precondition: expected clean working tree")
			}
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

/ms-spec-approve`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Command ran
			assertCommandRan(t, events, "mindspec", "approve")
			assertCommandContains(t, events, "mindspec", "spec")

			// Git state: spec branch still exists (not deleted during approve)
			assertHasBranches(t, sandbox, "spec/")

			// Git state: spec worktree still exists (persists through plan mode)
			assertHasWorktrees(t, sandbox)

			// Mode transitioned to plan
			assertMindspecMode(t, sandbox, "plan")
		},
	}
}

// ScenarioPlanApprove tests the /ms-plan-approve flow: plan mode → approve → implement mode.
//
// Before: spec worktree exists with draft plan, spec/001-planner branch, no bead/ branches,
//
//	plan mode, clean tree
//
// After:  approve plan ran, mindspec next ran, implement mode, bead/ branch + worktree created,
//
//	agent CWD moved to bead worktree
func ScenarioPlanApprove() Scenario {
	// Lift epicID so Assertions closure can verify bead creation.
	var epicID string
	return Scenario{
		Name:        "plan_approve",
		Description: "Approve a plan and transition to implement mode",
		MaxTurns:    20,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-planner"

			// Create epic
			epicID = sandbox.CreateSpecEpic(specID)

			// Create spec branch + worktree via shared helper
			wt := setupWorktrees(sandbox, specID, "", "plan")

			// Write approved spec
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", `---
title: Planner Feature
status: Approved
---
# Planner Feature

## Summary
Add a planning feature.

## Acceptance Criteria
- plan() returns a plan string
`)
			// Write draft plan with bead sections (must pass ValidatePlan: version, ADR Fitness,
			// Testing Strategy, 3+ steps per bead, verification with test artifact references)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/plan.md", `---
status: Draft
spec_id: 001-planner
version: "1"
last_updated: "2026-03-01"
adr_citations: []
---
# Plan: 001-planner — Planner Feature

## ADR Fitness

No existing ADRs are impacted by this change. This is a new standalone module.

## Testing Strategy

Unit tests via `+"`go test`"+` covering the Plan() function and edge cases.

## Bead 1: Core planner

**Steps**
1. Create internal/planner/planner.go with package declaration
2. Implement Plan() function that returns a plan string
3. Add input validation for empty arguments

**Verification**
- [ ] `+"`go test ./internal/planner/`"+` passes
- [ ] Plan() returns non-empty string

**Depends on**: None

## Bead 2: Tests

**Steps**
1. Create internal/planner/planner_test.go with table-driven tests
2. Add test for empty input edge case
3. Add test for normal plan generation

**Verification**
- [ ] `+"`go test ./internal/planner/`"+` passes with all cases
- [ ] Test coverage above 80%

**Depends on**: Bead 1
`)

			// Commit in worktree
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "setup: draft plan")

			// Commit setup state in main
			sandbox.Commit("setup: plan mode")

			// Verify preconditions
			if branch := sandbox.GitBranch(); branch != "main" {
				return fmt.Errorf("precondition: expected main branch, got %q", branch)
			}
			if branches := sandbox.ListBranches("bead/"); len(branches) != 0 {
				return fmt.Errorf("precondition: expected no bead/ branches, got %v", branches)
			}
			if !sandbox.GitStatusClean() {
				return fmt.Errorf("precondition: expected clean working tree")
			}
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

/ms-plan-approve`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Plan approval command ran
			assertCommandRan(t, events, "mindspec", "approve")
			assertCommandContains(t, events, "mindspec", "plan")

			// Git state: spec branch still exists (worktree is still needed)
			assertHasBranches(t, sandbox, "spec/")

			// Spec worktree persists through plan approval
			assertHasWorktrees(t, sandbox)

			// Beads were created by plan approve (plan has 2 bead sections)
			assertBeadsMinCount(t, sandbox, epicID, 2)

			// `mindspec next` should be called after plan approve to claim a bead.
			// If next ran, verify bead branch + worktree too.
			if commandRanSuccessfully(events, "mindspec", "next") {
				assertHasBranches(t, sandbox, "bead/")
				assertEventCWDContains(t, events, ".worktrees/")
			} else {
				t.Logf("mindspec next was not explicitly called")
			}
		},
	}
}

// ScenarioImplApprove tests the /ms-impl-approve flow: review mode → approve impl → idle.
//
// Before: spec worktree exists with impl content, spec/001-done branch, review mode,
//
//	all beads closed, clean tree
//
// After:  approve impl ran, idle mode, spec/ branch deleted (merged to main),
//
//	spec worktree removed, implementation content merged to main, clean tree
func ScenarioImplApprove() Scenario {
	return Scenario{
		Name:        "impl_approve",
		Description: "Approve implementation and transition to idle",
		MaxTurns:    15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-done"

			// Create epic + bead (already closed)
			epicID := sandbox.CreateSpecEpic(specID)
			beadID := sandbox.CreateBead("["+specID+"] Implement feature", "task", epicID)
			sandbox.ClaimBead(beadID)
			sandbox.runBDMust("close", beadID)

			// Create spec branch + worktree via shared helper (review = spec worktree only)
			wt := setupWorktrees(sandbox, specID, "", "plan")

			// Write spec files in the worktree (where they'd be in real workflow)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", `---
title: Done Feature
status: Approved
---
# Done Feature
A completed feature.
`)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Implement feature
Create done.go.
`, specID, beadID))

			// Write actual implementation file in the worktree
			sandbox.WriteFile(wt.SpecWtDir+"/done.go", `package main

func Done() string { return "done" }
`)
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "impl: implement feature")

			// Commit setup state (review mode derived from beads — all closed)
			sandbox.Commit("setup: review mode")

			// Verify preconditions
			if branch := sandbox.GitBranch(); branch != "main" {
				return fmt.Errorf("precondition: expected main branch, got %q", branch)
			}
			if branches := sandbox.ListBranches("spec/"); len(branches) == 0 {
				return fmt.Errorf("precondition: expected spec/ branch to exist")
			}
			if wts := sandbox.ListWorktrees(); len(wts) == 0 {
				return fmt.Errorf("precondition: expected spec worktree to exist")
			}
			if !sandbox.GitStatusClean() {
				return fmt.Errorf("precondition: expected clean working tree")
			}
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

/ms-impl-approve`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Command ran
			assertCommandRan(t, events, "mindspec", "approve")
			assertCommandContains(t, events, "mindspec", "impl")
			assertNoPreApproveImplMainMergeOrPR(t, events)

			// Git state: spec branch deleted after merge
			assertNoBranches(t, sandbox, "spec/")

			// Git state: spec worktree removed after merge
			assertNoWorktrees(t, sandbox)

			// Git state: implementation content merged to main
			if !sandbox.FileExists("done.go") {
				t.Error("expected done.go to be merged to main")
			}

		},
	}
}

// ScenarioSpecStatus tests the /ms-spec-status flow: check current mode and report.
//
// Before: implement mode, spec worktree + bead worktree exist, spec/001-status + bead/ branches,
//
//	active bead in_progress
//
// After:  no state changes (read-only command), still implement mode, worktrees+branches unchanged
func ScenarioSpecStatus() Scenario {
	return Scenario{
		Name:        "spec_status",
		Description: "Check current MindSpec status and report mode",
		MaxTurns:    10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-status"

			// Set up in implement mode with realistic worktree structure
			epicID := sandbox.CreateSpecEpic(specID)
			beadID := sandbox.CreateBead("["+specID+"] Feature", "task", epicID)
			sandbox.ClaimBead(beadID)

			wt := setupWorktrees(sandbox, specID, beadID, "implement")

			// Write spec files in the spec worktree
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", `---
title: Status Feature
status: Approved
---
# Status Feature
`)
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "setup: spec files")

			sandbox.Commit("setup: implement mode with active bead")
			mainCount := mustRun(sandbox.t, sandbox.Root, "git", "rev-list", "--count", "main")
			sandbox.WriteFile(".harness/main_commit_count", strings.TrimSpace(mainCount))

			// Verify preconditions
			if branch := sandbox.GitBranch(); branch != "main" {
				return fmt.Errorf("precondition: expected main branch, got %q", branch)
			}
			if wts := sandbox.ListWorktrees(); len(wts) < 2 {
				return fmt.Errorf("precondition: expected spec + bead worktrees, got %v", wts)
			}
			if !sandbox.GitStatusClean() {
				return fmt.Errorf("precondition: expected clean working tree")
			}
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

/ms-spec-status`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Command ran: agent ran state show or instruct
			ran := false
			for _, e := range events {
				if e.Command != "mindspec" {
					continue
				}
				if e.ExitCode != 0 {
					continue
				}
				args := eventArgs(e)
				for _, arg := range args {
					if arg == "state" || arg == "instruct" {
						ran = true
						break
					}
				}
				if ran {
					break
				}
			}
			if !ran {
				t.Error("expected agent to run 'mindspec state show' or 'mindspec instruct'")
			}

			// Read-only: no state changes — still implement mode

			// Read-only: worktrees still exist (status is read-only)
			assertHasWorktrees(t, sandbox)

			// Read-only: branches still exist
			assertHasBranches(t, sandbox, "spec/")
			assertHasBranches(t, sandbox, "bead/")

			// Read-only: no non-infrastructure files modified.
			assertNoUserFilesModified(t, sandbox)
			assertMainCommitCountUnchanged(t, sandbox)
		},
	}
}

// ScenarioMultipleActiveSpecs tests that the agent can implement a bead when
// two specs are active simultaneously without disrupting the other spec.
// The bead worktree provides enough context for the CLI to auto-resolve the
// target spec, so --spec disambiguation is not required (worktree path
// resolution supersedes the ambiguity check).
//
// Before: Two specs active (001-alpha in implement mode, 002-beta in plan mode),
//
//	one bead claimed for 001-alpha, agent is asked to implement the bead
//
// After:  Agent implements the bead and completes 001-alpha into review mode;
//
//	002-beta's epic remains untouched
func ScenarioMultipleActiveSpecs() Scenario {
	var epicAlpha, beadAlpha, epicBeta string
	return Scenario{
		Name:        "multiple_active_specs",
		Description: "Two active specs, agent completes one without disrupting the other",
		MaxTurns:    30,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-alpha"

			// --- Spec 001-alpha: implement mode with a claimed bead ---
			epicAlpha = sandbox.CreateSpecEpic("001-alpha")
			beadAlpha = sandbox.CreateBead("[001-alpha] Implement greeting", "task", epicAlpha)
			sandbox.ClaimBead(beadAlpha)

			sandbox.WriteFile(".mindspec/docs/specs/001-alpha/spec.md", `---
title: Alpha Feature
status: Approved
---
# Alpha Feature
Add a greeting function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/001-alpha/plan.md", `---
status: Approved
spec_id: 001-alpha
---
# Plan
## Bead 1: Implement greeting
Create greeting.go with a Greet function.
`)

			// --- Spec 002-beta: plan mode (no beads yet) ---
			epicBeta = sandbox.CreateSpecEpic("002-beta")
			sandbox.WriteFile(".mindspec/docs/specs/002-beta/spec.md", `---
title: Beta Feature
status: Approved
---
# Beta Feature
Add a calculator function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/002-beta/plan.md", `---
status: Draft
spec_id: 002-beta
---
# Plan — Draft
TBD
`)

			sandbox.Commit("setup: two active specs")
			setupWorktrees(sandbox, specID, beadAlpha, "implement")

			sandbox.Commit("setup: two active specs with active worktree")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

Implement the 001-alpha bead by creating greeting.go with a function
Greet(name string) string that returns "Hello, <name>!".
Finish 001-alpha through the MindSpec lifecycle so 001-alpha ends in review mode
while 002-beta remains unchanged. Do not close beads directly with bd commands.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent may create greeting.go in a temporary worktree and clean it up.
			greetingObserved := sandbox.FileExists("greeting.go") || fileExistsInWorktrees(sandbox.Root, "greeting.go")
			if !greetingObserved {
				for _, e := range events {
					if e.Command != "git" || e.ExitCode != 0 {
						continue
					}
					args := eventArgs(e)
					if containsAll(args, "add") && containsAll(args, "greeting.go") {
						greetingObserved = true
						break
					}
				}
			}
			if !greetingObserved {
				t.Error("greeting.go was not created")
			}

			// Agent should have run mindspec complete
			assertCommandRan(t, events, "mindspec", "complete")

			// Bead was closed by mindspec complete
			assertBeadsState(t, sandbox, epicAlpha, map[string]string{
				beadAlpha: "closed",
			})

			// Bead branch was merged into spec branch
			assertMergeTopology(t, sandbox, "spec/001-alpha")

			// 002-beta epic should have no children (agent didn't touch it)
			betaChildren, _ := sandbox.runBD("list", "--json", "--parent", epicBeta)
			bc := strings.TrimSpace(betaChildren)
			if bc != "" && bc != "[]" && bc != "null" {
				t.Errorf("002-beta epic should have no children, got: %s", bc)
			}
		},
	}
}
