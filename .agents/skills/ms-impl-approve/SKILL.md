---
name: ms-impl-approve
description: Approve implementation and close out the spec lifecycle
---

# Implementation Approval

1. Identify the active spec via `mindspec state show`
2. If not in review mode, run `mindspec complete` first to transition
3. Run `mindspec approve impl <id>` in the terminal (verifies review mode, transitions to idle, emits guidance)
4. If approval fails, show the error and help the user resolve it
5. On success: run the session close protocol:
   - `bd sync`
   - `git add` all changed files (state, specs, recordings, beads)
   - `git commit`
   - `bd sync`
   - `git push`

## If approval refuses: a bead was closed without `mindspec complete`

`mindspec impl approve` REFUSES to finalize (non-zero exit; nothing is
closed, written, merged, or pushed) when any closed bead under the spec's
epic is DETECTABLY unsettled: its `bead/<id>` branch is still an unmerged
non-ancestor of the spec branch — it was closed without `mindspec
complete` and never merged — or it carries a durable refutation
obligation not covered by a durable `panel_refuted` record. The refusal
names the bead. Recovery: run `mindspec complete <bead>` — it tolerates
the already-closed bead, re-runs the full panel gate (blocking until any
unresolved REQUEST_CHANGES is resolved or durably refuted), merges the
bead branch, and the orphan/obligation gate no longer blocks `mindspec
impl approve` (which then finalizes subject to its other gates). If the
bead's `bead/<id>` branch no longer exists, the refusal names the
restoration prerequisite instead: restore the branch ref (or settle the
obligation out-of-band) so `mindspec complete <bead>` can reach its
reconciliation step. Skip/abandon hatches do not bypass this refusal.
Disclosed residual: a bead branch already raw-`git merge`d and then
`bd close`d, carrying no durable refutation obligation, leaves nothing
for this gate to detect — see ADR-0037.
