package harness

import (
	"fmt"
	"strings"
	"testing"
)

// ScenarioSingleBead tests implementing a single pre-approved bead.
func ScenarioSingleBead() Scenario {
	// Lift IDs so both Setup and Assertions closures can access them.
	var epicID, beadID string
	return Scenario{
		Name:        "single_bead",
		Description: "Pre-approved plan, implement a single bead",
		MaxTurns:    35,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-greeting"

			// Create real beads: epic + child task
			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Implement greeting", "task", epicID)
			sandbox.ClaimBead(beadID)

			// Set up as if spec and plan are already approved
			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Greeting Feature
status: Approved
---
# Greeting Feature
Add a greeting function.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
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

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Multi-bead Feature
status: Approved
---
# Multi-bead Feature
Implement a feature in three phases.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
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
			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Process Feature
status: Approved
---
# Process Feature
Add a process function.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
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
			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Blocker Feature
status: Approved
---
# Blocker Feature
Test blocked bead transition.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
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
			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Guard Feature
status: Approved
---
# Guard Feature
Test unmerged bead guard.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
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

// ScenarioStopAfterComplete tests the SOP: after completing a bead, the agent
// STOPS and does NOT automatically claim the next bead via `mindspec next`.
//
// Before: Multi-bead spec (2 beads), bead-1 claimed, implement mode
// After:  Agent completes bead-1 and STOPS — does NOT run `mindspec next` for bead-2
func ScenarioStopAfterComplete() Scenario {
	var epicID, bead1, bead2 string
	return Scenario{
		Name:        "stop_after_complete",
		Description: "Agent stops after completing a bead, does not auto-claim next",
		MaxTurns:    35,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-stop"

			epicID = sandbox.CreateSpecEpic(specID)
			bead1 = sandbox.CreateBead("["+specID+"] First task", "task", epicID)
			bead2 = sandbox.CreateBead("["+specID+"] Second task", "task", epicID)
			// bead2 depends on bead1 — blocked until bead1 is closed.
			// This prevents Haiku from getting distracted by bead2 at session start.
			sandbox.runBDMust("dep", "add", bead2, bead1)
			sandbox.ClaimBead(bead1)

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Stop Test
status: Approved
---
# Stop Test
Two tasks.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
status: Approved
spec_id: `+specID+`
---
# Plan
## Bead 1: First task
Create first.go with a First() function.
## Bead 2: Second task
Create second.go with a Second() function.
`)
			sandbox.Commit("setup: two-bead spec")
			setupWorktrees(sandbox, specID, bead1, "implement")
			sandbox.Commit("setup: implement mode")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

Create a file called first.go with a function First() string that returns "first".
Then finish the currently claimed bead through the MindSpec lifecycle.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Find the first bead-close event (mindspec complete or bd close).
			firstCloseIdx := -1
			for i, e := range events {
				if e.ExitCode != 0 {
					continue
				}
				args := eventArgs(e)
				isMSComplete := e.Command == "mindspec" && containsAll(args, "complete")
				isBDClose := e.Command == "bd" && containsAll(args, "close")
				if isMSComplete || isBDClose {
					firstCloseIdx = i
					break
				}
			}
			if firstCloseIdx == -1 {
				t.Fatal("agent never completed any bead (no mindspec complete or bd close succeeded)")
			}

			// CRITICAL: Agent did NOT run mindspec next AFTER closing a bead.
			// Running `mindspec next` before the first completion is OK (agent orienting).
			for _, e := range events[firstCloseIdx+1:] {
				if e.Command == "mindspec" && e.ExitCode == 0 && containsAll(eventArgs(e), "next") {
					t.Error("agent ran 'mindspec next' after completing a bead — should have STOPPED (SOP violation)")
					break
				}
			}

			// Exactly one bead closed, the other still open.
			// The agent might complete bead-1 (as prompted) or bead-2 (via mindspec next).
			// Either way, only one should be closed if the agent stopped.
			b1Status := beadStatusStr(sandbox, bead1)
			b2Status := beadStatusStr(sandbox, bead2)
			closedCount := 0
			if b1Status == "closed" {
				closedCount++
			}
			if b2Status == "closed" {
				closedCount++
			}
			if closedCount == 0 {
				t.Error("no beads were closed — agent failed to complete any bead")
			} else if closedCount == 2 {
				t.Error("both beads were closed — agent should have STOPPED after closing the first one")
			}

			// Preferred: agent used mindspec complete (not bd close).
			if !commandRanSuccessfully(events, "mindspec", "complete") {
				t.Log("NOTE: agent used bd close instead of mindspec complete")
			}
		},
	}
}

// ScenarioStopDoesNotBlockApproveImpl tests that the STOP instruction after
// `mindspec complete` does not prevent the agent from running `approve impl`
// when the prompt explicitly asks for the spec to reach idle/review.
//
// Before: Single-bead spec, bead-1 claimed, implement mode
// After:  Agent completes bead-1 AND runs approve impl to reach idle
func ScenarioStopDoesNotBlockApproveImpl() Scenario {
	var epicID, beadID string
	return Scenario{
		Name:        "stop_does_not_block_approve_impl",
		Description: "STOP after complete does not prevent approve impl when prompted",
		MaxTurns:    35,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-approve"

			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Implement feature", "task", epicID)
			sandbox.ClaimBead(beadID)

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Approve Test
status: Approved
---
# Approve Test
Single feature.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Implement feature
Create feature.go with a Feature() function.
`, specID, beadID))
			sandbox.Commit("setup: single-bead spec")
			setupWorktrees(sandbox, specID, beadID, "implement")
			sandbox.Commit("setup: implement mode")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

Create a file called feature.go with a function Feature() string that returns "feature".
Finish the bead and take this spec all the way to idle — complete the bead, then
approve the implementation so the project returns to idle mode.
Do not close beads directly with bd commands.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent completed the bead (mindspec complete preferred, bd close tolerated).
			if !commandRanSuccessfully(events, "mindspec", "complete") {
				if !commandRanSuccessfully(events, "bd", "close") {
					t.Error("agent never closed the bead (no mindspec complete or bd close)")
				}
				t.Log("NOTE: agent used bd close instead of mindspec complete")
			}

			// CRITICAL: Agent continued past STOP to run approve impl
			assertCommandSucceeded(t, events, "mindspec", "approve", "impl")

			// Bead was closed
			assertBeadsState(t, sandbox, epicID, map[string]string{
				beadID: "closed",
			})
		},
	}
}

