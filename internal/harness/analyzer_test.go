package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzer_Classify_Forward(t *testing.T) {
	events := []ActionEvent{
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Read", Args: map[string]string{"file_path": "main.go"}},
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{Turn: 1, ActionType: "command", Command: "go", Args: map[string]string{"0": "test", "1": "./..."}},
	}

	a := NewAnalyzer()
	summaries := a.Classify(events)

	if len(summaries) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(summaries))
	}
	if summaries[0].Class != ClassForward {
		t.Errorf("turn 0: class = %q, want forward", summaries[0].Class)
	}
	if summaries[1].Class != ClassForward {
		t.Errorf("turn 1: class = %q, want forward", summaries[1].Class)
	}
}

func TestAnalyzer_Classify_Recovery(t *testing.T) {
	events := []ActionEvent{
		{Turn: 0, ActionType: "hook_block", Blocked: true, BlockReason: "code edits not allowed in spec mode"},
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Read"},
	}

	a := NewAnalyzer()
	summaries := a.Classify(events)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(summaries))
	}
	if summaries[0].Class != ClassRecovery {
		t.Errorf("turn 0: class = %q, want recovery", summaries[0].Class)
	}
}

func TestAnalyzer_Classify_Correction(t *testing.T) {
	events := []ActionEvent{
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "main.go"}},
		{Turn: 1, ActionType: "tool_invoke", ToolName: "Edit", Args: map[string]string{"file_path": "main.go"}},
	}

	a := NewAnalyzer()
	summaries := a.Classify(events)

	if len(summaries) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(summaries))
	}
	if summaries[0].Class != ClassForward {
		t.Errorf("turn 0: class = %q, want forward", summaries[0].Class)
	}
	if summaries[1].Class != ClassCorrection {
		t.Errorf("turn 1: class = %q, want correction", summaries[1].Class)
	}
}

func TestAnalyzer_Classify_Overhead(t *testing.T) {
	events := []ActionEvent{
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Read"},
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Glob"},
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Grep"},
	}

	a := NewAnalyzer()
	summaries := a.Classify(events)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(summaries))
	}
	if summaries[0].Class != ClassOverhead {
		t.Errorf("turn 0: class = %q, want overhead", summaries[0].Class)
	}
}

func TestAnalyzer_Classify_WrongAction(t *testing.T) {
	events := []ActionEvent{
		{
			Turn:       0,
			Phase:      "spec",
			ActionType: "tool_invoke",
			ToolName:   "Write",
			Args:       map[string]string{"file_path": "internal/service.go"},
		},
	}

	a := NewAnalyzer()
	summaries := a.Classify(events)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(summaries))
	}
	if summaries[0].Class != ClassWrongAction {
		t.Errorf("turn 0: class = %q, want wrong_action", summaries[0].Class)
	}
}

func TestAnalyzer_WrongAction_CodeInSpecMode(t *testing.T) {
	events := []ActionEvent{
		{Phase: "spec", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{Phase: "spec", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": ".mindspec/docs/spec.md"}},
	}

	a := NewAnalyzer()
	results := a.DetectWrongActions(events)

	// Expect code_in_spec_mode + skip_next (code edit before any mindspec next).
	hasRule := false
	for _, r := range results {
		if r.Rule == "code_in_spec_mode" {
			hasRule = true
		}
	}
	if !hasRule {
		t.Errorf("expected code_in_spec_mode rule to fire, got: %v", results)
	}
}

func TestAnalyzer_WrongAction_CodeInPlanMode(t *testing.T) {
	events := []ActionEvent{
		{Phase: "plan", ActionType: "tool_invoke", ToolName: "Edit", Args: map[string]string{"file_path": "cmd/main.go"}},
		{Phase: "plan", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "docs/plan.md"}},
	}

	a := NewAnalyzer()
	results := a.DetectWrongActions(events)

	// Expect code_in_plan_mode + skip_next (code edit before any mindspec next).
	hasRule := false
	for _, r := range results {
		if r.Rule == "code_in_plan_mode" {
			hasRule = true
		}
	}
	if !hasRule {
		t.Errorf("expected code_in_plan_mode rule to fire, got: %v", results)
	}
}

func TestAnalyzer_WrongAction_ForceBypass(t *testing.T) {
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec", Args: map[string]string{"0": "next", "1": "--force"}},
	}

	a := NewAnalyzer()
	results := a.DetectWrongActions(events)

	if len(results) != 1 {
		t.Fatalf("expected 1 wrong action, got %d", len(results))
	}
	if results[0].Rule != "force_bypass" {
		t.Errorf("rule = %q, want force_bypass", results[0].Rule)
	}
}

