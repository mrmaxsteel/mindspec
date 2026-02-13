package bead

import (
	"os/exec"
	"strings"
	"testing"
)

// --- CreateGate tests ---

func TestCreateGate_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", `{"id":"gate-abc","title":"[GATE spec-approve 008b]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}`)
	}

	result, err := CreateGate("[GATE spec-approve 008b] Spec approval", "spec-bead-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != "gate-abc" {
		t.Errorf("ID: got %q, want %q", result.ID, "gate-abc")
	}
	if !result.IsNew {
		t.Error("expected IsNew=true")
	}

	// Verify args
	if capturedArgs[0] != "bd" {
		t.Errorf("expected command 'bd', got %q", capturedArgs[0])
	}
	if capturedArgs[1] != "create" {
		t.Errorf("expected 'create', got %q", capturedArgs[1])
	}
	if capturedArgs[2] != "[GATE spec-approve 008b] Spec approval" {
		t.Errorf("expected gate title in args, got %q", capturedArgs[2])
	}

	hasType := false
	hasParent := false
	for _, arg := range capturedArgs {
		if arg == "--type=gate" {
			hasType = true
		}
		if arg == "--parent=spec-bead-123" {
			hasParent = true
		}
	}
	if !hasType {
		t.Error("expected --type=gate in args")
	}
	if !hasParent {
		t.Error("expected --parent=spec-bead-123 in args")
	}
}

func TestCreateGate_NoParent(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", `{"id":"gate-xyz","title":"test","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}`)
	}

	_, err := CreateGate("[GATE test]", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, arg := range capturedArgs {
		if strings.HasPrefix(arg, "--parent") {
			t.Error("should not include --parent when parent is empty")
		}
	}
}

// --- FindGate tests ---

func TestFindGate_Found(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `[{"id":"gate-123","title":"[GATE spec-approve 008b]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
	}

	gate, err := FindGate("[GATE spec-approve 008b]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gate == nil {
		t.Fatal("expected gate, got nil")
	}
	if gate.ID != "gate-123" {
		t.Errorf("ID: got %q, want %q", gate.ID, "gate-123")
	}
}

func TestFindGate_NotFound(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `[]`)
	}

	gate, err := FindGate("[GATE nonexistent]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gate != nil {
		t.Errorf("expected nil, got %+v", gate)
	}
}

// --- FindOrCreateGate tests ---

func TestFindOrCreateGate_ExistingGate(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		// First call is Search (from FindGate)
		if callCount == 1 {
			return exec.Command("echo", `[{"id":"existing-gate","title":"[GATE spec-approve 008b]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
		}
		// Should not reach here — create should not be called
		t.Error("CreateGate should not be called when gate exists")
		return exec.Command("echo", `{}`)
	}

	result, err := FindOrCreateGate("[GATE spec-approve 008b]", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "existing-gate" {
		t.Errorf("ID: got %q, want %q", result.ID, "existing-gate")
	}
	if result.IsNew {
		t.Error("expected IsNew=false for existing gate")
	}
}

func TestFindOrCreateGate_NewGate(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		// First call is Search (from FindGate) — returns empty
		if callCount == 1 {
			return exec.Command("echo", `[]`)
		}
		// Second call is Create
		return exec.Command("echo", `{"id":"new-gate","title":"[GATE spec-approve 008b]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}`)
	}

	result, err := FindOrCreateGate("[GATE spec-approve 008b]", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "new-gate" {
		t.Errorf("ID: got %q, want %q", result.ID, "new-gate")
	}
	if !result.IsNew {
		t.Error("expected IsNew=true for newly created gate")
	}
}

// --- ResolveGate tests ---

func TestResolveGate_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "resolved")
	}

	err := ResolveGate("gate-abc", "Spec approved by user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "gate", "resolve", "gate-abc", "--reason=Spec approved by user"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestResolveGate_NoReason(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "resolved")
	}

	err := ResolveGate("gate-abc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have --reason when reason is empty
	for _, arg := range capturedArgs {
		if strings.HasPrefix(arg, "--reason") {
			t.Error("should not include --reason when reason is empty")
		}
	}
}

// --- IsGateResolved tests ---

func TestIsGateResolved_OpenGate(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `[{"id":"gate-123","title":"[GATE spec-approve 008b]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
	}

	resolved, err := IsGateResolved("[GATE spec-approve 008b]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved {
		t.Error("expected resolved=false for open gate")
	}
}

func TestIsGateResolved_ClosedGate(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `[{"id":"gate-123","title":"[GATE spec-approve 008b]","description":"","status":"closed","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
	}

	resolved, err := IsGateResolved("[GATE spec-approve 008b]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Error("expected resolved=true for closed gate")
	}
}

func TestIsGateResolved_NoGate(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `[]`)
	}

	resolved, err := IsGateResolved("[GATE nonexistent]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Error("expected resolved=true when no gate exists (backward compat)")
	}
}

// --- Title convention tests ---

func TestSpecGateTitle(t *testing.T) {
	title := SpecGateTitle("008b-human-gates")
	expected := "[GATE spec-approve 008b-human-gates]"
	if title != expected {
		t.Errorf("got %q, want %q", title, expected)
	}
}

func TestPlanGateTitle(t *testing.T) {
	title := PlanGateTitle("008b-human-gates")
	expected := "[GATE plan-approve 008b-human-gates]"
	if title != expected {
		t.Errorf("got %q, want %q", title, expected)
	}
}
