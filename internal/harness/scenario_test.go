package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// skipUnlessClaudeCode skips the test if the claude CLI is not available.
func skipUnlessClaudeCode(t *testing.T) {
	t.Helper()
	if !ClaudeCodeAvailable() {
		t.Skip("claude CLI not available (install Claude Code and authenticate to run LLM tests)")
	}
}

func resolveTestAgent(t *testing.T) Agent {
	t.Helper()
	name := DefaultAgentName()
	agent, err := ResolveAgent(name)
	if err != nil {
		t.Fatalf("resolving agent %q: %v", name, err)
	}
	return agent
}

func runScenario(t *testing.T, scenario Scenario) (*Report, *Sandbox) {
	t.Helper()
	skipUnlessClaudeCode(t)

	agent := resolveTestAgent(t)
	sandbox := NewSandbox(t)

	timeoutMin := scenario.TimeoutMin
	if timeoutMin <= 0 {
		timeoutMin = 10
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMin)*time.Minute)
	defer cancel()

	result, err := RunSession(ctx, agent, scenario, sandbox)
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	events := sandbox.ReadEvents()

	// Log recorded shim events for observability (before assertions, so we see
	// them even if assertions fatal). This is the primary diagnostic for
	// understanding how far the agent got.
	t.Logf("--- Recorded events (%d) ---", len(events))
	for i, ev := range events {
		args := strings.Join(ev.ArgsList, " ")
		if args == "" {
			args = fmt.Sprintf("%v", ev.Args)
		}
		blocked := ""
		if ev.Blocked {
			blocked = fmt.Sprintf(" [BLOCKED: %s]", ev.BlockReason)
		}
		t.Logf("  [%d] %s %s (exit=%d)%s", i+1, ev.Command, args, ev.ExitCode, blocked)
	}

	// Log agent output early — before analysis/assertions so it's visible on failure.
	t.Logf("--- Agent output (exit=%d, dur=%s) ---\n%s", result.ExitCode, result.Duration, result.Output)

	// Run analyzer
	analyzer := NewAnalyzer()
	summaries := analyzer.Classify(events)
	wrongActions := analyzer.DetectWrongActions(events)

	report := NewReport(scenario.Name, agent.Name(), summaries, wrongActions, 0)
	t.Logf("Report:\n%s", report.FormatText())

	// Run scenario-specific assertions
	if scenario.Assertions != nil {
		scenario.Assertions(t, sandbox, events)
	}

	return report, sandbox
}

// --- Happy path scenarios ---

func TestLLM_SpecToIdle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioSpecToIdle())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
		for _, wa := range report.WrongActions {
			t.Logf("  [%s] %s", wa.Rule, wa.Reason)
		}
	}
}

func TestLLM_SingleBead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioSingleBead())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_MultiBeadDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioMultiBeadDeps())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

// --- Alternative flow scenarios ---

func TestLLM_InterruptForBug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioInterruptForBug())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_ResumeAfterCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioResumeAfterCrash())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

// --- Slash command scenarios ---

func TestLLM_SpecInit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioSpecInit())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_SpecApprove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioSpecApprove())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_PlanApprove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioPlanApprove())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_ImplApprove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioImplApprove())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_SpecStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioSpecStatus())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

// --- Error recovery scenarios ---

func TestLLM_MultipleActiveSpecs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioMultipleActiveSpecs())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_StaleWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioStaleWorktree())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_CompleteFromSpecWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioCompleteFromSpecWorktree())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_ApproveSpecFromWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioApproveSpecFromWorktree())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_ApprovePlanFromWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioApprovePlanFromWorktree())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_BugfixBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioBugfixBranch())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_BlockedBeadTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioBlockedBeadTransition())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

func TestLLM_UnmergedBeadGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioUnmergedBeadGuard())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

// --- Assertion helper unit tests ---

func TestAssertMergeTopology(t *testing.T) {
	sandbox := newMinimalSandbox(t)

	t.Run("no merge commits fails", func(t *testing.T) {
		ft := &fakeTB{}
		assertMergeTopology(ft, sandbox, "main")
		if !ft.failed {
			t.Error("expected failure when no merge commits exist")
		}
	})

	t.Run("merge from bead branch passes", func(t *testing.T) {
		// Create a bead branch, make a commit, merge it back.
		mustRun(t, sandbox.Root, "git", "checkout", "-b", "bead/test-bead")
		sandbox.WriteFile("bead-file.txt", "bead work")
		mustRun(t, sandbox.Root, "git", "add", "-A")
		mustRun(t, sandbox.Root, "git", "commit", "-m", "impl(test-bead): bead work")
		mustRun(t, sandbox.Root, "git", "checkout", "main")
		mustRun(t, sandbox.Root, "git", "merge", "--no-ff", "bead/test-bead", "-m", "Merge bead/test-bead into main")

		ft := &fakeTB{}
		assertMergeTopology(ft, sandbox, "main")
		if ft.failed {
			t.Errorf("expected pass after bead merge, got: %v", ft.errors)
		}
	})
}

