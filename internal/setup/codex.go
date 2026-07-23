package setup

import (
	"fmt"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/githooks"
)

// RunCodex sets up OpenAI Codex CLI integration at root.
// If check is true, reports what would be created without writing.
func RunCodex(root string, check bool) (*Result, error) {
	r := &Result{}

	// 1. AGENTS.md (create or append with marker)
	if err := ensureAgentsMD(root, check, r); err != nil {
		return nil, err
	}

	// 2. Skills (.agents/skills/<name>/SKILL.md) — create new, refresh
	// previously-shipped (provenance-gated), skip user-modified with a
	// notice (Reqs 18-19, HC-6).
	if err := installSkills(filepath.Join(root, ".agents", "skills"), filepath.Join(".agents", "skills"), skillFiles(), check, r); err != nil {
		return nil, err
	}

	// 3. Install/upgrade git hooks (pre-commit, post-checkout)
	if !check {
		if err := githooks.InstallAll(root); err != nil {
			return nil, fmt.Errorf("installing git hooks: %w", err)
		}
	}

	// 4. Optionally chain bd setup codex
	if !check {
		chainBeadsSetup(root, "codex", r)
	}

	// 5. Surface .beads/config.yaml drift via the shared helper so RunClaude,
	// RunCodex, and RunCopilot stay aligned on ordering and semantics.
	applyBeadsConfig(root, check, r)

	// 6. Ensure MindSpec's runtime files are gitignored (spec 123 R4b).
	if err := ensureGitignore(root, check, r); err != nil {
		return nil, err
	}

	return r, nil
}

// ensureAgentsMD creates or appends the MindSpec block to AGENTS.md. It routes
// through the shared ensureManagedDoc helper so every write (including the
// managed AGENTS.md document) goes through safeio and refuses symlinked targets.
func ensureAgentsMD(root string, check bool, r *Result) error {
	return ensureManagedDoc(root, "AGENTS.md", agentsMDFull, agentsMDAppendBlock, check, r)
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

Transition between modes using ` + "`mindspec spec approve`" + ` / ` + "`mindspec plan approve`" + ` / ` + "`mindspec impl approve`" + ` and ` + "`mindspec complete`" + `.

## Conventions

- Every functional change must reference a spec in ` + "`.mindspec/specs/`" + `
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run ` + "`mindspec doctor`" + ` to verify project structure health

## Bead-loop guardrails (mindspec)

The canonical authority for the autonomous bead loop. Surviving ` + "`/ms-*`" + ` skills reference this section instead of re-stating these rules.

### Orchestrator rules

- The cycle owns the merge: only the orchestrator runs ` + "`mindspec complete`" + `, and only after the panel gate passes.
- **Never merge a bead branch with raw ` + "`git merge bead/<id>`" + `** — only ` + "`mindspec complete`" + ` merges. Raw merge bypasses ` + "`bd`" + ` closure, worktree cleanup, AND the panel gate (no git hook fires on automatic merge commits, so raw merge is the obvious gate workaround).
- Do NOT ` + "`git push`" + ` after a bead merge — a single push at end-of-spec, after ` + "`/ms-impl-approve`" + `.

### Subagent prompt fences

Every impl/fix subagent prompt includes these verbatim:

- No ` + "`mindspec complete`" + `; no ` + "`git push`" + `.
- No exceeding the files-in-scope list; no reimplementing helpers earlier beads landed.
- Exactly ONE commit, ending with a ` + "`Deviations: <list or \"none\">`" + ` line.
- **Tests must PASS** — run the bead's test scope before reporting (a report-only bead is satisfied by faithfully reporting failures, not by hiding them).
- Report back: commit SHA + pass/fail/skip counts + deviations.
`

// agentsMDFull is written when AGENTS.md doesn't exist.
var agentsMDFull = "# AGENTS.md\n" + mindspecMarkerBegin + "\n" + agentsMDManagedBlock + mindspecMarkerEnd + "\n"

// agentsMDAppendBlock is the same managed content, used when appending.
var agentsMDAppendBlock = agentsMDManagedBlock
