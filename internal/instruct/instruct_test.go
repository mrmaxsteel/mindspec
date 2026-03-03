package instruct

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

// mockPrimeUnavailable sets execPrime to simulate bd not being installed.
func mockPrimeUnavailable(t *testing.T) {
	t.Helper()
	orig := execPrime
	execPrime = func() *exec.Cmd {
		return exec.Command("false")
	}
	t.Cleanup(func() { execPrime = orig })
}

// mockPrimeAvailable sets execPrime to return the given content.
func mockPrimeAvailable(t *testing.T, content string) {
	t.Helper()
	orig := execPrime
	execPrime = func() *exec.Cmd {
		return exec.Command("echo", content)
	}
	t.Cleanup(func() { execPrime = orig })
}

func setupTestProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	// Create spec directories
	for _, spec := range []string{"001-skeleton", "002-glossary", "004-instruct"} {
		specDir := filepath.Join(tmp, "docs", "specs", spec)
		os.MkdirAll(specDir, 0755)
	}

	// Create spec file with goal
	os.WriteFile(
		filepath.Join(tmp, "docs", "specs", "004-instruct", "spec.md"),
		[]byte("# Spec 004\n\n## Goal\n\nReplace static files with dynamic instruct command.\n\n## Approval\n\n- **Status**: APPROVED\n"),
		0644,
	)

	// Create plan file
	os.WriteFile(
		filepath.Join(tmp, "docs", "specs", "004-instruct", "plan.md"),
		[]byte("---\nstatus: Approved\napproved_at: 2026-02-12\n---\n# Plan\n"),
		0644,
	)

	return tmp
}

func TestRender_IdleMode(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeIdle}
	ctx := BuildContext(root, s)

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(output, "No Active Work") {
		t.Error("expected idle heading")
	}
	if !strings.Contains(output, "001-skeleton") {
		t.Error("expected available specs to be listed")
	}
	if !strings.Contains(output, "mindspec spec create") {
		t.Error("expected mindspec spec create suggestion")
	}
}

func TestRender_SpecMode(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeSpec, ActiveSpec: "004-instruct"}
	ctx := BuildContext(root, s)

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(output, "Spec Mode") {
		t.Error("expected spec mode heading")
	}
	if !strings.Contains(output, "004-instruct") {
		t.Error("expected active spec in output")
	}
	if !strings.Contains(output, "Permitted Actions") {
		t.Error("expected permitted actions section")
	}
	if !strings.Contains(output, "Forbidden Actions") {
		t.Error("expected forbidden actions section")
	}
	if !strings.Contains(output, "mindspec spec approve") {
		t.Error("expected mindspec spec approve gate")
	}
}

func TestRender_PlanMode(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModePlan, ActiveSpec: "004-instruct"}
	ctx := BuildContext(root, s)

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(output, "Plan Mode") {
		t.Error("expected plan mode heading")
	}
	if !strings.Contains(output, "004-instruct") {
		t.Error("expected active spec")
	}
	if !strings.Contains(output, "Required Review") {
		t.Error("expected required review section")
	}
	if !strings.Contains(output, "mindspec plan approve") {
		t.Error("expected mindspec plan approve gate")
	}

	// Plan is approved in test fixture → should show post-approval guidance
	if !ctx.PlanApproved {
		t.Error("expected PlanApproved=true for approved plan")
	}
	if !strings.Contains(output, "mindspec next") {
		t.Error("expected 'mindspec next' guidance for approved plan")
	}
}

func TestRender_ImplementMode(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: "beads-001"}
	ctx := BuildContext(root, s)

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(output, "Implementation Mode") {
		t.Error("expected implementation mode heading")
	}
	if !strings.Contains(output, "004-instruct") {
		t.Error("expected active spec")
	}
	if !strings.Contains(output, "beads-001") {
		t.Error("expected active bead")
	}
	if !strings.Contains(output, "Scope discipline") {
		t.Error("expected scope discipline obligation")
	}
	if !strings.Contains(output, "mindspec complete") {
		t.Error("expected mindspec complete command")
	}
}

func TestRender_SpecGoalExtracted(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModePlan, ActiveSpec: "004-instruct"}
	ctx := BuildContext(root, s)

	if !strings.Contains(ctx.SpecGoal, "Replace static files") {
		t.Errorf("expected spec goal to be extracted, got %q", ctx.SpecGoal)
	}
}

func TestRender_Warnings(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	// Spec mode but spec is already approved → should produce drift warning
	s := &state.Focus{Mode: state.ModeSpec, ActiveSpec: "004-instruct"}
	ctx := BuildContext(root, s)

	if len(ctx.Warnings) == 0 {
		t.Error("expected drift warning for spec mode with approved spec")
	}

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(output, "## Warnings") {
		t.Error("expected warnings section in output")
	}
}

func TestRenderJSON_Structure(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModePlan, ActiveSpec: "004-instruct"}
	ctx := BuildContext(root, s)

	output, err := RenderJSON(ctx)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("JSON parse failed: %v", err)
	}

	if parsed.Mode != "plan" {
		t.Errorf("mode: got %q, want %q", parsed.Mode, "plan")
	}
	if parsed.ActiveSpec != "004-instruct" {
		t.Errorf("active_spec: got %q, want %q", parsed.ActiveSpec, "004-instruct")
	}
	if parsed.Guidance == "" {
		t.Error("guidance should not be empty")
	}
	if len(parsed.Gates) == 0 {
		t.Error("gates should not be empty for plan mode")
	}
	if parsed.Warnings == nil {
		t.Error("warnings should be an array, not nil")
	}
}

