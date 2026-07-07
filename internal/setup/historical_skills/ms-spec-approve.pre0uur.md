---
name: ms-spec-approve
description: Approve a spec and transition to Plan Mode
managed-by: mindspec
---

# Spec Approval

1. Identify the active spec via `mindspec state show`
2. Run `mindspec spec approve <id>` in the terminal (validates, closes the spec-approve gate, generates context pack, sets state, emits guidance)
3. If approval fails, show the validation errors and help the user fix them
4. On success: immediately begin planning (the approval is the authorization)
