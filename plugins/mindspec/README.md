# mindspec — autonomous spec-driven implementation, with multi-reviewer panels

A Claude Code skills package that takes a `mindspec`-approved plan and drives it to completion bead-by-bead, with a 6-reviewer (3 Claude + 3 Codex) panel gating each bead before merge. The orchestrator's main context stays small; every step is delegated to subagents.

Proven on `lola` (specs 044-050) across ~25 beads. The pattern reliably catches asymmetric defects — a Claude reviewer approving while a Codex reviewer empirically tests and finds the routing bug, or vice versa.

## What this plugin replaces

| Without | With |
|:--------|:-----|
| "Implement bead. Eyeball diff. Merge." | "Implement → 6-reviewer panel → fix → re-panel → merge" |
| Single-reviewer LLM gating | Mixed-family panel; family disagreement is the signal |
| Manual "what's the next ready bead?" | `/ms-bead-cycle` step 0 reads `bd ready` + the plan dep graph, claims, sets up the worktree |
| Manual fix-up after review | `/ms-bead-fix` consolidates verdicts and dispatches a fix subagent |
| Manual per-bead orchestration | `/ms-bead-cycle` runs the whole pick → impl → review → merge loop |
| Manual per-spec orchestration | `/ms-spec-autopilot` cycles every bead until the spec is done |

## Skills

Eleven skills total: four lifecycle gates + seven plugin skills. This is the
post-thin-down inventory (spec 093) — orchestration only; operational
knowledge (recovery recipes, gate semantics) lives in CLI point-of-use output
and hook messages, not in skill prose. Following the Anthropic agent-skills
pattern, each skill is a thin, composable orchestration unit.

### Spec lifecycle (defined in `internal/setup/claude.go::lifecycleSkillFiles()`)

| Skill | Purpose |
|:------|:--------|
| `/ms-spec-create` | Create a new specification (enters Spec Mode) |
| `/ms-spec-approve` | Approve spec → Plan Mode |
| `/ms-plan-approve` | Approve plan → Implementation Mode |
| `/ms-impl-approve` | Approve implementation → Idle |

These are the existing mindspec gating skills. The new skills in this plugin assume they're already wired. (Spec status is no longer a skill — run `mindspec state show` / `mindspec instruct` directly.)

### Bead lifecycle (new)

| Skill | Purpose |
|:------|:--------|
| `/ms-bead-impl` | Phase A stages the impl prompt (plan section + dep helper signatures); Phase B dispatches the implementation subagent |
| `/ms-bead-fix` | Dispatch a fix-up subagent with the consolidated change list |

### Review panel (new)

| Skill | Purpose |
|:------|:--------|
| `/ms-panel-run` | Step 0 writes the panel dir + BRIEF + `panel.json`; then launch 6 reviewers (3 Claude `Agent`s + 3 Codex CLI sessions) and collect verdicts |
| `/ms-panel-tally` | Single decision authority: decision matrix + N−1 threshold, artifact gates, consolidation, halt-recovery, escape hatch |

### Orchestrators (new)

| Skill | Purpose |
|:------|:--------|
| `/ms-bead-cycle` | Single bead end-to-end: pick + claim (step 0) → impl → panel → fix → re-panel → merge terminal (`mindspec complete`) |
| `/ms-spec-autopilot` | Whole spec: keep calling `/ms-bead-cycle` until no beads remain |
| `/ms-spec-final-review` | Final panel of the whole spec branch vs main, before `/ms-impl-approve` |

Pick + claim and merge are folded into `/ms-bead-cycle` (step 0 and the merge terminal); prompt-staging is folded into `/ms-bead-impl` (Phase A); panel-dir creation is folded into `/ms-panel-run` (step 0).

## The autonomous loop

`/ms-spec-autopilot` is the headline skill. It:

1. Calls `/ms-bead-cycle`, whose step 0 claims the next ready bead.
2. The cycle drives it through impl → panel → fix → merge.
3. Repeats until `bd ready` shows no spec-owned beads.
4. Calls `/ms-spec-final-review`, then `/ms-impl-approve` to close the spec.

