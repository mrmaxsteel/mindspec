package bead

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// --- isSpecApproved tests ---

func TestIsSpecApproved_StatusApproved(t *testing.T) {
	content := "# Spec\n\n## Approval\n\n- **Status**: APPROVED\n- **Approved By**: user\n"
	if !isSpecApproved(content) {
		t.Error("expected approved for **Status**: APPROVED format")
	}
}

func TestIsSpecApproved_PlainStatus(t *testing.T) {
	content := "# Spec\n\n## Approval\n\nStatus: APPROVED\n"
	if !isSpecApproved(content) {
		t.Error("expected approved for plain Status: APPROVED format")
	}
}

func TestIsSpecApproved_Draft(t *testing.T) {
	content := "# Spec\n\n## Approval\n\n- **Status**: DRAFT\n"
	if isSpecApproved(content) {
		t.Error("expected not approved for DRAFT status")
	}
}

func TestIsSpecApproved_NoApprovalSection(t *testing.T) {
	content := "# Spec\n\n## Goal\n\nDo something.\n"
	if isSpecApproved(content) {
		t.Error("expected not approved when no Approval section")
	}
}

// --- extractSpecTitle tests ---

func TestExtractSpecTitle(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"# Spec 007: Beads Integration\n\n## Goal\n", "Beads Integration"},
		{"# Spec 006: Workflow Validation\n", "Workflow Validation"},
		{"# Some Title\n", "Some Title"},
	}
	for _, tt := range tests {
		got := extractSpecTitle(tt.content)
		if got != tt.expected {
			t.Errorf("extractSpecTitle(%q) = %q, want %q", tt.content[:30], got, tt.expected)
		}
	}
}

// --- extractGoalSummary tests ---

func TestExtractGoalSummary(t *testing.T) {
	content := "## Goal\n\nProvide CLI commands that codify conventions. More detail here.\n\n## Background\n"
	got := extractGoalSummary(content)
	if got != "Provide CLI commands that codify conventions." {
		t.Errorf("unexpected goal summary: %q", got)
	}
}

func TestExtractGoalSummary_LongGoal(t *testing.T) {
	long := "## Goal\n\n" + string(make([]byte, 200)) + "\n\n## Background\n"
	got := extractGoalSummary(long)
	if len(got) > 120 {
		t.Errorf("goal summary too long: %d chars", len(got))
	}
}

// --- extractDomains tests ---

func TestExtractDomains(t *testing.T) {
	content := "## Impacted Domains\n\n- **workflow**: new commands\n- **tracking**: bead interface\n\n## ADR\n"
	got := extractDomains(content)
	if got != "workflow, tracking" {
		t.Errorf("unexpected domains: %q", got)
	}
}

// --- buildSpecDescription tests ---

func TestBuildSpecDescription_Format(t *testing.T) {
	desc := buildSpecDescription("Provide CLI commands", "007-beads-tooling", "workflow, tracking")
	if len(desc) > 400 {
		t.Errorf("description too long: %d chars", len(desc))
	}
	if !contains(desc, "Summary: Provide CLI commands") {
		t.Errorf("missing Summary line: %s", desc)
	}
	if !contains(desc, "Spec: docs/specs/007-beads-tooling/spec.md") {
		t.Errorf("missing Spec line: %s", desc)
	}
	if !contains(desc, "Domains: workflow, tracking") {
		t.Errorf("missing Domains line: %s", desc)
	}
}

func TestBuildSpecDescription_Cap(t *testing.T) {
	longGoal := string(make([]byte, 500))
	desc := buildSpecDescription(longGoal, "007-beads-tooling", "workflow, tracking")
	if len(desc) > 400 {
		t.Errorf("description exceeds 400 char cap: %d chars", len(desc))
	}
}

// --- CreateSpecBead integration tests ---

func TestCreateSpecBead_Approved(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	os.MkdirAll(specDir, 0755)
	specContent := `# Spec 010: Test Feature

## Goal

Build a test feature for validation.

## Impacted Domains

- **core**: test impact

## Approval

- **Status**: APPROVED
- **Approved By**: user
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0644)

	// Mock Search (return empty = no existing bead)
	origExec := execCommand
	defer func() { execCommand = origExec }()

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if name == "bd" && len(args) > 0 {
			if args[0] == "search" {
				return exec.Command("echo", `[]`)
			}
			if args[0] == "create" {
				// Both spec bead and gate creation use "create"
				return exec.Command("echo", `{"id":"test-bead-010","title":"[SPEC 010-test] Test Feature","description":"","status":"open","priority":2,"issue_type":"feature","owner":"","created_at":"","updated_at":""}`)
			}
		}
		return exec.Command("echo", "")
	}

	result, err := CreateSpecBead(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Bead.ID != "test-bead-010" {
		t.Errorf("expected bead ID test-bead-010, got %s", result.Bead.ID)
	}
	if result.GateID == "" {
		t.Error("expected gate ID to be set")
	}
}

func TestCreateSpecBead_Unapproved(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	os.MkdirAll(specDir, 0755)
	specContent := `# Spec 010: Test Feature

## Goal

Build something.

## Approval

- **Status**: DRAFT
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0644)

	_, err := CreateSpecBead(tmp, "010-test")
	if err == nil {
		t.Fatal("expected error for unapproved spec")
	}
	if !contains(err.Error(), "not approved") {
		t.Errorf("error should mention not approved: %v", err)
	}
}

func TestCreateSpecBead_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	os.MkdirAll(specDir, 0755)
	specContent := `# Spec 010: Test Feature

## Goal

Build a test.

## Approval

- **Status**: APPROVED
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0644)

	origExec := execCommand
	defer func() { execCommand = origExec }()

	// Mock Search returns existing bead (first call for spec, second for gate)
	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if name == "bd" && len(args) > 0 && args[0] == "search" {
			if callCount == 1 {
				// Spec bead lookup
				return exec.Command("echo", `[{"id":"existing-bead","title":"[SPEC 010-test] Test Feature","description":"","status":"open","priority":2,"issue_type":"feature","owner":"","created_at":"","updated_at":""}]`)
			}
			// Gate lookup
			return exec.Command("echo", `[{"id":"existing-gate","title":"[GATE spec-approve 010-test]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
		}
		return exec.Command("echo", "")
	}

	result, err := CreateSpecBead(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Bead.ID != "existing-bead" {
		t.Errorf("expected existing bead ID, got %s", result.Bead.ID)
	}
	if result.GateID != "existing-gate" {
		t.Errorf("expected existing gate ID, got %s", result.GateID)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
