package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

	// Should create settings.json, 5 lifecycle skill files, 11 plugin skill
	// files, and CLAUDE.md = 18 items.
	if len(r.Created) != 18 {
		t.Errorf("expected 18 created items, got %d: %v", len(r.Created), r.Created)
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
	// Spec 093 Req 9: the PreToolUse pre-complete panel-gate entry ships
	// from wantedHooks(). It must be present after a fresh setup.
	if !hooksContainCommand(hooks, "PreToolUse", "mindspec hook pre-complete") {
		t.Error("missing PreToolUse pre-complete panel-gate hook")
	}

	// Verify skill files exist — both the 5 lifecycle gates and the 11
	// plugin skills (embedded from plugins/mindspec/skills/).
	for _, name := range []string{
		// Lifecycle gates
		"ms-spec-create", "ms-spec-approve", "ms-plan-approve", "ms-impl-approve", "ms-spec-status",
		// Plugin skills (bead/panel/orchestrator)
		"ms-bead-cycle", "ms-bead-fix", "ms-bead-impl", "ms-bead-merge", "ms-bead-next",
		"ms-bead-prep", "ms-panel-create", "ms-panel-run", "ms-panel-tally",
		"ms-spec-autopilot", "ms-spec-final-review",
	} {
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
	if len(r2.Skipped) != 18 {
		t.Errorf("second run should skip 18 items, got %d: %v", len(r2.Skipped), r2.Skipped)
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

// TestWantedHooks_HasPreCompletePreToolUse pins the Spec 093 Req 9 entry:
// wantedHooks() ships a PreToolUse "Bash" entry whose command is
// `mindspec hook pre-complete` with the "Checking panel verdicts..."
// statusMessage.
func TestWantedHooks_HasPreCompletePreToolUse(t *testing.T) {
	t.Parallel()

	hooks := wantedHooks()
	entries, ok := hooks["PreToolUse"]
	if !ok || len(entries) != 1 {
		t.Fatalf("wantedHooks should include exactly one PreToolUse entry, got %v", entries)
	}
	e := entries[0]
	if e["matcher"] != "Bash" {
		t.Errorf("PreToolUse matcher = %v, want Bash", e["matcher"])
	}
	hl, _ := e["hooks"].([]map[string]any)
	if len(hl) != 1 || hl[0]["command"] != "mindspec hook pre-complete" {
		t.Errorf("PreToolUse command = %v, want mindspec hook pre-complete", hl)
	}
	if hl[0]["statusMessage"] != "Checking panel verdicts..." {
		t.Errorf("PreToolUse statusMessage = %v", hl[0]["statusMessage"])
	}
}

// jsonEntry builds the JSON-decoded shape ([]any hooks list) of a hook entry,
// mirroring what ensureSettings sees after unmarshalling settings.json.
func jsonEntry(matcher string, commands ...string) map[string]any {
	var hooksList []any
	for _, cmd := range commands {
		hooksList = append(hooksList, map[string]any{
			"type":    "command",
			"command": cmd,
		})
	}
	return map[string]any{
		"matcher": matcher,
		"hooks":   hooksList,
	}
}

// preCompleteWantedEntry is the Bead-4 PreToolUse gate entry shape from spec
// 093 Req 9, used synthetically until wantedHooks() carries the real one —
// the regression trio for Reqs 7-8 must pin the merge machinery against the
// exact shape the gate will ship with.
func preCompleteWantedEntry() map[string]any {
	return map[string]any{
		"matcher": "Bash",
		"hooks": []map[string]any{
			{
				"type":          "command",
				"command":       "mindspec hook pre-complete",
				"statusMessage": "Checking panel verdicts...",
			},
		},
	}
}

// wantedWithPreComplete returns wantedHooks() plus the synthetic PreToolUse
// pre-complete entry.
func wantedWithPreComplete() map[string][]map[string]any {
	wanted := wantedHooks()
	wanted["PreToolUse"] = []map[string]any{preCompleteWantedEntry()}
	return wanted
}

func TestIsMindspecOwned(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cmd  string
		want bool
	}{
		{"hook prefix", "mindspec hook session-start", true},
		{"legacy worktree hook", "mindspec hook worktree-bash", true},
		{"legacy instruct form", "mindspec instruct --check", true},
		{"user lint hook", "./scripts/lint.sh", false},
		{"user hook mentioning mindspec elsewhere", "echo run mindspec later", false},
		{"hook substring not at command start", "echo mindspec hook docs", false},
	}
	for _, tc := range cases {
		entry := jsonEntry("Bash", tc.cmd)
		if got := isMindspecOwned(entry); got != tc.want {
			t.Errorf("%s: isMindspecOwned(%q) = %v, want %v", tc.name, tc.cmd, got, tc.want)
		}
	}
}

func TestMergeWantedEntry_UpdatesOwnedEntryInPlace(t *testing.T) {
	t.Parallel()

	existing := []any{jsonEntry("", "mindspec hook session-start --old-flag")}
	wanted := wantedHooks()["SessionStart"][0]

	merged, changed := mergeWantedEntry(existing, wanted)
	if !changed {
		t.Error("drifted mindspec-owned entry should be updated")
	}
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry (in-place update), got %d", len(merged))
	}
	if !entryEqualsWanted(merged[0].(map[string]any), wanted) {
		t.Errorf("entry not updated to wanted shape: %v", merged[0])
	}
}

func TestMergeWantedEntry_NoChangeWhenCurrent(t *testing.T) {
	t.Parallel()

	existing := []any{jsonEntry("", "mindspec hook session-start")}
	wanted := wantedHooks()["SessionStart"][0]

	merged, changed := mergeWantedEntry(existing, wanted)
	if changed {
		t.Error("up-to-date mindspec entry should not be reported as changed")
	}
	if len(merged) != 1 {
		t.Errorf("expected 1 entry, got %d", len(merged))
	}
}

// TestMergeWantedEntry_AppendsAlongsideUserEntry pins Landmine A (spec 093
// Req 7): a user entry sharing the wanted matcher is NEVER replaced —
// mindspec's entry is appended alongside it.
func TestMergeWantedEntry_AppendsAlongsideUserEntry(t *testing.T) {
	t.Parallel()

	userEntry := jsonEntry("Bash", "./scripts/lint-guard.sh")
	existing := []any{userEntry}

	merged, changed := mergeWantedEntry(existing, preCompleteWantedEntry())
	if !changed {
		t.Error("appending the wanted entry should report a change")
	}
	if len(merged) != 2 {
		t.Fatalf("expected user entry + appended mindspec entry, got %d entries: %v", len(merged), merged)
	}
	if merged[0].(map[string]any)["matcher"] != "Bash" {
		t.Error("user entry moved")
	}
	if cmds := entryCommands(merged[0].(map[string]any)); len(cmds) != 1 || cmds[0] != "./scripts/lint-guard.sh" {
		t.Errorf("user entry mutated: %v", cmds)
	}
	if !entryEqualsWanted(merged[1].(map[string]any), preCompleteWantedEntry()) {
		t.Errorf("appended entry is not the wanted entry: %v", merged[1])
	}
}

// TestMergeWantedEntry_LegacyInstructUpdatedInPlace covers the retained
// `mindspec instruct` ownership arm (N1): an instruct-form entry is
// mindspec-owned, so it is rewritten in place rather than duplicated.
func TestMergeWantedEntry_LegacyInstructUpdatedInPlace(t *testing.T) {
	t.Parallel()

	existing := []any{jsonEntry("", "mindspec instruct")}
	wanted := wantedHooks()["SessionStart"][0]

	merged, changed := mergeWantedEntry(existing, wanted)
	if !changed {
		t.Error("legacy instruct entry should be updated")
	}
	if len(merged) != 1 {
		t.Fatalf("expected in-place update, got %d entries", len(merged))
	}
	if !entryEqualsWanted(merged[0].(map[string]any), wanted) {
		t.Errorf("entry not updated to wanted shape: %v", merged[0])
	}
}

// TestRemoveStaleMindspecEntries_WantedEntrySurvives pins Landmine B (spec
// 093 Req 8, the self-strip bug): a mindspec-owned entry that IS in the
// wanted set must survive the strip pass — merge-then-strip in the same
// ensureSettings pass must not remove what was just merged.
func TestRemoveStaleMindspecEntries_WantedEntrySurvives(t *testing.T) {
	t.Parallel()

	hooks := map[string]any{
		"PreToolUse": []any{
			jsonEntry("Bash", "mindspec hook pre-complete"),
		},
	}

	if removeStaleMindspecEntries(hooks, wantedWithPreComplete()) {
		t.Error("wanted entry must not be stripped")
	}
	remaining, ok := hooks["PreToolUse"].([]any)
	if !ok || len(remaining) != 1 {
		t.Fatalf("wanted PreToolUse entry was stripped: %v", hooks["PreToolUse"])
	}
}

func TestRemoveStaleMindspecEntries_RemovesLegacyEntries(t *testing.T) {
	t.Parallel()

	hooks := map[string]any{
		"PreToolUse": []any{
			jsonEntry("Write", "mindspec hook worktree-file"),
			jsonEntry("Bash", "mindspec hook worktree-bash"),
			jsonEntry("Bash", "mindspec instruct --guard"),
		},
	}

	if !removeStaleMindspecEntries(hooks, wantedHooks()) {
		t.Error("should have removed stale entries")
	}
	if _, ok := hooks["PreToolUse"]; ok {
		t.Error("PreToolUse should be completely removed when all entries are stale mindspec hooks")
	}
}

func TestRemoveStaleMindspecEntries_PreservesUserEntries(t *testing.T) {
	t.Parallel()

	hooks := map[string]any{
		"PreToolUse": []any{
			jsonEntry("Write", "echo custom hook"),
			jsonEntry("Bash", "mindspec hook worktree-bash"),
		},
	}

	if !removeStaleMindspecEntries(hooks, wantedHooks()) {
		t.Error("should have removed the stale mindspec entry")
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

func TestRemoveStaleMindspecEntries_NoOpWhenNothingStale(t *testing.T) {
	t.Parallel()

	hooks := map[string]any{
		"SessionStart": []any{},
		"PostToolUse": []any{
			jsonEntry("Write", "echo user hook"),
		},
	}

	if removeStaleMindspecEntries(hooks, wantedHooks()) {
		t.Error("should return false when nothing is stale")
	}
}

// --- Spec 093 Reqs 7-8 regression trio (the Landmine A/B gate) ---

// writeSettings marshals and writes a settings map to root/.claude/settings.json.
func writeSettings(t *testing.T, root string, settings map[string]any) {
	t.Helper()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readSettingsHooks reads root/.claude/settings.json and returns its hooks map.
func readSettingsHooks(t *testing.T, root string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	return hooks
}

// entriesWithCommandPrefix returns the entries under hooks[event] whose first
// command has the given prefix.
func entriesWithCommandPrefix(hooks map[string]any, event, prefix string) []map[string]any {
	var out []map[string]any
	entries, _ := hooks[event].([]any)
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		for _, cmd := range entryCommands(m) {
			if strings.HasPrefix(cmd, prefix) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// hooksContainCommand reports whether any entry under event carries a hook
// command exactly equal to cmd. Handles both decoded ([]any) and literal
// shapes via entryCommands.
func hooksContainCommand(hooks map[string]any, event, cmd string) bool {
	entries, _ := hooks[event].([]any)
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		for _, c := range entryCommands(m) {
			if c == cmd {
				return true
			}
		}
	}
	return false
}

// TestEnsureSettings_MergePathInstallsPreCompleteEntry is regression (i),
// the merge path (spec AC "Merge path"): a repo with a PRE-EXISTING
// settings.json (no mindspec entries) gets the wanted PreToolUse
// pre-complete entry through ensureSettings' merge branch — pinning the
// install-time self-strip (Landmine B) where the entry was merged and then
// removed by the strip pass in the same run.
func TestEnsureSettings_MergePathInstallsPreCompleteEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSettings(t, root, map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{jsonEntry("Write", "echo user post hook")},
		},
		"env": map[string]any{"MY_VAR": "value"},
	})

	r := &Result{}
	if err := ensureSettingsWith(root, false, r, wantedWithPreComplete()); err != nil {
		t.Fatalf("ensureSettingsWith: %v", err)
	}

	hooks := readSettingsHooks(t, root)
	got := entriesWithCommandPrefix(hooks, "PreToolUse", "mindspec hook pre-complete")
	if len(got) != 1 {
		t.Fatalf("pre-complete entry not installed via merge path (Landmine B): got %d entries, hooks=%v", len(got), hooks["PreToolUse"])
	}
	if len(entriesWithCommandPrefix(hooks, "SessionStart", "mindspec hook session-start")) != 1 {
		t.Error("session-start entry missing after merge")
	}
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Error("user PostToolUse hook lost during merge")
	}
}

// TestEnsureSettings_UserPreToolUseEntrySurvives is regression (ii), user
// hook preservation (spec AC "User-entry survival", Landmine A): a
// pre-existing user PreToolUse Bash entry shares the wanted matcher; after
// setup BOTH entries are present and the user's entry is byte-identical.
func TestEnsureSettings_UserPreToolUseEntrySurvives(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	userEntry := map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": "./scripts/bash-lint-guard.sh",
				"timeout": float64(30),
			},
		},
	}
	userBytes, err := json.Marshal(userEntry)
	if err != nil {
		t.Fatal(err)
	}
	writeSettings(t, root, map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{userEntry},
		},
	})

	r := &Result{}
	if err := ensureSettingsWith(root, false, r, wantedWithPreComplete()); err != nil {
		t.Fatalf("ensureSettingsWith: %v", err)
	}

	hooks := readSettingsHooks(t, root)
	entries, _ := hooks["PreToolUse"].([]any)
	if len(entries) != 2 {
		t.Fatalf("expected user entry + mindspec entry, got %d: %v", len(entries), entries)
	}

	got := entriesWithCommandPrefix(hooks, "PreToolUse", "./scripts/bash-lint-guard.sh")
	if len(got) != 1 {
		t.Fatal("user PreToolUse Bash entry was replaced by mindspec's (Landmine A)")
	}
	gotBytes, err := json.Marshal(got[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBytes) != string(userBytes) {
		t.Errorf("user entry not byte-identical after setup:\nbefore: %s\nafter:  %s", userBytes, gotBytes)
	}

	if len(entriesWithCommandPrefix(hooks, "PreToolUse", "mindspec hook pre-complete")) != 1 {
		t.Error("mindspec pre-complete entry not appended alongside the user entry")
	}
}

