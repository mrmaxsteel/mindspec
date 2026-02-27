package specinit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/specmeta"
	"github.com/mindspec/mindspec/internal/state"
)

func mockMoleculeSuccess(t *testing.T, testRoot string) {
	t.Helper()
	origPreflight := preflightFn
	origPour := pourFormulaFn
	origRunBDCombined := runBDCombined
	origLoadConfig := loadConfigFn
	origCreateBranch := createBranchFn
	origBranchExists := branchExistsFn
	origWorktreeCreate := worktreeCreateFn
	origEnsureGitignore := ensureGitignore
	origWriteSpecMeta := writeSpecMeta
	t.Cleanup(func() {
		preflightFn = origPreflight
		pourFormulaFn = origPour
		runBDCombined = origRunBDCombined
		loadConfigFn = origLoadConfig
		createBranchFn = origCreateBranch
		branchExistsFn = origBranchExists
		worktreeCreateFn = origWorktreeCreate
		ensureGitignore = origEnsureGitignore
		writeSpecMeta = origWriteSpecMeta
	})

	preflightFn = func(root string) error { return nil }
	pourFormulaFn = func(specID string) (string, map[string]string, error) {
		return "mol-123", map[string]string{
			"spec":           "step-spec",
			"spec-approve":   "step-spec-approve",
			"plan":           "step-plan",
			"plan-approve":   "step-plan-approve",
			"implement":      "step-impl",
			"review":         "step-review",
			"spec-lifecycle": "mol-123",
		}, nil
	}
	runBDCombined = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	writeSpecMeta = func(specDir string, meta *specmeta.Meta) error { return nil }
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
	mockMoleculeSuccess(t, root)

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
	mockMoleculeSuccess(t, root)

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
	mockMoleculeSuccess(t, root)

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
	mockMoleculeSuccess(t, root)

	_, err := Run(root, "012-state-test", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	mc, err := state.ReadModeCache(root)
	if err != nil {
		t.Fatalf("state.ReadModeCache() error: %v", err)
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
	mockMoleculeSuccess(t, root)

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

func TestRunCreatesFormulaIfMissing(t *testing.T) {
	root := setupTestRoot(t)
	mockMoleculeSuccess(t, root)

	// Ensure .beads dir exists but formula does not
	os.MkdirAll(filepath.Join(root, ".beads"), 0755)

	formulaPath := filepath.Join(root, ".beads", "formulas", "spec-lifecycle.formula.toml")
	if _, err := os.Stat(formulaPath); err == nil {
		t.Fatal("formula should not exist before test")
	}

	_, err := Run(root, "014-formula-test", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(formulaPath)
	if err != nil {
		t.Fatalf("formula not created: %v", err)
	}
	if !strings.Contains(string(data), `formula = "spec-lifecycle"`) {
		t.Error("formula file does not contain expected content")
	}
}

func TestRunSkipsFormulaIfExists(t *testing.T) {
	root := setupTestRoot(t)
	mockMoleculeSuccess(t, root)

	// Pre-create formula with custom content
	formulaDir := filepath.Join(root, ".beads", "formulas")
	os.MkdirAll(formulaDir, 0755)
	customContent := "# custom formula\n"
	os.WriteFile(filepath.Join(formulaDir, "spec-lifecycle.formula.toml"), []byte(customContent), 0644)

	_, err := Run(root, "015-formula-exists", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify it was NOT overwritten
	data, err := os.ReadFile(filepath.Join(formulaDir, "spec-lifecycle.formula.toml"))
	if err != nil {
		t.Fatalf("reading formula: %v", err)
	}
	if string(data) != customContent {
		t.Error("existing formula was overwritten")
	}
}

func TestRunFailsWhenMoleculeUnavailable(t *testing.T) {
	root := setupTestRoot(t)
	// Set up stubs for Phase 1 (worktree creation) but fail at Phase 3 (preflight).
	mockMoleculeSuccess(t, root)
	preflightFn = func(root string) error { return fmt.Errorf("bd unavailable") }

	_, err := Run(root, "013-molecule-required", "")
	if err == nil {
		t.Fatal("expected error when molecule setup is unavailable")
	}
	if !strings.Contains(err.Error(), "lifecycle molecule") {
		t.Errorf("expected lifecycle molecule error, got: %v", err)
	}
}
