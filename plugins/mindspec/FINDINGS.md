# Plugin Review Findings + Claude Code Workflows Research

Captured during the spec-050 (lola) session that wrote the plugin and ran it live against four beads (Bead 2 round-2 fix, Bead 3 round-1, Bead 4 round-1). This document is the punch list of gaps + open questions; the README is the polished view.

## Closed in this PR (spec-050 followups)

Branch: `feat/mindspec-plugin-spec050-followups` — closes 4 of the 10 documented gaps.

| # | Finding | Commit | Status |
|:--|:--------|:-------|:-------|
| 3 | No `/ms-bead-prep` skill for drafting impl prompts | `f823476` | CLOSED |
| 4 | Codex-substitution logic is prose, not deterministic | `c8ef5c8` | CLOSED |
| 11 (new) | Plugin SKILL.md files not embedded in `skillFiles()` — projects had to opt in by copying skills/ | `4562985` | CLOSED |
| 12 (new) | `MINDSPEC_ALLOW_MAIN=1` escape hatch undocumented in `/ms-bead-fix` + `/ms-spec-final-review` | `a85f432` | CLOSED |
| 13 (new) | Item-4 codex-detection used wrong signal (`"Output JSON to"` is prompt echo, not write confirmation) | `64b8eec` | CLOSED |
| 14 (new) | Codex launch double-backgrounded (`&` + `run_in_background: true`) — task-notification fired on bash-exit not codex-exit | `777deff` | CLOSED |

Items 1, 2, 5-10 below remain OPEN and are out of scope for this PR. bd issue: `mindspec-ch8h`.

## Part 1 — Plugin self-review

### Gaps surfaced while writing the skills

1. **`/ms-spec-autopilot` `parallel-window: true` is documented but hand-wavy.**
   The dep-graph-aware fan-out logic in the SKILL.md says "find all beads whose blockers are now satisfied" and "pick beads in disjoint file-sets only" but doesn't define the actual algorithm. In practice on lola spec-050 the dep graph is a near-linear chain (1 → 2 → 3 → 4 → 5 → 6 → 8a → 8b, with 7 forking off 4 and 9 forking off 8a), so the parallel window is small and the serial path wins. The skill should either:
   - Remove the parallel-window option as YAGNI, or
   - Add a worked example of when fan-out actually pays off (likely specs that branch into multiple feature surfaces, not migration chains).

2. **No `/ms-panel-halt-recover` for stalled cycles.**
   `/ms-panel-tally` says "halt and ask the user" on REJECT or repeated halts, but doesn't define the resumption path. After a halt, the bead is in `in_progress`, the panel has stale verdicts, and the next session needs to either:
   - Pick up from the latest commit and re-launch the next round, or
   - Roll back to a known-good commit and start over.
   Should be a skill so the resumption path is reproducible.

3. **No `/ms-bead-prep` skill for drafting impl prompts.** **[CLOSED — commit `f823476`]**
   The pre-staged prompts at `<repo>/review/prep/bead<N>_impl_prompt.md` are referenced by `/ms-bead-impl` as "if absent, draft one in-conversation". In practice good prompts are the single biggest lever on impl-subagent quality — they should have their own skill that reads the plan section + spec context + prior-bead helper signatures and produces a structured prompt. The lola session benefited massively from pre-staged prompts; the plugin doesn't tell you how to write them.

4. **Codex-substitution logic is prose, not deterministic.** **[CLOSED — commit `c8ef5c8`]**
   `/ms-panel-run` says "if output is empty or just the prompt echoed, substitute a Claude `Agent` in the same slot". In practice the detection signal is `"ERROR: You've hit your usage limit"` in the codex output file. Should be a one-liner: `grep -q "ERROR: You've hit your usage limit" /tmp/codex_*.out && launch_claude_sub`. The plugin should encode this as a check, not as advice.

### Gaps surfaced while running the skills live

