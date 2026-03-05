package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	mindspecMarkerBegin  = "<!-- BEGIN mindspec:managed -->"
	mindspecMarkerEnd    = "<!-- END mindspec:managed -->"
	mindspecMarkerLegacy = "<!-- mindspec:managed -->"
)

// Result tracks what the init operation created or skipped.
type Result struct {
	Created  []string
	Appended []string
	Skipped  []string
	BeadsOK  bool // true if bd/beads found in PATH
}

// FormatSummary returns a human-readable summary of the init result.
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

	if len(r.Appended) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Appended MindSpec block:\n")
		for _, p := range r.Appended {
			sb.WriteString("  ~ ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if len(r.Skipped) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Skipped (already exist):\n")
		for _, p := range r.Skipped {
			sb.WriteString("  - ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if !r.BeadsOK {
		sb.WriteString("\nNote: 'bd' (Beads CLI) not found in PATH.\n")
		sb.WriteString("  Install Beads and run 'beads init' to enable task tracking.\n")
		sb.WriteString("  MindSpec works without Beads but the full workflow requires it.\n")
	}

	sb.WriteString("\nNext steps:\n")
	sb.WriteString("  mindspec setup claude    # Configure Claude Code integration\n")
	sb.WriteString("  mindspec setup copilot   # Configure GitHub Copilot integration\n")

	return sb.String()
}

// Run bootstraps a MindSpec project at root. If dryRun is true, no files are
// written — the result shows what would be created.
func Run(root string, dryRun bool) (*Result, error) {
	r := &Result{}

	// Check for Beads CLI
	r.BeadsOK = checkBeadsCLI()

	for _, item := range manifest() {
		target := filepath.Join(root, item.path)

		if item.isDir {
			if dirExists(target) {
				r.Skipped = append(r.Skipped, item.path+"/")
				continue
			}
			r.Created = append(r.Created, item.path+"/")
			if !dryRun {
				if err := os.MkdirAll(target, 0755); err != nil {
					return nil, fmt.Errorf("creating %s: %w", item.path, err)
				}
			}
		} else {
			if fileExists(target) {
				// If this item supports appending, check for the marker
				if item.appendBlock != "" {
					existing, err := os.ReadFile(target)
					if err != nil {
						return nil, fmt.Errorf("reading %s: %w", item.path, err)
					}
					content := string(existing)
					if strings.Contains(content, mindspecMarkerBegin) || strings.Contains(content, mindspecMarkerLegacy) {
						r.Skipped = append(r.Skipped, item.path+" (MindSpec block present)")
					} else {
						r.Appended = append(r.Appended, item.path)
						if !dryRun {
							block := "\n" + mindspecMarkerBegin + "\n" + item.appendBlock + mindspecMarkerEnd + "\n"
							f, err := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0644)
							if err != nil {
								return nil, fmt.Errorf("appending to %s: %w", item.path, err)
							}
							_, writeErr := f.WriteString(block)
							closeErr := f.Close()
							if writeErr != nil {
								return nil, fmt.Errorf("writing to %s: %w", item.path, writeErr)
							}
							if closeErr != nil {
								return nil, fmt.Errorf("closing %s: %w", item.path, closeErr)
							}
						}
					}
				} else {
					r.Skipped = append(r.Skipped, item.path)
				}
				continue
			}
			r.Created = append(r.Created, item.path)
			if !dryRun {
				// Ensure parent dir exists
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return nil, fmt.Errorf("creating parent for %s: %w", item.path, err)
				}
				content := item.content
				if item.contentFunc != nil {
					content = item.contentFunc()
				}
				if err := os.WriteFile(target, []byte(content), 0644); err != nil {
					return nil, fmt.Errorf("writing %s: %w", item.path, err)
				}
			}
		}
	}

	return r, nil
}

type manifestItem struct {
	path        string
	isDir       bool
	content     string
	contentFunc func() string // lazy content (e.g. timestamp)
	appendBlock string        // if set, append this block to existing files (idempotent via marker)
}

func manifest() []manifestItem {
	items := []manifestItem{
		// Required directories
		{path: ".mindspec", isDir: true},
		{path: ".mindspec/docs/domains", isDir: true},
		{path: ".mindspec/docs/specs", isDir: true},

		// Root files
		{path: "AGENTS.md", content: starterAgentsMD, appendBlock: appendAgentsBlock},
		{path: "CLAUDE.md", content: starterClaudeMD, appendBlock: appendClaudeBlock},
		{path: ".github/copilot-instructions.md", content: starterCopilotInstructionsMD, appendBlock: appendCopilotBlock},
		// Gitignore: session.json and focus are local runtime files, not version-controlled
		{path: ".gitignore", content: starterGitignore},
	}

	return items
}

