package harness

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// This file owns worktree-topology scenarios plus the shared worktree
// helpers (mustRunGit, setupWorktrees, fileExistsInWorktrees, gitBranchExists,
// worktreePaths) used by scenarios across all four scenario_*.go files.

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

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Widget Feature
status: Approved
---
# Widget Feature
Add a widget function.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
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
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/spec.md", `---
title: Greeting Feature
status: Approved
---
# Greeting Feature
Add a greeting function.
`)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
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
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/spec.md", fmt.Sprintf(`---
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
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/spec.md", fmt.Sprintf(`---
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

			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
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

// gitBranchExists checks if a git branch exists in the sandbox.
//
// R7 (spec 120): branch is a dynamic operand reaching a git spawn —
// guard with gitutil.RejectOptionLike, fail-fast t.Fatalf before the
// spawn (SEC-5).
func gitBranchExists(sandbox *Sandbox, branch string) bool {
	sandbox.t.Helper()
	if err := gitutil.RejectOptionLike(branch); err != nil {
		sandbox.t.Fatalf("gitBranchExists: %v", err)
	}
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
	// No *config.Config is available in this test helper; the sandbox
	// hard-codes ".worktrees" for fixture setup, so use the default.
	worktreeRoot := workspace.DefaultWorktreesDir(root)
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
