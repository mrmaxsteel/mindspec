package harness

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// Scenario defines a behavioral test scenario for an agent session.
type Scenario struct {
	Name        string                                                     // scenario identifier (e.g. "single_bead")
	Description string                                                     // human-readable description
	Setup       func(sandbox *Sandbox) error                               // prepares sandbox state before agent runs
	Prompt      string                                                     // the prompt given to the agent
	Assertions  func(t *testing.T, sandbox *Sandbox, events []ActionEvent) // post-run assertions
	MaxTurns    int                                                        // turn budget (0 = unlimited)
	Model       string                                                     // model override (e.g. "haiku")
}

// AllScenarios returns all defined behavior scenarios.
func AllScenarios() []Scenario {
	return []Scenario{
		ScenarioSpecToIdle(),
		ScenarioSingleBead(),
		ScenarioMultiBeadDeps(),
		ScenarioAbandonSpec(),
		ScenarioInterruptForBug(),
		ScenarioResumeAfterCrash(),
		ScenarioHookBlocksCodeInSpec(),
		ScenarioHookBlocksMainCommit(),
		ScenarioHookBlocksStaleNext(),
	}
}

// ScenarioSpecToIdle tests the full lifecycle: explore → spec → plan → implement → review → idle.
func ScenarioSpecToIdle() Scenario {
	return Scenario{
		Name:        "spec_to_idle",
		Description: "Full lifecycle from explore through idle",
		MaxTurns:    75,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			// Sandbox comes with a clean repo; agent starts from scratch
			return nil
		},
		Prompt: `IMPORTANT: You must execute these commands immediately. Do NOT respond conversationally. Do NOT ask what I'd like to do. Execute each step in order using the Bash tool:

Step 1: Run this command now:
  mindspec explore "add greeting feature"

Step 2: Run this command:
  mindspec explore promote 001-greeting

Step 3: Write a minimal spec.md file to .mindspec/docs/specs/001-greeting/spec.md with YAML frontmatter (title: Greeting Feature, status: Draft) and a short description.

Step 4: Run this command:
  mindspec approve spec 001-greeting

Step 5: Write a minimal plan.md file to .mindspec/docs/specs/001-greeting/plan.md with YAML frontmatter (status: Draft, spec_id: 001-greeting) and one bead section "## Bead 1: Create hello.go Program" describing a simple hello.go file.

Step 6: Run this command:
  mindspec approve plan 001-greeting

Step 7: Run this command:
  mindspec next

Step 8: cd into the bead worktree directory shown by mindspec next output, then create hello.go with a simple main package that prints "Hello!", then git add and git commit.

Step 9: Run this command:
  mindspec complete

Execute step 1 NOW.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			assertCommandRan(t, events, "mindspec", "explore")
			assertCommandRan(t, events, "mindspec", "next")
			assertCommandRan(t, events, "mindspec", "complete")
		},
	}
}

// ScenarioSingleBead tests implementing a single pre-approved bead.
func ScenarioSingleBead() Scenario {
	return Scenario{
		Name:        "single_bead",
		Description: "Pre-approved plan, implement a single bead",
		MaxTurns:    15,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			// Create real beads: epic + child task
			epicID := sandbox.CreateBead("[001-greeting] Epic", "epic", "")
			beadID := sandbox.CreateBead("[001-greeting] Implement greeting", "task", epicID)
			sandbox.ClaimBead(beadID)

			// Set up as if spec and plan are already approved
			sandbox.WriteFile(".mindspec/docs/specs/001-greeting/spec.md", `---
title: Greeting Feature
status: Approved
---
# Greeting Feature
Add a greeting function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/001-greeting/plan.md", `---
status: Approved
spec_id: 001-greeting
---
# Plan
## Bead 1: Implement greeting
Create greeting.go with a Greet function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/001-greeting/lifecycle.yaml",
				fmt.Sprintf("phase: implement\nepic_id: %s\n", epicID))
			sandbox.WriteFocus(mustJSON(map[string]string{
				"mode":       "implement",
				"activeSpec": "001-greeting",
				"activeBead": beadID,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}))
			sandbox.Commit("setup: pre-approved spec and plan")
			return nil
		},
		Prompt: `You are in implement mode for a pre-approved spec. A bead is already claimed.
Your task: create a file called greeting.go with a function Greet(name string) string
that returns "Hello, <name>!". Then git add and git commit your changes.
Finally run 'mindspec complete' to finish the bead. You MUST commit before completing.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Agent should have created the file
			if !sandbox.FileExists("greeting.go") {
				t.Error("greeting.go was not created")
			}
			// Agent should have run mindspec complete
			assertCommandRan(t, events, "mindspec", "complete")
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
			// Create real beads: epic + 3 child tasks
			epicID := sandbox.CreateBead("[002-multi] Epic", "epic", "")
			bead1 := sandbox.CreateBead("[002-multi] Core types", "task", epicID)
			sandbox.CreateBead("[002-multi] Formatter", "task", epicID)
			sandbox.CreateBead("[002-multi] Tests", "task", epicID)
			sandbox.ClaimBead(bead1)

			sandbox.WriteFile(".mindspec/docs/specs/002-multi/spec.md", `---
