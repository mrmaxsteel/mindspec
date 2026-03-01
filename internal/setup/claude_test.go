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

	// Should create settings.json, 6 command files, 6 skill files, and CLAUDE.md = 14 items
	if len(r.Created) != 14 {
		t.Errorf("expected 14 created items, got %d: %v", len(r.Created), r.Created)
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
	for _, name := range []string{"ms:explore.md", "ms:spec-init.md", "ms:spec-approve.md", "ms:plan-approve.md", "ms:impl-approve.md", "ms:spec-status.md"} {
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
	if len(r2.Skipped) != 14 {
		t.Errorf("second run should skip 14 items, got %d: %v", len(r2.Skipped), r2.Skipped)
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

func TestWantedHooks_UseMindspecHookCommands(t *testing.T) {
	t.Parallel()

	hooks := wantedHooks()
	preToolUse, ok := hooks["PreToolUse"]
	if !ok {
		t.Fatal("missing PreToolUse hooks")
	}

	// Collect all commands from all PreToolUse entries
	var allCommands []string
	for _, entry := range preToolUse {
		hooksList, ok := entry["hooks"].([]map[string]any)
		if !ok {
			continue
		}
		for _, h := range hooksList {
			cmd, _ := h["command"].(string)
			allCommands = append(allCommands, cmd)
		}
	}

	// All PreToolUse hooks should use mindspec hook commands, not inline shell
	for _, cmd := range allCommands {
		if !strings.HasPrefix(cmd, "mindspec hook ") {
			t.Errorf("PreToolUse hook should use 'mindspec hook' command, got: %s", cmd)
		}
	}

	// Should have no jq references
	for _, cmd := range allCommands {
		if strings.Contains(cmd, "jq") {
			t.Errorf("hooks should not contain jq references, got: %s", cmd)
		}
	}
}

func TestHookEntryStale_DetectsChangedCommand(t *testing.T) {
	t.Parallel()

	existing := []any{
		map[string]any{
			"matcher": "Bash",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "old command",
				},
			},
		},
	}

	wanted := map[string]any{
		"matcher": "Bash",
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
			"matcher": "Bash",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "same command",
				},
			},
		},
	}

	wanted := map[string]any{
		"matcher": "Bash",
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

func TestRunClaude_UpdatesStaleHooks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// First run: create settings with hooks
	if _, err := RunClaude(root, false); err != nil {
		t.Fatalf("first RunClaude: %v", err)
	}

	// Tamper with a hook command to simulate stale state
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, _ := os.ReadFile(settingsPath)
	// Replace a known substring to make it stale
	tampered := strings.Replace(string(data), "mindspec hook worktree-file", "STALE_OLD_SCRIPT", 1)
	os.WriteFile(settingsPath, []byte(tampered), 0o644)

	// Second run: should detect and update stale hooks
	r2, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("second RunClaude: %v", err)
	}

	// Should report hooks were merged (updated), not skipped
	foundMerged := false
	for _, c := range r2.Created {
		if strings.Contains(c, "settings.json") && strings.Contains(c, "merged") {
			foundMerged = true
		}
	}
	if !foundMerged {
		t.Errorf("expected stale hooks to be updated (merged), got created=%v skipped=%v", r2.Created, r2.Skipped)
	}

	// Verify the updated file no longer has the stale content
	updated, _ := os.ReadFile(settingsPath)
	if strings.Contains(string(updated), "STALE_OLD_SCRIPT") {
		t.Error("stale hook command was not replaced")
	}
	if !strings.Contains(string(updated), "mindspec hook worktree-file") {
		t.Error("updated hook should contain mindspec hook worktree-file")
	}
}

func TestWantedHooks_BashPreToolUseIncludesNeedsClearGuard(t *testing.T) {
	t.Parallel()

	hooks := wantedHooks()
	preToolUse, ok := hooks["PreToolUse"]
	if !ok {
		t.Fatal("missing PreToolUse hooks")
	}

	// Find the Bash matcher entry
	var bashEntry map[string]any
	for _, entry := range preToolUse {
		if m, _ := entry["matcher"].(string); m == "Bash" {
			bashEntry = entry
			break
		}
	}
	if bashEntry == nil {
		t.Fatal("missing Bash matcher in PreToolUse")
	}

	// Should have at least 3 hooks (worktree-bash + needs-clear + workflow-guard)
	hooksList, ok := bashEntry["hooks"].([]map[string]any)
	if !ok {
		t.Fatal("Bash hooks is not []map[string]any")
	}
	if len(hooksList) < 3 {
		t.Fatalf("expected at least 3 Bash hooks, got %d", len(hooksList))
	}

	// Verify needs-clear guard is present
	found := false
	for _, h := range hooksList {
		cmd, _ := h["command"].(string)
		if cmd == "mindspec hook needs-clear" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Bash PreToolUse hooks missing mindspec hook needs-clear")
	}
}

func TestWantedHooks_SessionStartIncludesClearFlag(t *testing.T) {
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
	if !strings.Contains(cmd, "state write-session") {
		t.Errorf("SessionStart command should include 'state write-session', got: %s", cmd)
	}
	if !strings.Contains(cmd, "mindspec instruct") {
		t.Error("SessionStart command should still include 'mindspec instruct'")
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
