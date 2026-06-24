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
| 15 (new) | F5 artifact-gate findings could flip to APPROVE on PR-body-only fixes — caused lola-f4a8 ($417 prod incident) | `b232413` | CLOSED |

Items 1, 2, 5-10 below remained OPEN after this PR. bd issue: `mindspec-ch8h`.

**Spec 093 (skills-thin-down) update:** items 1, 2, 8, 9, 10 are now ADJUDICATED (annotated inline below), and the Part 2 `PreToolUse`-gate primitive (Part 3 follow-up 5) is ENFORCED — it ships as the panel gate on `mindspec complete`. Remaining OPEN: items 5, 6, 7 (upstream Dolt 1105, `.gitignore` OWNERSHIP, plan↔bd dep sync) — out of scope for 093, tracked separately. `mindspec-ch8h` is closed out by spec 093 Bead 7.

## Open after this PR

| # | Finding | Status | Notes |
|:--|:--------|:-------|:------|
| 16 (new) | F6 operator-readiness lens doesn't catch IaC-vs-manual-env-var drift risk | OPEN | filed (not yet closed) |

See item 16 in Part 1 below for details.

## Part 1 — Plugin self-review

### Gaps surfaced while writing the skills

1. **`/ms-spec-autopilot` `parallel-window: true` is documented but hand-wavy.** **[ADJUDICATED — spec 093, Bead 6: parallel-window deleted as YAGNI]**
   The dep-graph-aware fan-out logic in the SKILL.md says "find all beads whose blockers are now satisfied" and "pick beads in disjoint file-sets only" but doesn't define the actual algorithm. In practice on lola spec-050 the dep graph is a near-linear chain (1 → 2 → 3 → 4 → 5 → 6 → 8a → 8b, with 7 forking off 4 and 9 forking off 8a), so the parallel window is small and the serial path wins. The skill should either:
   - Remove the parallel-window option as YAGNI, or
   - Add a worked example of when fan-out actually pays off (likely specs that branch into multiple feature surfaces, not migration chains).

   **Resolution:** Removed as YAGNI in spec 093 (Bead 6) — `/ms-spec-autopilot` is now serial-by-design, no `parallel-window` reference remains. Parallel-bead scheduling is deferred to the Claude Code Dynamic Workflows port (see Part 2); a low-priority bd note tracks that future work.

2. **No `/ms-panel-halt-recover` for stalled cycles.** **[ADJUDICATED — spec 093, Bead 6: halt-recovery folded into `/ms-panel-tally`]**
   `/ms-panel-tally` says "halt and ask the user" on REJECT or repeated halts, but doesn't define the resumption path. After a halt, the bead is in `in_progress`, the panel has stale verdicts, and the next session needs to either:
   - Pick up from the latest commit and re-launch the next round, or
   - Roll back to a known-good commit and start over.
   Should be a skill so the resumption path is reproducible.

   **Resolution:** No separate skill — the resumption path lives in the canonical halt-recover section of `/ms-panel-tally` (the single decision authority), which now defines the resume-from-latest-commit and abandon procedures. Keeping it there avoids a 12th skill and the operational-knowledge fan-out the 093 thin-down removed (orchestration in skills, recovery recipes at point-of-use).

3. **No `/ms-bead-prep` skill for drafting impl prompts.** **[CLOSED — commit `f823476`]**
   The pre-staged prompts at `<spec-dir>/reviews/prep/bead<N>_impl_prompt.md` are referenced by `/ms-bead-impl` as "if absent, draft one in-conversation". In practice good prompts are the single biggest lever on impl-subagent quality — they should have their own skill that reads the plan section + spec context + prior-bead helper signatures and produces a structured prompt. The lola session benefited massively from pre-staged prompts; the plugin doesn't tell you how to write them.

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

8. **`mindspec instruct` (SessionStart hook) doesn't surface panel/subagent state.** **[ADJUDICATED — spec 093, Bead 5: `--panel-state` shipped]**
   The hook prints "Multiple Active Specs" with phases, but doesn't tell a fresh post-compaction session which beads have in-flight implementation subagents or pending panel rounds. Cost us several minutes after compaction to reconstruct state from `<spec-dir>/reviews/<panel>/*.json` and `git worktree list`. Should add a `--panel-state` block to `mindspec instruct` that lists:
   - In-progress beads (worktree + last commit on bead branch)
   - Open panel rounds (`<spec-dir>/reviews/<panel-slug>/` with mismatched verdict-file count vs expected)
   - Any locked agent worktrees (`.claude/worktrees/agent-*` still in place)

   **Resolution:** Shipped in spec 093 (Bead 5, Reqs 14-15). `mindspec instruct --panel-state` renders the three-block Panel/Subagent State view (in-progress beads, open panel rounds, locked worktrees), auto-included in implement-mode SessionStart output and zero-cost when no panel dir exists. The "gate would BLOCK" line agrees with a direct `mindspec hook pre-complete` invocation (one-source-of-truth proof).