func TestAnalyzer_WrongAction_BlockedEventsSkipped(t *testing.T) {
	events := []ActionEvent{
		{Phase: "spec", ActionType: "tool_invoke", ToolName: "Write",
			Args:    map[string]string{"file_path": "internal/foo.go"},
			Blocked: true, BlockReason: "code edits blocked"},
	}

	a := NewAnalyzer()
	results := a.DetectWrongActions(events)

	if len(results) != 0 {
		t.Errorf("expected 0 wrong actions for blocked events, got %d", len(results))
	}
}

func TestAnalyzer_WrongAction_NoFalsePositives(t *testing.T) {
	events := []ActionEvent{
		{Phase: "implement", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{Phase: "implement", ActionType: "command", Command: "go", Args: map[string]string{"0": "test"}},
		{Phase: "spec", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": ".mindspec/docs/spec.md"}},
	}

	a := NewAnalyzer()
	results := a.DetectWrongActions(events)

	if len(results) != 0 {
		t.Errorf("expected 0 wrong actions, got %d: %+v", len(results), results)
	}
}

func TestSkipNext_NoViolation(t *testing.T) {
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"next"}},
		{Phase: "implement", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"complete", "done"}},
	}

	results := detectSkipNext(events)
	if len(results) != 0 {
		t.Errorf("expected no violation when next runs before code, got %d: %v", len(results), results)
	}
}

func TestSkipNext_Violation(t *testing.T) {
	events := []ActionEvent{
		{Phase: "plan", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"next"}},
	}

	results := detectSkipNext(events)
	if len(results) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(results))
	}
	if results[0].Rule != "skip_next" {
		t.Errorf("rule = %q, want skip_next", results[0].Rule)
	}
}

func TestSkipNext_ImplementPhaseNoViolation(t *testing.T) {
	// Agent starts in implement mode with pre-claimed bead — no next needed.
	events := []ActionEvent{
		{Phase: "implement", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "greeting.go"}},
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"complete", "done"}},
	}

	results := detectSkipNext(events)
	if len(results) != 0 {
		t.Errorf("expected no violation in pre-claimed implement mode, got %d: %v", len(results), results)
	}
}

func TestSkipNext_NonImplementSessionNoViolation(t *testing.T) {
	// A session with only spec/plan phase events and git commits (no implement
	// phase, no mindspec next) should NOT trigger skip_next. These commits are
	// lifecycle artifacts (e.g. spec-init auto-commit, plan approval commit).
	events := []ActionEvent{
		{Turn: 1, Phase: "spec", ActionType: "command", Command: "mindspec",
			ArgsList: []string{"spec-init", "010-test", "--title", "Test"}},
		{Turn: 1, Phase: "spec", ActionType: "command", Command: "git",
			ArgsList: []string{"commit", "-m", "chore: initialize spec"}},
		{Turn: 2, Phase: "spec", ActionType: "tool_invoke", ToolName: "Write",
			Args: map[string]string{"file_path": ".mindspec/docs/specs/010-test/spec.md"}},
		{Turn: 3, Phase: "plan", ActionType: "command", Command: "mindspec",
			ArgsList: []string{"approve", "spec", "010-test"}},
		{Turn: 3, Phase: "plan", ActionType: "command", Command: "git",
			ArgsList: []string{"commit", "-m", "chore: approve spec"}},
	}

	results := detectSkipNext(events)
	if len(results) != 0 {
		t.Errorf("expected no violation in non-implement session, got %d: %v", len(results), results)
	}
}

