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

	// Should create settings.json, 5 skill files, and CLAUDE.md = 7 items
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
	// PreToolUse should NOT be present
	if _, ok := hooks["PreToolUse"]; ok {
		t.Error("PreToolUse should not be present (guard hooks removed)")
	}

	// Verify skill files exist
	for _, name := range []string{"ms-spec-create", "ms-spec-approve", "ms-plan-approve", "ms-impl-approve", "ms-spec-status"} {
		skillPath := filepath.Join(root, ".claude", "skills", name, "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			t.Errorf("missing skill file: %s", name)
		}
	}

	// Verify CLAUDE.md exists with marker
	claudePath := filepath.Join(root, "CLAUDE.md")
	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claudeData), mindspecMarkerBegin) {
		t.Error("CLAUDE.md missing mindspec marker")
	}
	if !strings.Contains(string(claudeData), "AGENTS.md") {
		t.Error("CLAUDE.md missing AGENTS.md reference")
	}
}

func TestRunClaude_Idempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	r1, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("first RunClaude: %v", err)
	}
	if len(r1.Created) == 0 {
		t.Fatal("first run should create files")
	}

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

	if len(r.Created) == 0 {
		t.Error("check mode should report items to create")
	}

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

	found := false
	for _, c := range r.Created {
		if strings.Contains(c, "settings.json") {
			found = true
		}
	}
	if !found {
		t.Error("expected settings.json to be in Created list (merged hooks)")
	}

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

	env := merged["env"].(map[string]any)
	if env["MY_VAR"] != "value" {
		t.Error("custom env var was lost during merge")
	}
}

func TestWantedHooks_SessionStartUsesShim(t *testing.T) {
	t.Parallel()

	hooks := wantedHooks()
	sessionStart, ok := hooks["SessionStart"]
	if !ok {
		t.Fatal("missing SessionStart hooks")
	}

	if len(sessionStart) == 0 {
		t.Fatal("SessionStart has no entries")
	}

	hooksList, ok := sessionStart[0]["hooks"].([]map[string]any)
	if !ok || len(hooksList) == 0 {
		t.Fatal("SessionStart hooks is empty")
	}

	cmd, _ := hooksList[0]["command"].(string)
	if cmd != "mindspec hook session-start" {
		t.Errorf("SessionStart command should be 'mindspec hook session-start', got: %s", cmd)
	}
}

func TestWantedHooks_NoPreToolUse(t *testing.T) {
	t.Parallel()

	hooks := wantedHooks()
	if _, ok := hooks["PreToolUse"]; ok {
		t.Error("wantedHooks should not include PreToolUse (guard hooks removed)")
	}
}

func TestHookEntryStale_DetectsChangedCommand(t *testing.T) {
	t.Parallel()

	existing := []any{
		map[string]any{
			"matcher": "SessionStart",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "old command",
				},
			},
		},
	}

	wanted := map[string]any{
		"matcher": "SessionStart",
		"hooks": []map[string]any{
			{
				"type":    "command",
				"command": "new command",
			},
		},
	}

	if !hookEntryStale(existing, wanted) {
		t.Error("should detect stale hook when command differs")
	}
}

func TestHookEntryStale_NotStaleWhenSame(t *testing.T) {
	t.Parallel()

	existing := []any{
		map[string]any{
			"matcher": "SessionStart",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "same command",
				},
			},
		},
	}

	wanted := map[string]any{
		"matcher": "SessionStart",
		"hooks": []map[string]any{
			{
				"type":    "command",
				"command": "same command",
			},
		},
	}

	if hookEntryStale(existing, wanted) {
		t.Error("should not detect stale hook when command is the same")
	}
}

func TestRemoveStalePreToolUse_RemovesMindspecEntries(t *testing.T) {
	t.Parallel()

	hooks := map[string]any{
		"PreToolUse": []any{
			map[string]any{
				"matcher": "Write",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "mindspec hook worktree-file",
					},
				},
			},
			map[string]any{
				"matcher": "Bash",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "mindspec hook worktree-bash",
					},
				},
			},
		},
	}

	if !removeStalePreToolUse(hooks) {
		t.Error("should have removed stale entries")
	}
	if _, ok := hooks["PreToolUse"]; ok {
		t.Error("PreToolUse should be completely removed when all entries are mindspec hooks")
	}
}

func TestRemoveStalePreToolUse_PreservesNonMindspecEntries(t *testing.T) {
	t.Parallel()

	hooks := map[string]any{
		"PreToolUse": []any{
			map[string]any{
				"matcher": "Write",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "echo custom hook",
					},
				},
			},
			map[string]any{
				"matcher": "Bash",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "mindspec hook worktree-bash",
					},
				},
			},
		},
	}

	if !removeStalePreToolUse(hooks) {
		t.Error("should have removed stale entries")
	}
	remaining, ok := hooks["PreToolUse"].([]any)
	if !ok || len(remaining) != 1 {
		t.Fatalf("expected 1 remaining entry, got %v", hooks["PreToolUse"])
	}
	m := remaining[0].(map[string]any)
	if m["matcher"] != "Write" {
		t.Error("non-mindspec entry should be preserved")
	}
}

func TestRemoveStalePreToolUse_NoOpWhenNoPreToolUse(t *testing.T) {
	t.Parallel()

	hooks := map[string]any{
		"SessionStart": []any{},
	}

	if removeStalePreToolUse(hooks) {
		t.Error("should return false when no PreToolUse")
	}
}

func TestRunClaude_CleansStalePreToolUse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// First run to create settings
	if _, err := RunClaude(root, false); err != nil {
		t.Fatalf("first RunClaude: %v", err)
	}

	// Manually inject stale PreToolUse entries
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(data, &settings)
	hooks := settings["hooks"].(map[string]any)
	hooks["PreToolUse"] = []any{
		map[string]any{
			"matcher": "Write",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "mindspec hook worktree-file",
				},
			},
		},
	}
	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, append(out, '\n'), 0o644)

	// Second run should clean stale entries
	r2, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("second RunClaude: %v", err)
	}

	hasCleanup := false
	for _, c := range r2.Created {
		if strings.Contains(c, "settings.json") {
			hasCleanup = true
		}
	}
	if !hasCleanup {
		t.Error("expected settings.json to be updated (stale PreToolUse cleaned)")
	}

	// Verify PreToolUse is removed
	data, _ = os.ReadFile(settingsPath)
	var updated map[string]any
	json.Unmarshal(data, &updated)
	updatedHooks := updated["hooks"].(map[string]any)
	if _, ok := updatedHooks["PreToolUse"]; ok {
		t.Error("PreToolUse should have been removed")
	}
}

func TestRunClaude_AppendExistingClaudeMD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	original := "# My Project\n\nExisting instructions.\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("RunClaude: %v", err)
	}

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
	if !strings.Contains(content, mindspecMarkerBegin) {
		t.Error("marker not appended")
	}
	if !strings.Contains(content, "AGENTS.md") {
		t.Error("AGENTS.md reference not appended")
	}
}