// TestEnsureSettings_RerunIdempotent is regression (iii), re-run idempotence
// (spec AC "Re-run idempotence"): starting from a settings.json carrying a
// user entry plus legacy mindspec hooks (spec-072 guard form AND the legacy
// `mindspec instruct` form — the N1 regression), a second setup run leaves
// exactly one mindspec entry per wanted hook, the legacy entries removed,
// and reports no further change.
func TestEnsureSettings_RerunIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSettings(t, root, map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				jsonEntry("Write", "mindspec hook worktree-file"),   // legacy spec-072 guard
				jsonEntry("Bash", "mindspec instruct --check-bash"), // legacy instruct form (N1)
				jsonEntry("Bash", "./scripts/user-hook.sh"),         // user entry
			},
		},
	})

	wanted := wantedWithPreComplete()

	r1 := &Result{}
	if err := ensureSettingsWith(root, false, r1, wanted); err != nil {
		t.Fatalf("first ensureSettingsWith: %v", err)
	}
	r2 := &Result{}
	if err := ensureSettingsWith(root, false, r2, wanted); err != nil {
		t.Fatalf("second ensureSettingsWith: %v", err)
	}
	if len(r2.Created) != 0 {
		t.Errorf("second run should change nothing, got Created=%v", r2.Created)
	}
	if len(r2.Skipped) != 1 {
		t.Errorf("second run should skip settings.json, got Skipped=%v", r2.Skipped)
	}

	hooks := readSettingsHooks(t, root)

	// Exactly one mindspec entry per wanted hook.
	if n := len(entriesWithCommandPrefix(hooks, "SessionStart", "mindspec hook session-start")); n != 1 {
		t.Errorf("expected exactly 1 session-start entry, got %d", n)
	}
	if n := len(entriesWithCommandPrefix(hooks, "PreToolUse", "mindspec hook pre-complete")); n != 1 {
		t.Errorf("expected exactly 1 pre-complete entry, got %d", n)
	}

	// Legacy entries removed (worktree-file guard + instruct form, N1).
	if n := len(entriesWithCommandPrefix(hooks, "PreToolUse", "mindspec hook worktree-file")); n != 0 {
		t.Errorf("legacy worktree-file guard hook not removed, got %d", n)
	}
	if n := len(entriesWithCommandPrefix(hooks, "PreToolUse", "mindspec instruct")); n != 0 {
		t.Errorf("legacy instruct-form hook not removed (N1 regression), got %d", n)
	}

	// User entry intact.
	if n := len(entriesWithCommandPrefix(hooks, "PreToolUse", "./scripts/user-hook.sh")); n != 1 {
		t.Errorf("user entry lost across re-runs, got %d", n)
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
		// Legacy instruct-form guard hook (spec-072 retirement, N1): the
		// ownership rule must keep its `mindspec instruct` arm so these
		// still get cleaned through the real entry point.
		map[string]any{
			"matcher": "Bash",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "mindspec instruct --check",
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

	// Verify the STALE PreToolUse entries are removed but the legitimate
	// Spec 093 pre-complete gate entry remains (it is in the wanted set, so
	// the second run merges it in while cleaning the stale ones).
	data, _ = os.ReadFile(settingsPath)
	var updated map[string]any
	json.Unmarshal(data, &updated)
	updatedHooks := updated["hooks"].(map[string]any)
	if n := len(entriesWithCommandPrefix(updatedHooks, "PreToolUse", "mindspec hook worktree-file")); n != 0 {
		t.Errorf("stale worktree-file guard not removed, got %d", n)
	}
	if n := len(entriesWithCommandPrefix(updatedHooks, "PreToolUse", "mindspec instruct")); n != 0 {
		t.Errorf("stale instruct-form guard not removed (N1), got %d", n)
	}
	if !hooksContainCommand(updatedHooks, "PreToolUse", "mindspec hook pre-complete") {
		t.Error("legitimate pre-complete gate entry should be present after cleanup")
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

// deprecatedApproveOrder is the hardened Req 11 pattern from
// internal/instruct/instruct_test.go: matches any verb-noun
// `mindspec approve ...` plus bare `approve spec|plan|impl` so partial
// regressions can't sneak back in; word boundaries keep canonical
// noun-verb forms (`mindspec spec approve`) from matching.
var deprecatedApproveOrder = regexp.MustCompile(`(?i)mindspec\s+approve\b|\bapprove\s+(spec|plan|impl)\b`)

// TestLifecycleSkills_CanonicalApproveOrder pins spec 092 Req 11 on the
// setup-generated lifecycle skills (Bead 9 verification stop-#3, Bead 8
// panel R2 minor): the inlined ms-*-approve SKILL.md contents taught the
// deprecated verb-noun order, were installed agent-visible into every
// project (and harness sandbox) by setup, and seeded the deprecated
// `approve impl` in the approval_gate_discovery scenario. Every rendered
// lifecycle skill must be free of the deprecated order AND each approve
// skill must teach its canonical noun-verb command.
//
// NOTE (boundary): this fixes the TEMPLATE only. Existing installs keep
// their old skill files — setup's create-or-skip semantics never refresh
// them; the provenance-gated refresh that propagates wording fixes to
// existing installs is jkhd.3 Req 19's charter, not this test's.
func TestLifecycleSkills_CanonicalApproveOrder(t *testing.T) {
	skills := lifecycleSkillFiles()

	for name, content := range skills {
		if m := deprecatedApproveOrder.FindString(content); m != "" {
			t.Errorf("lifecycle skill %s teaches deprecated approve order %q (spec 092 Req 11)\n--- content ---\n%s\n--- end ---", name, m, content)
		}
	}

	wantCanonical := map[string]string{
		"ms-spec-approve": "mindspec spec approve <id>",
		"ms-plan-approve": "mindspec plan approve <id>",
		"ms-impl-approve": "mindspec impl approve <id>",
	}
	for name, want := range wantCanonical {
		content, ok := skills[name]
		if !ok {
			t.Errorf("lifecycle skill %s missing from lifecycleSkillFiles()", name)
			continue
		}
		if !strings.Contains(content, want) {
			t.Errorf("lifecycle skill %s must teach the canonical %q; content:\n%s", name, want, content)
		}
	}
}
