---
name: ms-impl-approve
description: Approve implementation and close out the spec lifecycle
---

# Implementation Approval

1. Identify the active spec via `mindspec state show`
2. If not in review mode, run `mindspec complete` first to transition
3. Run `mindspec approve impl <id>` in the terminal (verifies review mode, transitions to idle, emits guidance)
4. If approval fails, show the error and help the user resolve it
5. On success: sync beads state:
   - `bd dolt push`
