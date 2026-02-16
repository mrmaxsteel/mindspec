package specinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

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

	err := Run(root, "010-my-feature", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	specPath := filepath.Join(root, "docs", "specs", "010-my-feature", "spec.md")
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

	err := Run(root, "011-custom", "Custom Title")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	specPath := filepath.Join(root, "docs", "specs", "011-custom", "spec.md")
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

	// Pre-create the spec directory
	specDir := filepath.Join(root, "docs", "specs", "010-exists")
	os.MkdirAll(specDir, 0755)

	err := Run(root, "010-exists", "")
	if err == nil {
		t.Fatal("expected error for existing directory, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRunSetsState(t *testing.T) {
	root := setupTestRoot(t)

	err := Run(root, "012-state-test", "")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	s, err := state.Read(root)
	if err != nil {
		t.Fatalf("state.Read() error: %v", err)
	}

	if s.Mode != state.ModeSpec {
		t.Errorf("expected mode=%q, got %q", state.ModeSpec, s.Mode)
	}
	if s.ActiveSpec != "012-state-test" {
		t.Errorf("expected activeSpec=%q, got %q", "012-state-test", s.ActiveSpec)
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
