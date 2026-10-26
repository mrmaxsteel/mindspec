---
name: ms-panel-tally
description: Read the panel decision, consolidate findings and report recovery actions
---

# Tally Panel Verdicts

## Steps

1. Run `mindspec panel tally <panel-slug>` or read its output from the workflow.
The binary resolves the current round and roster.
PASS requires a fresh and intact panel, all roster slots present, 0 REJECT, 0 hard_block, 0 INVALID and 0 unresolved REQUEST_CHANGES.
`mindspec panel tally` writes the coverage manifest only when the panel passes.
2. Consolidate `concrete_changes_required` by defect and criticality for `/ms-bead-fix`.
Keep distinct asks and evidence. Save `<panel-dir>/consolidated-round-<N>.md`.
Apply `review.reproduce` before fix dispatch or refutation. Apply `review.no-vote` and `panel.artifact-gate`.
3. Append one disposition per distinct finding that no discharge verb captured.
Use stdin from a quoted heredoc. Do not interpolate finding text into shell arguments.
The `--spec` flag takes the full spec id. The JSON `spec` field takes its number.
Do not supply an `id`. The verb derives it for an idempotent retry.
```bash
mindspec panel disposition append --spec <spec-id> --panel <panel-slug> --data - <<'JSON'
{"record":"disposition","spec":"<number>","gate":"<gate>","panel":"<panel-slug>","reviewer":"<slot>","model":"<model>","severity":"<blocking|major|minor>","summary":"<summary>","convergent_with":[],"disposition":"<confirmed-fixed|confirmed-scope-trim|deferred|false-contamination>","evidence_ref":"<reference>","note":"<note>","created_at":"<RFC-3339>","backfilled":false}
JSON
```
`reviewer` and each `convergent_with` value must match a roster slot id.
Run `mindspec panel disposition check --spec <spec-id> --panel <panel-slug>` to check coverage.
4. Report the tally decision and consolidated path to `/ms-bead-cycle`.
Take family counts and `replaced` annotations from `mindspec panel verify <panel-slug> --json`.
Apply `panel.fix-after-final` and `panel.one-fixer` before a fix dispatch.

## After a halt — recovery

`mindspec instruct --panel-state` lists the panel state.
For a stale bead panel, run `mindspec panel create <slug> --spec <spec-id> --gate bead --target bead/<bead-id> --bead <bead-id> --round <N+1> --brief-file <absolute path to a fresh brief>`.
Use the recorded gate and target for other panels. Omit `--spec` for adhoc panels.
`mindspec panel rebind <slug> --reason "<who>: <why>"` accepts a passing panel when changed paths need no new review.
`mindspec panel abandon <panel-slug> --reason "<why>" --by "<name>"` records an audited exit from the panel flow.
`mindspec panel refute <panel-slug> --slot <slot> --round N --finding 'ccr[0]' --ground disproven --reason "<why>" --evidence <reference> --by "<name>"` records a refutation.
A REJECT or missing required measurement artifact needs operator action. A text edit does not supply an artifact.

## Escape hatch

Only a human can set `MINDSPEC_SKIP_PANEL=1` in the session environment before launch.
An agent must not set it on a command line. Completion records the audited skip.

## Defer a finding

Run `mindspec panel defer <panel-slug> --slot <slot> --round N --finding 'ccr[0]' --bead <follow-up> --by "<name>"`.
Omit `--finding` for the whole slot. Use an open follow-up outside the owning epic and its descendants, with a discovered-from edge and each finding in its description.
Spec approval and adhoc panels cannot defer. Use a fix or refutation. The verbs write their own disposition rows.

## Then

PASS with artifact gates clear: return to `/ms-bead-cycle` for completion.
BLOCK with fixes: return to `/ms-bead-fix`. Otherwise halt and report the panel state.
