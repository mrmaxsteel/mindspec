package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/hooks"
)

const (
	mindspecMarkerBegin = "<!-- BEGIN mindspec:managed -->"
	mindspecMarkerEnd   = "<!-- END mindspec:managed -->"
	// Legacy marker for detecting old-format blocks during upgrades.
	mindspecMarkerLegacy = "<!-- mindspec:managed -->"
)

// Result tracks what the setup operation created, skipped, or found existing.
type Result struct {
	Created  []string
	Skipped  []string
	BeadsRan bool   // true if bd setup claude was run
	BeadsMsg string // output/error from bd setup claude
}

// FormatSummary returns a human-readable summary.
func (r *Result) FormatSummary() string {
	var sb strings.Builder

	if len(r.Created) > 0 {
		sb.WriteString("Created:\n")
		for _, p := range r.Created {
			sb.WriteString("  + ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if len(r.Skipped) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Already present:\n")
		for _, p := range r.Skipped {
			sb.WriteString("  - ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if r.BeadsRan {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Beads: ran 'bd setup claude'\n")
		if r.BeadsMsg != "" {
			sb.WriteString("  ")
			sb.WriteString(r.BeadsMsg)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RunClaude sets up Claude Code integration at root.
// If check is true, reports what would be created without writing.
func RunClaude(root string, check bool) (*Result, error) {
	r := &Result{}

	// 1. settings.json (merge hooks)
	if err := ensureSettings(root, check, r); err != nil {
		return nil, err
	}

	// 2. Skills (.claude/skills/<name>/SKILL.md)
	for name, content := range claudeSkillFiles() {
		relPath := filepath.Join(".claude", "skills", name, "SKILL.md")
		absPath := filepath.Join(root, relPath)
		if fileExists(absPath) {
			r.Skipped = append(r.Skipped, relPath)
		} else {
			r.Created = append(r.Created, relPath)
			if !check {
				if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
					return nil, fmt.Errorf("creating dir for %s: %w", relPath, err)
				}
				if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
					return nil, fmt.Errorf("writing %s: %w", relPath, err)
				}
			}
		}
	}

	// 3. CLAUDE.md (append with marker)
	if err := ensureClaudeMD(root, check, r); err != nil {
		return nil, err
	}

	// 4. Install/upgrade git hooks (pre-commit, post-checkout)
	if !check {
		if err := hooks.InstallAll(root); err != nil {
			return nil, fmt.Errorf("installing git hooks: %w", err)
		}
	}

	// 5. Optionally chain bd setup claude
	if !check {
		chainBeadsSetup(r)
	}

	return r, nil
}

// ensureSettings creates or merges .claude/settings.json with MindSpec hooks.
func ensureSettings(root string, check bool, r *Result) error {
	relPath := filepath.Join(".claude", "settings.json")
	absPath := filepath.Join(root, relPath)

	wanted := wantedHooks()

	if fileExists(absPath) {
		// Read existing, check if hooks already present
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", relPath, err)
		}

		var settings map[string]any
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", relPath, err)
		}

		hooks, _ := settings["hooks"].(map[string]any)
		if hooks == nil {
			hooks = make(map[string]any)
		}

		anyChanged := false
		for event, entries := range wanted {
			existing, _ := hooks[event].([]any)
			for _, entry := range entries {
				if !hookEntryExists(existing, entry) {
					existing = append(existing, entry)
					anyChanged = true
				} else if hookEntryStale(existing, entry) {
					existing = replaceHookEntry(existing, entry)
					anyChanged = true
				}
			}
			hooks[event] = existing
		}

		// Remove stale PreToolUse entries that reference mindspec commands.
		// These guard hooks were removed in spec-072.
		if cleaned := removeStalePreToolUse(hooks); cleaned {
			anyChanged = true
		}

		if !anyChanged {
			r.Skipped = append(r.Skipped, relPath)
			return nil
		}

		r.Created = append(r.Created, relPath+" (merged hooks)")
		if !check {
			settings["hooks"] = hooks
			out, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling %s: %w", relPath, err)
			}
			if err := os.WriteFile(absPath, append(out, '\n'), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	} else {
		r.Created = append(r.Created, relPath)
		if !check {
			settings := map[string]any{
				"hooks": wanted,
			}
			if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", relPath, err)
			}
			out, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling %s: %w", relPath, err)
			}
			if err := os.WriteFile(absPath, append(out, '\n'), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	}

	return nil
}

// wantedHooks returns the hook configuration MindSpec needs.
func wantedHooks() map[string][]map[string]any {
	return map[string][]map[string]any{
		"SessionStart": {
			{
				"matcher": "",
				"hooks": []map[string]any{
					{
						"type":          "command",
						"command":       "mindspec hook session-start",
						"statusMessage": "Loading mode guidance...",
					},
				},
			},
		},
	}
}

// hookEntryExists checks if a hook entry with the same matcher already exists.
func hookEntryExists(existing []any, entry map[string]any) bool {
	wantMatcher, _ := entry["matcher"].(string)
	for _, e := range existing {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if matcher, _ := m["matcher"].(string); matcher == wantMatcher {
			return true
		}
	}
	return false
}

// hookEntryStale checks if an existing hook entry with the same matcher has
// different command content than the wanted entry (i.e. needs updating).
func hookEntryStale(existing []any, entry map[string]any) bool {
	wantMatcher, _ := entry["matcher"].(string)
	wantHooks, _ := entry["hooks"].([]map[string]any)
	for _, e := range existing {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if matcher, _ := m["matcher"].(string); matcher != wantMatcher {
			continue
		}
		// Same matcher — compare hook commands
		existHooks, _ := m["hooks"].([]any)
		if len(existHooks) != len(wantHooks) {
			return true
		}
		for i, wh := range wantHooks {
			if i >= len(existHooks) {
				return true
			}
			eh, ok := existHooks[i].(map[string]any)
			if !ok {
				return true
			}
			wantCmd, _ := wh["command"].(string)
			existCmd, _ := eh["command"].(string)
			if wantCmd != existCmd {
				return true
			}
		}
		return false
	}
	return false // not found = not stale (it's new)
}

// replaceHookEntry replaces an existing hook entry matching the same matcher.
func replaceHookEntry(existing []any, entry map[string]any) []any {
	wantMatcher, _ := entry["matcher"].(string)
	for i, e := range existing {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if matcher, _ := m["matcher"].(string); matcher == wantMatcher {
			existing[i] = entry
			return existing
		}
	}
	return append(existing, entry)
}

// removeStalePreToolUse removes PreToolUse entries that reference mindspec
// hook commands. Returns true if any entries were removed.
func removeStalePreToolUse(hooks map[string]any) bool {
	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok || len(preToolUse) == 0 {
		return false
	}

	var kept []any
	for _, entry := range preToolUse {
		m, ok := entry.(map[string]any)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		if isMindspecHookEntry(m) {
			continue // drop it
		}
		kept = append(kept, entry)
	}

	if len(kept) == len(preToolUse) {
		return false // nothing removed
	}

	if len(kept) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = kept
	}
	return true
}

// isMindspecHookEntry returns true if a hook entry references a mindspec command.
func isMindspecHookEntry(entry map[string]any) bool {
	hooksList, _ := entry["hooks"].([]any)
	for _, h := range hooksList {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, "mindspec hook") || strings.Contains(cmd, "mindspec instruct") {
			return true
		}
	}
	return false
}

