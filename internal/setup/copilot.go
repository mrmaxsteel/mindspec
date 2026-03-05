package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// 3. Skills (.agents/skills/<name>/SKILL.md)
	for name, content := range skillFiles() {
		relPath := filepath.Join(".agents", "skills", name, "SKILL.md")
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

	return r, nil
}

// ensureCopilotInstructions creates or appends MindSpec block to copilot-instructions.md.
func ensureCopilotInstructions(root string, check bool, r *Result) error {
	relPath := filepath.Join(".github", "copilot-instructions.md")
	absPath := filepath.Join(root, relPath)

	if fileExists(absPath) {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", relPath, err)
		}
		content := string(data)
		if strings.Contains(content, mindspecMarkerBegin) {
			updated := replaceManagedBlock(content, copilotInstructionsAppendBlock)
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
				block := "\n" + mindspecMarkerBegin + "\n" + copilotInstructionsAppendBlock + mindspecMarkerEnd + "\n"
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
			if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", relPath, err)
			}
			if err := os.WriteFile(absPath, []byte(copilotInstructionsFull), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	}

	return nil
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
