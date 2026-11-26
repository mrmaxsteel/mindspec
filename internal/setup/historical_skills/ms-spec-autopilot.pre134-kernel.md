---
name: ms-spec-autopilot
description: Execute the binary's plan-approved spec workflow through final approval
---

# Spec Autopilot

Use `/ms-bead-cycle` to execute the binary's workflow for an approved plan.
The binary owns work selection, rounds, decisions and final approval readiness.
Apply the repository's Bead-loop guardrails.

## Inputs

- `spec-slug` (optional): select the spec. Ask the user when active specs are ambiguous.

## Sequence

1. Run `/ms-bead-cycle` for the spec. It loops `mindspec next-action --apply`,
   `lease`, dispatch, `submit` and each returned `run` command.
2. Dispatch only returned actions. Parallelism is across the configured panel reviewers + the impl subagent.
   Work items run serially. Preserve the binary's plan order.
3. Return each open decision to the user. Show the listed options and the affected gate.
   Submit the selected answer, then read the binary output again.
4. Execute the final-review and `mindspec impl approve` entries when returned.
   Use `/ms-spec-final-review` for review context and `/ms-impl-approve` for approval.
   The cycle owns panel creation, leases, submissions and decisions.
   Skip adapter commands that the current binary output has not authorised.
5. Exit when the binary reports `done`. Report the publication result.

## Conditions to return to the user

| Condition | Action |
|:----------|:-------|
| Open decision | Show its options; keep other eligible work under binary control |
| Failed merge | Preserve state and follow the completion recovery line |
| Dependency mismatch | Stop dispatch and report the mismatch |
| Binary reports `wait` | Report blockers and retry time; wait for the missing input |
| Binary reports `halted` | Report the durable halt and affected spec |
| Binary reports `indeterminate` | Surface the diagnostic and recovery line |

A rejected panel holds its own gate. Only a submitted `halt` answer stops every work item.

## Reporting

After a merge, report the work item, merge SHA, tests and deviations.
Report the panel outcome as printed. For example: `<N>/<N> APPROVE`.
After final approval, report the completed spec and publication result.

## Guardrails

Agents do not merge or push their own work.
The orchestrator publishes once at the end of the spec.
Use the binary's returned commands for each transition.
