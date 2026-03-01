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

	// Verify hooks JSON
	hooksPath := filepath.Join(root, ".github/hooks/mindspec.json")
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		t.Error("expected .github/hooks/mindspec.json to exist")
	}

}

func TestRunCopilot_HooksContent(t *testing.T) {
	root := t.TempDir()

	_, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("RunCopilot() error: %v", err)
	}

	// Check hooks JSON has the right structure
	data, err := os.ReadFile(filepath.Join(root, ".github/hooks/mindspec.json"))
	if err != nil {
		t.Fatalf("reading hooks JSON: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "mindspec instruct") {
		t.Error("hooks should contain sessionStart with mindspec instruct")
	}
	if !strings.Contains(content, "mindspec hook workflow-guard --format copilot") {
		t.Error("hooks should reference mindspec hook commands in preToolUse")
	}
	if !strings.Contains(content, "mindspec hook worktree-file --format copilot") {
		t.Error("hooks should reference worktree-file hook in preToolUse")
	}
	if !strings.Contains(content, `"version": 1`) {
		t.Error("hooks should have version 1")
	}
	// sessionStart now uses jq for source parsing (matching Claude pattern)
	if !strings.Contains(content, "state write-session") {
		t.Error("hooks should contain state write-session in sessionStart")
	}
	// needs-clear hook
	if !strings.Contains(content, "mindspec hook needs-clear --format copilot") {
		t.Error("hooks should contain needs-clear hook in preToolUse")
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

func TestRunCopilot_StaleHooksUpdated(t *testing.T) {
	root := t.TempDir()

	// First run to create everything
	_, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("first RunCopilot() error: %v", err)
	}

	// Write stale hooks JSON (missing needs-clear hook)
	hooksPath := filepath.Join(root, ".github/hooks/mindspec.json")
	staleContent := `{
  "version": 1,
  "hooks": {
    "sessionStart": [{"type":"command","bash":"mindspec instruct","timeoutSec":10}],
    "preToolUse": [{"type":"command","bash":"mindspec hook worktree-file --format copilot","timeoutSec":5}]
  }
}
`
	os.WriteFile(hooksPath, []byte(staleContent), 0o644)

	// Second run should detect stale and update
	r2, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("second RunCopilot() error: %v", err)
	}

	hasUpdate := false
	for _, c := range r2.Created {
		if strings.Contains(c, "mindspec.json") && strings.Contains(c, "updated") {
			hasUpdate = true
			break
		}
	}
	if !hasUpdate {
		t.Errorf("expected stale hooks to be updated, got created=%v skipped=%v", r2.Created, r2.Skipped)
	}

	// Verify updated content has needs-clear
	data, _ := os.ReadFile(hooksPath)
	if !strings.Contains(string(data), "needs-clear") {
		t.Error("updated hooks should contain needs-clear")
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
