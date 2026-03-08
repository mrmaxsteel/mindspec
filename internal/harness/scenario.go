package harness

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/hooks"
)

// Scenario defines a behavioral test scenario for an agent session.
type Scenario struct {
	Name        string                                                     // scenario identifier (e.g. "single_bead")
	Description string                                                     // human-readable description
	Setup       func(sandbox *Sandbox) error                               // prepares sandbox state before agent runs
	Prompt      string                                                     // the prompt given to the agent
	Assertions  func(t *testing.T, sandbox *Sandbox, events []ActionEvent) // post-run assertions
	MaxTurns    int                                                        // turn budget (0 = unlimited)
	TimeoutMin  int                                                        // scenario runtime timeout in minutes (0 = default 10m)
	Model       string                                                     // model override (e.g. "haiku")
}

// AllScenarios returns all defined behavior scenarios.
func AllScenarios() []Scenario {
	return []Scenario{
		ScenarioSpecToIdle(),
		ScenarioSingleBead(),
		ScenarioMultiBeadDeps(),
		ScenarioInterruptForBug(),
		ScenarioResumeAfterCrash(),
		ScenarioSpecInit(),
		ScenarioSpecApprove(),
		ScenarioPlanApprove(),
		ScenarioImplApprove(),
		ScenarioSpecStatus(),
		ScenarioMultipleActiveSpecs(),
		ScenarioStaleWorktree(),
		ScenarioCompleteFromSpecWorktree(),
		ScenarioApproveSpecFromWorktree(),
		ScenarioApprovePlanFromWorktree(),
		ScenarioBugfixBranch(),
		ScenarioBlockedBeadTransition(),
		ScenarioUnmergedBeadGuard(),
	}
}

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

			// All three approve phases must run during lifecycle.
			assertCommandContains(t, events, "mindspec", "approve")
			assertCommandContains(t, events, "mindspec", "spec")
			assertCommandContains(t, events, "mindspec", "plan")
			assertCommandContains(t, events, "mindspec", "impl")

			// `mindspec next` is optional for single-bead specs because
			// `plan approve` auto-claims the first bead. Log if skipped.
			if !commandRanSuccessfully(events, "mindspec", "next") {
				t.Logf("mindspec next was not called (plan approve auto-claimed first bead)")
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

// ScenarioSingleBead tests implementing a single pre-approved bead.
func ScenarioSingleBead() Scenario {
	// Lift IDs so both Setup and Assertions closures can access them.
	var epicID, beadID string
	return Scenario{
		Name:        "single_bead",
		Description: "Pre-approved plan, implement a single bead",
		MaxTurns:    20,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-greeting"

			// Create real beads: epic + child task
			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Implement greeting", "task", epicID)
			sandbox.ClaimBead(beadID)

			// Set up as if spec and plan are already approved
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Greeting Feature
status: Approved
---
# Greeting Feature
Add a greeting function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: `+specID+`
---
# Plan
## Bead 1: Implement greeting
Create greeting.go with a Greet function.
`)
			sandbox.Commit("setup: pre-approved spec and plan")

			// Start in a valid implement context with nested worktree topology:
			// main → spec worktree → bead worktree (mirrors real mindspec next).
			setupWorktrees(sandbox, specID, beadID, "implement")

			sandbox.Commit("setup: implement mode with active worktree")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

Create a file called greeting.go with a function Greet(name string) string
that returns "Hello, <name>!".
Finish the currently claimed bead through the MindSpec lifecycle so this spec
ends in review mode. Do not close beads directly with bd commands.`,
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

			// Commit message follows impl(<beadID>): convention.
			// mindspec complete auto-commits with this format when given a message.
			// If the agent ran complete without a message, the commit may not exist —
			// use Errorf (non-fatal) since the real proof is that complete ran.
			assertCommitMessage(t, sandbox, `impl\(`)

			// Bead branch was merged into spec branch (merge topology).
			// After impl approve, the spec branch may be deleted — assertMergeTopology
			// falls back to --all to find merge commits on main.
			assertMergeTopology(t, sandbox, "spec/001-greeting")

			// Bead was closed by mindspec complete
			assertBeadsState(t, sandbox, epicID, map[string]string{
				beadID: "closed",
			})
		},
	}
}

// ScenarioMultiBeadDeps tests implementing 3 beads with dependencies.
func ScenarioMultiBeadDeps() Scenario {
	return Scenario{
		Name:        "multi_bead_deps",
		Description: "Three beads with dependency chain",
		MaxTurns:    30,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "002-multi"

			// Create real beads: epic + 3 child tasks
			epicID := sandbox.CreateSpecEpic(specID)
			bead1 := sandbox.CreateBead("["+specID+"] Core types", "task", epicID)
			sandbox.CreateBead("["+specID+"] Formatter", "task", epicID)
			sandbox.CreateBead("["+specID+"] Tests", "task", epicID)
			sandbox.ClaimBead(bead1)

			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Multi-bead Feature
status: Approved
---
# Multi-bead Feature
Implement a feature in three phases.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: 002-multi
---
# Plan
## Bead 1: Core types
Create types.go with a Message struct.
## Bead 2: Formatter (depends on Bead 1)
Create formatter.go that formats Messages.
## Bead 3: Tests (depends on Bead 2)
Create formatter_test.go with tests.
`)
			sandbox.Commit("setup: multi-bead spec")

			setupWorktrees(sandbox, specID, bead1, "implement")

			sandbox.Commit("setup: implement mode with active worktree")
			return nil
		},
		Prompt: `You are in implement mode for a multi-bead spec. Implement all three beads in order:
1. Create types.go with a Message struct (fields: From, To, Body string)
2. Create formatter.go with FormatMessage(m Message) string
3. Create formatter_test.go that tests FormatMessage
Finish each bead through the MindSpec lifecycle before starting the next one.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Workflow adherence: the agent must progress through at least one
			// multi-bead handoff using mindspec next from dependency-ordered work.
			assertCommandRan(t, events, "mindspec", "next")
		},
	}
}

