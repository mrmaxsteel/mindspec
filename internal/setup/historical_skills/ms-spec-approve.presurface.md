---
name: ms-spec-approve
description: Approve a spec and transition to Plan Mode
managed-by: mindspec
---

# Spec Approval

1. Identify the active spec via `mindspec state show`
2. Run a pre-approve review panel: invoke `/ms-panel-run` with
   `mindspec panel create <slug> --spec <id> --gate spec_approve --target <ref>`
   against the spec DOCUMENT (not a code diff), and do NOT run
   `mindspec spec approve` until `/ms-panel-tally` returns Allow — see
   `/ms-panel-run`'s document-gate lens defaults for the review lens
   assignments (ADR-0044). This step is guidance, not a mechanized
   preflight: binary enforcement of these gates is a separate, tracked
   enhancement.
3. Run `mindspec spec approve <id>` in the terminal (validates, closes the spec-approve gate, generates context pack, sets state, emits guidance)
4. If approval fails, show the validation errors and help the user fix them
5. If validation flags the unchecked `grill deferred: headless session` marker:
   run /ms-spec-grill interactively and resolve it, or — in an orchestrated run
   whose spec review panel has PASSED — check the box, citing the panel.
6. On success: immediately begin planning (the approval is the authorization)
