package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// checkHooks checks for stale Claude Code and Copilot hooks.
func checkHooks(r *Report, root string) {
	checkClaudeHooksStale(r, root)
	checkCopilotHooksStale(r, root)
	checkPreCommitHookVersion(r, root)
}

// checkClaudeHooksStale detects stale PreToolUse entries in .claude/settings.json.
func checkClaudeHooksStale(r *Report, root string) {
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return // no settings.json = nothing to check
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok || len(preToolUse) == 0 {
		r.Checks = append(r.Checks, Check{
			Name:    "Claude Code hooks",
			Status:  OK,
			Message: "no stale PreToolUse hooks",
		})
		return
	}

	// Check if any entries reference mindspec
	hasMindspec := false
	for _, entry := range preToolUse {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooksList, _ := m["hooks"].([]any)
		for _, h := range hooksList {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			if strings.Contains(cmd, "mindspec") {
				hasMindspec = true
				break
			}
		}
		if hasMindspec {
			break
		}
	}

	if hasMindspec {
		r.Checks = append(r.Checks, Check{
			Name:    "Claude Code hooks",
			Status:  Warn,
			Message: "stale PreToolUse hooks found in .claude/settings.json — run 'mindspec setup claude' to clean up",
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "Claude Code hooks",
			Status:  OK,
			Message: "no stale PreToolUse hooks",
		})
	}
}

// checkCopilotHooksStale detects stale preToolUse entries in .github/hooks/mindspec.json.
func checkCopilotHooksStale(r *Report, root string) {
	hooksPath := filepath.Join(root, ".github", "hooks", "mindspec.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		return // no hooks file = nothing to check
	}

	if strings.Contains(string(data), "preToolUse") {
		r.Checks = append(r.Checks, Check{
			Name:    "Copilot hooks",
			Status:  Warn,
			Message: "stale preToolUse hooks found in .github/hooks/mindspec.json — run 'mindspec setup copilot' to clean up",
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "Copilot hooks",
			Status:  OK,
			Message: "no stale preToolUse hooks",
		})
	}
}

// checkPreCommitHookVersion checks if the git pre-commit hook is at the current version.
func checkPreCommitHookVersion(r *Report, root string) {
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return // no pre-commit hook = nothing to check
	}

	content := string(data)
	if !strings.Contains(content, "MindSpec pre-commit hook") {
		return // not a mindspec hook
	}

	if strings.Contains(content, "pre-commit hook v5") {
		r.Checks = append(r.Checks, Check{
			Name:    "git pre-commit hook",
			Status:  OK,
			Message: "v5 (current)",
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "git pre-commit hook",
			Status:  Warn,
			Message: "outdated MindSpec pre-commit hook — run 'mindspec setup claude' to upgrade",
		})
	}
}
