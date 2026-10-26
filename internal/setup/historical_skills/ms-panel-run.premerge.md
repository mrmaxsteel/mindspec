---
name: ms-panel-run
description: Create a review panel and dispatch each slot through its configured runner
---

# Run a Review Panel

## Inputs

Use `panel-slug`, `spec`, `gate`, `target`, `bead-id` and `round` from the caller.
The binary resolves the roster, lenses, acceptance question and verdict contract.

## Prepare the brief

Before starting a workflow or creating a panel, write a filled brief to an absolute
scratch file. Include Summary, Files in Scope, Prior-Round Asks (or "none"), and
Lens assignments per slot. Include known implementation or fixer deviations.
Use the configured roster to assign lenses, and confirm it with verify after creation.
Remove every `TODO(skill)` stub. Pass this absolute path as `brief_file`.
For a later round, write a fresh brief with the previous asks and their outcomes.

## Runner dispatch

For `runner: claude-code-workflow`, invoke `/ms-panel` once with `{slug, spec, gate, target, bead_id, round, brief_file}`.
Use the returned verify and tally output. If workflows are unavailable, use the steps below.
For `runner: claude-code-skills` and `runner: external`, use the steps below.

## Step 0 — create

Run `mindspec panel create <panel-slug> --spec <spec-id> --gate <gate> --target <ref> --round <N> --brief-file <absolute-brief-file>`.
For a bead panel, add `--bead <bead-id>`. Keep the reported panel directory path.
For an adhoc panel, run `mindspec panel create <panel-slug> --gate adhoc --target <ref> --brief-file <absolute-brief-file>`.
Its artifacts are at `<repo>/.mindspec/reviews/<panel-slug>/`.

## Dispatch and wait

Run `mindspec panel verify <panel-slug> --json`. Read each slot's effective kind from this output.
Use the current roster, not a preview from `config resolve`.
Act on each slot with this table. Start process launches in the background.
"Once" means one retry per slot and round. After that retry, report the next non-final, non-running status.

| Status | Process slot | Harness slot |
|:-------|:-------------|:-------------|
| `missing` | `panel launch` in the background | redispatch |
| `running` | wait | wait while the agent runs; if the agent returned and the slot is still running, redispatch once, then report |
| `draft` | `panel launch` once more, then report | redispatch once, then report |
| `unsubmitted` | `panel launch` once more, then report | redispatch once, then report |
| `void` | `panel launch` once more, then report | redispatch once, then report |
| `launch_failed` | `panel launch` once more, then report | redispatch once, then report |
| `malformed` | `panel launch` once more, then report | redispatch once, then report |
| `quota_wall` | report | with `handoff_pending`: redispatch; else report |
| `damaged` | `panel recover`, then report | report |

For a process slot, run `mindspec panel launch <panel-slug> <slot>`.
For recovery, run `mindspec panel recover <panel-slug> <slot> --reason "<reason>"`.
A harness substitute uses the harness column in the latest verify output.
For each harness redispatch:
1. Run `mindspec panel checkout <panel-slug> --slot <slot> --dispatch`. Add `--replace` after a final verdict other than VOID.
2. Run `mindspec panel prompt <panel-slug> --slot <slot>`.
If checkout or prompt fails, report its reason for that slot and stop that branch.
A setup failure does not use the reviewer retry. Continue the other slots.
3. Dispatch that prompt once with the harness agent tool, in the slot tree.
Pass the slot's effective model to the agent tool. If it cannot run that model, report the slot.
When model equals family, use the harness default model. Keep the slot model as the verdict label.
The prompt gives the absolute attempt path and the `mindspec panel submit --file <path>` command.
Run verify again after each process or agent ends. On exit 0, invoke `/ms-panel-tally`.

## Rules and decision

Apply `panel.isolated-checkout`, `panel.scratch-in-slot`, `panel.submit`, `review.classifier-redispatch` and `review.no-vote`.
Use `mindspec rules show <id>` for conduct. Use `panel.verbs-only` for panel artifacts.
`mindspec panel tally` writes the coverage manifest only when the panel passes. PASS requires a fresh and intact panel, all roster slots present, 0 REJECT, 0 hard_block, 0 INVALID and 0 unresolved REQUEST_CHANGES.
Resolve findings with fixes, `mindspec panel refute` or `mindspec panel defer`. Use `mindspec panel abandon` when work leaves the panel flow.