func TestSkipNext_NonImplementWithNextStillChecks(t *testing.T) {
	// If `mindspec next` appears in the event stream, skip_next should still
	// be checked even without implement phase events — next implies the agent
	// intended to enter implement mode.
	events := []ActionEvent{
		{Turn: 1, Phase: "plan", ActionType: "tool_invoke", ToolName: "Write",
			Args: map[string]string{"file_path": "internal/foo.go"}},
		{Turn: 2, ActionType: "command", Command: "mindspec",
			ArgsList: []string{"next"}},
	}

	results := detectSkipNext(events)
	if len(results) != 1 {
		t.Fatalf("expected 1 violation when next is present, got %d", len(results))
	}
	if results[0].Rule != "skip_next" {
		t.Errorf("rule = %q, want skip_next", results[0].Rule)
	}
}

func TestSkipNext_DocEditsIgnored(t *testing.T) {
	// Editing docs/markdown before next should not trigger.
	events := []ActionEvent{
		{Phase: "spec", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": ".mindspec/docs/spec.md"}},
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"next"}},
	}

	results := detectSkipNext(events)
	if len(results) != 0 {
		t.Errorf("expected no violation for doc edits, got %d", len(results))
	}
}

func TestSkipNext_ApproveFlowExempt(t *testing.T) {
	// Code edits before an approve command are part of the approval flow.
	events := []ActionEvent{
		{Phase: "spec", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"spec", "approve", "010-test"}},
	}

	results := detectSkipNext(events)
	if len(results) != 0 {
		t.Errorf("expected no violation for code edits before approve, got %d: %v", len(results), results)
	}
}

func TestSkipNext_ApproveFlowMixedViolation(t *testing.T) {
	// Code edits AFTER approve but before next should still trigger.
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"spec", "approve", "010-test"}},
		{Phase: "plan", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/bar.go"}},
	}

	results := detectSkipNext(events)
	if len(results) != 1 {
		t.Fatalf("expected 1 violation for code after approve without next, got %d", len(results))
	}
	if results[0].Rule != "skip_next" {
		t.Errorf("rule = %q, want skip_next", results[0].Rule)
	}
}

func TestSkipNext_LifecycleTurnCommitExempt(t *testing.T) {
	// Git commits in the same turn as a lifecycle command are side-effects.
	tests := []struct {
		name   string
		lcVerb string
		lcArgs []string
	}{
		{"spec-init", "spec-init", []string{"spec-init", "001-calc", "--title", "Calculator"}},
		{"approve-spec", "approve", []string{"approve", "spec", "001-greeting"}},
		{"approve-plan", "approve", []string{"approve", "plan", "001-greeting"}},
		{"approve-impl", "approve", []string{"approve", "impl", "001-done"}},
		{"complete", "complete", []string{"complete", "done"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Lifecycle command and git commit in same turn (turn=1).
			events := []ActionEvent{
				{Turn: 1, Phase: "plan", ActionType: "command", Command: "mindspec",
					ArgsList: tt.lcArgs},
				{Turn: 1, Phase: "plan", ActionType: "command", Command: "git",
					ArgsList: []string{"commit", "-m", "any message whatsoever"}},
			}
			results := detectSkipNext(events)
			if len(results) != 0 {
				t.Errorf("expected no violation for commit in lifecycle turn, got %d: %v",
					len(results), results)
			}
		})
	}
}