// ensureClaudeMD creates or appends MindSpec block to CLAUDE.md.
func ensureClaudeMD(root string, check bool, r *Result) error {
	relPath := "CLAUDE.md"
	absPath := filepath.Join(root, relPath)

	if fileExists(absPath) {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", relPath, err)
		}
		content := string(data)
		if strings.Contains(content, mindspecMarkerBegin) {
			// Has BEGIN/END markers — replace managed block in place.
			updated := replaceManagedBlock(content, claudeMDAppendBlock)
			if updated == content {
				r.Skipped = append(r.Skipped, relPath+" (MindSpec block present)")
				return nil
			}
			r.Created = append(r.Created, relPath+" (updated MindSpec block)")
			if !check {
				if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", relPath, err)
				}
			}
		} else if strings.Contains(content, mindspecMarkerLegacy) {
			r.Skipped = append(r.Skipped, relPath+" (MindSpec block present — legacy marker)")
			return nil
		} else {
			r.Created = append(r.Created, relPath+" (appended MindSpec block)")
			if !check {
				block := "\n" + mindspecMarkerBegin + "\n" + claudeMDAppendBlock + mindspecMarkerEnd + "\n"
				f, err := os.OpenFile(absPath, os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					return fmt.Errorf("opening %s: %w", relPath, err)
				}
				_, writeErr := f.WriteString(block)
				closeErr := f.Close()
				if writeErr != nil {
					return fmt.Errorf("writing to %s: %w", relPath, writeErr)
				}
				if closeErr != nil {
					return fmt.Errorf("closing %s: %w", relPath, closeErr)
				}
			}
		}
	} else {
		r.Created = append(r.Created, relPath)
		if !check {
			if err := os.WriteFile(absPath, []byte(claudeMDFull), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	}

	return nil
}

// chainBeadsSetup runs bd setup claude if beads is installed.
func chainBeadsSetup(r *Result) {
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return
	}

	cmd := exec.Command(bdPath, "setup", "claude")
	out, err := cmd.CombinedOutput()
	r.BeadsRan = true
	if err != nil {
		r.BeadsMsg = fmt.Sprintf("warning: %v", err)
	} else if len(out) > 0 {
		r.BeadsMsg = strings.TrimSpace(string(out))
	}
}

