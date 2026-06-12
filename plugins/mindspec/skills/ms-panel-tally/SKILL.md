---
name: ms-panel-tally
description: The single decision authority for a mindspec panel — read all verdict JSONs, apply the decision matrix + artifact gates, consolidate concrete_changes_required, and drive halt-recovery
---

# Tally Panel Verdicts

Read `<repo>/review/<panel-slug>/*-round-<N>.json`, summarise verdicts, apply the decision matrix (including the HARD artifact gates), consolidate the convergent `concrete_changes_required` list, and decide whether the panel passes. This skill is the **single decision authority** — both per-bead cycles and `/ms-spec-final-review` route their tally through here.

## Inputs

- `panel-slug` (required).
- `round` (required) — the round just completed.
- `expected-reviewers` (default `6`, = N) — read from `panel.json`.

## Steps

1. **Load all verdicts.**
   ```bash
   cd <repo>/review/<panel-slug>
   for f in *-round-<N>.json; do
     python3 -c "import json; d=json.load(open('$f')); print(f, d['verdict'], d.get('confidence'))"
   done
   ```
   The latest round is the filename-derived `max(N)` over `*-round-<N>.json` — never trust `panel.json.round`, which can lag. If `panel.json.round` ≠ the filename max, the panel state is stale: re-run `/ms-panel-run` step 0 to re-sync before tallying.

2. **Tabulate.** Report:
   - Per-slot: `verdict`, `confidence`, one-line rationale snippet.
   - Aggregate: APPROVE count / REQUEST_CHANGES count / REJECT count.
   - Family split: of the APPROVEs, how many Claude vs how many Codex? Family asymmetry matters.
   - Malformed verdict JSON counts as missing (name the file).

3. **Decision matrix.** The approval threshold is **N − 1** (one dissent tolerated): 5-of-6 for the default 6-reviewer panel. Scale as ceil(5N/6) if you change N.

   | Condition | Action |
   |:----------|:-------|
   | Any verdict carries `"hard_block": true` OR a `concrete_changes_required` item names a missing measurement artifact, drift report, cost projection, or regression baseline | **HARD block** (see § Artifact gates). Halt; commission the measurement run before merge. Not satisfiable by PR-body fixes. |
   | Any REJECT | Halt (see § After a halt — recovery). REJECTs usually mean the brief or plan needs work. |
   | Verdicts present < `expected_reviewers` | Incomplete — finish `/ms-panel-run` before deciding. |
   | APPROVE ≥ N−1 AND no HARD-block flags AND head SHA fresh | Panel passes → merge terminal. |
   | Below N−1 APPROVE | Fix-up needed → `/ms-bead-fix`. Flag to the user if ≤2 APPROVE (significant rework). |

   On a pass, hand off to the cycle's **merge terminal**: run `mindspec complete <bead-id> "<summary>"` (hook-gated — the pre-complete gate re-verifies this tally before the merge lands). The "Then" handoff points at `/ms-bead-cycle`'s merge terminal.

4. **Consolidate `concrete_changes_required`.** This is the input to `/ms-bead-fix`. Process:

   a. Collect every `concrete_changes_required` item across the REQUEST_CHANGES verdicts.

   b. Dedupe semantically — multiple reviewers often flag the same defect differently ("enforce Case-3 invariants" / "reject malformed Case-3 payloads" / "reuse Bead-1 case models"). Group by defect, list distinct asks under each group.

   c. Rank by criticality:
      - **Code defects** (functional bugs, broken contracts) — must fix
      - **Test coverage gaps** — must fix
      - **Refactors / sharing** (e.g. "reuse the shared model") — fix if it changes behaviour, defer if pure style
      - **Documentation / prose** — fix if user-facing, defer otherwise

   d. Write the consolidated list to `<repo>/review/<panel-slug>/consolidated-round-<N>.md` for the fix subagent to read.

5. **Report to the orchestrator** (`/ms-bead-cycle`):
   ```
   Panel <slug> round <N>: <A> APPROVE, <R> REQUEST_CHANGES, <X> REJECT (threshold N−1 = <N0−1>/<N0>)
   Family split (APPROVEs): <claude>/3 claude, <codex>/3 codex
   Decision: <merge | fix | halt>
   Consolidated changes: <path-to-md>
   ```

## Artifact gates (HARD block, not body fix)

Some findings name a measurement artifact (e.g. `cost_projection.json`) that the spec/plan declared a release-gate precondition. These are **HARD blocks** distinct from PR-body precision fixes:

