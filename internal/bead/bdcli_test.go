package bead

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- BeadInfo JSON parsing tests ---

func TestBeadInfo_JSONRoundTrip(t *testing.T) {
	original := BeadInfo{
		ID:          "mindspec-abc",
		Title:       "[SPEC 006-validate] Workflow Validation",
		Description: "Summary: Add validation\nSpec: docs/specs/006-validate/spec.md",
		Status:      "open",
		Priority:    2,
		IssueType:   "feature",
		Owner:       "user@example.com",
		CreatedAt:   "2026-02-12T10:00:00Z",
		UpdatedAt:   "2026-02-12T10:30:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed BeadInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.ID != original.ID {
		t.Errorf("ID: got %q, want %q", parsed.ID, original.ID)
	}
	if parsed.Title != original.Title {
		t.Errorf("Title: got %q, want %q", parsed.Title, original.Title)
	}
	if parsed.Description != original.Description {
		t.Errorf("Description: got %q, want %q", parsed.Description, original.Description)
	}
	if parsed.Status != original.Status {
		t.Errorf("Status: got %q, want %q", parsed.Status, original.Status)
	}
	if parsed.Priority != original.Priority {
		t.Errorf("Priority: got %d, want %d", parsed.Priority, original.Priority)
	}
	if parsed.IssueType != original.IssueType {
		t.Errorf("IssueType: got %q, want %q", parsed.IssueType, original.IssueType)
	}
}

func TestParseBeadList_Single(t *testing.T) {
	input := `[{
		"id": "mindspec-25p",
		"title": "Test bead",
		"description": "A test",
		"status": "open",
		"priority": 2,
		"issue_type": "task",
		"owner": "",
		"created_at": "2026-02-12T08:50:30Z",
		"updated_at": "2026-02-12T08:50:30Z"
	}]`

	items, err := parseBeadList([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "mindspec-25p" {
		t.Errorf("expected ID mindspec-25p, got %s", items[0].ID)
	}
}

func TestParseBeadList_Multiple(t *testing.T) {
	input := `[
		{"id": "a", "title": "First", "description": "", "status": "open", "priority": 1, "issue_type": "task", "owner": "", "created_at": "", "updated_at": ""},
		{"id": "b", "title": "Second", "description": "", "status": "open", "priority": 2, "issue_type": "feature", "owner": "", "created_at": "", "updated_at": ""}
	]`

	items, err := parseBeadList([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestParseBeadList_Empty(t *testing.T) {
	items, err := parseBeadList([]byte("[]"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestParseBeadList_InvalidJSON(t *testing.T) {
	_, err := parseBeadList([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Preflight tests ---

func TestPreflight_MissingBeadsDir(t *testing.T) {
	tmp := t.TempDir()
	// Init a git repo but no .beads/
	cmd := exec.Command("git", "init")
	cmd.Dir = tmp
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	err := Preflight(tmp)
	if err == nil {
		t.Fatal("expected error for missing .beads/")
	}
	if !strings.Contains(err.Error(), ".beads/") {
		t.Errorf("error should mention .beads/: %v", err)
	}
	if !strings.Contains(err.Error(), "beads init") {
		t.Errorf("error should suggest 'beads init': %v", err)
	}
}

func TestPreflight_NotGitRepo(t *testing.T) {
	tmp := t.TempDir()
	// No .git, but add .beads/ to test git check runs first
	os.MkdirAll(filepath.Join(tmp, ".beads"), 0755)

	err := Preflight(tmp)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("error should mention git: %v", err)
	}
}

func TestPreflight_Success(t *testing.T) {
	tmp := t.TempDir()
	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmp
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	// Create .beads/
	os.MkdirAll(filepath.Join(tmp, ".beads"), 0755)

	// bd must be on PATH for this test (skip if not available)
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH, skipping Preflight success test")
	}

	err := Preflight(tmp)
	if err != nil {
		t.Fatalf("unexpected preflight error: %v", err)
	}
}

// --- Create argument construction test ---

func TestCreate_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		// Return a command that produces valid JSON
		return exec.Command("echo", `{"id":"test-123","title":"test","description":"","status":"open","priority":2,"issue_type":"feature","owner":"","created_at":"","updated_at":""}`)
	}

	_, err := Create("[SPEC 006-validate] Workflow Validation", "Summary: test", "feature", 2, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify args
	if capturedArgs[0] != "bd" {
		t.Errorf("expected command 'bd', got %q", capturedArgs[0])
	}
	if capturedArgs[1] != "create" {
		t.Errorf("expected 'create' subcommand, got %q", capturedArgs[1])
	}
	if capturedArgs[2] != "[SPEC 006-validate] Workflow Validation" {
		t.Errorf("expected title in args, got %q", capturedArgs[2])
	}

	// Should NOT have --parent when parent is empty
	for _, arg := range capturedArgs {
		if strings.HasPrefix(arg, "--parent") {
			t.Error("should not include --parent when parent is empty")
		}
	}
}

func TestCreate_WithParent(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", `{"id":"test-456","title":"test","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":""}`)
	}

	_, err := Create("[IMPL 007.1] bdcli wrapper", "Scope: internal/bead/", "task", 2, "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasParent := false
	for _, arg := range capturedArgs {
		if arg == "--parent=parent-123" {
			hasParent = true
		}
	}
	if !hasParent {
		t.Error("expected --parent=parent-123 in args")
	}
}

// --- Search argument construction test ---

func TestSearch_PassesStatusOpen(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", `[]`)
	}

	_, err := Search("[SPEC 006-validate]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasStatusOpen := false
	for _, arg := range capturedArgs {
		if arg == "--status=open" {
			hasStatusOpen = true
		}
	}
	if !hasStatusOpen {
		t.Error("expected --status=open in search args")
	}
}

// --- Close tests ---

func TestClose_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "closed")
	}

	err := Close("bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "close", "bead-abc"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestClose_MultipleIDs(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "closed")
	}

	err := Close("bead-1", "bead-2", "bead-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "close", "bead-1", "bead-2", "bead-3"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestClose_NoIDs(t *testing.T) {
	err := Close()
	if err == nil {
		t.Fatal("expected error for no IDs")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Errorf("error should mention 'at least one': %v", err)
	}
}

// --- WorktreeCreate tests ---

func TestWorktreeCreate_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "created")
	}

	err := WorktreeCreate("worktree-bead-abc", "bead/bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "worktree", "create", "worktree-bead-abc", "--branch=bead/bead-abc"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestWorktreeCreate_NoBranch(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "created")
	}

	err := WorktreeCreate("worktree-abc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have --branch when branch is empty
	for _, arg := range capturedArgs {
		if strings.HasPrefix(arg, "--branch") {
			t.Error("should not include --branch when branch is empty")
		}
	}
}

// --- WorktreeList tests ---

func TestWorktreeList_ParsesJSON(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	listJSON := `[
		{"name":"mindspec","path":"/home/user/mindspec","branch":"main","is_main":true,"beads_state":"shared"},
		{"name":"worktree-bead-abc","path":"/home/user/worktree-bead-abc","branch":"bead/bead-abc","is_main":false,"beads_state":"shared"}
	]`

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", listJSON)
	}

	entries, err := WorktreeList()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Name != "mindspec" {
		t.Errorf("entry[0].Name: got %q, want %q", entries[0].Name, "mindspec")
	}
	if !entries[0].IsMain {
		t.Error("entry[0] should be main")
	}
	if entries[1].Name != "worktree-bead-abc" {
		t.Errorf("entry[1].Name: got %q, want %q", entries[1].Name, "worktree-bead-abc")
	}
	if entries[1].Branch != "bead/bead-abc" {
		t.Errorf("entry[1].Branch: got %q, want %q", entries[1].Branch, "bead/bead-abc")
	}
	if entries[1].IsMain {
		t.Error("entry[1] should not be main")
	}
}

func TestWorktreeList_Empty(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}

	entries, err := WorktreeList()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestWorktreeList_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "[]")
	}

	_, _ = WorktreeList()

	expected := []string{"bd", "worktree", "list", "--json"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

// --- WorktreeRemove tests ---

func TestWorktreeRemove_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "removed")
	}

	err := WorktreeRemove("worktree-bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "worktree", "remove", "worktree-bead-abc"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

// --- WorktreeInfo tests ---

func TestWorktreeInfo_ParsesJSON(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	infoJSON := `{"is_worktree":true,"name":"worktree-bead-abc","path":"/home/user/worktree-bead-abc","branch":"bead/bead-abc","beads_state":"redirect","main_repo":"/home/user/mindspec"}`

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", infoJSON)
	}

	info, err := WorktreeInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsWorktree {
		t.Error("expected IsWorktree=true")
	}
	if info.Name != "worktree-bead-abc" {
		t.Errorf("Name: got %q, want %q", info.Name, "worktree-bead-abc")
	}
	if info.Branch != "bead/bead-abc" {
		t.Errorf("Branch: got %q, want %q", info.Branch, "bead/bead-abc")
	}
	if info.MainRepo != "/home/user/mindspec" {
		t.Errorf("MainRepo: got %q, want %q", info.MainRepo, "/home/user/mindspec")
	}
}

