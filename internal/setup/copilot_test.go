package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCopilot_Greenfield(t *testing.T) {
	root := t.TempDir()

	result, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("RunCopilot() error: %v", err)
	}

	if len(result.Created) == 0 {
		t.Fatal("expected items to be created")
	}

	// Verify copilot-instructions.md
	data, err := os.ReadFile(filepath.Join(root, ".github/copilot-instructions.md"))
	if err != nil {
		t.Fatalf("reading copilot-instructions.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "AGENTS.md") {
		t.Error("copilot-instructions.md should reference AGENTS.md")
	}
	if !strings.Contains(content, mindspecMarker) {
		t.Error("copilot-instructions.md should contain marker")
	}

	// Verify prompt files
	expectedPrompts := []string{
		"spec-init.prompt.md",
		"spec-approve.prompt.md",
		"plan-approve.prompt.md",
		"impl-approve.prompt.md",
		"spec-status.prompt.md",
	}
	for _, name := range expectedPrompts {
		p := filepath.Join(root, ".github/prompts", name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected prompt file %s to exist", name)
		}
	}
}

func TestRunCopilot_PromptFileContent(t *testing.T) {
	root := t.TempDir()

	_, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("RunCopilot() error: %v", err)
	}

	// Check spec-approve prompt has correct frontmatter and content
	data, err := os.ReadFile(filepath.Join(root, ".github/prompts/spec-approve.prompt.md"))
	if err != nil {
		t.Fatalf("reading spec-approve.prompt.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "agent: \"agent\"") {
		t.Error("prompt file should have agent: \"agent\" in frontmatter")
	}
	if !strings.Contains(content, "mindspec approve spec") {
		t.Error("spec-approve prompt should reference mindspec approve spec command")
	}
}

func TestRunCopilot_Idempotent(t *testing.T) {
	root := t.TempDir()

	// First run
	r1, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("first RunCopilot() error: %v", err)
	}
	if len(r1.Created) == 0 {
		t.Fatal("first run created nothing")
	}

	// Second run
	r2, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("second RunCopilot() error: %v", err)
	}
	if len(r2.Created) != 0 {
		t.Errorf("second run created %d items, expected 0: %v", len(r2.Created), r2.Created)
	}
	if len(r2.Skipped) == 0 {
		t.Error("second run should have skipped items")
	}
}

func TestRunCopilot_CheckMode(t *testing.T) {
	root := t.TempDir()

	result, err := RunCopilot(root, true)
	if err != nil {
		t.Fatalf("RunCopilot(check=true) error: %v", err)
	}

	if len(result.Created) == 0 {
		t.Fatal("check mode reported nothing to create")
	}

	// Verify nothing written
	if _, err := os.Stat(filepath.Join(root, ".github")); !os.IsNotExist(err) {
		t.Error("check mode should not create files")
	}
}

func TestRunCopilot_ExistingInstructionsAppend(t *testing.T) {
	root := t.TempDir()

	// Pre-create copilot-instructions.md without marker
	os.MkdirAll(filepath.Join(root, ".github"), 0o755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# My custom Copilot instructions\n\nExisting content.\n"), 0o644)

	result, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("RunCopilot() error: %v", err)
	}

	// Check it was appended
	hasAppend := false
	for _, c := range result.Created {
		if strings.Contains(c, "copilot-instructions.md") && strings.Contains(c, "appended") {
			hasAppend = true
			break
		}
	}
	if !hasAppend {
		t.Error("expected copilot-instructions.md to be appended")
	}

	data, _ := os.ReadFile(filepath.Join(root, ".github/copilot-instructions.md"))
	content := string(data)
	if !strings.Contains(content, "My custom Copilot instructions") {
		t.Error("original content should be preserved")
	}
	if !strings.Contains(content, mindspecMarker) {
		t.Error("should contain mindspec marker")
	}
}

func TestRunCopilot_ExistingInstructionsSkip(t *testing.T) {
	root := t.TempDir()

	// Pre-create with marker
	os.MkdirAll(filepath.Join(root, ".github"), 0o755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# Custom\n"+mindspecMarker+"\nMindSpec block\n"), 0o644)

	result, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("RunCopilot() error: %v", err)
	}

	hasSkip := false
	for _, s := range result.Skipped {
		if strings.Contains(s, "copilot-instructions.md") {
			hasSkip = true
			break
		}
	}
	if !hasSkip {
		t.Error("expected copilot-instructions.md to be skipped when marker present")
	}
}