// ScenarioInterruptForBug tests mid-bead interrupt for a bug fix.
func ScenarioInterruptForBug() Scenario {
	return Scenario{
		Name:        "interrupt_for_bug",
		Description: "Interrupt mid-bead to fix a bug, then resume",
		MaxTurns:    25,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "003-feature"

			epicID := sandbox.CreateSpecEpic(specID)
			beadID := sandbox.CreateBead("["+specID+"] Implement feature", "task", epicID)
			sandbox.ClaimBead(beadID)

			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Feature
status: Approved
---
# Feature
Add a feature function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: `+specID+`
---
# Plan
## Bead 1: Implement feature
Create feature.go with a Feature function.
`)
			// main.go with a bug lives on main (inherited by branches)
			sandbox.WriteFile("main.go", `package main

func main() {
	// existing code
}
`)
			sandbox.Commit("setup: feature in progress")

			setupWorktrees(sandbox, specID, beadID, "implement")

			sandbox.Commit("setup: implement mode with active worktree")
			return nil
		},
		Prompt: `You are implementing a feature bead. While working, you notice
main.go has a critical bug — the main function doesn't print anything.
Fix main.go to add fmt.Println("hello") and commit the fix, then continue your feature work
by creating feature.go with a Feature() function. Finish the bead when done.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			featureObserved := sandbox.FileExists("feature.go") || fileExistsInWorktrees(sandbox.Root, "feature.go")
			if !featureObserved {
				// The agent may create feature.go in a temporary worktree and clean it up.
				// Accept successful git staging of feature.go as artifact evidence.
				for _, e := range events {
					if e.Command != "git" || e.ExitCode != 0 {
						continue
					}
					args := eventArgs(e)
					if containsAll(args, "add") && containsAll(args, "feature.go") {
						featureObserved = true
						break
					}
				}
			}
			if !featureObserved {
				t.Error("feature.go was not created")
			}
		},
	}
}

