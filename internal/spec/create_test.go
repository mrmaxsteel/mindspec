package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// newMockExecutor creates a MockExecutor that returns a workspace under testRoot.
func newMockExecutor(testRoot, specID string) *executor.MockExecutor {
	wtPath := filepath.Join(testRoot, ".worktrees", "worktree-spec-"+specID)
	return &executor.MockExecutor{
		InitSpecWorkspaceResult: executor.WorkspaceInfo{
			Path:   wtPath,
			Branch: "spec/" + specID,
		},
	}
}

// setupTestRoot creates a minimal project root with the spec template.
func setupTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create .mindspec marker dir
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)

	// Create template
	tmplDir := filepath.Join(root, "docs", "templates")
	os.MkdirAll(tmplDir, 0755)
	os.WriteFile(filepath.Join(tmplDir, "spec.md"), []byte("# Spec <ID>: <Title>\n\n## Goal\n"), 0644)

	return root
}

// ensureWorktreeDir creates the mock worktree directory so file operations succeed.
func ensureWorktreeDir(t *testing.T, mock *executor.MockExecutor) {
	t.Helper()
	os.MkdirAll(mock.InitSpecWorkspaceResult.Path, 0755)
}

func TestRunCreatesSpecFromTemplate(t *testing.T) {
	root := setupTestRoot(t)
	mock := newMockExecutor(root, "010-my-feature")
	ensureWorktreeDir(t, mock)

	result, err := Run(root, "010-my-feature", "", mock)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Spec files are written to the worktree, not to root (ADR-0006).
	specPath := filepath.Join(workspace.SpecDir(result.WorktreePath, "010-my-feature"), "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("spec.md not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Spec 010-my-feature: My Feature") {
		t.Errorf("expected slug-derived title, got:\n%s", content)
	}

	// Verify executor was called correctly.
	calls := mock.CallsTo("InitSpecWorkspace")
	if len(calls) != 1 {
		t.Fatalf("expected 1 InitSpecWorkspace call, got %d", len(calls))
	}
	if calls[0].Args[0] != "010-my-feature" {
		t.Errorf("InitSpecWorkspace called with %v, want 010-my-feature", calls[0].Args[0])
	}
}

func TestRunWithExplicitTitle(t *testing.T) {
	root := setupTestRoot(t)
	mock := newMockExecutor(root, "011-custom")
	ensureWorktreeDir(t, mock)

	result, err := Run(root, "011-custom", "Custom Title", mock)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Spec files are written to the worktree (ADR-0006).
	specPath := filepath.Join(workspace.SpecDir(result.WorktreePath, "011-custom"), "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("spec.md not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Spec 011-custom: Custom Title") {
		t.Errorf("expected custom title, got:\n%s", content)
	}
}

func TestRunErrorsOnExistingDirectory(t *testing.T) {
	root := setupTestRoot(t)
	mock := newMockExecutor(root, "010-exists")
	ensureWorktreeDir(t, mock)

	// Pre-create the spec directory in the worktree path (where Run() now checks).
	specDir := workspace.SpecDir(mock.InitSpecWorkspaceResult.Path, "010-exists")
	os.MkdirAll(specDir, 0755)

	_, err := Run(root, "010-exists", "", mock)
	if err == nil {
		t.Fatal("expected error for existing directory, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRunSetsState(t *testing.T) {
	root := setupTestRoot(t)
	mock := newMockExecutor(root, "012-state-test")
	ensureWorktreeDir(t, mock)

	result, err := Run(root, "012-state-test", "", mock)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Per ADR-0023: no focus file is written. State is derived from beads.
	// Verify worktree was created and spec files exist.
	if result.WorktreePath == "" {
		t.Fatal("expected worktree path to be set")
	}
	if result.SpecBranch != "spec/012-state-test" {
		t.Errorf("expected branch spec/012-state-test, got %q", result.SpecBranch)
	}

	// No focus file should exist in the worktree (ADR-0023: focus files eliminated).
	focusPath := filepath.Join(result.WorktreePath, ".mindspec", "focus")
	if _, statErr := os.Stat(focusPath); statErr == nil {
		t.Error("expected no focus file in worktree (ADR-0023)")
	}
}

func TestRunRejectsInvalidSpecID(t *testing.T) {
	root := setupTestRoot(t)

	tests := []struct {
		id      string
		wantErr bool
	}{
		// Valid
		{"010-my-feature", false},
		{"001-a", false},
		{"0001-long-number", false},
		{"999-three-part-slug", false},

		// Invalid: no numeric prefix
		{"my-feature", true},
		// Invalid: fewer than 3 digits
		{"01-short", true},
		{"1-x", true},
		// Invalid: no slug after number
		{"010", true},
		// Invalid: uppercase
		{"010-My-Feature", true},
		// Invalid: spaces
		{"010-my feature", true},
		// Invalid: slug starts with digit
		{"010-1bad", true},
		// Invalid: trailing hyphen
		{"010-bad-", true},
		// Invalid: double hyphen
		{"010-bad--slug", true},
		// Invalid: free-form text
		{"a feature to allow something", true},
	}

	for _, tt := range tests {
		mock := newMockExecutor(root, tt.id)
		ensureWorktreeDir(t, mock)
		_, err := Run(root, tt.id, "", mock)
		if tt.wantErr && err == nil {
			t.Errorf("Run(%q): expected error, got nil", tt.id)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("Run(%q): unexpected error: %v", tt.id, err)
		}
	}
}

func TestTitleFromSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"010-spec-init-cmd", "Spec Init Cmd"},
		{"002-glossary", "Glossary"},
		{"foo-bar-baz", "Foo Bar Baz"},
		{"123-a", "A"},
	}
	for _, tt := range tests {
		got := titleFromSlug(tt.input)
		if got != tt.want {
			t.Errorf("titleFromSlug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRunNoLifecycleFile(t *testing.T) {
	root := setupTestRoot(t)
	mock := newMockExecutor(root, "014-lifecycle-test")
	ensureWorktreeDir(t, mock)

	result, err := Run(root, "014-lifecycle-test", "", mock)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Per ADR-0023: no lifecycle.yaml should be created (eliminated).
	specDir := workspace.SpecDir(result.WorktreePath, "014-lifecycle-test")
	lcPath := filepath.Join(specDir, "lifecycle.yaml")
	if _, statErr := os.Stat(lcPath); statErr == nil {
		t.Error("expected no lifecycle.yaml (ADR-0023), but file was created")
	}
}

func TestRunNoEpicCreation(t *testing.T) {
	root := setupTestRoot(t)
	mock := newMockExecutor(root, "013-no-epic")
	ensureWorktreeDir(t, mock)

	// Per ADR-0023: epic creation moved to spec approve.
	// spec create should succeed without creating an epic.
	result, err := Run(root, "013-no-epic", "", mock)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.WorktreePath == "" {
		t.Fatal("expected worktree to be created")
	}
}

func TestRunCommitsViaExecutor(t *testing.T) {
	root := setupTestRoot(t)
	mock := newMockExecutor(root, "015-commit-test")
	ensureWorktreeDir(t, mock)

	_, err := Run(root, "015-commit-test", "", mock)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify CommitAll was called on the workspace path.
	commits := mock.CallsTo("CommitAll")
	if len(commits) != 1 {
		t.Fatalf("expected 1 CommitAll call, got %d", len(commits))
	}
	if commits[0].Args[0] != mock.InitSpecWorkspaceResult.Path {
		t.Errorf("CommitAll path = %v, want %v", commits[0].Args[0], mock.InitSpecWorkspaceResult.Path)
	}
}