func TestSkipNext_CommitOutsideLifecycleTurnViolation(t *testing.T) {
	// A git commit in a different turn from any lifecycle command is still flagged.
	events := []ActionEvent{
		{Turn: 1, Phase: "plan", ActionType: "command", Command: "mindspec",
			ArgsList: []string{"approve", "plan", "001-test"}},
		{Turn: 2, Phase: "plan", ActionType: "command", Command: "git",
			ArgsList: []string{"commit", "-m", "Bead 1: Create types.go"}},
	}
	results := detectSkipNext(events)
	if len(results) != 1 {
		t.Fatalf("expected 1 violation for commit outside lifecycle turn, got %d", len(results))
	}
	if results[0].Rule != "skip_next" {
		t.Errorf("rule = %q, want skip_next", results[0].Rule)
	}
}

func TestSkipNext_ImplementPhaseCommitNoViolation(t *testing.T) {
	// Agent in implement phase (recorded by shim via mindspec state show).
	// Git commits in implement phase are legitimate — no skip_next violation.
	events := []ActionEvent{
		{ActionType: "command", Command: "git", Phase: "implement",
			ArgsList: []string{"commit", "-m", "impl: add greeting feature"}},
		{ActionType: "command", Command: "mindspec",
			ArgsList: []string{"complete", "done"}},
	}

	results := detectSkipNext(events)
	if len(results) != 0 {
		t.Errorf("expected no violation for commit in implement phase, got %d: %v", len(results), results)
	}
}

func TestSkipNext_FailedCommitIgnored(t *testing.T) {
	// A git commit that failed (exit=1) should not be flagged as code modification.
	// This happens when the agent tries to commit after approve impl (nothing to commit).
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec",
			ArgsList: []string{"approve", "impl", "001-greeting"}, ExitCode: 0},
		{ActionType: "command", Command: "git", ExitCode: 1,
			ArgsList: []string{"commit", "-m", "Approve implementation"}},
	}

	results := detectSkipNext(events)
	if len(results) != 0 {
		t.Errorf("expected no violation for failed git commit, got %d: %v", len(results), results)
	}
}

func TestSkipComplete_NoViolation(t *testing.T) {
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"next"}},
		{Phase: "implement", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"complete", "done"}},
	}

	results := detectSkipComplete(events)
	if len(results) != 0 {
		t.Errorf("expected no violation when complete runs after code, got %d: %v", len(results), results)
	}
}

func TestSkipComplete_ViolationSessionEnd(t *testing.T) {
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"next"}},
		{Phase: "implement", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		// Session ends without complete
	}

	results := detectSkipComplete(events)
	if len(results) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(results))
	}
	if results[0].Rule != "skip_complete" {
		t.Errorf("rule = %q, want skip_complete", results[0].Rule)
	}
}

func TestSkipComplete_ViolationBeforeApprove(t *testing.T) {
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"next"}},
		{Phase: "implement", ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/foo.go"}},
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"approve", "impl", "001-test"}},
	}

	results := detectSkipComplete(events)
	if len(results) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(results))
	}
	if results[0].Rule != "skip_complete" {
		t.Errorf("rule = %q, want skip_complete", results[0].Rule)
	}
}

func TestSkipComplete_NoCodeNoViolation(t *testing.T) {
	// next without code edits — no violation
	events := []ActionEvent{
		{ActionType: "command", Command: "mindspec", ArgsList: []string{"next"}},
		{Phase: "implement", ActionType: "tool_invoke", ToolName: "Read", Args: map[string]string{"file_path": "internal/foo.go"}},
	}

	results := detectSkipComplete(events)
	if len(results) != 0 {
		t.Errorf("expected no violation without code edits, got %d", len(results))
	}
}