| Finding shape | Treatment |
|:--------------|:----------|
| "Evidence path UNNAMED in PR body" | Soft fix — name the path in the PR body, the slot re-verifies. |
| "Evidence path NAMED but artifact MISSING at that path" | **HARD block** — orchestrator must commission the measurement run + land the artifact at the named path. Cannot be resolved by editing the PR body. |
| "Operator sign-off PENDING" | Soft fix — capture sign-off reference in PR body or as a PR comment. |

The distinguishing question: **could the missing artifact have caught a real defect?** If yes (measurement artifact, cost projection, drift report, regression baseline), it's a HARD block. If no (operator acknowledgement, follow-up bd link), it's a soft fix.

Real failure case (lola-f4a8, $417): spec-050 final-review F5 round 1 flagged AC8c `cost_projection.json` as missing. The round-2 fix was a PR-body precision update naming the artifact landing path; F5 round 2 flipped to APPROVE because the path was named. PR #522 merged. The first post-spec-050 Monday cron burned $417 in one run because the alias-intersect prefilter has no cap — exactly what AC8c was meant to project. A round-2 APPROVE earned by *describing* a fix instead of *making* it is the known bypass; missing-artifact findings HARD-block regardless of vote count. Postmortem: `bd show lola-f4a8`.

## After a halt — recovery

When the matrix halts (REJECT, HARD block, or `max-rounds` exceeded):

1. **Inventory** the open panel(s) and in-progress beads:
   ```bash
   mindspec instruct --panel-state
   ```
   This surfaces the latest round, verdict count vs expected, APPROVE tally, `reviewed_head_sha` staleness, and a "gate would PASS/BLOCK" line computed by the same tally the hook uses.

2. **Classify the halt:**
   - **REJECT** → the brief or plan likely needs work. Return to the user with the verdict JSONs; do not auto-fix.
   - **HARD block** → commission the missing measurement run as a separate work unit, land the artifact at the named path, then re-panel.
   - **max-rounds exceeded** → halt with the bead `in_progress`; the user may revise the plan or split the bead.

3. **Stale-verdict rule (now mechanized).** If commits landed on the bead branch after the panel reviewed it (`reviewed_head_sha` ≠ current branch tip), the verdicts are stale — bump the round and re-panel via `/ms-panel-run` step 0 (which re-captures `reviewed_head_sha` in the same write). The pre-complete gate Blocks a stale-SHA complete, so this is enforced, not advisory.

4. **Abandon procedure (legitimate exit).** To abandon a panel without merging (e.g. the bead is being reworked outside the panel loop): set `"abandoned": true` in `panel.json` AND record who/why in `"abandon_reason"`. Completion then writes a `panel_abandoned` audit entry (plus the reason) to the bead metadata. Abandonment is a plain repo-file edit and therefore agent-performable — it is legitimate precisely because it is always audited, never silent. Do NOT abandon to skip a HARD block; abandon only when the bead is genuinely leaving the panel flow.

## Escape hatch

Skipping the panel gate entirely requires a **human**. A user sets `MINDSPEC_SKIP_PANEL=1` in their own shell environment before launching the session; the pre-complete hook inherits that environment and passes the gate, emitting an audited Warn, and `mindspec complete` records `panel_gate_skipped: true` + timestamp on the bead metadata. The variable is env-only and never pasted into a command line — a blocked agent cannot set it via a Bash-line prefix (the hook never consults the command string for the hatch). This is the only place `MINDSPEC_SKIP_PANEL` is documented; the hook's Block message deliberately never prints a paste-able skip incantation.

## Anti-patterns

- Don't auto-merge below the N−1 threshold. The threshold is N−1 (5/6 for the default panel), and you should still note family asymmetry.
- Don't pass raw verdict JSONs to the fix subagent — dedupe first. Six verdicts × ~3 items each = ~18 lines of duplicated asks otherwise.
- Don't ignore `confidence`. A 0.96 REQUEST_CHANGES from one slot should outweigh a 0.70 APPROVE from another. Note this in the report.
- Don't drop a REQUEST_CHANGES because "only one reviewer flagged it". A single empirically-grounded objection can be load-bearing — verify the claim before discarding.
- Don't satisfy an artifact-gate HARD block with a PR-body edit. The artifact must exist at the named path.

## Then

Decision-dependent:
- `merge` → `/ms-bead-cycle` merge terminal (`mindspec complete <id> "<summary>"`, hook-gated)
- `fix` → `/ms-bead-fix`
- `halt` → § After a halt — recovery; return to user
