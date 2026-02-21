package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunClaude_FreshSetup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	r, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("RunClaude: %v", err)
	}

	// Should create settings.json, 5 command files, and CLAUDE.md = 7 items
	if len(r.Created) != 7 {
		t.Errorf("expected 7 created items, got %d: %v", len(r.Created), r.Created)
	}

	// Verify settings.json exists and has hooks
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("settings.json missing hooks")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("missing SessionStart hook")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("missing PreToolUse hook")
	}

	// Verify command files exist
	for _, name := range []string{"spec-init.md", "spec-approve.md", "plan-approve.md", "impl-approve.md", "spec-status.md"} {
		cmdPath := filepath.Join(root, ".claude", "commands", name)
		if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
			t.Errorf("missing command file: %s", name)
		}
	}

	// Verify CLAUDE.md exists with marker
	claudePath := filepath.Join(root, "CLAUDE.md")
	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claudeData), mindspecMarker) {
		t.Error("CLAUDE.md missing mindspec marker")
	}
	if !strings.Contains(string(claudeData), "AGENTS.md") {
		t.Error("CLAUDE.md missing AGENTS.md reference")
	}
}

func TestRunClaude_Idempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// First run
	r1, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("first RunClaude: %v", err)
	}
	if len(r1.Created) == 0 {
		t.Fatal("first run should create files")
	}

	// Second run
	r2, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("second RunClaude: %v", err)
	}
	if len(r2.Created) != 0 {
		t.Errorf("second run should create nothing, got %d: %v", len(r2.Created), r2.Created)
	}
	if len(r2.Skipped) != 7 {
		t.Errorf("second run should skip 7 items, got %d: %v", len(r2.Skipped), r2.Skipped)
	}
}

func TestRunClaude_CheckMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	r, err := RunClaude(root, true)
	if err != nil {
		t.Fatalf("RunClaude check: %v", err)
	}

	// Should report items to create
	if len(r.Created) == 0 {
		t.Error("check mode should report items to create")
	}

	// But nothing should actually exist
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("check mode should not create settings.json")
	}
	claudePath := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Error("check mode should not create CLAUDE.md")
	}
}

func TestRunClaude_MergesExistingSettings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create existing settings.json with a custom hook
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "Write",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "echo custom hook",
						},
					},
				},
			},
		},
		"env": map[string]any{
			"MY_VAR": "value",
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("RunClaude: %v", err)
	}

	// Should have merged (not skipped) settings.json
	found := false
	for _, c := range r.Created {
		if strings.Contains(c, "settings.json") {
			found = true
		}
	}
	if !found {
		t.Error("expected settings.json to be in Created list (merged hooks)")
	}

	// Read back and verify both old and new hooks exist
	settingsData, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var merged map[string]any
	if err := json.Unmarshal(settingsData, &merged); err != nil {
		t.Fatal(err)
	}

	hooks := merged["hooks"].(map[string]any)
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Error("custom PostToolUse hook was lost during merge")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("SessionStart hook not added during merge")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse hook not added during merge")
	}

	// Verify env was preserved
	env := merged["env"].(map[string]any)
	if env["MY_VAR"] != "value" {
		t.Error("custom env var was lost during merge")
	}
}

func TestRunClaude_AppendExistingClaudeMD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create existing CLAUDE.md without marker
	original := "# My Project\n\nExisting instructions.\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("RunClaude: %v", err)
	}

	// Check CLAUDE.md was appended to
	appended := false
	for _, c := range r.Created {
		if strings.Contains(c, "CLAUDE.md") && strings.Contains(c, "appended") {
			appended = true
		}
	}
	if !appended {
		t.Error("CLAUDE.md should be appended, not created fresh")
	}

	data, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	content := string(data)
	if !strings.HasPrefix(content, original) {
		t.Error("original CLAUDE.md content was not preserved")
	}
	if !strings.Contains(content, mindspecMarker) {
		t.Error("marker not appended")
	}
	if !strings.Contains(content, "AGENTS.md") {
		t.Error("AGENTS.md reference not appended")
	}
}
