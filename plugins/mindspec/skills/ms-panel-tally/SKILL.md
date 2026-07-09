---
name: ms-panel-tally
description: The single decision authority for a mindspec panel — read all verdict JSONs, apply the decision matrix + artifact gates, consolidate concrete_changes_required, and drive halt-recovery
---

# Tally Panel Verdicts

Run `mindspec panel tally <panel-slug>` for the decision, apply the HARD artifact gates, consolidate the convergent `concrete_changes_required` list, and drive halt-recovery. This skill is the **single decision authority** — both per-bead cycles and `/ms-spec-final-review` route their tally through here.

> Reviews are co-located under the spec (spec 106 flat layout): `<spec-dir>` is `<repo>/.mindspec/specs/<spec-slug>/`, so panels live at `<spec-dir>/reviews/<panel-slug>/` — the location the `mindspec complete` gate scans.

## Inputs

- `panel-slug` (required) — passed as `mindspec panel tally <panel-slug>`. The round tallied, the expected-reviewer count, and the approve threshold are all resolved automatically from `panel.json` and the configured panel defaults — nothing else to pass in.

## Workflow-path note

When `runner: claude-code-workflow` (§ Runner dispatch in `/ms-panel-run`) dispatched this panel, the per-slot verdict table and decision below arrive **pre-rendered in the workflow result** — the `/ms-panel` workflow already ran `mindspec panel verify` and `mindspec panel tally` as its last two steps and returned their output verbatim (spec 111 R5). Step 1 below is then just reading that already-produced output rather than re-running the command yourself; this skill's job narrows to Step 2 consolidation and the merge terminal. The judgment sections below — Consolidate, § Artifact gates, § After a halt — recovery, and § Escape hatch — are unchanged on both paths.

## Steps

1. **Run the tally.**
   ```bash
   mindspec panel tally <panel-slug>
   ```
   This prints, in one shot: the per-slot verdict table (`verdict` + `hard_block`, malformed files named and counted as missing), the aggregate APPROVE / REQUEST_CHANGES / REJECT counts against the resolved threshold (**N − 1** by default — 5-of-6 for the standard 6-reviewer panel), the `panel.PanelGateDecision` decision (PASS / PASS with advisory / BLOCK), and the aggregated `concrete_changes_required` — read presentation-only from each REQUEST_CHANGES/REJECT verdict file, never feeding the decision. The exit code tracks the decision alone: `0` on Allow, `0` with the advisory printed on Warn, non-zero with a final recovery line (ADR-0035) on Block.

   This is the identical decision the in-binary `mindspec complete` gate enforces — including the filename-derived `max(N)` over `*-round-<N>.json` (never a possibly-lagging `panel.json.round`) and the `reviewed_head_sha` freshness check — so there is nothing left to hand-tabulate. On a Block from staleness, see § After a halt — recovery below.

   **Before handing off to the merge terminal on an Allow**, screen the tally's aggregated `concrete_changes_required` (printed even on Allow) against § Artifact gates below: a CCR item naming a missing measurement artifact / cost projection / drift report / regression baseline **HARD-blocks regardless of vote count and regardless of whether any reviewer set `hard_block`**. The binary mechanizes only the `hard_block`-flag disjunct (gate.go cases 9–10); this screen restores the other half — an aggregated-CCR item can HARD-block an Allow even when no single verdict flagged it. Only once that screen is clear does an Allow hand off to the cycle's merge terminal per § Then below.

2. **Consolidate `concrete_changes_required`.** This is the input to `/ms-bead-fix`. Process:

   a. Collect every `concrete_changes_required` item across the REQUEST_CHANGES/REJECT verdicts (the tally's aggregated output above, or the underlying verdict files directly).

   b. Dedupe semantically — multiple reviewers often flag the same defect differently ("enforce Case-3 invariants" / "reject malformed Case-3 payloads" / "reuse Bead-1 case models"). Group by defect, list distinct asks under each group.

   c. Rank by criticality:
      - **Code defects** (functional bugs, broken contracts) — must fix
      - **Test coverage gaps** — must fix
      - **Refactors / sharing** (e.g. "reuse the shared model") — fix if it changes behaviour, defer if pure style
      - **Documentation / prose** — fix if user-facing, defer otherwise

   d. Write the consolidated list to `<spec-dir>/reviews/<panel-slug>/consolidated-round-<N>.md` for the fix subagent to read.

3. **Report to the orchestrator** (`/ms-bead-cycle`): relay the tally's printed per-slot table + decision, a family-split note (APPROVEs among R1–R3 claude vs R4–R6 codex — see the per-slot table above), and the consolidated-changes path:
   ```
   <mindspec panel tally output>
   Family split (APPROVEs): <claude>/3 claude, <codex>/3 codex
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

3. **Stale-verdict rule (mechanized).** If commits landed on the bead branch after the panel reviewed it (`reviewed_head_sha` ≠ current branch tip), the verdicts are stale — re-panel via `mindspec panel create <slug> --spec <id> --target <ref> --round <N+1>` (the co-bumping verb: `round` and a freshly re-resolved `reviewed_head_sha` land in the same write). The in-binary `mindspec complete` gate Blocks a stale-SHA complete, so this is enforced, not advisory.

4. **Abandon procedure (legitimate exit).** To abandon a panel without merging (e.g. the bead is being reworked outside the panel loop): set `"abandoned": true` in `panel.json` AND record who/why in `"abandon_reason"`. Completion then writes a `panel_abandoned` audit entry (plus the reason) to the bead metadata. Abandonment is a plain repo-file edit and therefore agent-performable — it is legitimate precisely because it is always audited, never silent. Do NOT abandon to skip a HARD block; abandon only when the bead is genuinely leaving the panel flow.

## Escape hatch

Skipping the panel gate entirely requires a **human**. A user sets `MINDSPEC_SKIP_PANEL=1` in their own shell environment before launching the session; the in-binary `mindspec complete` gate reads that environment and passes the gate, emitting an audited Warn, and `mindspec complete` records `panel_gate_skipped: true` + timestamp on the bead metadata. The variable is env-only and never pasted into a command line — a blocked agent cannot set it via a Bash-line prefix (the gate never consults the command string for the hatch). This is the only place `MINDSPEC_SKIP_PANEL` is documented; the gate's Block message deliberately never prints a paste-able skip incantation.

## Anti-patterns

- Don't auto-merge below the N−1 threshold. The threshold is N−1 (5/6 for the default panel), and you should still note family asymmetry.
- Don't pass raw verdict JSONs to the fix subagent — dedupe first. Six verdicts × ~3 items each = ~18 lines of duplicated asks otherwise.
- Don't ignore `confidence`. A 0.96 REQUEST_CHANGES from one slot should outweigh a 0.70 APPROVE from another. `mindspec panel tally`'s printed table carries `verdict`/`hard_block` only, not `confidence` — read it from the underlying `<slot>-round-<N>.json` files when weighing verdicts, and note the weighting in the report.
- Don't drop a REQUEST_CHANGES because "only one reviewer flagged it". A single empirically-grounded objection can be load-bearing — verify the claim before discarding.
- Don't satisfy an artifact-gate HARD block with a PR-body edit. The artifact must exist at the named path.

## Then

Decision-dependent:
- `merge` → `/ms-bead-cycle` merge terminal (`mindspec complete <id> "<summary>"`, hook-gated)
- `fix` → `/ms-bead-fix`
- `halt` → § After a halt — recovery; return to user