func TestWorktreeInfo_NotWorktree(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `{"is_worktree":false}`)
	}

	info, err := WorktreeInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsWorktree {
		t.Error("expected IsWorktree=false")
	}
}

func TestWorktreeInfo_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", `{"is_worktree":false}`)
	}

	_, _ = WorktreeInfo()

	expected := []string{"bd", "worktree", "info", "--json"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

// --- MolReady tests ---

func TestMolReady_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", "[]")
	}

	_, err := MolReady("mol-parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "ready", "--parent", "mol-parent-123", "--json"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestMolReady_ParsesBeadList(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	readyJSON := `[
		{"id":"bead-1","title":"[IMPL 008.1] wrappers","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":""},
		{"id":"bead-2","title":"[IMPL 008.2] molecules","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":""}
	]`

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", readyJSON)
	}

	items, err := MolReady("mol-parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "bead-1" {
		t.Errorf("items[0].ID: got %q, want %q", items[0].ID, "bead-1")
	}
	if items[1].ID != "bead-2" {
		t.Errorf("items[1].ID: got %q, want %q", items[1].ID, "bead-2")
	}
}

func TestMolReady_Empty(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}

	items, err := MolReady("mol-parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

// --- MolShow tests ---

func TestMolShow_ArgsConstruction(t *testing.T) {
	var capturedArgs []string
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("echo", `{"id":"mol-123"}`)
	}

	_, err := MolShow("mol-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"bd", "mol", "show", "mol-123", "--json"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: got %v, want %v", capturedArgs, expected)
	}
	for i, arg := range expected {
		if capturedArgs[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestMolShow_ReturnsRawJSON(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	rawJSON := `{"id":"mol-123","children":[{"id":"child-1"},{"id":"child-2"}]}`

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", rawJSON)
	}

	out, err := MolShow("mol-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain the raw JSON (with possible trailing newline from echo)
	trimmed := strings.TrimSpace(string(out))
	if trimmed != rawJSON {
		t.Errorf("output: got %q, want %q", trimmed, rawJSON)
	}
}