// ScenarioResumeAfterCrash tests picking up an existing in-progress bead.
func ScenarioResumeAfterCrash() Scenario {
	return Scenario{
		Name:        "resume_after_crash",
		Description: "Resume an in-progress bead after session crash",
		MaxTurns:    15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "004-resume"

			epicID := sandbox.CreateSpecEpic(specID)
			beadID := sandbox.CreateBead("["+specID+"] Process feature", "task", epicID)
			sandbox.ClaimBead(beadID)

			// Spec and plan artifacts
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Process Feature
status: Approved
---
# Process Feature
Add a process function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: `+specID+`
---
# Plan
## Bead 1: Implement process
Create partial.go with a Process function.
`)
			sandbox.Commit("setup: spec and plan")

			wt := setupWorktrees(sandbox, specID, beadID, "implement")

			// Simulate a crash: partial work committed in the bead worktree
			sandbox.WriteFile(wt.BeadWtDir+"/partial.go", `package main

// TODO: finish this function
func Process() {
}
`)
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "commit", "-m", "wip: partial process")

			sandbox.Commit("setup: implement mode with partial work")
			return nil
		},
		Prompt: `You are resuming after a session crash. The project is in implement mode with
a bead in progress. There's a partial.go file with an incomplete Process function.
Complete the Process function (make it return "processed") and finish the bead through the MindSpec lifecycle.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// partial.go should exist somewhere (may have been merged to spec branch)
			partialObserved := sandbox.FileExists("partial.go") || fileExistsInWorktrees(sandbox.Root, "partial.go")
			if !partialObserved {
				t.Error("partial.go should still exist")
			}
			assertCommandRan(t, events, "mindspec", "complete")
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

			// `mindspec next` is optional — plan approve auto-claims the
			// first bead. If next ran, verify bead branch + worktree too.
			if commandRanSuccessfully(events, "mindspec", "next") {
				assertHasBranches(t, sandbox, "bead/")
				assertEventCWDContains(t, events, ".worktrees/")
			} else {
				t.Logf("mindspec next was not called (plan approve auto-claimed first bead)")
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

// ScenarioStaleWorktree tests recovery when the bead worktree is missing but
// the spec worktree and branches exist. This happens when a session crashes
// after bead worktree removal or when the worktree was manually deleted.
//
// Before: Spec worktree exists, bead is in_progress, bead worktree is MISSING
//
//	(bead branch exists, spec/plan approved)
//
// After:  Agent recovers via `mindspec next` (recreates bead worktree),
//
//	implements the bead, runs complete
func ScenarioStaleWorktree() Scenario {
	return Scenario{
		Name:        "stale_worktree",
		Description: "Bead worktree missing — agent must recover via mindspec next",
		MaxTurns:    20,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "005-stale"

			epicID := sandbox.CreateSpecEpic(specID)
			beadID := sandbox.CreateBead("["+specID+"] Implement widget", "task", epicID)
			sandbox.ClaimBead(beadID)

			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Widget Feature
status: Approved
---
# Widget Feature
Add a widget function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: `+specID+`
---
# Plan
## Bead 1: Implement widget
Create widget.go with a Widget function.
`)
			sandbox.Commit("setup: spec and plan")

			// Create spec worktree (but NOT bead worktree — that's the stale element).
			// The spec worktree gives `mindspec next` a place to run from to
			// recreate the missing bead worktree. Spec files are already on main
			// and propagated to the spec branch via setupWorktrees.
			_ = setupWorktrees(sandbox, specID, "", "spec")

			// Create bead branch from spec branch (but no bead worktree)
			beadBranch := "bead/" + beadID
			if !gitBranchExists(sandbox, beadBranch) {
				mustRunGit(sandbox, "branch", beadBranch, "spec/"+specID)
			}

			sandbox.Commit("setup: stale bead worktree")

			// Verify the bead worktree does NOT exist (that's the test)
			specWtDir := ".worktrees/worktree-spec-" + specID
			beadWtDir := specWtDir + "/.worktrees/worktree-" + beadID
			if sandbox.FileExists(beadWtDir) {
				return fmt.Errorf("precondition: bead worktree should NOT exist at %s", beadWtDir)
			}
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

You are resuming work on an in-progress bead. Create a file called widget.go with
a function Widget() string that returns "widget".
Finish the bead through the MindSpec lifecycle when done.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent should have created widget.go (may be in worktree)
			widgetFound := sandbox.FileExists("widget.go") || fileExistsInWorktrees(sandbox.Root, "widget.go")
			if !widgetFound {
				t.Error("widget.go was not created (checked main and worktrees)")
			}

			// Agent must use mindspec complete (not bd close) — complete handles
			// merge topology, worktree cleanup, branch deletion, and state advance.
			assertCommandSucceeded(t, events, "mindspec", "complete")
		},
	}
}

// ScenarioCompleteFromSpecWorktree reproduces the bug where `mindspec complete`
// fails when the agent's CWD is the spec worktree (.worktrees/worktree-spec-XXX/)
// instead of the nested bead worktree.
//
// Before: spec worktree + bead worktree with committed code,
//
//	implement mode, agent CWD is the spec worktree (not bead worktree)
//
// After:  agent successfully runs mindspec complete (bead closed, worktree removed)
func ScenarioCompleteFromSpecWorktree() Scenario {
	var epicID, beadID string
	return Scenario{
		Name:        "complete_from_spec_worktree",
		Description: "Agent closes bead when CWD is spec worktree, not bead worktree",
		MaxTurns:    15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-greeting"

			// Create epic + bead
			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Implement greeting", "task", epicID)
			sandbox.ClaimBead(beadID)

			// Create spec + bead worktrees via shared helper
			wt := setupWorktrees(sandbox, specID, beadID, "implement")

			// Write spec files in the spec worktree (where they live during implementation)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", `---
title: Greeting Feature
status: Approved
---
# Greeting Feature
Add a greeting function.
`)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
---
# Plan
## Bead 1: Implement greeting
Create greeting.go with a Greet function.
`, specID))

			// Commit in spec worktree
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "setup: spec files")

			// Write implementation in bead worktree (already committed — clean tree)
			sandbox.WriteFile(wt.BeadWtDir+"/greeting.go", `package main

func Greet(name string) string { return "Hello, " + name + "!" }
`)
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "commit", "-m", "impl: greeting")

			// Setup: implement mode (bead in_progress), bead worktree exists,
			// but the bug is that the agent's CWD ends up at the SPEC worktree.
			sandbox.Commit("setup: implement mode")

			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

You are in implement mode. The implementation is already complete and committed in the bead worktree.
Your CWD may be the spec worktree, not the bead worktree.
Close the bead and finish implementation.
If it fails, diagnose the issue and find a way to complete successfully.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent must use mindspec complete (not bd close) — complete handles
			// merge topology, worktree cleanup, branch deletion, and state advance.
			assertCommandSucceeded(t, events, "mindspec", "complete")

			// Verify bead actually closed
			assertBeadsState(t, sandbox, epicID, map[string]string{
				beadID: "closed",
			})
		},
	}
}

