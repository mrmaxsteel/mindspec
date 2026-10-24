---
name: ms-spec-final-review
description: Review the whole spec branch after the last bead and before implementation approval
---

# Spec Final Review

## Inputs

Use `pr-number`, `spec-slug` and `merge-base-against` (default `origin/main`).
Run this panel once after the last bead merges and before `/ms-impl-approve`.

## Steps

1. Run `git fetch origin`. Read the cumulative diff against the selected base.
Record the changed paths, totals, merged commits and known fixer deviations.
2. Invoke `/ms-panel-run` with gate `final_review` and target `spec/<spec-slug>`.
First write the brief to an absolute scratch file with Summary, Files in Scope,
Prior-Round Asks and Lens assignments per slot. Include the PR link, cumulative
context and fixer deviations. Pass its path as `brief_file` to `/ms-panel-run`.
The binary creates the registration and copies the filled brief to BRIEF.md.
3. Use `mindspec panel checkout <panel-slug> --slot <slot>` for isolated reviewer work.
Run tests and mutations in that slot's `tree/`. Use the slot directory for scratch.
The adapter uses `panel prompt` and dispatches the configured model for each slot.
Review the cumulative scope, contracts between beads, current main integration, PR accuracy, release artifacts and operator instructions.
4. Apply `review.ci-parity`: run the project's `commands.ci` verbatim, else `commands.test`, in the slot tree.
If neither key is set, state that limit in the rationale.
Apply `panel.submit`, `panel.isolated-checkout`, `panel.scratch-in-slot`, `review.no-vote` and `panel.artifact-gate`.
5. Invoke `/ms-panel-tally` for the decision and consolidated findings.

CI must also run on the PR. A local result does not replace the PR's CI result.

## Final-review fixes

After verify exits 0, use `/ms-bead-fix` with the spec worktree as the target.
The fixer uses `panel checkout --fixer` for scratch and works in the spec worktree.
The implement-mode commit gate permits a scoped final-review fix with this escape hatch:
```bash
MINDSPEC_ALLOW_MAIN=1 git commit -m "<review fix>"
```
Use it only for final-review corrections. Run the next panel round after the fix.

## Then

PASS with artifact gates clear: `/ms-impl-approve <spec-slug>`.
REQUEST_CHANGES: consolidate, fix and review the next round.
REJECT or a hard block: halt and report the panel state.
