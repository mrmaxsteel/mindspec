package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/hooks"
)

// RunCodex sets up OpenAI Codex CLI integration at root.
// If check is true, reports what would be created without writing.
func RunCodex(root string, check bool) (*Result, error) {
	r := &Result{}

	// 1. AGENTS.md (create or append with marker)
	if err := ensureAgentsMD(root, check, r); err != nil {
		return nil, err
	}

	// 2. Skills (.agents/skills/<name>/SKILL.md)
	for name, content := range codexSkillFiles() {
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

	// 3. Install/upgrade git hooks (pre-commit, post-checkout)
	if !check {
		if err := hooks.InstallAll(root); err != nil {
			return nil, fmt.Errorf("installing git hooks: %w", err)
		}
	}

	// 4. Optionally chain bd setup codex
	if !check {
		chainBeadsSetupCodex(r)
	}

	return r, nil
}

// ensureAgentsMD creates or appends MindSpec block to AGENTS.md.
func ensureAgentsMD(root string, check bool, r *Result) error {
	relPath := "AGENTS.md"
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
			block := "\n" + mindspecMarker + "\n" + agentsMDAppendBlock
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
			if err := os.WriteFile(absPath, []byte(agentsMDFull), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	}

	return nil
}

// chainBeadsSetupCodex runs bd setup codex if beads is installed.
func chainBeadsSetupCodex(r *Result) {
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return
	}

	cmd := exec.Command(bdPath, "setup", "codex")
	out, err := cmd.CombinedOutput()
	r.BeadsRan = true
	if err != nil {
		r.BeadsMsg = fmt.Sprintf("warning: %v", err)
	} else if len(out) > 0 {
		r.BeadsMsg = strings.TrimSpace(string(out))
	}
}

// codexSkillFiles returns the SKILL.md contents keyed by skill directory name.
func codexSkillFiles() map[string]string {
	return map[string]string{
		"ms-explore": `---
name: ms-explore
description: Enter, promote, or dismiss a MindSpec Explore Mode session
---

# Explore Mode

- Enter: ` + "`mindspec explore \"short description\"`" + `
- Promote to spec: ` + "`mindspec explore promote <spec-id>`" + `
- Dismiss: ` + "`mindspec explore dismiss`" + ` (optionally ` + "`--adr`" + ` to record decision)
`,

		"ms-spec-init": `---
name: ms-spec-init
description: Initialize a new MindSpec specification
---

# Spec Init

1. Ask the user for a spec ID (check ` + "`docs/specs/`" + ` for next available number)
2. Run ` + "`mindspec spec-init <id>`" + ` in the terminal (optionally with ` + "`--title \"...\"`" + `)
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

// agentsMDFull is written when AGENTS.md doesn't exist.
const agentsMDFull = `# AGENTS.md
<!-- mindspec:managed -->

This project uses [MindSpec](https://github.com/mindspec/mindspec), a spec-driven development framework.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance.

## Build & Test

` + "```bash" + `
make build    # Build binary
make test     # Run all tests
` + "```" + `

## Modes

This project follows a strict spec-driven workflow with human gates:

1. **Explore** — evaluate whether an idea is worth pursuing
2. **Spec** — define the problem and acceptance criteria (no code)
3. **Plan** — break the spec into implementation beads (no code)
4. **Implement** — write code against the approved plan
5. **Review** — verify implementation meets acceptance criteria

Transition between modes using ` + "`mindspec approve spec|plan`" + ` and ` + "`mindspec complete`" + `.

## Conventions

- Every functional change must reference a spec in ` + "`.mindspec/docs/specs/`" + `
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run ` + "`mindspec doctor`" + ` to verify project structure health
`

// agentsMDAppendBlock is appended to an existing AGENTS.md.
const agentsMDAppendBlock = `
## MindSpec

This project uses [MindSpec](https://github.com/mindspec/mindspec), a spec-driven development framework.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance.

### Build & Test

` + "```bash" + `
make build    # Build binary
make test     # Run all tests
` + "```" + `

### Modes

This project follows a strict spec-driven workflow with human gates:

1. **Explore** — evaluate whether an idea is worth pursuing
2. **Spec** — define the problem and acceptance criteria (no code)
3. **Plan** — break the spec into implementation beads (no code)
4. **Implement** — write code against the approved plan
5. **Review** — verify implementation meets acceptance criteria

Transition between modes using ` + "`mindspec approve spec|plan`" + ` and ` + "`mindspec complete`" + `.

### Conventions

- Every functional change must reference a spec in ` + "`.mindspec/docs/specs/`" + `
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run ` + "`mindspec doctor`" + ` to verify project structure health
`
