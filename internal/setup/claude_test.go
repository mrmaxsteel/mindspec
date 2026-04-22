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

// installFakeBD writes a shell stub named `bd` into dir that records its
// working directory and argv to $dir/invocations.log, then returns success.
// Callers must prepend dir to PATH (via t.Setenv) to shadow the real bd.
func installFakeBD(t *testing.T, dir string) string {
	t.Helper()
	log := filepath.Join(dir, "invocations.log")
	script := "#!/bin/sh\nprintf 'cwd=%s args=%s\\n' \"$PWD\" \"$*\" >> \"" + log + "\"\n"
	bdPath := filepath.Join(dir, "bd")
	if err := os.WriteFile(bdPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake bd: %v", err)
	}
	return log
}

func TestChainBeadsSetup_UsesRootAsCWD(t *testing.T) {
	fakeBin := t.TempDir()
	log := installFakeBD(t, fakeBin)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	r := &Result{}
	chainBeadsSetup(root, r)

	if !r.BeadsRan {
		t.Fatal("expected BeadsRan=true")
	}
	assertFakeBDCWD(t, log, root, "setup claude")
}

func TestChainBeadsSetupCodex_UsesRootAsCWD(t *testing.T) {
	fakeBin := t.TempDir()
	log := installFakeBD(t, fakeBin)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	r := &Result{}
	chainBeadsSetupCodex(root, r)

	if !r.BeadsRan {
		t.Fatal("expected BeadsRan=true")
	}
	assertFakeBDCWD(t, log, root, "setup codex")
}

// assertFakeBDCWD checks that the fake bd was invoked exactly once with CWD
// equal to root (resolving symlinks on both sides — macOS temp dirs resolve
// through /private) and an argv matching wantArgs.
func assertFakeBDCWD(t *testing.T, logPath, root, wantArgs string) {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading invocations log: %v", err)
	}
	entry := strings.TrimSpace(string(data))
	// entry format: "cwd=<path> args=<argv>"
	const cwdPrefix = "cwd="
	if !strings.HasPrefix(entry, cwdPrefix) {
		t.Fatalf("unexpected log format: %q", entry)
	}
	rest := entry[len(cwdPrefix):]
	sp := strings.Index(rest, " args=")
	if sp < 0 {
		t.Fatalf("missing args= in log entry: %q", entry)
	}
	gotCWD := rest[:sp]
	gotArgs := rest[sp+len(" args="):]

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	resolvedGot, err := filepath.EvalSymlinks(gotCWD)
	if err != nil {
		resolvedGot = gotCWD
	}
	if resolvedGot != resolvedRoot {
		t.Errorf("bd was invoked with wrong CWD.\n  got:  %s\n  want: %s", resolvedGot, resolvedRoot)
	}
	if gotArgs != wantArgs {
		t.Errorf("unexpected argv.\n  got:  %q\n  want: %q", gotArgs, wantArgs)
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

// TestRunClaude_PatchesBeadsConfig verifies that a project which already ran
// `bd init` (so .beads/ exists with a user-authored config.yaml) ends up with
// a mindspec-ready config after `mindspec setup claude`. This is the
// "brownfield" entry point — users who installed bd first and mindspec second.
func TestRunClaude_PatchesBeadsConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existing := "issue-prefix: \"proj-x\"\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("RunClaude: %v", err)
	}
	if r.BeadsConfig == nil {
		t.Fatalf("expected BeadsConfig populated, got nil (err=%v)", r.BeadsConfErr)
	}
	added := map[string]bool{}
	for _, k := range r.BeadsConfig.Added {
		added[k] = true
	}
	for _, k := range []string{"types.custom", "status.custom", "export.git-add"} {
		if !added[k] {
			t.Errorf("expected %q in Added, got %v", k, r.BeadsConfig.Added)
		}
	}

	// Re-read the config and assert mindspec keys landed while the existing
	// issue-prefix was preserved.
	data, err := os.ReadFile(filepath.Join(beadsDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	wantFragments := []string{
		"issue-prefix:",
		"proj-x",
		"types.custom:",
		"status.custom:",
		"export.git-add:",
	}
	for _, f := range wantFragments {
		if !strings.Contains(got, f) {
			t.Errorf("config.yaml missing %q; full content:\n%s", f, got)
		}
	}
}

// TestRunClaude_BeadsConfigIdempotent verifies that running setup twice on an
// already-mindspec-ready .beads/config.yaml is a byte-identical no-op.
func TestRunClaude_BeadsConfigIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	ready := `issue-prefix: "proj"
types.custom: "gate"
status.custom: "resolved"
export.git-add: false
`
	cfgPath := filepath.Join(beadsDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(ready), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(cfgPath)

	if _, err := RunClaude(root, false); err != nil {
		t.Fatalf("first RunClaude: %v", err)
	}
	r2, err := RunClaude(root, false)
	if err != nil {
		t.Fatalf("second RunClaude: %v", err)
	}
	if r2.BeadsConfig == nil {
		t.Fatal("expected BeadsConfig on second run")
	}
	if n := len(r2.BeadsConfig.Added); n != 0 {
		t.Errorf("second run added %d keys: %v", n, r2.BeadsConfig.Added)
	}
	if n := len(r2.BeadsConfig.UserAuthored); n != 0 {
		t.Errorf("second run reported drift: %+v", r2.BeadsConfig.UserAuthored)
	}
	after, _ := os.ReadFile(cfgPath)
	if string(before) != string(after) {
		t.Errorf("config.yaml changed on idempotent run:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestRunClaude_CheckModeScansBeadsConfigWithoutMutating verifies that check
// mode returns a read-only scan (so users can preview drift via --check) but
// does not touch .beads/config.yaml on disk.
func TestRunClaude_CheckModeScansBeadsConfigWithoutMutating(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := "issue-prefix: \"proj\"\n"
	cfgPath := filepath.Join(beadsDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := RunClaude(root, true)
	if err != nil {
		t.Fatalf("RunClaude(check=true): %v", err)
	}
	if r.BeadsConfig == nil {
		t.Fatal("check mode should scan and return a ConfigResult, got nil")
	}
	if !r.BeadsScan {
		t.Error("BeadsScan should be true in check mode")
	}
	added := map[string]bool{}
	for _, k := range r.BeadsConfig.Added {
		added[k] = true
	}
	for _, k := range []string{"types.custom", "status.custom", "export.git-add"} {
		if !added[k] {
			t.Errorf("check-mode scan missing %q in Added, got %v", k, r.BeadsConfig.Added)
		}
	}
	data, _ := os.ReadFile(cfgPath)
	if string(data) != original {
		t.Errorf("check mode modified config.yaml:\nwant: %q\ngot:  %q", original, string(data))
	}
}