Each cycle iteration:

```
step 0              → pick + claim next bead, create worktree
/ms-bead-impl       → implementation subagent commits to bead/<id>
/ms-panel-run       → step 0 writes BRIEF + panel.json at review/<panel>/; 6 reviewers fan out in parallel
/ms-panel-tally     → verdicts summarised; if ≥ N−1 APPROVE (5/6) → done
/ms-bead-fix        → consolidated changes → fix subagent → new commit
/ms-panel-run       → round 2 re-bumps round + reviewed_head_sha, verifies the fix
... iterate until APPROVE or max-rounds reached
merge terminal      → mindspec complete <bead-id> "<summary>" (hook-gated)
```

## Why six reviewers, mixed families

Single-LLM gating misses defects asymmetrically:

- Anthropic models lean narrative-coherent — they explain the diff well, but may miss empirical edge cases.
- OpenAI models lean empirical — they run validators and probe edge cases, but their natural-language synthesis is less reliable.

A six-slot panel with 3+3 lets you weight convergence: if all three Claudes APPROVE and all three Codex REQUEST_CHANGES, that's a different signal from a unanimous APPROVE. The orchestrator's tally pays attention to family asymmetry.

Empirically (lola, ~25 beads): unanimous APPROVE on round 1 is rare. Most beads need exactly one fix round. The reviewers that flag changes are usually one Claude and two Codex, or two Claudes and three Codex — different lenses landing on different defects.

## Configuration

- `codex` CLI on PATH (`codex exec --skip-git-repo-check`).
- `bd` (beads) for issue tracking.
- `mindspec` for bead lifecycle.
- Claude Code host with the `Agent` tool.

When codex hits its usage limit mid-panel, the orchestrator detects the empty/truncated output and substitutes a Claude `Agent` in the same slot (R4 → R4 claude-sub, etc.). Verdict comparability is preserved by keeping the slot name and writing the same JSON shape.

## Disk layout

```
<repo>/
  .mindspec/docs/specs/<spec-id>/
    spec.md
    plan.md
  review/
    prep/                                   # pre-staged impl prompts (optional)
      bead<N>_impl_prompt.md
    <panel-slug>/                           # one per panel round
      BRIEF.md
      claude-1-round-1.json
      ...
      codex-6-round-2.json                  # round 2 after fix-up
```

## Integration with mindspec core

As of 2026-06, the plugin's SKILL.md files are embedded into the `mindspec`
binary via `plugins/mindspec/embed.go` (a `//go:embed skills/*/SKILL.md`
block) and merged into `internal/setup/claude.go::skillFiles()` alongside
the 4 lifecycle gate skills. Every `mindspec setup <agent>` user gets the
full autonomous-loop skill set by default — no opt-in copy step. Setup
refreshes a mindspec-shipped skill file in place when its content
byte-matches a previously-shipped version, and removes retired skill dirs on
the same provenance check; a user-modified file is left untouched with a
notice (HC-6).

- **Lifecycle gate skills** are defined inline in
  `internal/setup/claude.go::lifecycleSkillFiles()` (the 4 spec lifecycle
  transitions). They win on key collision — they are the canonical
  authority.
- **Plugin skills** live here as on-disk SKILL.md files and are embedded
  via `pluginmindspec.SkillFiles()`. Editing the SKILL.md on this branch
  + rebuilding the binary is the iteration path; the on-disk copy under
  `plugins/mindspec/skills/` is the source of truth.

## Stop conditions for autopilot

`/ms-spec-autopilot` halts on any of:

- No more ready beads → `/ms-impl-approve` and exit.
- A REJECT verdict from any reviewer → halt; ask user (usually means the brief or plan needs work, not just a code fix).
- `max-rounds` exceeded on a bead (default 3) → halt with the bead in `in_progress`.
- Implementation subagent fails twice in a row on the same bead → halt.
- `bd dolt` push fails → halt before merging more beads.