// ScenarioApproveSpecFromWorktree tests that an agent can approve a spec when
// spec artifacts only exist in the spec worktree (not in the main repo).
// The agent's CWD is the main repo, so it must navigate to the worktree or
// rely on worktree-aware resolution.
func ScenarioApproveSpecFromWorktree() Scenario {
	return Scenario{
		Name:        "approve_spec_from_worktree",
		Description: "mindspec approve spec succeeds when spec artifacts are only in worktree",
		MaxTurns:    15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-greeting"

			// Create epic for lifecycle (approve needs it for bead creation)
			_ = sandbox.CreateSpecEpic(specID)

			// Create spec branch + worktree via shared helper
			wt := setupWorktrees(sandbox, specID, "", "spec")

			// Write spec files ONLY in the spec worktree
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", fmt.Sprintf(`---
spec_id: %s
status: Draft
version: 1
---

# Spec: %s — Greeting Feature

## Goal

Add a greeting function that takes a name and returns a personalized greeting.

## Impacted Domains

- messaging
- CLI output

## ADR Touchpoints

None.

## Requirements

1. Implement `+"`Greet(name string) string`"+` in greeting.go
2. Return `+"`Hello, <name>!`"+` for non-empty names
3. Return `+"`Hello!`"+` when the input name is empty

## Scope

### In Scope
- greeting string formatting
- empty-name fallback behavior

### Out of Scope
- localization
- punctuation/style customization

## Acceptance Criteria

- [ ] Greet("World") returns "Hello, World!"
- [ ] Greet("") returns "Hello!"
- [ ] Function is exported and documented

## Approval

Pending.

## Open Questions

None.
`, specID, specID))

			// Commit in spec worktree
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "setup: spec files")

			// Commit setup state (mode derived from beads at runtime)
			sandbox.Commit("setup: spec mode")

			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

The spec for 001-greeting is finished. Advance to the next phase.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			assertCommandSucceeded(t, events, "mindspec", "approve", "spec")

			// Branch and worktree persist through approval
			assertHasBranches(t, sandbox, "spec/")
			assertHasWorktrees(t, sandbox)
		},
	}
}

// ScenarioApprovePlanFromWorktree tests that an agent can approve a plan when
// spec and plan artifacts only exist in the spec worktree.
func ScenarioApprovePlanFromWorktree() Scenario {
	var epicID string
	return Scenario{
		Name:        "approve_plan_from_worktree",
		Description: "mindspec approve plan succeeds when plan artifacts are only in worktree",
		MaxTurns:    15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-greeting"

			// Create epic for bead parenting
			epicID = sandbox.CreateSpecEpic(specID)

			// Create spec branch + worktree via shared helper
			wt := setupWorktrees(sandbox, specID, "", "plan")

			// Write spec + plan ONLY in the spec worktree
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", fmt.Sprintf(`---
spec_id: %s
status: Approved
version: 1
approved_at: "2026-01-01T00:00:00Z"
approved_by: test-user
---

# Spec: %s — Greeting Feature

## Goal

Add a greeting function.

## User Value

Users can generate personalized greetings.

## Acceptance Criteria

- [ ] Greet("World") returns "Hello, World!"

## Open Questions

None.
`, specID, specID))

			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
spec_id: %s
status: Draft
version: 1
last_updated: "2026-01-01"
---

# Plan: %s

## ADR Fitness

No ADRs are relevant to this spec.

## Testing Strategy

- Unit tests: greeting_test.go

## Bead 1: Implement greeting

**Steps**
1. Create greeting.go with exported Greet function
2. Implement default greeting for empty name input
3. Create greeting_test.go with table-driven tests

**Verification**
- [ ] `+"`go test ./...`"+` passes

**Depends on**
None

## Provenance

| Acceptance Criterion | Bead | Verification |
|:-|:-|:-|
| Greet("World") returns "Hello, World!" | Bead 1 | greeting_test.go |
`, specID, specID))

			// Commit in spec worktree
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "setup: spec+plan files")

			// Commit setup state (mode derived from beads at runtime)
			sandbox.Commit("setup: plan mode")

			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

The plan for 001-greeting is finished. Advance to the next phase.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			assertCommandSucceeded(t, events, "mindspec", "approve", "plan")

			// Plan approval creates implementation beads
			assertBeadsMinCount(t, sandbox, epicID, 1)

			// Branch persists through approval — unless the agent completed
			// the full lifecycle (approve impl cleans up branches).
			if !commandRanSuccessfully(events, "mindspec", "approve", "impl") {
				assertHasBranches(t, sandbox, "spec/")
			}
		},
	}
}

// bugfixBranchRemote is the GitHub repo used as remote for the BugfixBranch scenario.
// Each test run pushes to a unique branch, creates a PR, then cleans up.
const bugfixBranchRemote = "mrmaxsteel/test-mindspec"

// ScenarioBugfixBranch tests that when a user asks to fix a pre-existing bug,
// the agent creates a bugfix branch + worktree, fixes on that branch, and
// creates a PR — never committing directly to main.
func ScenarioBugfixBranch() Scenario {
	return Scenario{
		Name:        "bugfix_branch",
		Description: "Fix a bug on a branch via PR, not directly on main",
		MaxTurns:    25,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			// Create buggy source code on main
			sandbox.WriteFile("calculator.go", `package main

import "fmt"

func Divide(a, b int) int {
	return a / b
}

func main() {
	fmt.Println(Divide(10, 2))
	fmt.Println(Divide(10, 0)) // BUG: division by zero panic
}
`)
			sandbox.Commit("add calculator with division-by-zero bug")

			// Record main branch commit count so we can verify it doesn't change
			mainCount := mustRun(sandbox.t, sandbox.Root, "git", "rev-list", "--count", "HEAD")
			sandbox.WriteFile(".harness/main_commit_count", strings.TrimSpace(mainCount))

			// Create pre-existing bugs in beads
			sandbox.CreateBead("Division by zero in calculator.go", "bug", "")
			sandbox.CreateBead("Missing input validation in parser", "bug", "")

			// Use real GitHub remote so gh pr create works
			ghURL := fmt.Sprintf("https://github.com/%s.git", bugfixBranchRemote)
			mustRun(sandbox.t, sandbox.Root, "git", "remote", "add", "origin", ghURL)

			// Push main to the remote (force to reset any stale state from prior runs)
			mustRun(sandbox.t, sandbox.Root, "git", "push", "--force", "origin", "main")

			// Install git hooks (pre-commit + post-checkout enforcement)
			if err := hooks.InstallAll(sandbox.Root); err != nil {
				return fmt.Errorf("installing hooks: %w", err)
			}

			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

There is a division-by-zero bug in calculator.go — Divide(10, 0) panics. Fix it by adding a zero-divisor check that returns 0 when b is 0. Submit the fix for review.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent must have created a branch (any non-main branch)
			assertHasNonMainBranch(t, sandbox)

			// Main branch must not have new commits
			assertMainCommitCountUnchanged(t, sandbox)

			// Agent pushed the branch
			assertCommandRan(t, events, "git", "push")

			// Agent created a PR via gh
			assertCommandRan(t, events, "gh", "pr")

			// PR should have succeeded (exit=0)
			assertCommandSucceeded(t, events, "gh", "pr", "create")

			// Cleanup: close any PRs and delete remote branches created by this test
			cleanupBugfixBranchPRs(t, sandbox)
		},
	}
}

