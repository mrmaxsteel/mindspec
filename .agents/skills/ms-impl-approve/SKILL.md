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
epic was closed without `mindspec complete` — such a bead lacks proof of
panel settlement, and only `mindspec complete` settles it. The refusal
names the bead. Recovery: run `mindspec complete <bead>` — it tolerates
the already-closed bead, re-runs the full panel gate (blocking until any
unresolved REQUEST_CHANGES is resolved or durably refuted), merges the
bead branch, and then `mindspec impl approve` succeeds. If the bead's
`bead/<id>` branch no longer exists, the refusal names the restoration
prerequisite instead: restore the branch ref (or settle the obligation
out-of-band) so `mindspec complete <bead>` can reach its reconciliation
step. Skip/abandon hatches do not bypass this refusal.