func TestAssertCommitMessage(t *testing.T) {
	sandbox := newMinimalSandbox(t)

	t.Run("no matching commit fails", func(t *testing.T) {
		ft := &fakeTB{}
		assertCommitMessage(ft, sandbox, `impl\(bead-xyz\):`)
		if !ft.failed {
			t.Error("expected failure when no commit matches")
		}
	})

	t.Run("matching commit passes", func(t *testing.T) {
		sandbox.WriteFile("code.go", "package main")
		mustRun(t, sandbox.Root, "git", "add", "-A")
		mustRun(t, sandbox.Root, "git", "commit", "-m", "impl(bead-abc): add code")

		ft := &fakeTB{}
		assertCommitMessage(ft, sandbox, `impl\(bead-abc\):`)
		if ft.failed {
			t.Errorf("expected pass for matching commit, got: %v", ft.errors)
		}
	})
}

func TestAssertBeadsState(t *testing.T) {
	sandbox := newMinimalSandbox(t)

	// Check that bd is available; skip if not.
	if _, err := sandbox.runBD("version"); err != nil {
		t.Skip("bd not available, skipping beads state test")
	}

	// Initialize beads in the minimal sandbox.
	initBeads(t, sandbox.Root)

	// Create an epic and a child task.
	epicID := sandbox.CreateBead("Test Epic", "epic", "")
	taskID := sandbox.CreateBead("Task 1", "task", epicID)

	t.Run("open beads match", func(t *testing.T) {
		ft := &fakeTB{}
		assertBeadsState(ft, sandbox, epicID, map[string]string{
			taskID: "open",
		})
		if ft.failed {
			t.Errorf("expected open task to match, got: %v", ft.errors)
		}
	})

	t.Run("wrong status fails", func(t *testing.T) {
		ft := &fakeTB{}
		assertBeadsState(ft, sandbox, epicID, map[string]string{
			taskID: "closed",
		})
		if !ft.failed {
			t.Error("expected failure for wrong status")
		}
	})
}

// newMinimalSandbox creates a bare-bones git sandbox (no beads, no shims)
// for testing assertion helpers deterministically.
func newMinimalSandbox(t *testing.T) *Sandbox {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, root, "git", "init")
	mustRun(t, root, "git", "branch", "-M", "main")
	mustRun(t, root, "git", "config", "user.email", "test@mindspec.dev")
	mustRun(t, root, "git", "config", "user.name", "Test")
	// Write a dummy file so initial commit isn't empty.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, root, "git", "add", "-A")
	mustRun(t, root, "git", "commit", "-m", "initial commit")

	return &Sandbox{Root: root, t: t}
}

// fakeTB is a minimal testing.TB substitute that captures failures without
// stopping the test, so we can test assertion helpers that call t.Errorf/t.Fatalf.
type fakeTB struct {
	testing.TB
	failed bool
	errors []string
}

func (f *fakeTB) Helper() {}
func (f *fakeTB) Errorf(format string, args ...interface{}) {
	f.failed = true
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}
func (f *fakeTB) Fatalf(format string, args ...interface{}) {
	f.failed = true
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
	// Can't truly stop execution in a fake, but callers check f.failed.
}
func (f *fakeTB) Logf(format string, args ...interface{}) {}

func TestAssertNoPreApproveImplMainMergeOrPR(t *testing.T) {
	t.Run("rejects merge spec into main before approve impl", func(t *testing.T) {
		events := []ActionEvent{
			{
				ActionType: "command",
				Command:    "git",
				ExitCode:   0,
				ArgsList: []string{
					"-C", "/tmp/repo",
					"checkout", "main",
				},
			},
			{
				ActionType: "command",
				Command:    "git",
				ExitCode:   0,
				ArgsList: []string{
					"merge", "--no-ff", "spec/001-test", "main",
				},
			},
			{
				ActionType: "command",
				Command:    "mindspec",
				ExitCode:   0,
				ArgsList:   []string{"approve", "impl", "001-test"},
			},
		}

		err := preApproveImplMainMergeOrPRViolation(events)
		if err == nil {
			t.Fatal("expected violation on merge-to-main (impl approve no longer merges)")
		}
		if !strings.Contains(err.Error(), "merge-to-main occurred before approve impl") {
			t.Fatalf("expected merge violation error, got: %v", err)
		}
	})

	t.Run("fails on pre-approve pr create", func(t *testing.T) {
		events := []ActionEvent{
			{
				ActionType: "command",
				Command:    "gh",
				ExitCode:   0,
				ArgsList:   []string{"pr", "create", "--base", "main", "--head", "spec/001-test"},
			},
			{
				ActionType: "command",
				Command:    "mindspec",
				ExitCode:   0,
				ArgsList:   []string{"approve", "impl", "001-test"},
			},
		}

		err := preApproveImplMainMergeOrPRViolation(events)
		if err == nil {
			t.Fatal("expected violation when PR is created before approve impl")
		}
		if !strings.Contains(err.Error(), "PR command ran before approve impl") {
			t.Fatalf("expected PR violation error, got: %v", err)
		}
	})

	t.Run("fails on non-canonical pre-approve merge-to-main", func(t *testing.T) {
		events := []ActionEvent{
			{
				ActionType: "command",
				Command:    "git",
				ExitCode:   0,
				ArgsList:   []string{"merge", "main"},
			},
			{
				ActionType: "command",
				Command:    "mindspec",
				ExitCode:   0,
				ArgsList:   []string{"approve", "impl", "001-test"},
			},
		}

		err := preApproveImplMainMergeOrPRViolation(events)
		if err == nil {
			t.Fatal("expected violation on merge-to-main before approve impl")
		}
		if !strings.Contains(err.Error(), "merge-to-main occurred before approve impl") {
			t.Fatalf("expected merge violation error, got: %v", err)
		}
	})
}