func checkBeadsCLI() bool {
	_, err := exec.LookPath("bd")
	if err == nil {
		return true
	}
	_, err = exec.LookPath("beads")
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// starterGitignore ensures session.json and focus are gitignored in new projects.
const starterGitignore = `# MindSpec local runtime files (not version-controlled)
.mindspec/session.json
.mindspec/focus
`

// --- Starter file content ---

const starterAgentsMD = `# AGENTS.md — MindSpec Project
<!-- BEGIN mindspec:managed -->

This project uses [MindSpec](https://github.com/mindspec/mindspec), a spec-driven development framework.

## Workflow

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance.

## Build & Test

` + "```bash" + `
make build    # Build binary
make test     # Run all tests
` + "```" + `

## Modes

This project follows a strict spec-driven workflow with human gates:

1. **Spec** — define the problem and acceptance criteria (no code)
2. **Plan** — break the spec into implementation beads (no code)
3. **Implement** — write code against the approved plan
4. **Review** — verify implementation meets acceptance criteria

Transition between modes using ` + "`mindspec approve spec|plan`" + ` and ` + "`mindspec complete`" + `.

## Conventions

- Every functional change must reference a spec in ` + "`.mindspec/docs/specs/`" + `
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run ` + "`mindspec doctor`" + ` to verify project structure health
<!-- END mindspec:managed -->
`

const starterClaudeMD = `# CLAUDE.md
<!-- BEGIN mindspec:managed -->

See [AGENTS.md](AGENTS.md) for project conventions shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Skills

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-spec-create`" + ` | Create a new specification (enters Spec Mode) |
| ` + "`/ms-spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms-plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms-impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/ms-spec-status`" + ` | Check current mode and active spec/bead state |
<!-- END mindspec:managed -->
`

// appendAgentsBlock is appended to an existing AGENTS.md when the marker is absent.
const appendAgentsBlock = `
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

1. **Spec** — define the problem and acceptance criteria (no code)
2. **Plan** — break the spec into implementation beads (no code)
3. **Implement** — write code against the approved plan
4. **Review** — verify implementation meets acceptance criteria

Transition between modes using ` + "`mindspec approve spec|plan`" + ` and ` + "`mindspec complete`" + `.

### Conventions

- Every functional change must reference a spec in ` + "`.mindspec/docs/specs/`" + `
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run ` + "`mindspec doctor`" + ` to verify project structure health
`

// appendClaudeBlock is appended to an existing CLAUDE.md when the marker is absent.
// When appended, it is wrapped with BEGIN/END markers by Run().
const appendClaudeBlock = `
## MindSpec

See [AGENTS.md](AGENTS.md) for project conventions shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

### Skills

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-spec-create`" + ` | Create a new specification (enters Spec Mode) |
| ` + "`/ms-spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms-plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms-impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/ms-spec-status`" + ` | Check current mode and active spec/bead state |
`

// starterCopilotInstructionsMD is written when .github/copilot-instructions.md doesn't exist.
const starterCopilotInstructionsMD = `# Copilot Instructions
<!-- BEGIN mindspec:managed -->

See [AGENTS.md](../AGENTS.md) for project conventions shared across all coding agents.

On session start, run ` + "`mindspec instruct`" + ` in the terminal for mode-appropriate operating guidance.

## Skills

MindSpec workflow skills are available in ` + "`.agents/skills/`" + `. Each skill directory contains a ` + "`SKILL.md`" + ` with instructions.
<!-- END mindspec:managed -->
`

// appendCopilotBlock is appended to an existing copilot-instructions.md when the marker is absent.
// When appended, it is wrapped with BEGIN/END markers by Run().
const appendCopilotBlock = `
## MindSpec

See [AGENTS.md](../AGENTS.md) for project conventions shared across all coding agents.

On session start, run ` + "`mindspec instruct`" + ` in the terminal for mode-appropriate operating guidance.

### Skills

MindSpec workflow skills are available in ` + "`.agents/skills/`" + `. Each skill directory contains a ` + "`SKILL.md`" + ` with instructions.
`
