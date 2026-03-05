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
	if !strings.Contains(content, mindspecMarkerBegin) {
		t.Error("copilot-instructions.md should contain BEGIN marker")
	}
	if !strings.Contains(content, mindspecMarkerEnd) {
		t.Error("copilot-instructions.md should contain END marker")
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

	data, err := os.ReadFile(filepath.Join(root, ".github/hooks/mindspec.json"))
	if err != nil {
		t.Fatalf("reading hooks JSON: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "mindspec hook session-start") {
		t.Error("hooks should contain sessionStart with mindspec hook session-start")
	}
	if !strings.Contains(content, `"version": 1`) {
		t.Error("hooks should have version 1")
	}
	// preToolUse should NOT be present (guard hooks removed)
	if strings.Contains(content, "preToolUse") {
		t.Error("hooks should not contain preToolUse (guard hooks removed)")
	}
}

func TestRunCopilot_Idempotent(t *testing.T) {
	root := t.TempDir()

	r1, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("first RunCopilot() error: %v", err)
	}
	if len(r1.Created) == 0 {
		t.Fatal("first run created nothing")
	}

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

	if _, err := os.Stat(filepath.Join(root, ".github")); !os.IsNotExist(err) {
		t.Error("check mode should not create files")
	}
}

func TestRunCopilot_ExistingInstructionsAppend(t *testing.T) {
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, ".github"), 0o755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# My custom Copilot instructions\n\nExisting content.\n"), 0o644)

	result, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("RunCopilot() error: %v", err)
	}

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
	if !strings.Contains(content, mindspecMarkerBegin) {
		t.Error("should contain BEGIN marker")
	}
}

func TestRunCopilot_StaleHooksUpdated(t *testing.T) {
	root := t.TempDir()

	_, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("first RunCopilot() error: %v", err)
	}

	// Write stale hooks JSON (old format with preToolUse)
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

	// Verify updated content has no preToolUse
	data, _ := os.ReadFile(hooksPath)
	if strings.Contains(string(data), "preToolUse") {
		t.Error("updated hooks should not contain preToolUse")
	}
}

func TestRunCopilot_ExistingInstructionsSkip_Legacy(t *testing.T) {
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, ".github"), 0o755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# Custom\n"+mindspecMarkerLegacy+"\nMindSpec block\n"), 0o644)

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
		t.Error("expected copilot-instructions.md to be skipped when legacy marker present")
	}
}

func TestRunCopilot_ExistingInstructionsSkip_BeginEnd(t *testing.T) {
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, ".github"), 0o755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# Custom\n"+mindspecMarkerBegin+"\n"+copilotManagedBlock+mindspecMarkerEnd+"\n"), 0o644)

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
		t.Error("expected copilot-instructions.md to be skipped when BEGIN/END markers present with current content")
	}
}

func TestRunCopilot_ExistingInstructionsUpdate_StaleBeginEnd(t *testing.T) {
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, ".github"), 0o755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# Custom\n"+mindspecMarkerBegin+"\nOld stale content\n"+mindspecMarkerEnd+"\n"), 0o644)

	result, err := RunCopilot(root, false)
	if err != nil {
		t.Fatalf("RunCopilot() error: %v", err)
	}

	hasUpdate := false
	for _, c := range result.Created {
		if strings.Contains(c, "copilot-instructions.md") && strings.Contains(c, "updated") {
			hasUpdate = true
			break
		}
	}
	if !hasUpdate {
		t.Error("expected copilot-instructions.md to be updated when BEGIN/END markers present with stale content")
	}

	data, _ := os.ReadFile(filepath.Join(root, ".github/copilot-instructions.md"))
	content := string(data)
	if !strings.Contains(content, "Custom") {
		t.Error("original content outside markers should be preserved")
	}
	if !strings.Contains(content, ".agents/skills/") {
		t.Error("managed block should contain updated content")
	}
}
