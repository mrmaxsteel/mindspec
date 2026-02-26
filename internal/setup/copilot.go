package setup

import (
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

	// 2. prompt files (.github/prompts/*.prompt.md)
	for name, content := range copilotPromptFiles() {
		relPath := filepath.Join(".github", "prompts", name)
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
		if strings.Contains(string(data), mindspecMarker) {
			r.Skipped = append(r.Skipped, relPath+" (MindSpec block present)")
			return nil
		}
		r.Created = append(r.Created, relPath+" (appended MindSpec block)")
		if !check {
			block := "\n" + mindspecMarker + "\n" + copilotInstructionsAppendBlock
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

// copilotPromptFiles returns the prompt file contents keyed by filename.
func copilotPromptFiles() map[string]string {
	return map[string]string{
		"spec-init.prompt.md": `---
description: "Initialize a new MindSpec specification"
agent: "agent"
---

# Spec Init

1. Ask the user for a spec ID (check ` + "`docs/specs/`" + ` for next available number)
2. Run ` + "`mindspec spec-init <id>`" + ` in the terminal (optionally with ` + "`--title \"...\"`" + `)
3. If it fails, show the error and help the user fix it
4. On success: begin drafting the spec (the init output includes guidance)
`,

		"spec-approve.prompt.md": `---
description: "Approve a spec and transition to Plan Mode"
agent: "agent"
---

# Spec Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve spec <id>`" + ` in the terminal (validates, closes the spec-approve gate, generates context pack, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: immediately begin planning (the approval is the authorization)
`,

		"plan-approve.prompt.md": `---
description: "Approve a plan and transition toward Implementation Mode"
agent: "agent"
---

# Plan Approval

1. Identify the active spec/plan via ` + "`mindspec state show`" + `
2. Run ` + "`mindspec approve plan <id>`" + ` in the terminal (validates, closes the plan-approve gate, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: run ` + "`mindspec next`" + ` to claim the first bead and enter Implementation Mode
`,

		"impl-approve.prompt.md": `---
description: "Approve implementation and close out the spec lifecycle"
agent: "agent"
---

# Implementation Approval

1. Identify the active spec via ` + "`mindspec state show`" + `
2. If not in review mode, run ` + "`mindspec complete`" + ` first to transition
3. Run ` + "`mindspec approve impl <id>`" + ` in the terminal (verifies review mode, transitions to idle, emits guidance)
4. If approval fails, show the error and help the user resolve it
5. On success: run the session close protocol:
   - ` + "`bd sync`" + `
   - ` + "`git add`" + ` all changed files
   - ` + "`git commit`" + `
   - ` + "`bd sync`" + `
   - ` + "`git push`" + `
`,

		"spec-status.prompt.md": `---
description: "Check the current MindSpec mode and active specification"
agent: "agent"
---

# Spec Status

1. Run ` + "`mindspec state show`" + ` and ` + "`mindspec instruct`" + ` in the terminal
2. Summarize the mode, active spec/bead, and any warnings to the user
`,
	}
}

// copilotInstructionsFull is written when .github/copilot-instructions.md doesn't exist.
const copilotInstructionsFull = `# Copilot Instructions
<!-- mindspec:managed -->

See [AGENTS.md](../AGENTS.md) for project conventions shared across all coding agents.

On session start, run ` + "`mindspec instruct`" + ` in the terminal for mode-appropriate operating guidance.

## Prompt Commands

This project includes MindSpec workflow prompt files in ` + "`.github/prompts/`" + `:

| Command | Purpose |
|:--------|:--------|
| ` + "`/spec-init`" + ` | Initialize a new specification (enters Spec Mode) |
| ` + "`/spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/spec-status`" + ` | Check current mode and active spec/bead state |
`

// copilotInstructionsAppendBlock is appended to an existing copilot-instructions.md.
const copilotInstructionsAppendBlock = `
## MindSpec

See [AGENTS.md](../AGENTS.md) for project conventions shared across all coding agents.

On session start, run ` + "`mindspec instruct`" + ` in the terminal for mode-appropriate operating guidance.

### Prompt Commands

This project includes MindSpec workflow prompt files in ` + "`.github/prompts/`" + `:

| Command | Purpose |
|:--------|:--------|
| ` + "`/spec-init`" + ` | Initialize a new specification (enters Spec Mode) |
| ` + "`/spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/spec-status`" + ` | Check current mode and active spec/bead state |
`