5. **`bd` Dolt Error 1105 blocks any event-recording update when the bead description is large.**
   Bead 4's `bd update --claim` failed with `Error 1105: string '{"id":"lola-8gbp.4",...` (description exceeds `events.old_value` column). Workaround: skip the bd state update, claim manually via `git worktree add`, rely on `mindspec complete` at merge time. Should be:
   - An upstream beads/mindspec fix (resize the column), and/or
   - A plugin skill that detects 1105 and falls back automatically without erroring.

6. **`mindspec complete` ADR-divergence on `.gitignore` consistently across beads.**
   Beads 2, 3, and 4 all hit `[adr-divergence-unowned] file .gitignore is not claimed by any OWNERSHIP.yaml`. Each time the workaround was `--override-adr ".gitignore root-level entries are repo-wide infra, not domain-owned"`. Either:
   - The plugin should pre-add the override, or
   - The lola repo should add a `.gitignore` OWNERSHIP claim (probably under a generic `infra` domain), or
   - Mindspec should treat root-level dotfiles as repo-infra-by-default.
   Filing the root cause upstream is the right fix; documenting the override in `/ms-bead-merge` is the short-term cope.

7. **`bd ready` and the plan dep graph disagree.**
   Bead 5 showed in `bd ready` before Bead 4 merged because the plan dep ("Beads 1-4") wasn't reflected in bd's explicit dep edges. Fixed manually with `bd dep add lola-8gbp.5 lola-8gbp.4`. The plan-to-bd dep-graph sync should be automatic — either `mindspec plan approve` should walk the plan's `**Depends on:**` lines and call `bd dep add` for each, or `/ms-bead-next` should refuse to claim a bead whose plan deps don't match bd.

8. **`mindspec instruct` (SessionStart hook) doesn't surface panel/subagent state.**
   The hook prints "Multiple Active Specs" with phases, but doesn't tell a fresh post-compaction session which beads have in-flight implementation subagents or pending panel rounds. Cost us several minutes after compaction to reconstruct state from `review/<panel>/*.json` and `git worktree list`. Should add a `--panel-state` block to `mindspec instruct` that lists:
   - In-progress beads (worktree + last commit on bead branch)
   - Open panel rounds (`review/<panel-slug>/` with mismatched verdict-file count vs expected)
   - Any locked agent worktrees (`.claude/worktrees/agent-*` still in place)

### Underdocumented decisions

9. **Why 6 reviewers specifically.** The README says "mixed families catch different defects" but doesn't justify 6 vs 4 or 8. Empirically on lola: ≥5 covers comfortable majority-vote; <5 has too much variance (one reviewer flagging changes is a 33% reroute, vs 20% at 5/6). But that's an empirical claim, not a derived one. Could be documented as "tunable, 6 was the sweet spot for our compute budget".

10. **Why round-2 panels and not just one-shot review.** The pattern "fix → re-review" is described in `/ms-bead-cycle` but the rationale isn't pinned. In practice the round-2 panel catches fix-author deviations the original BRIEF couldn't predict (the lola Bead 2 r2 panel found a routing bug the r1 panel approved). Should be a sidebar in the README.

### Gaps surfaced while shipping spec-050 (added during this PR)

11. **Plugin SKILL.md files not embedded in `mindspec` setup binary.** **[CLOSED — commit `4562985`]**
    Before this PR, `internal/setup/claude.go::skillFiles()` shipped only the 5 lifecycle gate skills inline. The 11 bead/panel/orchestrator skills under `plugins/mindspec/skills/` were opt-in — projects had to copy the skills/ tree into `.claude/skills/` manually. Closed by adding `plugins/mindspec/embed.go` with a `//go:embed skills/*/SKILL.md` block and merging `pluginmindspec.SkillFiles()` into `skillFiles()`; lifecycle skills win on key collision. Every `mindspec setup <agent>` user now gets the full autonomous loop out of the box.

12. **`MINDSPEC_ALLOW_MAIN=1` escape hatch undocumented.** **[CLOSED — commit `a85f432`]**
    The implement-mode commit gate blocks direct commits on `spec/<slug>` and `bead/<id>` branches as a scope-creep guardrail. Final-review fix-ups (panel-driven chore commits that intentionally land on the spec branch — PR-body precision corrections, stray-file reverts, CI-unblocking test fixes) hit the gate. The env-var escape hatch existed but wasn't documented. Surfaced by lola spec-050 commits `1bb9751` and `04d26f5`. Closed by adding a "Working around the implement-mode commit gate" section to `/ms-bead-fix/SKILL.md` and `/ms-spec-final-review/SKILL.md`.

13. **Item-4's deterministic codex-failure check used the wrong signal.** **[CLOSED — commit `64b8eec`]**
    `c8ef5c8` checked for `"Output JSON to"` in the codex `.out` log as the healthy-ack signal, but that string is the codex echoing back the prompt — it appears even if codex never writes the file. Reported by user in a follow-up session: "codex finished thinking but didn't write to disk. Extracting verdicts from the .out logs." Fixed by replacing the single-grep with three-layer detection (file-exists primary, attempt-marker diagnostic, log-extraction recovery), strengthening the prompt template's write-or-error instruction, documenting the codex sandbox workdir convention, and shipping a `codex_verdict_extract.sh` helper.

14. **Codex launches double-backgrounded.** **[CLOSED — commit `777deff`]**
    The launch pattern `nohup bash -c '... codex exec ...' &` combined with `run_in_background: true` puts codex into two background layers. Bash exits ~immediately after `&` returns; the Claude Code task-notification fires on bash-exit, not codex-exit. The orchestrator sees "completed in 1 sec, output file empty" and falsely concludes codex failed.

    This is the root cause of the "codex hit usage limit" interpretations throughout spec-050 panels that turned out to have empty output but no `usage limit` error string. The orchestrator was reading the file before codex finished writing to it.

    User-reported in a third-spec session: "my codex launches use & inside the command AND run_in_background: true on the Bash tool. That double-backgrounds them so the task-notification fires immediately when bash exits, not when codex finishes. That's why I never got R4/R5/R6 done-notifications."

    Fixed by dropping `&` and `nohup` from the launch example in `ms-panel-run/SKILL.md`, using only the Bash tool's `run_in_background: true`. Added an explicit anti-pattern callout. Confirms item 13's three-layer detection works correctly when the notification timing is honest.

## Part 2 — Claude Code "Workflows" research

### What exists (May 2026 research preview)

**Claude Code Dynamic Workflows** is a real feature:
- JavaScript runtime that orchestrates subagents at scale (up to 1000 per run, 16 concurrent).
- Claude writes the orchestration script; runtime executes it in the background while the session stays responsive.
- Invoked via `/deep-research` (bundled), the `ultracode` keyword, or custom commands saved alongside other slash commands.
- **Distinct from skills/hooks/subagents**: skills are passive instruction sets, hooks are tool-call interceptors, subagents are ephemeral workers, workflows are *executable orchestration plans written as code*.
- Docs: `code.claude.com/docs/en/workflows.md`, `claude.com/blog/introducing-dynamic-workflows-in-claude-code`.

### Where it does and doesn't fit this plugin

| Plugin component | Workflow fit | Verdict |
|:-----------------|:-------------|:--------|
| `/ms-panel-run` | Bad — workflows are opaque mid-run; we need to detect codex usage-limit failures and swap in claude-subs reactively. | **Stay with skills.** |
| `/ms-bead-impl` | Neutral — could run as a workflow but a single dispatched `Agent` already does the job; no benefit. | **Stay with skills.** |
| `/ms-bead-cycle` | Possible — the impl → panel → fix → re-panel loop maps to a workflow. But we lose mid-cycle deviation visibility. | **Stay with skills** for now; revisit if the cycle stabilises. |
| `/ms-spec-autopilot` | **Good fit.** The top-level loop currently runs in the main session and dies on context compaction (we saw this happen mid-spec-050). A workflow could run it in the background indefinitely. | **Future port.** |

### Other Claude Code primitives not yet used

| Primitive | What it'd buy us | Priority |
|:----------|:-----------------|:---------|
| **`PreToolUse` hook on `mindspec complete`** | Declaratively enforce "≥5/6 APPROVE before merge"; no orchestrator skill can bypass it. | **High** — gate enforcement should be a contract, not a convention. |
| **`Monitor` on codex output files** | Detect usage-limit / hang within seconds instead of after timeout; swap in claude-sub immediately. | **Medium** — saves 5-10 min per failed codex slot. |
| **Existing `SessionStart` hook + extended `mindspec instruct`** | Surface panel/subagent state on fresh sessions; recover from compaction faster. | **Medium** — cheap, high-leverage. |
| **`/deep-research` and `ultracode` workflows** | Port `/ms-spec-autopilot` to a durable background workflow for large specs. | **Low** — premature until a spec actually has 50+ beads. |

### Building Effective Agents pattern map

The plugin's mechanics are an implicit composition of the Anthropic patterns. Naming them in the README signals to other Claude Code users why the plugin works:

| Pattern | Plugin component |
|:--------|:----------------|
| Parallelization | 6-reviewer panel — independent subtasks, voting aggregator |
| Orchestrator-workers | `/ms-spec-autopilot` + `/ms-bead-cycle` dispatching to specialist subagents |
| Evaluator-optimizer | impl → panel → fix → re-panel iterative loop |
| Prompt chaining | spec → plan → bead lifecycle |
| **Routing** | **Not used.** Could classify bead type (migration / logic / docs) and pick different impl + panel shapes. |

## Part 3 — Suggested follow-ups

1. **Add a `Why these patterns` section to the README** naming the four Anthropic patterns the plugin uses + the one (Routing) it doesn't.
2. **Add a `When to consider workflows` section** with the table from Part 2.
3. **Write `/ms-bead-prep`** as a sibling to `/ms-bead-impl` for drafting impl prompts.
4. **Encode codex-substitution as a deterministic check** in `/ms-panel-run`.
5. **Draft a `PreToolUse` hook spec** that blocks `mindspec complete <bead-id>` without a passing-verdict file.
6. **File the upstream Dolt 1105** as a beads issue.
7. **File the upstream `.gitignore` OWNERSHIP** gap as a mindspec issue.
8. **Extend `mindspec instruct`** with `--panel-state`.
9. **Add a worked example** to `parallel-window` or remove it as YAGNI.
10. **Capture the rationale for 6 reviewers + round-2 panels** in the README.
