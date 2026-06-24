package harness

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/githooks"
)

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

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Feature
status: Approved
---
# Feature
Add a feature function.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", `---
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
			if err := githooks.InstallAll(sandbox.Root); err != nil {
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
