---
name: ms-spec-approve
description: Approve a spec and transition to Plan Mode
managed-by: mindspec
---

# Spec Approval

1. Identify the active spec via `mindspec state show`
2. Run `mindspec spec approve <id>` in the terminal (validates, closes the spec-approve gate, generates context pack, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. If validation flags the unchecked `grill deferred: headless session` marker:
   run /ms-spec-grill interactively and resolve it, or — in an orchestrated run
   whose spec review panel has PASSED — check the box, citing the panel.
5. On success: immediately begin planning (the approval is the authorization)
