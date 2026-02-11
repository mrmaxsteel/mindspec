---
description: Request approval to transition from Spec Mode to Plan Mode
---

# Spec Approval Workflow

Use this workflow to gate the transition from Spec Mode to Plan Mode.

## Trigger

User invokes `/spec-approve` or expresses readiness to plan.

## Steps

### 1. Identify Active Spec

Find the active spec. If unclear, ask the user which spec they want to approve.

### 2. Read and Parse Spec

Read `docs/specs/<id>/spec.md` and extract:
- Goal section
- Impacted domains
- ADR touchpoints
- Requirements list
- Scope boundaries
- Acceptance Criteria
- Open Questions
- Current Approval status

### 3. Validate Spec Quality

Check each quality criterion:

| Criterion | Check |
|:----------|:------|
| **Goal defined** | Goal section is not empty or placeholder |
| **Domains declared** | At least one impacted domain listed |
| **ADR touchpoints** | Relevant ADRs identified (or explicitly "none") |
| **Requirements listed** | At least 2 concrete requirements |
| **Scope bounded** | In Scope and Out of Scope sections filled |
| **Criteria count** | At least 3 acceptance criteria defined |
| **Criteria quality** | Each criterion is specific and measurable |
| **Not vague** | No criteria like "works correctly" or "is fast" |
| **Open questions resolved** | All open questions are resolved or removed |

### 4. Handle Validation Failure

If any checks fail:

> **Spec not ready for approval**
>
> The following issues need to be addressed:
> - <Issue 1>
> - <Issue 2>
>
> Please update `docs/specs/<id>/spec.md` and try again.

Remain in Spec Mode.

### 5. Present Spec Summary

If validation passes, present a summary:

> **Spec Summary: <id>**
>
> **Goal**: <goal summary>
>
> **Impacted Domains**: <domain list>
>
> **ADR Touchpoints**: <ADR list>
>
> **Scope**: <key files/components>
>
> **Acceptance Criteria** (<N> items):
> - <criterion 1>
> - <criterion 2>
> - ...
>
> **Ready to approve and begin planning?**

### 6. Request Explicit Approval

Ask the user:

> Do you approve this spec for planning? (yes/no)

### 7. On Approval

If user approves:

1. Update `docs/specs/<id>/spec.md` Approval section:
   ```markdown
   ## Approval

   - **Status**: APPROVED
   - **Approved By**: user
   - **Approval Date**: <today's date>
   - **Notes**: Approved via /spec-approve workflow
   ```

2. Inform user:
   > **Spec approved!**
   >
   > You are now in **Plan Mode**.
   >
   > **Next steps:**
   > 1. Review domain docs and accepted ADRs for impacted domains
   > 2. Check Context Map for neighboring context contracts
   > 3. Decompose spec into implementation beads (bounded work chunks)
   > 4. Define verification steps for each bead
   > 5. When ready, use `/plan-approve` to request plan approval

### 8. On Rejection

If user declines:

> Spec remains in **DRAFT** status.
>
> What changes would you like to make before approval?

Remain in Spec Mode.

---

## Notes

- This is a human gate — the user must explicitly confirm
- Approval is recorded in git (the spec file itself)
- Re-approval is required if the spec is modified after approval
- This transitions to Plan Mode, not Implementation Mode