// ScenarioBeadsArtifactPassthrough canaries the combined effect of spec 082
// Bead 1a (artifact-aware dirty-tree guard) and Bead 2 (executor bd export):
// a pre-session `bd create` has left `.beads/issues.jsonl` dirty, and the
// agent should run `mindspec next` → implement → `mindspec complete` without
// stashing, branching for the artifact, or otherwise touching it. The prompt
// deliberately omits the dirty JSONL so this tests the product behavior, not
// prompt guidance.
func ScenarioBeadsArtifactPassthrough() Scenario {
	var epicID, taskBeadID, driveByID string
	return Scenario{
		Name:        "beads_artifact_passthrough",
		Description: "mindspec next/complete handle a pre-session dirty .beads/issues.jsonl without workarounds",
		MaxTurns:    35,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-hello"

			// Track .beads/issues.jsonl while keeping runtime files ignored, so the
			// scenario mirrors a real MindSpec repo where the JSONL is a versioned
			// projection of Dolt state (ADR-0025). Retains the default sandbox
			// ignores for .harness/, .worktrees/, and ephemeral .mindspec files.
			sandbox.WriteFile(".gitignore", ".beads/*\n!.beads/issues.jsonl\n.harness/\n.worktrees/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n")

			// Spec epic + one open task bead — the ready work the agent must claim.
			epicID = sandbox.CreateSpecEpic(specID)
			taskBeadID = sandbox.CreateBead("["+specID+"] Create hello.go", "task", epicID)

			// Deferred sibling: keeps the epic from auto-closing when the task
			// bead closes (Beads molecule auto-close fires once every child is
			// closed). Deferred beads stay out of `bd ready`, so they don't
			// perturb `mindspec next` selection. Without this, `mindspec complete`
			// can hit "no active specs found" during its post-close state advance.
			keepaliveID := sandbox.CreateBead("["+specID+"] future: follow-up", "task", epicID)
			sandbox.runBDMust("defer", keepaliveID)

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Hello Feature
status: Approved
---
# Hello Feature
Add a hello.go file that prints a greeting.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Create hello.go
Create hello.go with a Hello() function that returns "hi".
`, specID, taskBeadID))

			// Establish the baseline JSONL so it's tracked by git before setup
			// completes. Subsequent exports will show as modifications.
			if _, err := sandbox.runBD("export", "-o", ".beads/issues.jsonl"); err != nil {
				return fmt.Errorf("bd export baseline: %w", err)
			}
			sandbox.Commit("setup: spec + approved plan + baseline JSONL")

			// Plan-phase worktree only — no bead worktree. The agent must run
			// `mindspec next` to claim the bead and create its worktree, which is
			// the moment where the dirty-tree guard is exercised.
			setupWorktrees(sandbox, specID, "", "plan")
			sandbox.Commit("setup: plan mode")

			// Simulate a drive-by `bd create` from before the session (e.g. the
			// user filed a quick cleanup issue without committing). Parent-less so
			// it stays out of the spec's ready list; closed so `bd ready` ignores
			// it entirely. The ID still lands in the JSONL diff, which is the
			// canary for Bead 2.
			driveByID = sandbox.CreateBead("Drive-by: refresh stale dashboard", "task", "")
			sandbox.runBDMust("close", driveByID)
			if _, err := sandbox.runBD("export", "-o", ".beads/issues.jsonl"); err != nil {
				return fmt.Errorf("bd export dirty: %w", err)
			}
			// Intentionally not committed. `.beads/issues.jsonl` is now dirty on main.

			// Precondition: the whole scenario is meaningless if the tree isn't
			// actually dirty on the artifact. Surface setup drift loudly instead
			// of silently testing nothing.
			if sandbox.GitStatusClean() {
				return fmt.Errorf("precondition: expected .beads/issues.jsonl dirty after drive-by export, but tree is clean")
			}

			return nil
		},
		// The prompt deliberately names the lifecycle commands (`mindspec next`,
		// `mindspec complete`) because the friction under test lives inside
		// `mindspec next`'s dirty-tree guard, not in the agent's command
		// discovery. Keeping the task prescriptive isolates the variable this
		// canary is supposed to measure: did the guard let the claim through
		// without the agent resorting to `git stash` or a `chore/` branch?
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

The spec 001-hello is approved and has one ready bead.
1. Run mindspec next to claim the ready bead.
2. Create hello.go with a function Hello() string that returns "hi".
3. Run mindspec complete to finish the bead.

Do not close beads directly with bd commands.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// (a) No git stash — the dirty JSONL should not have prompted one.
			// No chore/ branch — the agent should not have branched for the artifact.
			for _, e := range events {
				if e.Command != "git" {
					continue
				}
				args := eventArgs(e)
				if containsAll(args, "stash") {
					t.Errorf("agent ran git stash (artifact should not require stashing): %v", args)
				}
				for _, a := range args {
					if strings.HasPrefix(a, "chore/") {
						t.Errorf("agent referenced chore/ branch (artifact should not require branching): %v", args)
						break
					}
				}
			}

			// (b) mindspec next succeeded — the dirty-tree guard allowed the claim
			// through without retry on the artifact path.
			assertCommandSucceeded(t, events, "mindspec", "next")

			// The agent carried out the implementation task.
			helloObserved := sandbox.FileExists("hello.go") || fileExistsInWorktrees(sandbox.Root, "hello.go")
			if !helloObserved {
				for _, e := range events {
					if e.Command != "git" || e.ExitCode != 0 {
						continue
					}
					args := eventArgs(e)
					if containsAll(args, "add") && containsAll(args, "hello.go") {
						helloObserved = true
						break
					}
				}
			}
			if !helloObserved {
				t.Error("hello.go was not created")
			}

			// mindspec complete ran — the commit step is where Bead 2's executor
			// bd export runs before `git add -A`.
			assertCommandSucceeded(t, events, "mindspec", "complete")
			assertBeadsState(t, sandbox, epicID, map[string]string{
				taskBeadID: "closed",
			})

			// (c) Post-session `.beads/issues.jsonl` contains the pre-seeded
			// drive-by issue's ID. This proves Bead 2's executor export ran
			// during the lifecycle and carried pre-session Dolt state into the
			// tracked artifact. The stricter check ("the specific `mindspec
			// complete` commit's diff contains this ID") is deferred until the
			// sandbox gains per-worktree `.beads/redirect` wiring — the pinned
			// `bd` shim currently redirects every `bd export` back to sandbox
			// root, so it never lands inside a bead worktree's commit. See
			// mindspec-4u93 for the redirect-gap tracking bug.
			jsonl := sandbox.ReadFile(".beads/issues.jsonl")
			if !strings.Contains(jsonl, driveByID) {
				t.Errorf("post-session .beads/issues.jsonl does not contain drive-by issue %q — bd export was not carried through the lifecycle", driveByID)
			}
		},
	}
}
