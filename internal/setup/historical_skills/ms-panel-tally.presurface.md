---
name: ms-panel-tally
description: The single decision authority for a mindspec panel — read all verdict JSONs, apply the decision matrix + artifact gates, consolidate concrete_changes_required, and drive halt-recovery
---

# Tally Panel Verdicts

Run `mindspec panel tally <panel-slug>` for the decision, apply the HARD artifact gates, consolidate the convergent `concrete_changes_required` list, and drive halt-recovery. This skill is the **single decision authority** — both per-bead cycles and `/ms-spec-final-review` route their tally through here.

> Reviews are co-located under the spec: `<spec-dir>` is `<repo>/.mindspec/specs/<spec-slug>/`, so panels live at `<spec-dir>/reviews/<panel-slug>/` — the location the `mindspec complete` gate scans.

## Inputs

- `panel-slug` (required) — passed as `mindspec panel tally <panel-slug>`. The round tallied and the expected-reviewer count are resolved automatically from `panel.json` and the configured panel defaults — nothing else to pass in.

## Workflow-path note

When `runner: claude-code-workflow` (the Runner dispatch in `/ms-panel-run`) dispatched this panel, the per-slot verdict table and decision below arrive **pre-rendered in the workflow result** — the `/ms-panel` workflow already ran `mindspec panel verify` and `mindspec panel tally` as its last two steps and returned their output verbatim. Step 1 below is then just reading that already-produced output rather than re-running the command yourself; this skill's job narrows to the Artifact gates Allow-screen, Step 2 consolidation, and the merge terminal — the workflow mechanizes the tally, not the human artifact-gate and consolidation judgment layered on top of it. The judgment sections below — Consolidate, the Artifact gates, the After a halt — recovery, and the Escape hatch — are unchanged on both paths.

## Steps

1. **Run the tally.**
   ```bash
   mindspec panel tally <panel-slug>
   ```
   This prints, in one shot: the per-slot verdict table (`verdict` + `hard_block`, malformed files named and counted as missing), the aggregate APPROVE / REQUEST_CHANGES / REJECT counts — the configured panel mix (shipped defaults: 6/6/4/6 reviewers for spec/plan/bead/final; adhoc 6; alternating `claude` + `codex`; see the Panel-size ladder in `ms-panel-run` for how an operator scales this per gate) — the `panel.PanelGateDecision` decision (PASS / PASS with advisory / BLOCK), and the aggregated `concrete_changes_required` from the latest-round verdict files; unresolved REQUEST_CHANGES findings block the decision. The exit code tracks the decision alone: `0` on Allow, `0` with the advisory printed on Warn, non-zero with a final recovery line on Block.

   PASS requires a fresh and intact panel, all roster slots present, 0 REJECT, 0 hard_block, 0 INVALID and 0 unresolved REQUEST_CHANGES; each finding must be fixed, refuted or effectively deferred.

   This is the identical decision the in-binary `mindspec complete` gate enforces — including the filename-derived `max(N)` over `*-round-<N>.json` (never a possibly-lagging `panel.json.round`) and the `reviewed_head_sha` freshness check — so there is nothing left to hand-tabulate. On a Block from staleness, see the After a halt — recovery below.

   **Before handing off to the merge terminal on an Allow**, screen the tally's aggregated `concrete_changes_required` (printed even on Allow) against the Artifact gates below: a CCR item naming a missing measurement artifact / cost projection / drift report / regression baseline **HARD-blocks regardless of vote count and regardless of whether any reviewer set `hard_block`**. The binary mechanizes only the `hard_block`-flag disjunct; this screen restores the other half — an aggregated-CCR item can HARD-block an Allow even when no single verdict flagged it. Only once that screen is clear does an Allow hand off to the cycle's merge terminal per the Then below.

