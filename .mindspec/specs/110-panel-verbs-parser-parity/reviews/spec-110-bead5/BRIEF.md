# spec-110-bead5 — Round 1 Review Panel (8 reviewers)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.worktrees/worktree-mindspec-fbel.5`
**Branch**: `bead/mindspec-fbel.5`
**Commit under review**: `13788634a95b83380e1beadef747c9dcd17df90f` — `docs(110): thin ms-panel-run/tally skills onto the panel verbs [mindspec-fbel.5]`
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, R8 codex. **Pass = ≥7 APPROVE, no REJECT.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `13788634`; leave `git status` clean. Any scratch under ABSOLUTE /tmp only (never a relative `.mindspec/` write).

## What the work does (bead fbel.5 — delivers spec 110 R8, the skills half)
This is the FINAL bead of spec 110. It thins the `ms-panel-run` and `ms-panel-tally` skills: the mechanized prose (hand-typed `panel.json` schema, the `mkdir`, the round/SHA co-bump invariant, the decision-matrix table, the skill-authored verdict-JSON template) is REPLACED by references to the new `mindspec panel create|verify|tally` verbs (Beads 1–4, merged). The **judgment sections stay** (that's the L4 layer — orchestration + lens judgment that shouldn't mechanize). This is the payoff of the whole spec: skills stop re-implementing what the binary now owns.

Files changed (5, +74/−170): `plugins/mindspec/skills/ms-panel-run/SKILL.md`, `plugins/mindspec/skills/ms-panel-tally/SKILL.md`, their **byte-identical mirrors** `.claude/skills/ms-panel-run/SKILL.md` + `.claude/skills/ms-panel-tally/SKILL.md`, and doc-sync `.mindspec/domains/workflow/runbook.md`.

## What to verify (this is a docs/skills bead)
1. **Correct mechanization (ms-panel-run § Step 0)**: the `mkdir` + hand-typed `panel.json` schema + "capture reviewed_head_sha NOW / on re-panel bump round AND reviewed_head_sha in the SAME write" invariant prose are GONE, replaced by ONE `mindspec panel create <slug> --spec <id> --target <ref> [--bead <id>] [--round N]` invocation. The step-3→step-2 BRIEF composition is now "fill the stub `create` wrote" (Summary / Files in Scope / Prior-Round Asks / Lens headings). Confirm the invocation's flags/grammar match the ACTUAL `panel create` CLI (Bead 4) — build the binary if useful, or read `cmd/mindspec/panel.go`.
2. **The single-verdict-instruction invariant (load-bearing)**: the skill's "## Your job" / verdict-JSON output block (reviewer_id/verdict/confidence/rationale/concrete_changes_required/findings + the "hard_block: true" line) must be REMOVED from the skill-authored template — because Bead 1's `panel create` stub now writes it ONCE, machine-managed, inside the delimited header. Confirm there is now exactly ONE verdict-JSON instruction per BRIEF (the stub's), not a skill-re-authored second one that could drift. Cross-check the skill's description of the stub against `internal/panel/create.go`'s `briefStubBody`/`renderBriefHeader` — does the skill accurately describe what the binary actually writes?
3. **hard_block clarification**: the retired "an artifact-gate finding may set hard_block: true" phrasing — confirm the stub (per Bead 1) states `hard_block` is a TOP-LEVEL key (sibling of `verdict`), matching `internal/panel`'s `verdictJSON.HardBlock`, never nested in a `findings` entry.
4. **Judgment sections PRESERVED**: ms-panel-run must KEEP **Launch the panel**, **Codex failure detection**, **Working directory matters**, **Slot lens defaults**, **Anti-patterns**. ms-panel-tally must KEEP **Consolidate** (dedup/ranking), **Artifact gates**, **After a halt — recovery**, **Escape hatch**, **Anti-patterns**. Confirm nothing load-bearing was removed with the mechanization.
5. **Mirror byte-identity**: `diff -q plugins/mindspec/skills/<name>/SKILL.md .claude/skills/<name>/SKILL.md` must be identical for BOTH skills. Find and run the test/gate that asserts this (embed/SkillFiles or a mirror test).
6. **R8 grep gate**: the acceptance greps (verb references present; no `| Condition | Action |` table; no quoted `"reviewed_head_sha"`; no "artifact-gate finding may set"; has `## Artifact gates`, `Slot lens defaults`) pass on BOTH copies — run the exact one-liners from `bd show mindspec-fbel.5`.
7. **Doc-sync**: `runbook.md`'s new Maintenance Notes entry accurately describes the mechanization + which judgment sections survive.
8. **Fix-author judgment call**: the implementer lightly edited ms-panel-tally's intro paragraph (prose-quality, to reflect the mechanized matrix) and deliberately left the YAML frontmatter `description:` untouched (load-bearing for the skill catalog). Assess whether that's appropriate and grep-gate-neutral.

## Known pre-existing failure (NOT this bead)
`go test ./internal/instruct/...` fails only `TestRun_IdleNoBeads` (z4ps, cross-spec test-isolation leak). Not this diff (touches zero Go files).

## Per-slot lens defaults
- **R1 Opus** — author-of-record: diff matches plan Bead 5 / R8, no more no less.
- **R2 Opus** — codebase-pin: mirror byte-identity + grep gates + the referenced verbs/flags actually exist (Bead 4).
- **R3 Opus** — completeness / judgment-preservation: mechanized prose removed AND all judgment sections kept (item 4); nothing load-bearing lost.
- **R4 Sonnet** — empirical: does the skill's `panel create` invocation match the real CLI grammar? does the BRIEF-stub description match `create.go`'s actual stub output? (build to /tmp, run `panel create`, compare).
- **R5 Sonnet** — the single-verdict-instruction + hard_block invariants (items 2, 3): no skill-authored second verdict block; hard_block top-level-key accuracy vs `verdictJSON.HardBlock`.
- **R6 Sonnet** — operator-usability / integration: can an operator still run a panel correctly from the thinned skill? does it flow with the surviving judgment sections + the verb outputs (e.g. `panel create`'s `panel directory:` line from fbel.4)?
- **R7 Fable** — adversarial: hunt for a load-bearing invariant or safety instruction lost in the thinning; scrutinize the intro-paragraph edit + any drift between skill prose and actual binary behavior.
- **R8 codex** — empirical: run the exact R8 grep-gate one-liners on both copies; verify mirror identity; confirm the verbs/flags the skill references exist and behave as described.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