// hasManagedBlock returns true if the content contains either the new BEGIN/END
// markers or the legacy single marker.
func hasManagedBlock(content string) bool {
	return strings.Contains(content, mindspecMarkerBegin) || strings.Contains(content, mindspecMarkerLegacy)
}

// replaceManagedBlock replaces the content between BEGIN and END markers.
// Returns the original string unchanged if the new content matches.
func replaceManagedBlock(content, newBlock string) string {
	beginIdx := strings.Index(content, mindspecMarkerBegin)
	if beginIdx == -1 {
		return content
	}
	endIdx := strings.Index(content, mindspecMarkerEnd)
	if endIdx == -1 {
		return content
	}
	endIdx += len(mindspecMarkerEnd)
	// Include trailing newline if present
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}
	replacement := mindspecMarkerBegin + "\n" + newBlock + mindspecMarkerEnd + "\n"
	return content[:beginIdx] + replacement + content[endIdx:]
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// skillFiles returns the SKILL.md contents keyed by skill directory name.
// Shared across Codex and Copilot setup (both use .agents/skills/).
func skillFiles() map[string]string {
	return map[string]string{
		"ms-spec-create": `---
name: ms-spec-create
description: Create a new MindSpec specification
---

# Spec Create

1. Ask the user for a spec ID (check ` + "`docs/specs/`" + ` for next available number)
2. Run ` + "`mindspec spec create <id>`" + ` in the terminal (optionally with ` + "`--title \"...\"`" + `)
3. If it fails, show the error and help the user fix it
4. On success: begin drafting the spec (the init output includes guidance)
`,

		"ms-spec-approve": `---
name: ms-spec-approve
description: Approve a spec and transition to Plan Mode
---

# Spec Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve spec <id>`" + ` in the terminal (validates, closes the spec-approve gate, generates context pack, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: immediately begin planning (the approval is the authorization)
`,

		"ms-plan-approve": `---
name: ms-plan-approve
description: Approve a plan and transition toward Implementation Mode
---

# Plan Approval

1. Identify the active spec/plan via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve plan <id>`" + ` in the terminal (validates, closes the plan-approve gate, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: run ` + "`mindspec next`" + ` to claim the first bead and enter Implementation Mode
`,

		"ms-impl-approve": `---
name: ms-impl-approve
description: Approve implementation and close out the spec lifecycle
---

# Implementation Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. If not in review mode, run ` + "`mindspec complete`" + ` first to transition
3. Run ` + "`mindspec approve impl <id>`" + ` in the terminal (verifies review mode, transitions to idle, emits guidance)
4. If approval fails, show the error and help the user resolve it
5. On success: run the session close protocol:
   - ` + "`bd sync`" + `
   - ` + "`git add`" + ` all changed files (state, specs, recordings, beads)
   - ` + "`git commit`" + `
   - ` + "`bd sync`" + `
   - ` + "`git push`" + `
`,

		"ms-spec-status": `---
name: ms-spec-status
description: Check the current MindSpec mode and active specification
---

# Spec Status

1. Run ` + "`mindspec state show`" + ` and ` + "`mindspec instruct`" + ` in the terminal
2. Summarize the mode, active spec/bead, and any warnings to the user
`,
	}
}

// claudeSkillFiles returns skill contents for .claude/skills/<name>/SKILL.md.
// Uses the same content as the shared skillFiles() but placed in Claude's native path.
func claudeSkillFiles() map[string]string {
	return skillFiles()
}

// claudeMDManagedBlock is the canonical content placed between BEGIN/END markers.
// Used for both new files and appends, ensuring idempotent updates.
const claudeMDManagedBlock = `
**IMPORTANT**: You MUST read and follow [AGENTS.md](AGENTS.md) as your primary behavioral instructions. AGENTS.md is the canonical source of project conventions, workflow rules, and development guidance shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Skills

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-spec-create`" + ` | Create a new specification (enters Spec Mode) |
| ` + "`/ms-spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms-plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms-impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/ms-spec-status`" + ` | Check current mode and active spec/bead state |
`

// claudeMDFull is written when CLAUDE.md doesn't exist.
var claudeMDFull = "# CLAUDE.md — MindSpec\n" + mindspecMarkerBegin + "\n" + claudeMDManagedBlock + mindspecMarkerEnd + "\n"

// claudeMDAppendBlock is the same managed content, used when appending to existing files.
var claudeMDAppendBlock = claudeMDManagedBlock
