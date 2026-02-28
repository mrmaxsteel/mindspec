package specinit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/state"
)

func mockSuccess(t *testing.T, testRoot string) {
	t.Helper()
	origPreflight := preflightFn
	origRunBD := runBDFn
	origRunBDCombined := runBDCombined
	origLoadConfig := loadConfigFn
	origCreateBranch := createBranchFn
	origBranchExists := branchExistsFn
	origWorktreeCreate := worktreeCreateFn
	origEnsureGitignore := ensureGitignore
	t.Cleanup(func() {
		preflightFn = origPreflight
		runBDFn = origRunBD
		runBDCombined = origRunBDCombined
		loadConfigFn = origLoadConfig
		createBranchFn = origCreateBranch
		branchExistsFn = origBranchExists
		worktreeCreateFn = origWorktreeCreate
		ensureGitignore = origEnsureGitignore
	})

	preflightFn = func(root string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) {
		// Stub epic creation: return a JSON object with an ID.
		return []byte(`{"id":"epic-123"}`), nil
	}
	runBDCombined = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	// Stub out git/worktree operations for unit tests.
	loadConfigFn = func(root string) (*config.Config, error) { return config.DefaultConfig(), nil }
	createBranchFn = func(name, from string) error { return nil }
	branchExistsFn = func(name string) bool { return false }
	worktreeCreateFn = func(relPath, branch string) error {
		// Simulate worktree creation by creating the absolute directory.
		absPath := filepath.Join(testRoot, relPath)
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("mock worktree create: %w", err)
		}
		return nil
	}
	ensureGitignore = func(root, entry string) error { return nil }
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

func TestRunCreatesSpecFromTemplate(t *testing.T) {
	root := setupTestRoot(t)
	mockSuccess(t, root)

	result, err := Run(root, "010-my-feature", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Spec files are written to the worktree, not to root (ADR-0006).
	specPath := filepath.Join(result.WorktreePath, "docs", "specs", "010-my-feature", "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("spec.md not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Spec 010-my-feature: My Feature") {
		t.Errorf("expected slug-derived title, got:\n%s", content)
	}
}

func TestRunWithExplicitTitle(t *testing.T) {
	root := setupTestRoot(t)
	mockSuccess(t, root)

	result, err := Run(root, "011-custom", "Custom Title")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Spec files are written to the worktree (ADR-0006).
	specPath := filepath.Join(result.WorktreePath, "docs", "specs", "011-custom", "spec.md")
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
	mockSuccess(t, root)

	// Pre-create the spec directory in the worktree path (where Run() now checks).
	cfg := config.DefaultConfig()
	wtName := "worktree-spec-010-exists"
	wtPath := cfg.WorktreePath(root, wtName)
	specDir := filepath.Join(wtPath, "docs", "specs", "010-exists")
	os.MkdirAll(specDir, 0755)

	_, err := Run(root, "010-exists", "")
	if err == nil {
		t.Fatal("expected error for existing directory, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRunSetsState(t *testing.T) {
	root := setupTestRoot(t)
	mockSuccess(t, root)

	_, err := Run(root, "012-state-test", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	mc, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("state.ReadFocus() error: %v", err)
	}

	if mc.Mode != state.ModeSpec {
		t.Errorf("expected mode=%q, got %q", state.ModeSpec, mc.Mode)
	}
	if mc.ActiveSpec != "012-state-test" {
		t.Errorf("expected activeSpec=%q, got %q", "012-state-test", mc.ActiveSpec)
	}
}

func TestRunRejectsInvalidSpecID(t *testing.T) {
	root := setupTestRoot(t)
	mockSuccess(t, root)

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
		_, err := Run(root, tt.id, "")
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

func TestRunCreatesLifecycle(t *testing.T) {
	root := setupTestRoot(t)
	mockSuccess(t, root)

	result, err := Run(root, "014-lifecycle-test", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify lifecycle.yaml was created in the worktree spec dir.
	specDir := filepath.Join(result.WorktreePath, "docs", "specs", "014-lifecycle-test")
	lc, err := state.ReadLifecycle(specDir)
	if err != nil {
		t.Fatalf("ReadLifecycle() error: %v", err)
	}
	if lc == nil {
		t.Fatal("expected lifecycle.yaml to be created")
	}
	if lc.Phase != state.ModeSpec {
		t.Errorf("expected phase=%q, got %q", state.ModeSpec, lc.Phase)
	}
	if lc.EpicID != "epic-123" {
		t.Errorf("expected epic_id=%q, got %q", "epic-123", lc.EpicID)
	}
}

func TestRunContinuesWhenBeadsUnavailable(t *testing.T) {
	root := setupTestRoot(t)
	mockSuccess(t, root)
	// Override preflight to fail — Run should still succeed with a warning.
	preflightFn = func(root string) error { return fmt.Errorf("bd unavailable") }

	result, err := Run(root, "013-no-beads", "")
	if err != nil {
		t.Fatalf("Run() error: %v (should succeed even without beads)", err)
	}

	// lifecycle.yaml should still be created, but with empty epic_id.
	specDir := filepath.Join(result.WorktreePath, "docs", "specs", "013-no-beads")
	lc, err := state.ReadLifecycle(specDir)
	if err != nil {
		t.Fatalf("ReadLifecycle() error: %v", err)
	}
	if lc.EpicID != "" {
		t.Errorf("expected empty epic_id when beads unavailable, got %q", lc.EpicID)
	}
}
