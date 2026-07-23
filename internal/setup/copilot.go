package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RunCopilot sets up GitHub Copilot integration at root.
// If check is true, reports what would be created without writing.
func RunCopilot(root string, check bool) (*Result, error) {
	r := &Result{}

	// 1. copilot-instructions.md (create or append with marker)
	if err := ensureCopilotInstructions(root, check, r); err != nil {
		return nil, err
	}

	// 2. hooks (.github/hooks/mindspec.json + helper scripts)
	if err := ensureCopilotHooks(root, check, r); err != nil {
		return nil, err
	}

	// 3. Skills (.agents/skills/<name>/SKILL.md) — create new, refresh
	// previously-shipped (provenance-gated), skip user-modified with a
	// notice (Reqs 18-19, HC-6).
	if err := installSkills(filepath.Join(root, ".agents", "skills"), filepath.Join(".agents", "skills"), skillFiles(), check, r); err != nil {
		return nil, err
	}

	// 4. Surface .beads/config.yaml drift. Copilot setup doesn't chain
	// `bd setup`, so this runs against whatever `.beads/` state the project
	// already has. Shared helper keeps the three entry points aligned.
	applyBeadsConfig(root, check, r)

	// 5. Ensure MindSpec's runtime files are gitignored (spec 123 R4b).
	if err := ensureGitignore(root, check, r); err != nil {
		return nil, err
	}

	return r, nil
}

// ensureCopilotInstructions creates or appends the MindSpec block to
// .github/copilot-instructions.md via the shared ensureManagedDoc helper, so the
// managed write goes through safeio and refuses symlinked targets.
func ensureCopilotInstructions(root string, check bool, r *Result) error {
	return ensureManagedDoc(root, filepath.Join(".github", "copilot-instructions.md"), copilotInstructionsFull, copilotInstructionsAppendBlock, check, r)
}

// ensureCopilotHooks creates .github/hooks/mindspec.json and helper scripts.
func ensureCopilotHooks(root string, check bool, r *Result) error {
	// Hook config file
	hooksRelPath := filepath.Join(".github", "hooks", "mindspec.json")
	hooksAbsPath := filepath.Join(root, hooksRelPath)

	if fileExists(hooksAbsPath) {
		// Stale detection: compare existing content against wanted config
		stale := false
		if !check {
			existingData, err := os.ReadFile(hooksAbsPath)
			if err == nil {
				wantedData, err2 := json.MarshalIndent(copilotHooksConfig(), "", "  ")
				if err2 == nil {
					// Compare canonical JSON (append newline to match file format)
					if string(existingData) != string(append(wantedData, '\n')) {
						stale = true
					}
				}
			}
		}
		if stale {
			r.Created = append(r.Created, hooksRelPath+" (updated)")
			data, err := json.MarshalIndent(copilotHooksConfig(), "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling hooks config: %w", err)
			}
			if err := os.WriteFile(hooksAbsPath, append(data, '\n'), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", hooksRelPath, err)
			}
		} else {
			r.Skipped = append(r.Skipped, hooksRelPath)
		}
	} else {
		r.Created = append(r.Created, hooksRelPath)
		if !check {
			if err := os.MkdirAll(filepath.Dir(hooksAbsPath), 0o755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", hooksRelPath, err)
			}
			data, err := json.MarshalIndent(copilotHooksConfig(), "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling hooks config: %w", err)
			}
			if err := os.WriteFile(hooksAbsPath, append(data, '\n'), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", hooksRelPath, err)
			}
		}
	}

	// Helper scripts
	scripts := copilotHookScripts()
	for name, content := range scripts {
		relPath := filepath.Join(".github", "hooks", name)
		absPath := filepath.Join(root, relPath)
		if fileExists(absPath) {
			r.Skipped = append(r.Skipped, relPath)
		} else {
			r.Created = append(r.Created, relPath)
			if !check {
				if err := os.WriteFile(absPath, []byte(content), 0o755); err != nil {
					return fmt.Errorf("writing %s: %w", relPath, err)
				}
			}
		}
	}

	return nil
}

// copilotHooksConfig returns the Copilot hooks configuration structure.
func copilotHooksConfig() map[string]any {
	return map[string]any{
		"version": 1,
		"hooks": map[string]any{
			"sessionStart": []map[string]any{
				{
					"type":       "command",
					"bash":       "mindspec hook session-start --format copilot",
					"timeoutSec": 10,
				},
			},
		},
	}
}

// copilotHookScripts returns helper scripts for Copilot hooks.
// Now empty — all logic moved to `mindspec hook` commands.
func copilotHookScripts() map[string]string {
	return map[string]string{}
}

// copilotManagedBlock is the canonical content placed between BEGIN/END markers.
const copilotManagedBlock = `
**IMPORTANT**: You MUST read and follow [AGENTS.md](../AGENTS.md) as your primary behavioral instructions. AGENTS.md is the canonical source of project conventions, workflow rules, and development guidance shared across all coding agents.

On session start, run ` + "`mindspec instruct`" + ` in the terminal for mode-appropriate operating guidance.

## Skills

MindSpec workflow skills are available in ` + "`.agents/skills/`" + `. Each skill directory contains a ` + "`SKILL.md`" + ` with instructions.
`

// copilotInstructionsFull is written when .github/copilot-instructions.md doesn't exist.
var copilotInstructionsFull = "# Copilot Instructions\n" + mindspecMarkerBegin + "\n" + copilotManagedBlock + mindspecMarkerEnd + "\n"

// copilotInstructionsAppendBlock is the same managed content, used when appending.
var copilotInstructionsAppendBlock = copilotManagedBlock