title: Multi-bead Feature
status: Approved
---
# Multi-bead Feature
Implement a feature in three phases.
`)
			sandbox.WriteFile(".mindspec/docs/specs/002-multi/plan.md", `---
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
			sandbox.WriteFile(".mindspec/docs/specs/002-multi/lifecycle.yaml",
				fmt.Sprintf("phase: implement\nepic_id: %s\n", epicID))
			sandbox.WriteFocus(mustJSON(map[string]string{
				"mode":       "implement",
				"activeSpec": "002-multi",
				"activeBead": bead1,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}))
			sandbox.Commit("setup: multi-bead spec")
			return nil
		},
		Prompt: `You are in implement mode for a multi-bead spec. Implement all three beads in order:
1. Create types.go with a Message struct (fields: From, To, Body string)
2. Create formatter.go with FormatMessage(m Message) string
3. Create formatter_test.go that tests FormatMessage
Run 'mindspec complete' after each bead.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			for _, f := range []string{"types.go", "formatter.go", "formatter_test.go"} {
				if !sandbox.FileExists(f) {
					t.Errorf("%s was not created", f)
				}
			}
		},
	}
}

// ScenarioAbandonSpec tests explore → dismiss flow.
func ScenarioAbandonSpec() Scenario {
	return Scenario{
		Name:        "abandon_spec",
		Description: "Enter explore mode and dismiss without promoting",
		MaxTurns:    10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			return nil
		},
		Prompt: `IMPORTANT: Execute these commands immediately. Do NOT respond conversationally. Do NOT ask what I'd like to do. Execute each step in order using the Bash tool:

Step 1: Run this command now:
  mindspec explore "bad idea"

Step 2: Run this command:
  mindspec explore dismiss

Execute step 1 NOW.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			assertCommandRan(t, events, "mindspec", "explore")
			// Check that dismiss was called
			assertCommandContains(t, events, "mindspec", "dismiss")
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
			epicID := sandbox.CreateBead("[003-feature] Epic", "epic", "")
			beadID := sandbox.CreateBead("[003-feature] Implement feature", "task", epicID)
			sandbox.ClaimBead(beadID)

			sandbox.WriteFile(".mindspec/docs/specs/003-feature/lifecycle.yaml",
				fmt.Sprintf("phase: implement\nepic_id: %s\n", epicID))
			sandbox.WriteFocus(mustJSON(map[string]string{
				"mode":       "implement",
				"activeSpec": "003-feature",
				"activeBead": beadID,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}))
			sandbox.WriteFile("main.go", `package main

func main() {
	// existing code
}
`)
			sandbox.Commit("setup: feature in progress")
			return nil
		},
		Prompt: `You are implementing a feature bead. While working, you notice
main.go has a critical bug — the main function doesn't print anything.
Fix main.go to add fmt.Println("hello") and commit the fix, then continue your feature work
by creating feature.go with a Feature() function. Run 'mindspec complete' when done.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			if !sandbox.FileExists("feature.go") {
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
			epicID := sandbox.CreateBead("[004-resume] Epic", "epic", "")
			beadID := sandbox.CreateBead("[004-resume] Process feature", "task", epicID)
			sandbox.ClaimBead(beadID)

			// Simulate a crash: focus says implement, bead is in_progress, partial work exists
			sandbox.WriteFile(".mindspec/docs/specs/004-resume/lifecycle.yaml",
				fmt.Sprintf("phase: implement\nepic_id: %s\n", epicID))
			sandbox.WriteFocus(mustJSON(map[string]string{
				"mode":       "implement",
				"activeSpec": "004-resume",
				"activeBead": beadID,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}))
			sandbox.WriteFile("partial.go", `package main

// TODO: finish this function
func Process() {
}
`)
			sandbox.Commit("setup: partial work before crash")
			return nil
		},
		Prompt: `You are resuming after a session crash. The project is in implement mode with