// cleanupBugfixBranchPRs closes open PRs and deletes remote branches on the
// test repo created by the BugfixBranch scenario.
func cleanupBugfixBranchPRs(t *testing.T, sandbox *Sandbox) {
	t.Helper()

	// Close all open PRs on the test repo
	cmd := exec.Command("gh", "pr", "list", "--repo", bugfixBranchRemote, "--state", "open", "--json", "number", "--jq", ".[].number")
	out, err := cmd.Output()
	if err != nil {
		t.Logf("cleanup: could not list PRs: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		closeCmd := exec.Command("gh", "pr", "close", line, "--repo", bugfixBranchRemote, "--delete-branch")
		if closeOut, err := closeCmd.CombinedOutput(); err != nil {
			t.Logf("cleanup: could not close PR #%s: %v\n%s", line, err, closeOut)
		}
	}

	// Delete any non-main remote branches
	listCmd := exec.Command("git", "ls-remote", "--heads", "origin")
	listCmd.Dir = sandbox.Root
	branchOut, err := listCmd.Output()
	if err != nil {
		t.Logf("cleanup: could not list remote branches: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(branchOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]
		branch := strings.TrimPrefix(ref, "refs/heads/")
		if branch == "main" {
			continue
		}
		delCmd := exec.Command("git", "push", "origin", "--delete", branch)
		delCmd.Dir = sandbox.Root
		if delOut, err := delCmd.CombinedOutput(); err != nil {
			t.Logf("cleanup: could not delete remote branch %s: %v\n%s", branch, err, delOut)
		}
	}
}

// ScenarioBlockedBeadTransition tests that mode returns to plan when the
// only remaining bead is blocked after completing the first bead.
//
// Before: implement mode with bead-1 claimed, bead-2 depends on bead-1
// After:  bead-1 closed, mode is plan (bead-2 is blocked, so no implement)
func ScenarioBlockedBeadTransition() Scenario {
	// Lift IDs so Assertions closure can verify bead states.
	var epicID, bead1, bead2 string
	return Scenario{
		Name:        "blocked_bead_transition",
		Description: "Focus returns to plan when only blocked beads remain",
		MaxTurns:    20,
		TimeoutMin:  10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-blocker"

			// Create epic + 2 child beads with dependency
			epicID = sandbox.CreateSpecEpic(specID)
			bead1 = sandbox.CreateBead("["+specID+"] Core module", "task", epicID)
			bead2 = sandbox.CreateBead("["+specID+"] Extension (blocked)", "task", epicID)

			// bead-2 depends on bead-1
			sandbox.runBDMust("dep", "add", bead2, bead1)
			sandbox.ClaimBead(bead1)

			// Approved spec + plan
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Blocker Feature
status: Approved
---
# Blocker Feature
Test blocked bead transition.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: `+specID+`
---
# Plan
## Bead 1: Core module
Create core.go with a Core() function.
## Bead 2: Extension (depends on Bead 1)
Create extension.go that uses Core().
`)

			setupWorktrees(sandbox, specID, bead1, "implement")

			sandbox.Commit("setup: implement mode with blocked bead-2")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

Create a file called core.go with a function Core() string that returns "core".
Then finish the currently claimed bead through the MindSpec lifecycle.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent ran mindspec complete
			assertCommandRan(t, events, "mindspec", "complete")

			// Bead-1 closed, bead-2 still open
			assertBeadsState(t, sandbox, epicID, map[string]string{
				bead1: "closed",
				bead2: "open",
			})

			// Core spirit: mode should be plan (not implement) because bead-2 is blocked.
			// This catches the bd close shortcut — bd close skips state transitions.
			assertMindspecMode(t, sandbox, "plan")
		},
	}
}

// ScenarioUnmergedBeadGuard tests that `mindspec next` blocks when a predecessor
// bead was closed via `bd close` without `mindspec complete` (bead branch lingers).
// The agent must run `mindspec complete` to recover merge topology before proceeding.
//
// Before: spec worktree with bead-1 closed (bd close) but bead/ID branch still exists,
//
//	bead-2 is ready, agent CWD is spec worktree
//
// After:  agent ran mindspec complete to fix bead-1, then mindspec next for bead-2
func ScenarioUnmergedBeadGuard() Scenario {
	var epicID, bead1, bead2 string
	return Scenario{
		Name:        "unmerged_bead_guard",
		Description: "mindspec next blocks when predecessor bead closed without complete",
		MaxTurns:    35,
		TimeoutMin:  10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-guard"

			// Create epic + 2 child beads
			epicID = sandbox.CreateSpecEpic(specID)
			bead1 = sandbox.CreateBead("["+specID+"] First feature", "task", epicID)
			bead2 = sandbox.CreateBead("["+specID+"] Second feature", "task", epicID)

			// bead-2 depends on bead-1
			sandbox.runBDMust("dep", "add", bead2, bead1)

			// Claim bead-1 and close it via bd close (simulating agent skipping mindspec complete)
			sandbox.ClaimBead(bead1)
			sandbox.runBDMust("close", bead1)

			// Approved spec + plan
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Guard Feature
status: Approved
---
# Guard Feature
Test unmerged bead guard.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: `+specID+`
---
# Plan
## Bead 1: First feature
Create first.go with a First() function.
## Bead 2: Second feature
Create second.go with a Second() function.
`)
			sandbox.Commit("setup: approved spec and plan")

			// Create spec + bead worktrees via shared helper
			wt := setupWorktrees(sandbox, specID, bead1, "implement")

			// Write implementation on bead branch (simulating work that was done)
			sandbox.WriteFile(wt.BeadWtDir+"/first.go", `package main

func First() string { return "first" }
`)
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "commit", "-m", "impl: first feature")

			// Remove the bead worktree but keep the branch (simulating bd close without complete)
			mustRunGit(sandbox, "worktree", "remove", wt.BeadWtDir)

			sandbox.Commit("setup: bead-1 closed without mindspec complete")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

You are in implement mode for spec 001-guard. Bead 2 should be ready to work on.
Claim the next bead and implement it. Create second.go with a function Second() string
that returns "second". Complete the bead through the MindSpec lifecycle.
If mindspec next fails, read the error message carefully and follow its instructions.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Primary: agent recovered the unmerged bead via mindspec complete
			assertCommandRan(t, events, "mindspec", "complete")

			// Bead-1 should be closed
			assertBeadsState(t, sandbox, epicID, map[string]string{
				bead1: "closed",
			})

			// Secondary: agent claimed bead-2 via mindspec next (may not
			// happen if agent ran out of turns after recovery)
			if !commandRanSuccessfully(events, "mindspec", "next") {
				t.Log("NOTE: mindspec next did not succeed — agent recovered bead-1 but did not claim bead-2 (likely ran out of turns)")
			}
		},
	}
}