func TestAnalyzer_PlanFidelity_HighScore(t *testing.T) {
	planContent := `---
spec_id: 001-test
status: Approved
version: 1
---

# Plan

## Bead 1: Core

**Steps**

1. Create internal/harness/analyzer.go
2. Run go test ./internal/harness/

**Verification**

- [ ] go test passes
`
	planPath := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatal(err)
	}

	events := []ActionEvent{
		{ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "internal/harness/analyzer.go"}},
		{ActionType: "command", Command: "go", Args: map[string]string{"0": "test"}},
	}

	score, err := PlanFidelity(planPath, events)
	if err != nil {
		t.Fatalf("PlanFidelity: %v", err)
	}
	if score < 0.5 {
		t.Errorf("expected high fidelity score, got %.2f", score)
	}
}

func TestAnalyzer_PlanFidelity_LowScore(t *testing.T) {
	planContent := `---
spec_id: 001-test
status: Approved
version: 1
---

# Plan

## Bead 1: Core

**Steps**

1. Create internal/harness/analyzer.go
2. Create internal/harness/report.go
3. Run go test ./internal/harness/

**Verification**

- [ ] go test passes
`
	planPath := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Agent did completely different work
	events := []ActionEvent{
		{ActionType: "tool_invoke", ToolName: "Write", Args: map[string]string{"file_path": "cmd/unrelated.go"}},
		{ActionType: "command", Command: "ls"},
	}

	score, err := PlanFidelity(planPath, events)
	if err != nil {
		t.Fatalf("PlanFidelity: %v", err)
	}
	if score > 0.5 {
		t.Errorf("expected low fidelity score, got %.2f", score)
	}
}

func TestReport_FormatText(t *testing.T) {
	summaries := []TurnSummary{
		{Turn: 0, Class: ClassForward},
		{Turn: 1, Class: ClassForward},
		{Turn: 2, Class: ClassCorrection, Reason: "re-editing main.go"},
		{Turn: 3, Class: ClassOverhead, Reason: "read-only"},
	}
	wrongActions := []WrongActionResult{
		{Rule: "code_in_spec_mode", Reason: "code edit in spec mode"},
	}

	report := NewReport("test-session", "claude-code", summaries, wrongActions, 0.85)
	text := report.FormatText()

	checks := []string{"test-session", "claude-code", "4", "forward", "correction", "overhead", "Wrong actions", "code_in_spec_mode"}
	for _, check := range checks {
		if !contains(text, check) {
			t.Errorf("FormatText missing %q", check)
		}
	}
}

func TestReport_FormatJSON(t *testing.T) {
	summaries := []TurnSummary{
		{Turn: 0, Class: ClassForward},
		{Turn: 1, Class: ClassWrongAction, Reason: "test"},
	}

	report := NewReport("json-test", "claude-code", summaries, nil, 0.9)
	jsonStr, err := report.FormatJSON()
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}

	checks := []string{"session_name", "json-test", "total_turns", "forward_turn_ratio", "plan_fidelity_score"}
	for _, check := range checks {
		if !contains(jsonStr, check) {
			t.Errorf("FormatJSON missing %q", check)
		}
	}
}

func TestReport_TurnsByClass(t *testing.T) {
	summaries := []TurnSummary{
		{Turn: 0, Class: ClassForward},
		{Turn: 1, Class: ClassForward},
		{Turn: 2, Class: ClassCorrection},
		{Turn: 3, Class: ClassRecovery},
		{Turn: 4, Class: ClassForward},
	}

	report := NewReport("count-test", "test", summaries, nil, 1.0)

	if report.TurnsByClass[ClassForward] != 3 {
		t.Errorf("forward turns = %d, want 3", report.TurnsByClass[ClassForward])
	}
	if report.TurnsByClass[ClassCorrection] != 1 {
		t.Errorf("correction turns = %d, want 1", report.TurnsByClass[ClassCorrection])
	}
	if report.TurnsByClass[ClassRecovery] != 1 {
		t.Errorf("recovery turns = %d, want 1", report.TurnsByClass[ClassRecovery])
	}
	if report.ForwardTurnRatio != 0.6 {
		t.Errorf("forward ratio = %.2f, want 0.60", report.ForwardTurnRatio)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
