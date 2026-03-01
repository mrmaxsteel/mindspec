package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/hooks"
)

const mindspecMarker = "<!-- mindspec:managed -->"

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

	// 2. slash commands
	for name, content := range commandFiles() {
		relPath := filepath.Join(".claude", "commands", name)
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
						"command":       "source=$(cat | jq -r '.source // \"unknown\"'); mindspec state write-session --source=\"$source\" 2>/dev/null; mindspec instruct 2>/dev/null || echo 'mindspec instruct unavailable — run make build'",
						"statusMessage": "Loading mode guidance...",
					},
				},
			},
		},
		"PreToolUse": {
			{
				"matcher": "ExitPlanMode",
				"hooks": []map[string]any{
					{
						"type":          "command",
						"command":       "mindspec hook plan-gate-exit",
						"statusMessage": "Checking MindSpec plan gate...",
					},
				},
			},
			{
				"matcher": "EnterPlanMode",
				"hooks": []map[string]any{
					{
						"type":          "command",
						"command":       "mindspec hook plan-gate-enter",
						"statusMessage": "Checking MindSpec plan gate...",
					},
				},
			},
			{
				"matcher": "Write",
				"hooks": []map[string]any{
					{
						"type":          "command",
						"command":       "mindspec hook worktree-file",
						"statusMessage": "Checking worktree enforcement...",
					},
					{
						"type":          "command",
						"command":       "mindspec hook workflow-guard",
						"statusMessage": "Checking workflow guard...",
					},
				},
			},
			{
				"matcher": "Edit",
				"hooks": []map[string]any{
					{
						"type":          "command",
						"command":       "mindspec hook worktree-file",
						"statusMessage": "Checking worktree enforcement...",
					},
					{
						"type":          "command",
						"command":       "mindspec hook workflow-guard",
						"statusMessage": "Checking workflow guard...",
					},
				},
			},
			{
				"matcher": "Bash",
				"hooks": []map[string]any{
					{
						"type":          "command",
						"command":       "mindspec hook worktree-bash",
						"statusMessage": "Checking worktree enforcement...",
					},
					{
						"type":          "command",
						"command":       "mindspec hook needs-clear",
						"statusMessage": "Checking context clear gate...",
					},
					{
						"type":          "command",
						"command":       "mindspec hook workflow-guard",
						"statusMessage": "Checking workflow guard...",
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

// ensureClaudeMD creates or appends MindSpec block to CLAUDE.md.
func ensureClaudeMD(root string, check bool, r *Result) error {
	relPath := "CLAUDE.md"
	absPath := filepath.Join(root, relPath)

	if fileExists(absPath) {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", relPath, err)
		}
		if strings.Contains(string(data), mindspecMarker) {
			r.Skipped = append(r.Skipped, relPath+" (MindSpec block present)")
			return nil
		}
		r.Created = append(r.Created, relPath+" (appended MindSpec block)")
		if !check {
			block := "\n" + mindspecMarker + "\n" + claudeMDAppendBlock
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// skillFiles returns the SKILL.md contents keyed by skill directory name.
// Shared across Codex and Copilot setup (both use .agents/skills/).
func skillFiles() map[string]string {
	return map[string]string{
		"ms:explore": `---
name: ms:explore
description: Enter, promote, or dismiss a MindSpec Explore Mode session
---

# Explore Mode

- Enter: ` + "`mindspec explore \"short description\"`" + `
- Promote to spec: ` + "`mindspec explore promote <spec-id>`" + `
- Dismiss: ` + "`mindspec explore dismiss`" + ` (optionally ` + "`--adr`" + ` to record decision)
`,

		"ms:spec-init": `---
name: ms:spec-init
description: Initialize a new MindSpec specification
---

# Spec Init

1. Ask the user for a spec ID (check ` + "`docs/specs/`" + ` for next available number)
2. Run ` + "`mindspec spec-init <id>`" + ` in the terminal (optionally with ` + "`--title \"...\"`" + `)
3. If it fails, show the error and help the user fix it
4. On success: begin drafting the spec (the init output includes guidance)
`,

		"ms:spec-approve": `---
name: ms:spec-approve
description: Approve a spec and transition to Plan Mode
---

# Spec Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve spec <id>`" + ` in the terminal (validates, closes the spec-approve gate, generates context pack, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: immediately begin planning (the approval is the authorization)
`,

		"ms:plan-approve": `---
name: ms:plan-approve
description: Approve a plan and transition toward Implementation Mode
---

# Plan Approval

1. Identify the active spec/plan via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve plan <id>`" + ` in the terminal (validates, closes the plan-approve gate, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: run ` + "`mindspec next`" + ` to claim the first bead and enter Implementation Mode
`,

		"ms:impl-approve": `---
name: ms:impl-approve
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

		"ms:spec-status": `---
name: ms:spec-status
description: Check the current MindSpec mode and active specification
---

# Spec Status

1. Run ` + "`mindspec state show`" + ` and ` + "`mindspec instruct`" + ` in the terminal
2. Summarize the mode, active spec/bead, and any warnings to the user
`,
	}
}

// commandFiles returns the slash command file contents keyed by filename.
func commandFiles() map[string]string {
	return map[string]string{
		"ms:explore.md": `---
description: Enter, promote, or dismiss an Explore Mode session
---

# Explore Mode

- Enter: ` + "`mindspec explore \"short description\"`" + `
- Promote to spec: ` + "`mindspec explore promote <spec-id>`" + `
- Dismiss: ` + "`mindspec explore dismiss`" + ` (optionally ` + "`--adr`" + ` to record decision)
`,

		"ms:spec-init.md": `---
description: Initialize a new MindSpec specification
---

# Spec Init

1. Ask the user for a spec ID (check ` + "`docs/specs/`" + ` for next available number)
2. Run ` + "`mindspec spec-init <id>`" + ` (optionally with ` + "`--title \"...\"`" + `)
3. If it fails, show the error and help the user fix it
4. On success: begin drafting the spec (the init output includes guidance)
`,

		"ms:spec-approve.md": `---
description: Approve a spec and transition to Plan Mode
---

# Spec Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve spec <id>`" + ` (validates, closes the spec-approve molecule step, generates context pack, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: immediately begin planning (the approval is the authorization)
`,

		"ms:plan-approve.md": `---
description: Approve a plan and transition toward Implementation Mode
---

# Plan Approval

1. Identify the active spec/plan via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve plan <id>`" + ` (validates, closes the plan-approve molecule step, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: run ` + "`mindspec next`" + ` to claim the first bead and enter Implementation Mode (do NOT ask the user — just do it)
`,

		"ms:impl-approve.md": `---
description: Approve implementation and close out the spec lifecycle
---

# Implementation Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. If not in review mode, run ` + "`mindspec complete`" + ` first to transition
3. Run ` + "`mindspec approve impl <id>`" + ` (verifies review mode, transitions to idle, emits guidance)
4. If approval fails, show the error and help the user resolve it
5. On success: run the session close protocol as the LAST step (after idle, after recordings stop):
   - ` + "`bd sync`" + `
   - ` + "`git add`" + ` all changed files (state, specs, recordings, beads)
   - ` + "`git commit`" + `
   - ` + "`bd sync`" + `
   - ` + "`git push`" + `
`,

		"ms:spec-status.md": `---
description: Check the current MindSpec mode and active specification
---

# Spec Status

1. Run ` + "`mindspec state show`" + ` and ` + "`mindspec instruct`" + `
2. Summarize the mode, active spec/bead, and any warnings to the user
`,
	}
}

// claudeMDFull is written when CLAUDE.md doesn't exist.
const claudeMDFull = `# CLAUDE.md — MindSpec
<!-- mindspec:managed -->

**IMPORTANT**: You MUST read and follow [AGENTS.md](AGENTS.md) as your primary behavioral instructions. AGENTS.md is the canonical source of project conventions, workflow rules, and development guidance shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Custom Commands

| Command | Purpose |
|:--------|:--------|
| ` + "`/ms:explore`" + ` | Enter, promote, or dismiss an Explore Mode session |
| ` + "`/ms:spec-init`" + ` | Initialize a new specification (enters Spec Mode) |
| ` + "`/ms:spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms:plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms:impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/ms:spec-status`" + ` | Check current mode and active spec/bead state |
`

// claudeMDAppendBlock is appended to an existing CLAUDE.md.
const claudeMDAppendBlock = `
## MindSpec

**IMPORTANT**: You MUST read and follow [AGENTS.md](AGENTS.md) as your primary behavioral instructions. AGENTS.md is the canonical source of project conventions, workflow rules, and development guidance shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

### Custom Commands

| Command | Purpose |
|:--------|:--------|
| ` + "`/ms:explore`" + ` | Enter, promote, or dismiss an Explore Mode session |
| ` + "`/ms:spec-init`" + ` | Initialize a new specification (enters Spec Mode) |
| ` + "`/ms:spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms:plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms:impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/ms:spec-status`" + ` | Check current mode and active spec/bead state |
`