### Underdocumented decisions

9. **Why 6 reviewers specifically.** **[ADJUDICATED — spec 093, Bead 7: rationale paragraph landed in README]**
   The README says "mixed families catch different defects" but doesn't justify 6 vs 4 or 8. Empirically on lola: ≥5 covers comfortable majority-vote; <5 has too much variance (one reviewer flagging changes is a 33% reroute, vs 20% at 5/6). But that's an empirical claim, not a derived one. Could be documented as "tunable, 6 was the sweet spot for our compute budget".

   **Resolution:** Landed. The "Why six and not four or eight" paragraph (majority arithmetic + verdict variance + ceil(5N/6) threshold scaling) is appended to README § "Why six reviewers, mixed families".

10. **Why round-2 panels and not just one-shot review.** **[ADJUDICATED — spec 093, Bead 7: rationale sidebar landed in README]**
    The pattern "fix → re-review" is described in `/ms-bead-cycle` but the rationale isn't pinned. In practice the round-2 panel catches fix-author deviations the original BRIEF couldn't predict (the lola Bead 2 r2 panel found a routing bug the r1 panel approved). Should be a sidebar in the README.

    **Resolution:** Landed. The "Why a second panel round instead of one-shot review" sidebar (reviews the fix author, scoped to `concrete_changes_required` deltas, lola Bead 2 routing-bug example, artifact-gate HARD-block corollary) is added after README § "The autonomous loop".

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

15. **F5 artifact-gate findings could be soft-fixed via PR-body edits.** **[CLOSED — commit `b232413`]**
    `/ms-spec-final-review` F5 lens flagged "Evidence MISSING (4 ACs)" for spec-050 round 1, including AC8c `cost_projection.json`. The round-2 fix-up subagent treated this as a PR-body precision update — named the artifact landing path in the PR description without actually producing the artifact. F5 round-2 flipped to APPROVE because the body precision was correct. The plugin's 5/6 APPROVE merge gate cleared, PR #522 merged, prod deployed.

    The first post-deploy Monday cron (today, 2026-06-08) burned **$417+** on OpenRouter in one run — 26,044 LLM calls vs 18 baseline, 3,221 brands/creator vs 133 baseline. The spec-050 alias-intersect prefilter has no cap; AC8c's `cost_projection.json` measurement was specifically what would have projected the post-cutover Stage-2 envelope and caught this.

    Postmortem: `bd show lola-f4a8`.

    Fixed by:
    - `/ms-spec-final-review`: F5 lens explicitly distinguishes "evidence path unnamed" (soft fix) from "evidence path named but artifact missing" (HARD block). The step 5 tally adds a HARD-block terminal that halts regardless of vote count.
    - `/ms-panel-tally`: decision matrix adds a HARD-block row for missing measurement artifacts at the top of the table.
    - `/ms-bead-fix`: anti-pattern callout that the fix subagent MUST NOT mark an artifact-gate finding as ADDRESSED via a body edit alone. If the subagent cannot produce the artifact in its scope, it returns the finding UNCHANGED and flags PARTIAL.