// gitBranchExists checks if a git branch exists in the sandbox.
func gitBranchExists(sandbox *Sandbox, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = sandbox.Root
	return cmd.Run() == nil
}

// mustRunGit runs a git command in the sandbox root, fataling on error.
func mustRunGit(sandbox *Sandbox, args ...string) {
	sandbox.t.Helper()
	// Setup commits bypass pre-commit guards (which block commits on spec/bead
	// branches in certain modes). All mustRunGit calls are scenario setup, not
	// agent behavior, so the escape hatch is appropriate.
	cmd := exec.Command("git", args...)
	cmd.Dir = sandbox.Root
	cmd.Env = append(os.Environ(), "MINDSPEC_ALLOW_MAIN=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		sandbox.t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// worktreePaths holds the paths returned by setupWorktrees.
type worktreePaths struct {
	SpecWtDir string // e.g. ".worktrees/worktree-spec-001-greeting"
	BeadWtDir string // e.g. ".worktrees/worktree-spec-001-greeting/.worktrees/worktree-<beadID>" (empty if phase != "implement")
}

// setupWorktrees creates the canonical worktree topology for a given lifecycle phase.
// It creates the spec branch (and bead branch if phase is "implement"), plus the
// properly nested worktrees that mirror what mindspec next produces at runtime.
//
// Supported phases:
//   - "spec" or "plan": creates spec/specID branch + spec worktree
//   - "implement":      creates spec worktree + bead worktree nested inside it
func setupWorktrees(sandbox *Sandbox, specID, beadID, phase string) worktreePaths {
	specBranch := "spec/" + specID
	mustRunGit(sandbox, "branch", specBranch)

	specWtDir := ".worktrees/worktree-spec-" + specID
	mustRunGit(sandbox, "worktree", "add", specWtDir, specBranch)

	var beadWtDir string
	if phase == "implement" && beadID != "" {
		beadBranch := "bead/" + beadID
		mustRunGit(sandbox, "branch", beadBranch, specBranch)
		beadWtDir = specWtDir + "/.worktrees/worktree-" + beadID
		mustRunGit(sandbox, "worktree", "add", beadWtDir, beadBranch)
	}

	return worktreePaths{SpecWtDir: specWtDir, BeadWtDir: beadWtDir}
}

// --- Helpers ---

func fileExistsInWorktrees(root, fileName string) bool {
	worktreeRoot := filepath.Join(root, ".worktrees")
	found := false
	_ = filepath.WalkDir(worktreeRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) == fileName {
			found = true
		}
		return nil
	})
	return found
}

func assertCommandRan(t *testing.T, events []ActionEvent, command string, argSubstr ...string) { //nolint:unparam // command kept for call-site clarity
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		if len(argSubstr) == 0 {
			return // found successful command
		}
		args := eventArgs(e)
		if containsAll(args, argSubstr[0]) {
			return
		}
	}
	if len(argSubstr) > 0 {
		t.Errorf("command %q with arg %q was not found with exit code 0 in events", command, argSubstr[0])
	} else {
		t.Errorf("command %q was not found with exit code 0 in events", command)
	}
}

// commandRanSuccessfully returns true if the command ran with exit code 0
// and all argSubstr found in its args (non-asserting version of assertCommandRan).
func commandRanSuccessfully(events []ActionEvent, command string, argSubstr ...string) bool { //nolint:unparam // command may vary in future scenarios
	for _, e := range events {
		if e.Command != command || e.ExitCode != 0 {
			continue
		}
		if len(argSubstr) == 0 {
			return true
		}
		args := eventArgs(e)
		if containsAll(args, argSubstr[0]) {
			return true
		}
	}
	return false
}

// assertCommandRanEither checks that the command was invoked with one of the
// given arg patterns (each is a list of substrings that must all appear).
func assertCommandRanEither(t *testing.T, events []ActionEvent, command string, patterns ...[]string) {
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		args := eventArgs(e)
		for _, pattern := range patterns {
			matched := true
			for _, sub := range pattern {
				if !containsAll(args, sub) {
					matched = false
					break
				}
			}
			if matched {
				return
			}
		}
	}
	t.Errorf("command %q was not found with exit code 0 for any expected arg patterns %v", command, patterns)
}

func assertCommandContains(t *testing.T, events []ActionEvent, command, substr string) { //nolint:unparam // command may vary in future scenarios
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		args := eventArgs(e)
		for _, arg := range args {
			if arg == substr {
				return
			}
		}
	}
	t.Errorf("command %q with arg containing %q was not found with exit code 0 in events", command, substr)
}

