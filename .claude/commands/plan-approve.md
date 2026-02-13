---
description: Request approval to transition from Plan Mode to Implementation Mode
---

# Plan Approval Workflow

## Trigger

User invokes `/plan-approve` or expresses readiness to implement.

## Steps

1. **Identify the active spec and plan** — check `mindspec state show` or ask the user.

2. **Confirm with the user** — briefly summarize the plan (beads, scope, deps) and ask:
   > Do you approve this plan for implementation? (yes/no)

3. **On approval** — run the CLI command:
   ```bash
   mindspec approve plan <id>
   ```
   This validates the plan, updates frontmatter to Approved, resolves the Beads gate,
   sets state, and emits guidance. Then advise the user:
   > Run `mindspec next` to claim the first ready bead and enter Implementation Mode.

4. **On rejection** — inform user the plan remains unapproved. Ask what changes they want.

## Notes

- This is a human gate — the user must explicitly confirm
- `mindspec approve plan` owns all procedural logic (validation, frontmatter, gate, state, instruct)
- After approval, `mindspec next` claims a bead and transitions to Implementation Mode