a bead in progress. There's a partial.go file with an incomplete Process function.
Complete the Process function (make it return "processed") and run 'mindspec complete'.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			if !sandbox.FileExists("partial.go") {
				t.Error("partial.go should still exist")
			}
			assertCommandRan(t, events, "mindspec", "complete")
		},
	}
}

// ScenarioHookBlocksCodeInSpec tests that writing code in spec mode gets blocked.
func ScenarioHookBlocksCodeInSpec() Scenario {
	return Scenario{
		Name:        "hook_blocks_code_in_spec",
		Description: "Agent attempts code in spec mode, gets blocked, recovers",
		MaxTurns:    10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			sandbox.WriteFile(".mindspec/docs/specs/005-blocked/lifecycle.yaml",
				"phase: spec\n")
			sandbox.WriteFocus(mustJSON(map[string]string{
				"mode":       "spec",
				"activeSpec": "005-blocked",
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}))
			sandbox.Commit("setup: spec mode")
			return nil
		},
		Prompt: `You are in spec mode for spec 005-blocked. Write the spec document at
.mindspec/docs/specs/005-blocked/spec.md. Do NOT write any code files — you are in spec mode.
If you feel tempted to write code, remember that hooks will block it.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// No .go files should exist (they'd be blocked)
			log := NewEventLog(events)
			for _, e := range log.Events {
				if (e.ToolName == "Write" || e.ToolName == "Edit") && !e.Blocked {
					path := e.Args["file_path"]
					if isCodePath(path) {
						t.Errorf("code file written in spec mode: %s", path)
					}
				}
			}
		},
	}
}

// ScenarioHookBlocksMainCommit tests that committing from main worktree gets blocked.
func ScenarioHookBlocksMainCommit() Scenario {
	return Scenario{
		Name:        "hook_blocks_main_commit",
		Description: "Agent tries to commit from main worktree, gets blocked",
		MaxTurns:    10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			epicID := sandbox.CreateBead("[006-main] Epic", "epic", "")
			beadID := sandbox.CreateBead("[006-main] Implement work", "task", epicID)
			sandbox.ClaimBead(beadID)

			sandbox.WriteFile(".mindspec/docs/specs/006-main/lifecycle.yaml",
				fmt.Sprintf("phase: implement\nepic_id: %s\n", epicID))
			sandbox.WriteFocus(mustJSON(map[string]string{
				"mode":       "implement",
				"activeSpec": "006-main",
				"activeBead": beadID,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}))
			sandbox.Commit("setup: implement mode in main")
			return nil
		},
		Prompt: `You are in implement mode. Create a file called work.go with a Work() function.
Then commit your changes using git. Note: you should be working in a worktree, but
you are in the main repo. The hook system should guide you.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// We expect the agent to encounter the CWD enforcement
			// The test validates the scenario ran without panicking
		},
	}
}

// ScenarioHookBlocksStaleNext tests the session freshness gate.
func ScenarioHookBlocksStaleNext() Scenario {
	return Scenario{
		Name:        "hook_blocks_stale_next",
		Description: "Session freshness gate blocks stale mindspec next",
		MaxTurns:    5,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			// Set up a stale session (old timestamp)
			sandbox.WriteFile(".mindspec/session.json", mustJSON(map[string]string{
				"sessionSource":    "test",
				"sessionStartedAt": "2020-01-01T00:00:00Z",
			}))
			sandbox.Commit("setup: stale session")
			return nil
		},
		Prompt: `You are in a MindSpec project. Run 'mindspec next' to find work.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// The scenario should complete (agent may get blocked by freshness gate)
			assertCommandRan(t, events, "mindspec", "next")
		},
	}
}

// --- Helpers ---

func mustJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustJSON: %v", err))
	}
	return string(data)
}

func assertCommandRan(t *testing.T, events []ActionEvent, command string, argSubstr ...string) { //nolint:unparam // command kept for call-site clarity
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if len(argSubstr) == 0 {
			return // found the command
		}
		args := eventArgs(e)
		if containsAll(args, argSubstr[0]) {
			return
		}
	}
	if len(argSubstr) > 0 {
		t.Errorf("command %q with arg %q was not found in events", command, argSubstr[0])
	} else {
		t.Errorf("command %q was not found in events", command)
	}
}

func assertCommandContains(t *testing.T, events []ActionEvent, command, substr string) {
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		args := eventArgs(e)
		for _, arg := range args {
			if arg == substr {
				return
			}
		}
	}
	t.Errorf("command %q with arg containing %q was not found in events", command, substr)
}

// eventArgs returns args from both the Args map and ArgsList slice.
func eventArgs(e ActionEvent) []string {
	args := flatArgs(e.Args)
	args = append(args, e.ArgsList...)
	return args
}