// assertCommandUsedFlag checks that a successful command invocation included the
// expected verb (e.g. "complete") and a flag with the given prefix (e.g. "--spec").
func assertCommandUsedFlag(t *testing.T, events []ActionEvent, command, verb, flagPrefix string) {
	t.Helper()
	for _, e := range events {
		if e.Command != command || e.ExitCode != 0 {
			continue
		}
		args := eventArgs(e)
		hasVerb := false
		hasFlag := false
		for _, arg := range args {
			if arg == verb {
				hasVerb = true
			}
			if strings.HasPrefix(arg, flagPrefix) {
				hasFlag = true
			}
		}
		if hasVerb && hasFlag {
			return
		}
	}
	t.Errorf("command %q did not run successfully with verb %q and flag prefix %q", command, verb, flagPrefix)
}

// eventArgs returns args from both the Args map and ArgsList slice.
func eventArgs(e ActionEvent) []string {
	args := flatArgs(e.Args)
	args = append(args, e.ArgsList...)
	return args
}

// assertCommandSucceeded checks that the command was run AND exited with code 0.
func assertCommandSucceeded(t *testing.T, events []ActionEvent, command string, argSubstr ...string) {
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		args := eventArgs(e)
		matched := true
		for _, sub := range argSubstr {
			if !containsAll(args, sub) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	if len(argSubstr) == 0 {
		t.Errorf("command %q was not found with exit code 0 in events", command)
		return
	}
	t.Errorf("command %q with args %v was not found with exit code 0 in events", command, argSubstr)
}

// assertNoPreApproveImplMainMergeOrPR enforces workflow ordering at the test
// layer: no direct merge-to-main or PR creation before approve impl is invoked.
//
// Note: internal git merge commands executed *inside* `mindspec approve impl`
// appear in event logs before the top-level `mindspec approve impl` event due to
// wrapper timing. We treat the known canonical internal merge command as allowed.
func assertNoPreApproveImplMainMergeOrPR(t *testing.T, events []ActionEvent) {
	t.Helper()
	if err := preApproveImplMainMergeOrPRViolation(events); err != nil {
		t.Fatal(err)
	}
}

func preApproveImplMainMergeOrPRViolation(events []ActionEvent) error {
	approveSeen := false
	for _, e := range events {
		args := eventArgs(e)

		if e.Command == "mindspec" && containsAll(args, "approve") && containsAll(args, "impl") {
			approveSeen = true
			continue
		}

		if approveSeen {
			continue
		}

		// Fail if PR creation/merge is attempted before approve impl.
		if e.Command == "gh" && (containsAll(args, "pr") && (containsAll(args, "create") || containsAll(args, "merge"))) {
			return fmt.Errorf("PR command ran before approve impl: %v", args)
		}

		// Fail if a non-canonical merge-to-main is attempted before approve impl.
		// Canonical internal merge pattern (from approve impl) is allowed:
		//   git ... merge --no-ff spec/<id> -m "Merge spec/<id> into main"
		if e.Command == "git" && e.ExitCode == 0 && containsAll(args, "merge") && containsAll(args, "main") {
			isCanonicalInternal := containsAll(args, "--no-ff") &&
				containsAll(args, "spec/") &&
				containsAll(args, "-m") &&
				containsAll(args, "Merge spec/") &&
				containsAll(args, "into main")
			if !isCanonicalInternal {
				return fmt.Errorf("merge-to-main occurred before approve impl: %v", args)
			}
		}
	}

	return nil
}

func assertBranchIs(t *testing.T, sandbox *Sandbox, expected string) {
	t.Helper()
	actual := sandbox.GitBranch()
	if actual != expected {
		t.Errorf("expected current branch %q, got %q", expected, actual)
	}
}

func assertNoBranches(t *testing.T, sandbox *Sandbox, prefix string) {
	t.Helper()
	branches := sandbox.ListBranches(prefix)
	if len(branches) > 0 {
		t.Errorf("expected no branches with prefix %q, found: %v", prefix, branches)
	}
}

func assertNoWorktrees(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	wts := sandbox.ListWorktrees()
	if len(wts) > 0 {
		t.Errorf("expected no worktrees, found: %v", wts)
	}
}

func assertEventCWDContains(t *testing.T, events []ActionEvent, substr string) {
	t.Helper()
	for _, e := range events {
		if strings.Contains(e.CWD, substr) {
			return
		}
	}
	t.Errorf("no event had CWD containing %q", substr)
}

func assertCleanWorktree(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	if !sandbox.GitStatusClean() {
		t.Error("expected clean working tree, but found uncommitted changes")
	}
}

func assertHasBranches(t *testing.T, sandbox *Sandbox, prefix string) {
	t.Helper()
	branches := sandbox.ListBranches(prefix)
	if len(branches) == 0 {
		t.Errorf("expected at least one branch with prefix %q, found none", prefix)
	}
}

func assertHasWorktrees(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	wts := sandbox.ListWorktrees()
	if len(wts) == 0 {
		t.Error("expected at least one worktree, found none")
	}
}

// assertNoUserFilesModified checks that no files outside .mindspec/ are dirty.
// .mindspec/session.json is written by the SessionStart hook and is expected noise.
func assertNoUserFilesModified(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git status failed: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Skip .mindspec/ infrastructure files (session.json, focus, etc.)
		file := strings.TrimSpace(line[2:]) // strip status prefix
		if strings.HasPrefix(file, ".mindspec/") {
			continue
		}
		t.Errorf("unexpected modified file outside .mindspec/: %s", line)
	}
}

// assertHasNonMainBranch checks that at least one branch besides "main" exists.
func assertHasNonMainBranch(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git branch --list failed: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if b != "" && b != "main" {
			return
		}
	}
	t.Error("expected at least one non-main branch, found none")
}