func TestRenderJSON_AllModes(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)

	modes := []string{state.ModeIdle, state.ModeSpec, state.ModePlan, state.ModeImplement}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			s := &state.Focus{Mode: mode, ActiveSpec: "004-instruct", ActiveBead: "beads-001"}
			ctx := BuildContext(root, s)

			output, err := RenderJSON(ctx)
			if err != nil {
				t.Fatalf("RenderJSON failed for mode %s: %v", mode, err)
			}

			var parsed JSONOutput
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Fatalf("JSON parse failed for mode %s: %v", mode, err)
			}

			if parsed.Mode != mode {
				t.Errorf("mode: got %q, want %q", parsed.Mode, mode)
			}
		})
	}
}

func TestGatesForMode(t *testing.T) {
	tests := []struct {
		mode      string
		wantCount int
	}{
		{state.ModeIdle, 0},
		{state.ModeSpec, 1},
		{state.ModePlan, 2},
		{state.ModeImplement, 2},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			gates := gatesForMode(tt.mode)
			if len(gates) != tt.wantCount {
				t.Errorf("gatesForMode(%q): got %d gates, want %d", tt.mode, len(gates), tt.wantCount)
			}
		})
	}
}

func TestListSpecs(t *testing.T) {
	root := setupTestProject(t)
	specs := listSpecs(root)

	if len(specs) != 3 {
		t.Errorf("expected 3 specs, got %d: %v", len(specs), specs)
	}
}

func TestReadSpecGoal_Missing(t *testing.T) {
	tmp := t.TempDir()
	goal := readSpecGoal(tmp, "nonexistent")
	if goal != "" {
		t.Errorf("expected empty goal for missing spec, got %q", goal)
	}
}

func TestCapturePrime_Available(t *testing.T) {
	mockPrimeAvailable(t, "# Beads Workflow Context\nSome content here")
	result := CapturePrime()
	if !strings.Contains(result, "Beads Workflow Context") {
		t.Errorf("expected beads content, got %q", result)
	}
}

func TestCapturePrime_Unavailable(t *testing.T) {
	mockPrimeUnavailable(t)
	result := CapturePrime()
	if result != "" {
		t.Errorf("expected empty string on failure, got %q", result)
	}
}

func TestBuildContext_PopulatesBeadsContext(t *testing.T) {
	mockPrimeAvailable(t, "# Beads Workflow Context")
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeIdle}
	ctx := BuildContext(root, s)

	if !strings.Contains(ctx.BeadsContext, "Beads Workflow Context") {
		t.Errorf("expected BeadsContext to be populated, got %q", ctx.BeadsContext)
	}

	// Should NOT have the beads unavailable warning
	for _, w := range ctx.Warnings {
		if strings.Contains(w, "bd prime unavailable") {
			t.Error("should not have beads unavailable warning when prime succeeds")
		}
	}
}

func TestBuildContext_WarnsWhenPrimeUnavailable(t *testing.T) {
	mockPrimeUnavailable(t)
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeIdle}
	ctx := BuildContext(root, s)

	if ctx.BeadsContext != "" {
		t.Errorf("expected empty BeadsContext, got %q", ctx.BeadsContext)
	}

	found := false
	for _, w := range ctx.Warnings {
		if strings.Contains(w, "bd prime unavailable") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected beads unavailable warning")
	}
}

func TestRender_IncludesBeadsContext(t *testing.T) {
	ctx := &Context{
		Mode:         state.ModeIdle,
		BeadsContext: "# Beads Workflow\nSession close: bd sync --flush-only",
	}

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(output, "# Beads Workflow") {
		t.Error("expected Beads context in rendered output")
	}
	if !strings.Contains(output, "bd sync --flush-only") {
		t.Error("expected session close protocol in rendered output")
	}
}

func TestRender_BeadsContextBeforeWarnings(t *testing.T) {
	ctx := &Context{
		Mode:         state.ModeIdle,
		BeadsContext: "BEADS_MARKER",
		Warnings:     []string{"WARNING_MARKER"},
	}

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	beadsIdx := strings.Index(output, "BEADS_MARKER")
	warningIdx := strings.Index(output, "WARNING_MARKER")

	if beadsIdx == -1 {
		t.Fatal("BEADS_MARKER not found in output")
	}
	if warningIdx == -1 {
		t.Fatal("WARNING_MARKER not found in output")
	}
	if beadsIdx >= warningIdx {
		t.Error("Beads context should appear before warnings")
	}
}

func TestRender_NoBeadsContext(t *testing.T) {
	ctx := &Context{
		Mode:         state.ModeIdle,
		BeadsContext: "",
	}

	output, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Should not have a dangling separator for empty beads context
	if strings.Contains(output, "\n---\n\n\n") {
		t.Error("should not have empty beads section separator")
	}
}

func TestRenderJSON_IncludesBeadsContext(t *testing.T) {
	ctx := &Context{
		Mode:         state.ModePlan,
		ActiveSpec:   "test-spec",
		BeadsContext: "# Beads Workflow Context\nContent here",
	}

	output, err := RenderJSON(ctx)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("JSON parse failed: %v", err)
	}

	if parsed.BeadsContext != "# Beads Workflow Context\nContent here" {
		t.Errorf("beads_context: got %q, want beads content", parsed.BeadsContext)
	}
}
