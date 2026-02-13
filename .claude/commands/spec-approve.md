---
description: Request approval to transition from Spec Mode to Plan Mode
---

# Spec Approval Workflow

## Trigger

User invokes `/spec-approve` or expresses readiness to plan.

## Steps

1. **Identify the active spec** — check `mindspec state show` or ask the user which spec to approve.

2. **Confirm with the user** — briefly summarize the spec and ask:
   > Do you approve this spec for planning? (yes/no)

3. **On approval** — run the CLI command:
   ```bash
   mindspec approve spec <id>
   ```
   This validates the spec, updates frontmatter to APPROVED, resolves the Beads gate,
   sets state to Plan Mode, and emits plan mode guidance.

4. **Immediately begin planning** — the approval IS the authorization to start. Proceed to:
   - Review domain docs and accepted ADRs for impacted domains
   - Decompose spec into implementation beads
   - Draft `docs/specs/<id>/plan.md`
   - When the plan draft is complete, advise the user to use `/plan-approve`

5. **On rejection** — inform user the spec remains in DRAFT. Ask what changes they want.

## Notes

- This is a human gate — the user must explicitly confirm
- `mindspec approve spec` owns all procedural logic (validation, frontmatter, gate, state, instruct)
- Once approved, planning starts automatically (no second confirmation)
