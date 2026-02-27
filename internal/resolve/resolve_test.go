package resolve

import (
	"path/filepath"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

// stepMapping returns a standard test step mapping.
func testStepMapping() map[string]string {
	return map[string]string{
		"spec":         "step-spec",
		"spec-approve": "step-spec-approve",
		"plan":         "step-plan",
		"plan-approve": "step-plan-approve",
		"implement":    "step-implement",
		"review":       "step-review",
	}
}

func TestDeriveMode_SpecPhase(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "in_progress",
		"step-spec-approve": "open",
		"step-plan":         "open",
		"step-plan-approve": "open",
		"step-implement":    "open",
		"step-review":       "open",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModeSpec {
		t.Errorf("deriveMode() = %q, want %q", got, state.ModeSpec)
	}
}

func TestDeriveMode_SpecApprovePhase(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "open",
		"step-plan":         "open",
		"step-plan-approve": "open",
		"step-implement":    "open",
		"step-review":       "open",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModeSpec {
		t.Errorf("deriveMode() = %q, want %q (spec-approve → spec mode)", got, state.ModeSpec)
	}
}

func TestDeriveMode_PlanPhase(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "closed",
		"step-plan":         "in_progress",
		"step-plan-approve": "open",
		"step-implement":    "open",
		"step-review":       "open",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModePlan {
		t.Errorf("deriveMode() = %q, want %q", got, state.ModePlan)
	}
}

func TestDeriveMode_PlanApprovePhase(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "closed",
		"step-plan":         "closed",
		"step-plan-approve": "open",
		"step-implement":    "open",
		"step-review":       "open",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModePlan {
		t.Errorf("deriveMode() = %q, want %q (plan-approve → plan mode)", got, state.ModePlan)
	}
}

func TestDeriveMode_ImplementPhase(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "closed",
		"step-plan":         "closed",
		"step-plan-approve": "closed",
		"step-implement":    "in_progress",
		"step-review":       "open",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModeImplement {
		t.Errorf("deriveMode() = %q, want %q", got, state.ModeImplement)
	}
}

func TestDeriveMode_ReviewPhase(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "closed",
		"step-plan":         "closed",
		"step-plan-approve": "closed",
		"step-implement":    "closed",
		"step-review":       "open",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModeReview {
		t.Errorf("deriveMode() = %q, want %q", got, state.ModeReview)
	}
}

func TestDeriveMode_AllClosed(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "closed",
		"step-plan":         "closed",
		"step-plan-approve": "closed",
		"step-implement":    "closed",
		"step-review":       "closed",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModeIdle {
		t.Errorf("deriveMode() = %q, want %q (all closed → idle)", got, state.ModeIdle)
	}
}

func TestDeriveMode_EmptyMapping(t *testing.T) {
	got := deriveMode(nil, map[string]string{"some-id": "open"})
	if got != state.ModeIdle {
		t.Errorf("deriveMode() with nil mapping = %q, want %q", got, state.ModeIdle)
	}
}

func TestDeriveMode_PartialMapping(t *testing.T) {
	// Only spec and plan steps mapped, implement/review missing
	mapping := map[string]string{
		"spec": "step-spec",
		"plan": "step-plan",
	}
	statuses := map[string]string{
		"step-spec": "closed",
		"step-plan": "in_progress",
	}
	got := deriveMode(mapping, statuses)
	if got != state.ModePlan {
		t.Errorf("deriveMode() = %q, want %q", got, state.ModePlan)
	}
}

func TestIsActive_NoStepMapping(t *testing.T) {
	// No step mapping, some non-closed statuses → active
	statuses := map[string]string{
		"root":   "in_progress",
		"step-1": "open",
	}
	got := isActive(nil, statuses)
	if !got {
		t.Error("isActive() = false, want true (non-closed steps exist)")
	}
}

func TestIsActive_AllClosed_NoMapping(t *testing.T) {
	statuses := map[string]string{
		"root":   "closed",
		"step-1": "closed",
	}
	got := isActive(nil, statuses)
	if got {
		t.Error("isActive() = true, want false (all closed)")
	}
}

func TestIsActive_ReviewClosed(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "closed",
		"step-plan":         "closed",
		"step-plan-approve": "closed",
		"step-implement":    "closed",
		"step-review":       "closed",
	}
	got := isActive(mapping, statuses)
	if got {
		t.Error("isActive() = true, want false (review closed → lifecycle complete)")
	}
}

func TestIsActive_ImplementInProgress(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "closed",
		"step-spec-approve": "closed",
		"step-plan":         "closed",
		"step-plan-approve": "closed",
		"step-implement":    "in_progress",
		"step-review":       "open",
	}
	got := isActive(mapping, statuses)
	if !got {
		t.Error("isActive() = false, want true (implement in_progress)")
	}
}

func TestIsActive_SpecOpen(t *testing.T) {
	mapping := testStepMapping()
	statuses := map[string]string{
		"step-spec":         "open",
		"step-spec-approve": "open",
		"step-plan":         "open",
		"step-plan-approve": "open",
		"step-implement":    "open",
		"step-review":       "open",
	}
	got := isActive(mapping, statuses)
	if !got {
		t.Error("isActive() = false, want true (all open)")
	}
}

func TestFormatActiveList_Empty(t *testing.T) {
	got := FormatActiveList(nil)
	if got != "No active specs found.\n" {
		t.Errorf("FormatActiveList(nil) = %q", got)
	}
}

func TestResolveSpecBranch(t *testing.T) {
	got := ResolveSpecBranch("053-drop-state-json")
	if got != "spec/053-drop-state-json" {
		t.Errorf("ResolveSpecBranch() = %q, want %q", got, "spec/053-drop-state-json")
	}
}

func TestResolveWorktree(t *testing.T) {
	got := ResolveWorktree("/project", "053-drop-state-json")
	want := filepath.Join("/project", ".worktrees", "worktree-spec-053-drop-state-json")
	if got != want {
		t.Errorf("ResolveWorktree() = %q, want %q", got, want)
	}
}

func TestFormatActiveList_Multiple(t *testing.T) {
	specs := []SpecStatus{
		{SpecID: "001-alpha", Mode: "spec", MoleculeID: "mol-1"},
		{SpecID: "002-beta", Mode: "plan", MoleculeID: "mol-2"},
	}
	got := FormatActiveList(specs)
	if got == "No active specs found.\n" {
		t.Error("expected non-empty list")
	}
}
