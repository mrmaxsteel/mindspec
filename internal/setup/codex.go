package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/hooks"
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
		content := string(data)
		if strings.Contains(content, mindspecMarkerBegin) {
			updated := replaceManagedBlock(content, agentsMDAppendBlock)
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
				block := "\n" + mindspecMarkerBegin + "\n" + agentsMDAppendBlock + mindspecMarkerEnd + "\n"
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

// agentsMDManagedBlock is the canonical content placed between BEGIN/END markers.
const agentsMDManagedBlock = `
This project uses [MindSpec](https://github.com/mrmaxsteel/mindspec), a spec-driven development framework.

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

// agentsMDFull is written when AGENTS.md doesn't exist.
var agentsMDFull = "# AGENTS.md\n" + mindspecMarkerBegin + "\n" + agentsMDManagedBlock + mindspecMarkerEnd + "\n"

// agentsMDAppendBlock is the same managed content, used when appending.
var agentsMDAppendBlock = agentsMDManagedBlock