2. **Consolidate `concrete_changes_required`.** This is the input to `/ms-bead-fix`. Process:

   a. Collect every `concrete_changes_required` item across the REQUEST_CHANGES/REJECT verdicts (the tally's aggregated output above, or the underlying verdict files directly).

   b. Dedupe semantically — multiple reviewers often flag the same defect differently ("enforce Case-3 invariants" / "reject malformed Case-3 payloads" / "reuse Bead-1 case models"). Group by defect, list distinct asks under each group.

   c. Rank by criticality:
      - **Code defects** (functional bugs, broken contracts) — must fix
      - **Test coverage gaps** — must fix
      - **Refactors / sharing** (e.g. "reuse the shared model") — fix if it changes behaviour, defer if pure style
      - **Documentation / prose** — fix if user-facing, defer otherwise

   d. Write the consolidated list to `<spec-dir>/reviews/<panel-slug>/consolidated-round-<N>.md` for the fix subagent to read.

   e. **Contested-finding re-verify.** Before acting on an empirical claim another slot or the fix author disputes, re-verify it yourself in a fresh detached checkout at the reviewed SHA (same `git worktree add --detach` mechanics as `/ms-panel-run`'s Reviewer conduct section) — the reproduction (or its failure) IS the refutation evidence. More broadly: every BLOCKING finding from a non-interactive CLI reviewer must be reproduced in isolation before fix-dispatch or refutation — verify, never confirmed. Never out-vote by count in either direction; weight the reviewer that did the deeper code trace over a simple tally (extends the confidence/single-dissent notes in the Anti-patterns below).

3. **Capture dispositions.** As you — the single decision authority — resolve each DISTINCT finding above, at this consolidation pass or a later fix-round confirmation, append ONE row per distinct finding that no discharge verb already captured to that panel's durable store with the canonical write op — `mindspec panel disposition append` — never a hand-edited file. **Two DIFFERENT spec identifiers appear below, do not conflate them:** the `--spec` FLAG takes the full `<NNN-slug>` spec id, the same value `mindspec validate spec` takes; the `spec` FIELD inside the row/manifest JSON is the BARE number `<NNN>`, which is what the migrated seed carries and what Q-queries group on.
   **NEVER inline untrusted finding text into a shell-quoted `--data '{...}'` argument.** A crafted `summary`/`note`/`reviewer` (e.g. one containing `'; touch /tmp/x; echo '`) breaks the single quote and runs arbitrary shell BEFORE `mindspec` even starts. Instead, write the JSON to a temp file with a QUOTED heredoc — the quotes on `<<'JSON'` disable ALL shell expansion and command substitution inside the body, so no finding text can escape — then pass it with `--data @<file>` (`--data -` reads the same JSON from stdin). Do NOT pass an `id`: the CLI DERIVES the canonical row `id` from the record content (a hash of `{spec,panel,reviewer,summary}`), overriding anything you supply, which is exactly what makes a re-append of the same finding an idempotent retry rather than a duplicate.
   ```bash
   cat > /tmp/disposition-row.json <<'JSON'
   {"record":"disposition","spec":"<NNN>","gate":"<spec_approve|plan_approve|bead|final_review|adhoc>","panel":"<panel-slug>","reviewer":"<slot>","model":"<actual model that produced the verdict>","severity":"<blocking|major|minor>","summary":"<one-line finding summary>","convergent_with":["<slot>", ...],"disposition":"<confirmed-fixed|confirmed-scope-trim|deferred|false-contamination>","evidence_ref":"<fix commit / follow-up bead / refutation record>","note":"<free prose>","created_at":"<RFC-3339>","backfilled":false}
   JSON
   mindspec panel disposition append --spec <NNN-slug> --panel <panel-slug> --data @/tmp/disposition-row.json
   ```
   **Slot-identity contract:** `reviewer` and every `convergent_with[]` entry MUST byte-match a verdict-file slot token (`<slot>-round-<N>.json` → `<slot>`) — never a display name, and never the verdict JSON's own (typically empty) `reviewer_id` field.

   `mindspec panel tally` writes the coverage manifest when the latest round is complete, intact and has no veto or unresolved finding. It is the only manifest writer. Discharge verbs write their own disposition rows; do not append a second row for a finding already discharged by a verb. Run `mindspec panel disposition check --spec <spec-id> --panel <panel-slug>` to check durable slot coverage.

4. **Report to the orchestrator** (`/ms-bead-cycle`): relay the tally's printed per-slot table + decision, a family-split note (APPROVEs per configured family — the claude-family slots vs the codex-family slots, see the per-slot table above), and the consolidated-changes path, with denominators DERIVED from the configured mix — count the verdict files' `reviewer_id`s per family (`panel.json` carries the roster and each slot's model) — never a hard-coded partition:
   ```
   <mindspec panel tally output>
   Family split (APPROVEs): <claude-approves>/<claude-slots> claude, <codex-approves>/<codex-slots> codex
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

Missing measurement artifacts HARD-block regardless of vote count. A description of a fix does not supply the artifact.

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

3. **Stale-verdict rule (mechanized).** When tally reports that the subject tree has changed since review, re-panel with `mindspec panel create <slug> --spec <spec-id> --gate bead --target bead/<bead-id> --bead <bead-id> --round <N+1>`. If the panel passes apart from staleness and the new commits need no new review (for example, an ownership-only commit), run `mindspec panel rebind <slug> --reason '<who>: <why>'` instead. It moves the panel to the new tip with no new round and records the changed paths. It refuses a failing panel. `create` derives the bead id from the target and records the new round, reviewed commit and subject digest together. An optional `--bead` must match that target. A changed commit alone does not make the review stale if the derived subject digest is unchanged. Use the registration's recorded gate and target; omit `--spec` for an adhoc panel.

4. **Abandon procedure (legitimate exit).** Run `mindspec panel abandon <panel-slug> --reason "<why>" --by "<name>"`. The verb requires both fields, sets the snapshot and appends an audit event. It writes no manifest and retains any existing manifest. Completion records the `panel_abandoned` audit in bead metadata. Use this only when the work leaves the panel flow; a HARD block still needs its artifact.

5. **Refutation procedure (always audited).** Reproduce contested claims before refuting them. Run:
   ```bash
   mindspec panel refute <panel-slug> --slot R2 --round N --finding 'ccr[0]' --ground disproven --reason "<why>" --evidence <reference> --by "<name>"
   ```
   Use the roster slot id and latest round. Omit `--finding` to cover the whole slot's ordered finding list. Grounds are `disproven`, `not-load-bearing` and `out_of_acceptance_scope`; the last requires `--ac AC-n` naming an acceptance criterion in the owning spec. Evidence must resolve to a commit, an existing repo path or an `https://` URL. The verb requires a reason and name, appends the refutation event and its `audited-refuted` disposition row together, and updates the snapshot. A rerun adds nothing. APPROVE, REJECT, INVALID and `hard_block` slots cannot be refuted. A changed finding text or newer round needs a new discharge. Completion records the durable `panel_refuted` audit.

## Escape hatch

Skipping the panel gate entirely requires a **human**. A user sets `MINDSPEC_SKIP_PANEL=1` in their own shell environment before launching the session; the in-binary `mindspec complete` gate reads that environment and passes the gate, emitting an audited Warn, and `mindspec complete` records `panel_gate_skipped: true` + timestamp on the bead metadata. The variable is env-only and never pasted into a command line — a blocked agent cannot set it via a Bash-line prefix (the gate never consults the command string for the hatch). This is the only place `MINDSPEC_SKIP_PANEL` is documented; the gate's Block message deliberately never prints a paste-able skip incantation.

## Anti-patterns

- PASS requires a fresh and intact panel, all roster slots present, 0 REJECT, 0 hard_block, 0 INVALID and 0 unresolved REQUEST_CHANGES; each finding must be fixed, refuted or effectively deferred.
- Don't pass raw verdict JSONs to the fix subagent — dedupe first. N verdicts × ~3 items each = ~3N lines of duplicated asks otherwise.
- Don't ignore `confidence`. A 0.96 REQUEST_CHANGES from one slot should outweigh a 0.70 APPROVE from another. `mindspec panel tally`'s printed table carries `verdict`/`hard_block` only, not `confidence` — read it from the underlying `<slot>-round-<N>.json` files when weighing verdicts, and note the weighting in the report.
- Don't drop a REQUEST_CHANGES because "only one reviewer flagged it". A single empirically-grounded objection can be load-bearing — verify the claim before discarding — and the gate now BLOCKS on any unresolved REQUEST_CHANGES; the only per-slot escape is the audited refutation procedure (the After a halt — recovery, item 5).
- Don't satisfy an artifact-gate HARD block with a PR-body edit. The artifact must exist at the named path.

## Then

Decision-dependent:
- `merge` → `/ms-bead-cycle` merge terminal (`mindspec complete <id> "<summary>"`, hook-gated)
- `fix` → `/ms-bead-fix`
- `halt` → the After a halt — recovery; return to user

## Defer a finding

Run `mindspec panel defer <panel-slug> --slot R2 --round N --finding 'ccr[0]' --bead <follow-up> --by "<name>"`.
Omit `--finding` to cover all findings in the slot.
Use an open follow-up outside the owning epic and all its descendants.
Add a `discovered-from` edge to the owning epic or reviewed bead.
Put each deferred finding in the follow-up description.
Spec approval and adhoc panels cannot defer findings.
Fix the finding or use `mindspec panel refute` on these panels.
The refute and defer verbs write the `audited-refuted` and `confirmed-deferred` rows.
Do not append these rows by hand.
Completion records the effective deferrals in the durable `panel_deferred` audit.
Completion and implementation approval check the follow-up again.
A closed, missing, re-parented or changed follow-up blocks the gate.
An applicable refutation supersedes a deferral.
