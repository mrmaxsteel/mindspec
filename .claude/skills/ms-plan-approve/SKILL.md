---
name: ms-plan-approve
description: Approve a plan and transition toward Implementation Mode
managed-by: mindspec
---

# Plan Approval

1. Identify the active spec/plan via `mindspec state show`
2. Run a pre-approve review panel: invoke `/ms-panel-run` with
   `mindspec panel create <slug> --spec <id> --gate plan_approve --target spec/<id>`
   against the plan DOCUMENT (not a code diff), and do NOT run
   `mindspec plan approve` until `/ms-panel-tally` returns PASS.
   The binary resolves the review lenses. `mindspec plan approve` refuses without a passing document panel.
   Gates without a panel only warn when `enforcement.panel_presence: warn`.
   For a spec that changes a gate, a schema, a trust boundary or a merge path,
   use reviewers from more than one family. `mindspec config resolve --gate plan_approve` shows the families.
3. Run `mindspec plan approve <id>` in the terminal (validates, closes the plan-approve gate, sets state, emits guidance)
4. If approval fails, show the validation errors and help the user fix them
5. On success: run `mindspec next` to claim the first bead and enter Implementation Mode
