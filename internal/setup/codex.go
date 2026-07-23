package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/githooks"
	"github.com/mrmaxsteel/mindspec/internal/safeio"
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

// ensureAgentsMD creates or appends the MindSpec block to AGENTS.md. It
// loads .mindspec/config.yaml so the managed block is config-sourced
// (spec 123 R7). A config LOAD ERROR is propagated, never swallowed
// (spec 123 FX-1): an unrelated malformed key (e.g. `runner: typoo`)
// alongside a valid `commands.build` must NOT cause setup to silently
// rewrite AGENTS.md from a DefaultConfig fallback — that would ERASE the
// consumer's declared build guidance. Failing loudly leaves the existing
// managed block untouched, matching how every other setup step handles a
// bad config. (config.Load returns DefaultConfig with a nil error when
// the file is simply ABSENT — the ordinary greenfield case — so this
// only fires on a genuinely corrupt/invalid config.)
//
// It then heals a legacy leaked TITLE line if present (see
// healLegacyAgentsMDTitle — the managed BLOCK's own BEGIN/END
// replacement cannot reach a line living BEFORE the marker), and routes
// through the shared ensureManagedDoc helper so every write goes through
// safeio and refuses symlinked targets. ensureManagedDoc's wholesale
// BEGIN/END block replacement (topology-validated per FX-4) is what
// heals an already-onboarded consumer's leaked mindspec-build content on
// the next setup run (AC-14b).
func ensureAgentsMD(root string, check bool, r *Result) error {
	cfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("loading .mindspec/config.yaml for AGENTS.md build guidance (fix the config; setup will not overwrite AGENTS.md from defaults): %w", err)
	}
	block := agentsMDManagedBlock(cfg)
	full := "# AGENTS.md\n" + mindspecMarkerBegin + "\n" + block + mindspecMarkerEnd + "\n"

	if !check {
		if err := healLegacyAgentsMDTitle(root); err != nil {
			return err
		}
	}
	return ensureManagedDoc(root, "AGENTS.md", full, block, check, r)
}

// legacyBadAgentsMDTitle is the exact pre-spec-123 AGENTS.md title line
// (bootstrap.go's old starterAgentsMD, #211) that leaked mindspec-the-
// framework's own identity into every consuming repo's AGENTS.md.
const legacyBadAgentsMDTitle = "# AGENTS.md — MindSpec Project"

// healLegacyAgentsMDTitle rewrites an EXISTING AGENTS.md's first line
// from the exact legacy title to the neutral "# AGENTS.md" (spec 123
// AC-14b). ensureManagedDoc's BEGIN/END block replacement only ever
// touches content BETWEEN the markers, so the pre-123 title line —
// which sits BEFORE the marker — would otherwise survive every future
// setup refresh forever.
//
// PROVENANCE-GATED (spec 123 FX-3): the heal fires ONLY when the file
// ALSO carries a well-formed MindSpec managed pair — positive proof
// mindspec generated this file. Without that gate, a first-line EXACT
// match alone would clobber an operator who legitimately titled a
// NON-mindspec file "# AGENTS.md — MindSpec Project" (a mindspec fork,
// or a project genuinely named "MindSpec Project"). A no-op when
// AGENTS.md is absent, its first line does not match exactly, or it
// carries no (or malformed) managed markers.
func healLegacyAgentsMDTitle(root string) error {
	path := filepath.Join(root, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	content := string(data)
	firstLine, rest, found := strings.Cut(content, "\n")
	if !found || firstLine != legacyBadAgentsMDTitle {
		return nil
	}
	// FX-3 provenance gate: only heal a file mindspec demonstrably
	// generated (one ordered BEGIN-before-END managed pair). This also
	// composes with FX-4: on malformed markers neither the title heal
	// nor the block rewrite writes, so there is no half-healed file.
	if !hasWellFormedManagedMarkers(content) {
		return nil
	}
	return safeio.WriteFileNoSymlink(path, []byte("# AGENTS.md\n"+rest), 0o644)
}

// agentsMDBlockTemplate is the canonical content placed between
// BEGIN/END markers. Spec 123 R7(a) removed the mindspec-repo-specific
// `make build`/`make test` hardcode; the %s placeholder is filled by
// agentsMDManagedBlock from cfg.RenderBuildTestSection(2) — the
// CONSUMER's own declared commands, or omitted entirely when unset
// (ADR-0040's consumer-identity clause). Everything else (Modes,
// Conventions, the Bead-loop guardrails section) is unchanged —
// framework-generic guidance that was never mindspec-repo-specific.
const agentsMDBlockTemplate = `
This project uses [MindSpec](https://github.com/mrmaxsteel/mindspec), a spec-driven development framework.

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance.
%s
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

// agentsMDManagedBlock renders the managed AGENTS.md content, config-
// sourced per spec 123 R7: framework-generic guidance plus the
// consumer's declared Build & Test commands (cfg.Commands) — never
// mindspec-the-framework's own (ADR-0040's consumer-identity clause).
func agentsMDManagedBlock(cfg *config.Config) string {
	return fmt.Sprintf(agentsMDBlockTemplate, cfg.RenderBuildTestSection(2))
}
