package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/domain"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/safeio"
)

const (
	mindspecMarkerBegin  = "<!-- BEGIN mindspec:managed -->"
	mindspecMarkerEnd    = "<!-- END mindspec:managed -->"
	mindspecMarkerLegacy = "<!-- mindspec:managed -->"
)

// Result tracks what the init operation created or skipped.
type Result struct {
	Created      []string
	Appended     []string
	Skipped      []string
	BeadsOK      bool               // true if bd/beads found in PATH
	BeadsConfig  *bead.ConfigResult // result of EnsureBeadsConfig (or ScanBeadsConfig in dry-run), nil if .beads/ absent
	BeadsScan    bool               // true when BeadsConfig came from a read-only scan (dry-run)
	BeadsConfErr error              // non-nil if the beads-config step failed (non-fatal)
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

	if r.BeadsConfig != nil {
		if summary := r.BeadsConfig.FormatSummary(); summary != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			if r.BeadsScan {
				// Tell the user this is a preview, not a completed action —
				// otherwise the "+" bullets read like "we added these keys"
				// when actually nothing was written.
				sb.WriteString("Beads config (dry-run preview — no writes):\n")
				// FormatSummary prints its own "Beads config …:" header, so
				// trim the helper's header line to avoid a duplicate.
				sb.WriteString(trimFirstLine(summary))
			} else {
				sb.WriteString(summary)
			}
		}
	}
	if r.BeadsConfErr != nil {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "Beads config: %v\n", r.BeadsConfErr)
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

	// Surface .beads/config.yaml drift. In normal mode we mutate (via
	// EnsureBeadsConfig); in dry-run we scan (via ScanBeadsConfig) so users
	// still see what would change without touching disk. Either way, failures
	// are reported but not fatal so a broken beads config doesn't block the
	// rest of the init flow.
	if bead.HasBeadsDir(root) {
		var cr *bead.ConfigResult
		var err error
		if dryRun {
			cr, err = bead.ScanBeadsConfig(root)
			r.BeadsScan = true
		} else {
			cr, err = bead.EnsureBeadsConfig(root, false)
		}
		if err != nil {
			r.BeadsConfErr = err
		} else {
			r.BeadsConfig = cr
		}
	}

	// Spec 123 R7(d): load .mindspec/config.yaml so a fresh init's
	// AGENTS.md is config-sourced exactly like setup's (the FR-3
	// asymmetry guard, AC-14) — never mindspec's own hardcoded build.
	// config.Load returns DefaultConfig with a nil error for the ordinary
	// greenfield case where .mindspec/config.yaml does not exist yet; a
	// genuinely corrupt/invalid existing config is propagated (spec 123
	// FX-1) rather than silently rendering AGENTS.md from a DefaultConfig
	// fallback, matching how every other mindspec command handles a bad
	// config. init never overwrites an existing AGENTS.md managed block
	// (the manifest is create-or-append-only), so there is no data-loss
	// path here, but failing loudly on a corrupt config keeps init's
	// generated content honest and consistent with setup.
	cfg, err := config.Load(root)
	if err != nil {
		return nil, fmt.Errorf("loading .mindspec/config.yaml for init's AGENTS.md build guidance (fix the config and re-run init): %w", err)
	}

	for _, item := range manifest(cfg) {
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
							f, err := safeio.OpenAppendNoSymlink(target, 0644)
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
				if err := safeio.WriteFileNoSymlink(target, []byte(content), 0644); err != nil {
					return nil, fmt.Errorf("writing %s: %w", item.path, err)
				}
			}
		}
	}

	// Provision the beads jsonl merge driver so a fresh repo merges
	// both-sides-changed .beads/issues.jsonl cleanly from commit 0
	// (mindspec-oe0u, ADR-0025). Wired AFTER the manifest write loop.
	if err := provisionBeadsMergeDriver(r, root, dryRun); err != nil {
		return nil, err
	}

	// Spec 123 R4a (#208): even when .gitignore already existed (the
	// manifest item above is Skipped in that case — it carries no
	// appendBlock), still ensure the two runtime entries are present, via
	// the entry-granular gitutil helper that never reorders or rewrites
	// existing bytes. This is a true no-op when the entries are already
	// there — including immediately after the greenfield create-from-
	// scratch write above, so it never double-writes a fresh starterGitignore.
	if !dryRun {
		if err := gitutil.EnsureGitignoreEntries(root, gitutil.RuntimeIgnoreEntries...); err != nil {
			return nil, fmt.Errorf("ensuring .gitignore runtime entries: %w", err)
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

func manifest(cfg *config.Config) []manifestItem {
	items := []manifestItem{
		// Required directories — NEW (greenfield) projects are born FLAT
		// (Req 2 / AC4): lifecycle artifacts live directly under .mindspec/,
		// so DetectLayout classifies a freshly bootstrapped tree `flat`.
		{path: ".mindspec", isDir: true},
		{path: ".mindspec/domains", isDir: true},
		{path: ".mindspec/specs", isDir: true},

		// Root files. AGENTS.md is config-sourced (spec 123 R7): the
		// Build & Test section renders cfg.Commands when populated and
		// is omitted entirely when unset — never mindspec's own build.
		{path: "AGENTS.md", content: renderStarterAgentsMD(cfg), appendBlock: renderAppendAgentsBlock(cfg)},
		{path: "CLAUDE.md", content: starterClaudeMD, appendBlock: appendClaudeBlock},
		{path: ".github/copilot-instructions.md", content: starterCopilotInstructionsMD, appendBlock: appendCopilotBlock},
		// Gitignore: session.json and focus are local runtime files, not version-controlled
		{path: ".gitignore", content: starterGitignore},
		// Spec 123 R1 (#207): the context-map skeleton, so the very first
		// `domain add` has a "## Bounded Contexts" section to insert into
		// instead of erroring "reading context map". Create-only, like every
		// other manifest item — never overwrites an existing context-map.md.
		{path: ".mindspec/context-map.md", contentFunc: domain.ContextMapSkeleton},
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

// trimFirstLine drops the first line of s (including its trailing newline).
// Used to splice our own dry-run header in front of ConfigResult.FormatSummary
// without a duplicate header line.
func trimFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
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

// starterAgentsMDTemplate is the full-doc AGENTS.md content `mindspec
// init` writes for a fresh (non-existing) file. Spec 123 R7(a) rewrote
// this from a mindspec-repo-specific document (title "MindSpec
// Project", hardcoded `make build`/`make test`) into a CONSUMER-generic
// one: neutral title, and a %s placeholder for the Build & Test section
// that renderStarterAgentsMD fills from cfg.RenderBuildTestSection(2) —
// populated-and-rendered or omitted entirely, never mindspec's own
// build (ADR-0040's consumer-identity clause).
const starterAgentsMDTemplate = `# AGENTS.md
<!-- BEGIN mindspec:managed -->

This project uses [MindSpec](https://github.com/mrmaxsteel/mindspec), a spec-driven development framework.

## Workflow

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance.
%s
## Modes

This project follows a strict spec-driven workflow with human gates:

1. **Spec** — define the problem and acceptance criteria (no code)
2. **Plan** — break the spec into implementation beads (no code)
3. **Implement** — write code against the approved plan
4. **Review** — verify implementation meets acceptance criteria

Transition between modes using ` + "`mindspec approve spec|plan`" + ` and ` + "`mindspec complete`" + `.

## Conventions

- Every functional change must reference a spec in ` + "`.mindspec/specs/`" + `
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run ` + "`mindspec doctor`" + ` to verify project structure health
<!-- END mindspec:managed -->
`

// renderStarterAgentsMD renders the full-doc AGENTS.md content for a
// fresh file, config-sourced per spec 123 R7(a)/(b).
func renderStarterAgentsMD(cfg *config.Config) string {
	return fmt.Sprintf(starterAgentsMDTemplate, cfg.RenderBuildTestSection(2))
}

const starterClaudeMD = `# CLAUDE.md
<!-- BEGIN mindspec:managed -->

See [AGENTS.md](AGENTS.md) for project conventions shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Skills

### Spec lifecycle gates

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-spec-create`" + ` | Create a new specification (enters Spec Mode) |
| ` + "`/ms-spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms-plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms-impl-approve`" + ` | Approve implementation → Idle |

### Bead lifecycle

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-bead-impl`" + ` | Stage the impl prompt (Phase A) + dispatch the subagent (Phase B) |
| ` + "`/ms-bead-fix`" + ` | Dispatch a fix-up subagent with the consolidated change list |

### Review panel

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-panel-run`" + ` | Launch 6 reviewers and collect verdicts |
| ` + "`/ms-panel-tally`" + ` | Single decision authority: decision matrix, artifact gates, consolidation |

### Orchestrators

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-bead-cycle`" + ` | Single bead end-to-end: pick+claim → impl → panel → fix → re-panel → merge |
| ` + "`/ms-spec-autopilot`" + ` | Whole spec: cycle every bead until the spec is done |
| ` + "`/ms-spec-final-review`" + ` | Final panel of the whole spec branch vs main, before ` + "`/ms-impl-approve`" + ` |
<!-- END mindspec:managed -->
`

// appendAgentsBlockTemplate is appended to an existing AGENTS.md when the
// marker is absent — config-sourced identically to
// starterAgentsMDTemplate (spec 123 R7), just nested (H3) under the
// pre-existing document's own top-level heading.
const appendAgentsBlockTemplate = `
## MindSpec

This project uses [MindSpec](https://github.com/mrmaxsteel/mindspec), a spec-driven development framework.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance.
%s
### Modes

This project follows a strict spec-driven workflow with human gates:

1. **Spec** — define the problem and acceptance criteria (no code)
2. **Plan** — break the spec into implementation beads (no code)
3. **Implement** — write code against the approved plan
4. **Review** — verify implementation meets acceptance criteria

Transition between modes using ` + "`mindspec approve spec|plan`" + ` and ` + "`mindspec complete`" + `.

### Conventions

- Every functional change must reference a spec in ` + "`.mindspec/specs/`" + `
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run ` + "`mindspec doctor`" + ` to verify project structure health
`

// renderAppendAgentsBlock renders the nested (H3) MindSpec block used
// when AGENTS.md already exists without a managed marker, config-sourced
// per spec 123 R7(a)/(b).
func renderAppendAgentsBlock(cfg *config.Config) string {
	return fmt.Sprintf(appendAgentsBlockTemplate, cfg.RenderBuildTestSection(3))
}

// appendClaudeBlock is appended to an existing CLAUDE.md when the marker is absent.
// When appended, it is wrapped with BEGIN/END markers by Run().
const appendClaudeBlock = `
## MindSpec

See [AGENTS.md](AGENTS.md) for project conventions shared across all coding agents.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

### Skills

#### Spec lifecycle gates

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-spec-create`" + ` | Create a new specification (enters Spec Mode) |
| ` + "`/ms-spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/ms-plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/ms-impl-approve`" + ` | Approve implementation → Idle |

#### Bead lifecycle

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-bead-impl`" + ` | Stage the impl prompt (Phase A) + dispatch the subagent (Phase B) |
| ` + "`/ms-bead-fix`" + ` | Dispatch a fix-up subagent with the consolidated change list |

#### Review panel

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-panel-run`" + ` | Launch 6 reviewers and collect verdicts |
| ` + "`/ms-panel-tally`" + ` | Single decision authority: decision matrix, artifact gates, consolidation |

#### Orchestrators

| Skill | Purpose |
|:------|:--------|
| ` + "`/ms-bead-cycle`" + ` | Single bead end-to-end: pick+claim → impl → panel → fix → re-panel → merge |
| ` + "`/ms-spec-autopilot`" + ` | Whole spec: cycle every bead until the spec is done |
| ` + "`/ms-spec-final-review`" + ` | Final panel of the whole spec branch vs main, before ` + "`/ms-impl-approve`" + ` |
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
