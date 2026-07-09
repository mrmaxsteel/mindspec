# spec-111-plan-approve — Round 4 (targeted re-verification: F1, F3, G3)

**Under review**: `.mindspec/specs/111-workflow-panel-runner/plan.md` @ **aec82bfde854d4ba24d137b942803eb6dc909a16** (commit `docs(spec-111): apply round-3 plan-panel changes`, on top of round-2's `631d7c14`; +41/-25, plan.md only) in worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner`. Read the APPROVED `spec.md` beside it.

**Panel**: 9 slots (F1–F3 Fable, O1–O3 Opus, G1–G3 GPT-5.5). Pass = **≥8 APPROVE, no REJECT**.
**Round-3 standing (carried)**: F2, G1, G2, O1, O2, O3 = APPROVE (6). Their lenses (DAG, coverage, step-counts, provenance, downstream contract, escaping/traversal) are untouched by this flag-handling reconciliation and are **carried forward** — they are NOT re-running.
**Round-4 re-runs (you)**: F1, F3 (round-2 RC voters who did not get to re-verify in round 3), and G3 (round-3 RC voter). Only these three write round-4 verdicts.

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; scratch under /tmp; pin all reads to SHA `aec82bfd`; leave `git status` clean. Do not edit any file except your own verdict JSON.

## Why round 4 exists

The round-2 fix (`631d7c14`) added a `buildCommand` argument-injection guard ("reject any `args` element starting with `-`"). In round 3, **G3 found this guard BLOCKS the workflow**: the prescribed call sites passed the fixed flags `--spec`/`--target` *through* `args`, so a literal implementation rejects them and registration can never run. F1 and F3 did not get to re-verify their own round-2 items in round 3 (Fable was walled).

The round-3 fix (`aec82bfd`) reconciles this: `buildCommand` now assembles each command from a **fixed per-verb template** that alone supplies the fixed flags (`--spec`/`--target`/`--bead`/`--round`; `--sandbox read-only --skip-git-repo-check` for codex). Callers pass **only user-derived values** (`slug`, `spec`, `target`, `bead_id`, `round`). The leading-dash guard is now **scoped to those values** — a flag-shaped value (`--json` slug/target, or an attempted second `--sandbox` override) is still rejected, while the template's own fixed flags run untouched.

## Your deltas

- **G3**: your delta is `631d7c14..aec82bfd` — exactly the dash-guard reconciliation.
  `git -C <worktree> diff 631d7c14..aec82bfd -- .mindspec/specs/111-workflow-panel-runner/plan.md`
- **F1, F3**: you last completed a verdict in **round 2** on `bf01a21a`; you did NOT review the round-2 fix (`631d7c14`). Your cumulative delta since you last looked is `bf01a21a..aec82bfd` — it contains **both** the round-2 fix that addressed your round-2 items **and** the round-3 dash-guard reconciliation.
  `git -C <worktree> diff bf01a21a..aec82bfd -- .mindspec/specs/111-workflow-panel-runner/plan.md`

## Per-slot jobs

### G3 (codex) — verify your round-3 blocking finding is resolved
Your round-3 REQUEST_CHANGES (verbatim): *"reconcile buildCommand's leading-dash guard with fixed CLI option tokens: either make buildCommand accept structured fixed flags outside the guarded user-value list, or state that the guard applies only to untrusted/user-derived values, then update the examples/tests accordingly."*
- Confirm the plan now states the guard applies only to user-derived values AND that the fixed flags come from the per-verb template (not `args`).
- Confirm the registration call-site example (Registration step R2, ~plan line 489) no longer passes `--spec`/`--target` through `args` — it now passes `buildCommand(CMD_PANEL_CREATE, slug, spec, target, bead_id?, round?)` (values only).
- Confirm the re-panel note passes the round **value** `N+1`, not a `--round` flag.
- Confirm Step 6's test assertions are still coherent with the reconciled model (they pin the ALLOWED_CLI array + call-site identifier usage, not the dash-guard scope — check they don't now contradict).
- Disposition your round-3 item: ADDRESSED / PARTIAL / MISSED / NEW_ISSUE.

### F1 (Fable) — re-verify your two round-2 items + confirm no regression
Your round-2 RC raised: **(item 1)** reconcile Step 6's chokepoint assertion with the call sites — destructure named verb constants from `ALLOWED_CLI`, route every call through `buildCommand`, add an identifier-count pin, soften the "by construction" claim; **(item 2)** an argument-injection guard so flag-shaped values (`--json`) can't be smuggled as flags and no `--sandbox`/`-s` override can be appended after the codex prefix.
- Item 1: confirm the destructured constants (`const [CMD_PANEL_CREATE, CMD_CODEX_EXEC, CMD_PANEL_VERIFY, CMD_PANEL_TALLY] = ALLOWED_CLI;`) are present, every call passes an identifier, and the identifier-count pin exists (landed in `631d7c14`; unchanged in `aec82bfd`).
- Item 2 (this is the round-4-critical one): confirm the guard, now **scoped to user-derived values**, STILL rejects your flag-shaped-value attack (a `slug`/`target` of `--json`) AND still forecloses a `--sandbox` override — verify that because callers can no longer pass ANY flag through `args` (flags live only in the template), the override path is closed *more* firmly than a blanket reject would. Confirm this did not open a new hole.
- Disposition each item: ADDRESSED / PARTIAL / MISSED / NEW_ISSUE.

### F3 (Fable) — re-verify your two round-2 items + confirm no regression
Your round-2 RC raised: **(item 1)** redefine the Manual e2e's `malformed-once` FIRST payload as a rendered verdict with known canned values in a malformed serialization (not valueless prose), and state run-2's value-fidelity check concretely; **(item 2)** write the scratch repo's `.mindspec/config.yaml` `panel:` mix to match each scenario's fixture mix, since 110's `panel create` stamps `expected_reviewers` from `cfg.PanelExpectedReviewers()` (config), not from the workflow's `mix` arg.
- Both items were addressed in `631d7c14` (which you never reviewed). Verify they landed correctly and are falsifiable as written.
- Confirm the round-3 dash-guard reconciliation (`631d7c14..aec82bfd`) introduces **no regression** in the e2e scenarios / shim spec (it touches only buildCommand flag handling, not the e2e).
- Disposition each item: ADDRESSED / PARTIAL / MISSED / NEW_ISSUE.

## Output

Write `<slot>-round-4.json` in this directory. Keys: `reviewer_id` ("<slot> <family>", e.g. "F1 fable" / "G3 gpt-5.5"), `verdict` (APPROVE / REQUEST_CHANGES / REJECT), `confidence`, `rationale` (≤160 words), `concrete_changes_required` (empty if APPROVE), `findings` (per-item dispositions). An artifact-gate finding may set `"hard_block": true` (not expected for a plan panel).
