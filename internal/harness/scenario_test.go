package harness

import (
	"context"
	"fmt"
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

func TestAssertNoPreApproveImplMainMergeOrPR(t *testing.T) {
	t.Run("allows canonical internal merge before approve event", func(t *testing.T) {
		events := []ActionEvent{
			{
				ActionType: "command",
				Command:    "git",
				ExitCode:   0,
				ArgsList: []string{
					"-C", "/tmp/repo",
					"merge", "--no-ff", "spec/001-test",
					"-m", "Merge spec/001-test into main",
				},
			},
			{
				ActionType: "command",
				Command:    "mindspec",
				ExitCode:   0,
				ArgsList:   []string{"approve", "impl", "001-test"},
			},
		}

		if err := preApproveImplMainMergeOrPRViolation(events); err != nil {
			t.Fatalf("expected no violation, got: %v", err)
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