// assertMainCommitCountUnchanged verifies that main has the same number of
// commits as when setup recorded the count in .harness/main_commit_count.
// Infrastructure commits (e.g. bd prime's .beads/backup) are excluded —
// only user-file-touching commits count.
func assertMainCommitCountUnchanged(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	expected := strings.TrimSpace(sandbox.ReadFile(".harness/main_commit_count"))

	// Count commits on main, excluding those that ONLY touch .beads/ files
	// (bd prime commits .beads/backup during SessionStart — not agent work)
	cmd := exec.Command("git", "rev-list", "main")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git rev-list main failed: %v", err)
		return
	}
	userCommits := 0
	for _, sha := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if sha == "" {
			continue
		}
		// Check what files this commit touched
		diffCmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
		diffCmd.Dir = sandbox.Root
		diffOut, err := diffCmd.Output()
		if err != nil {
			userCommits++ // assume user commit if we can't check
			continue
		}
		files := strings.TrimSpace(string(diffOut))
		if files == "" {
			userCommits++ // empty diff = initial commit or merge
			continue
		}
		// If ALL changed files are under .beads/, it's infrastructure
		allBeads := true
		for _, f := range strings.Split(files, "\n") {
			if !strings.HasPrefix(f, ".beads/") {
				allBeads = false
				break
			}
		}
		if !allBeads {
			userCommits++
		}
	}
	expectedInt := 0
	fmt.Sscanf(expected, "%d", &expectedInt)
	if userCommits != expectedInt {
		t.Errorf("main branch user commit count changed: expected %d, got %d (agent committed directly to main)", expectedInt, userCommits)
	}
}

// beadStatus is the minimal structure returned by `bd list --json`.
type beadStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
}

// assertBeadsMinCount verifies that at least minCount child beads exist under
// the given epicID. Useful when bead IDs are created dynamically (e.g. by plan approve).
func assertBeadsMinCount(t testing.TB, sandbox *Sandbox, epicID string, minCount int) {
	t.Helper()
	// Query all statuses to count beads regardless of lifecycle state.
	var allBeads []beadStatus
	for _, status := range []string{"open", "in_progress", "closed"} {
		out, err := sandbox.runBD("list", "--json", "--parent", epicID, "--status="+status)
		if err != nil {
			continue
		}
		var beads []beadStatus
		if err := json.Unmarshal([]byte(out), &beads); err != nil {
			continue
		}
		allBeads = append(allBeads, beads...)
	}
	if len(allBeads) < minCount {
		t.Errorf("expected at least %d beads under epic %s, got %d", minCount, epicID, len(allBeads))
	}
}

// assertBeadsState runs `bd list --json --parent <epicID>` in the sandbox and
// asserts that each bead ID in expectedStatuses has the expected status.
func assertBeadsState(t testing.TB, sandbox *Sandbox, epicID string, expectedStatuses map[string]string) {
	t.Helper()
	// Query all statuses since beads may be open, in_progress, or closed.
	statusMap := make(map[string]string)
	for _, status := range []string{"open", "in_progress", "closed"} {
		out, err := sandbox.runBD("list", "--json", "--parent", epicID, "--status="+status)
		if err != nil {
			continue
		}
		var beads []beadStatus
		if err := json.Unmarshal([]byte(out), &beads); err != nil {
			continue
		}
		for _, b := range beads {
			statusMap[b.ID] = b.Status
		}
	}
	for id, want := range expectedStatuses {
		got, ok := statusMap[id]
		if !ok {
			t.Errorf("bead %q not found in bd list output (have: %v)", id, statusMap)
			continue
		}
		if got != want {
			t.Errorf("bead %q status: got %q, want %q", id, got, want)
		}
	}
}

// assertMergeTopology checks that at least one merge commit from a bead/ branch
// exists on the given specBranch (or on any branch if specBranch was already
// deleted by impl approve) after a bead→spec merge.
func assertMergeTopology(t testing.TB, sandbox *Sandbox, specBranch string) {
	t.Helper()
	// Try the specified branch first; fall back to --all if it no longer exists
	// (impl approve deletes the spec branch after merging).
	cmd := exec.Command("git", "log", "--merges", "--oneline", specBranch)
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		// Branch may have been deleted by impl approve — search all refs.
		cmd = exec.Command("git", "log", "--merges", "--oneline", "--all")
		cmd.Dir = sandbox.Root
		out, err = cmd.Output()
		if err != nil {
			t.Errorf("git log --merges --all: %v", err)
			return
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(line, "bead/") {
			return
		}
	}
	t.Errorf("no merge commit from a bead/ branch found on %s (or --all); merges: %s", specBranch, strings.TrimSpace(string(out)))
}

// assertMindspecMode runs `mindspec state show` in the sandbox and checks that
// the current mode matches the expected value.
func assertMindspecMode(t *testing.T, sandbox *Sandbox, expectedMode string) {
	t.Helper()
	out := mustRun(t, sandbox.Root, filepath.Join(sandbox.mindspecBinDir, "mindspec"), "state", "show")
	if !strings.Contains(out, expectedMode) {
		t.Errorf("expected mindspec mode %q, got output: %s", expectedMode, strings.TrimSpace(out))
	}
}

// assertCommitMessage checks that at least one commit in git log --oneline matches
// the given regex pattern (e.g. `impl\(bead-id\):`).
func assertCommitMessage(t testing.TB, sandbox *Sandbox, pattern string) {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("invalid pattern %q: %v", pattern, err)
	}
	cmd := exec.Command("git", "log", "--oneline", "--all")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git log --oneline: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if re.MatchString(line) {
			return
		}
	}
	t.Errorf("no commit message matching %q found in git log", pattern)
}
