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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

func TestLLM_AbandonSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, ScenarioAbandonSpec())
	if len(report.WrongActions) > 0 {
		t.Errorf("unexpected wrong actions: %d", len(report.WrongActions))
	}
}

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