16. **`/ms-spec-final-review` F6 lens doesn't check rollout-flag IaC discipline.** **[OPEN]**
    The F6 operator-readiness lens currently checks for runbook coherence (revert commands, MTTR claims, escalation paths) but doesn't audit whether rollout flags / cost-bounding env vars are defined in IaC (`infra/tofu/`) versus set manually via `gcloud run ... --set-env-vars`. The latter is silently lost on any service-recreate deploy.

    Surfaced by lola-f4a8 revision (Mon + Thu 2026-06-08 / 2026-06-11 OR spikes both root-caused to manual env-var drift, NOT to spec-050 alias-intersect as initially diagnosed). Both spec-049's `SUGGESTION_SKIP_STAGE_1_7` and spec-050's `ZPAW_ENABLED` were set manually; a later deploy lost them and prod silently fell back to the unbounded pre-spec-049 legacy scorer.

    **Taxonomy (four states, ranked ladder — strongest pin wins):**

    For every release-gate / cost-bounding flag the spec names, classify by the strongest pin that applies:

    1. **IaC-managed (plaintext)** — set via tofu `cloud_run_plain_env_vars` in the relevant per-env tfvars (`infra/tofu/environments/production.tfvars`, `staging.tfvars`). Acceptable.
    2. **IaC-managed (Secret Manager mount)** — set via tofu `secret_env_vars` referencing a Secret Manager secret in the relevant per-env tfvars. Same status as state 1 — IaC-disciplined, not manual-only. Silence on this state would mis-classify legitimately-mounted secrets as HARD findings.
    3. **App-config-default-pinned** — Pydantic `field_validator(mode="before")` clamps the runtime value to a safe default regardless of the env override (NOT plain `Field(default=...)` — that's still env-overridable, so the safe default is lost the moment the env var is set to a dangerous value or `None` is coerced). Acceptable when the safe default matches the lola-f4a8 ask: "behaviour is safe even if the env override goes missing."
    4. **Manual-only** — set only via `gcloud run ... --set-env-vars`, no tfvars entry, no `field_validator` pin. **HARD finding** — silently lost on any service-recreate deploy.

    **Per-environment classification.** The four states apply per env: a flag pinned only in `staging.tfvars` is IaC-managed for staging but manual-only for production (lola-f4a8 was prod-specific). F6 must classify each (flag, env) pair independently and flag any (flag, prod) pair that lands in state 4.

    **States are orthogonal axes, not mutually exclusive.** A flag can be in multiple states at once (e.g. defaulted in `api/app/core/config.py` AND set in `production.tfvars`). Resolution rule: strongest pin wins for the (flag, env) pair — state 1 or 2 > state 3 > state 4.

    **Detection rule (deterministic).** For each release-gate flag `<VAR>` named in the spec:
    ```
    rg "\b<VAR>\b" infra/tofu/environments/*.tfvars
    ```
    Classify by which tfvars files match — `production.tfvars` match → state 1 or 2 for prod (inspect surrounding key: `cloud_run_plain_env_vars` vs `secret_env_vars`); only `staging.tfvars` match → state 4 for prod, state 1/2 for staging; no match → fall through to grep `api/app/core/config.py` for `field_validator` referencing `<VAR>` (state 3) or `Field(default=...)` only (still state 4 — overridable).

    **Spec-enumeration fallback.** The sub-check is gated on the spec enumerating its release-gate flags. If the spec under-enumerates, F6 must additionally grep `api/app/core/config.py` for `Field(default=...)` lines that look like release-gate / cost-bounding flags (heuristics: name contains `ENABLED`, `SKIP`, `THRESHOLD`, `MAX_`, `_RATE`, `ROLLOUT`) even if the spec doesn't list them, and audit each against the four-state taxonomy.

    **Integration point — TWO surfaces, parallel to item 15's pattern:**

    1. **F6 reviewer prompt** (`commands/ms-spec-final-review.md`): add a new sub-bullet under the existing F6 step that runs the detection rule above and reports each (flag, env) pair's state. The sub-bullet anchors under F6 (not a sibling lens F7) to keep operator-readiness concerns colocated.
    2. **`/ms-panel-tally` decision matrix** (`commands/ms-panel-tally.md`): add a HARD-block row mirroring item 15's pattern — "any (flag, prod) pair classified as state 4 (manual-only) → HARD-block, must be IaC-pinned before merge". Without this row, a clean F6 finding doesn't actually block the tally and the lens ships half-integrated. The follow-up PR must touch both files.

    Distinct from item 15 (artifact-gate HARD-block): item 15 covers measurement artifacts the spec.md plan declared; item 16 covers config-surface drift over the spec's prod lifetime. Same shape in the tally matrix; different reviewer-prompt anchor.

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
| **`PreToolUse` hook on `mindspec complete`** | Declaratively enforce "≥5/6 APPROVE before merge"; no orchestrator skill can bypass it. | **ENFORCED (spec 093)** — shipped as a `PreToolUse` Bash gate: fail-open without a panel, fail-closed with one (malformed/missing verdicts block). The gate is now an enforced contract, not a convention. |
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
