---
description: Request approval to transition from Plan Mode to Implementation Mode
---

# Plan Approval Workflow

Use this workflow to gate the transition from Plan Mode to Implementation Mode.

## Trigger

User invokes `/plan-approve` or expresses readiness to implement.

## Steps

### 1. Identify Active Spec and Plan

Find the active spec and its associated implementation beads. If unclear, ask the user.

### 2. Validate Plan Quality

Check each quality criterion:

| Criterion | Check |
|:----------|:------|
| **Beads defined** | At least one implementation bead exists |
| **Scope bounded** | Each bead has a small, defined scope |
| **Micro-plans** | Each bead has 3-7 step micro-plan |
| **Verification steps** | Each bead has explicit verification steps |
| **Dependencies explicit** | Inter-bead dependencies are declared |
| **ADRs cited** | Each bead cites the ADRs it relies on |
| **Coverage** | All spec requirements are covered by at least one bead |

### 3. Handle Validation Failure

If any checks fail:

> **Plan not ready for approval**
>
> The following issues need to be addressed:
> - <Issue 1>
> - <Issue 2>
>
> Please update the plan and try again.

Remain in Plan Mode.

### 4. Present Plan Summary

If validation passes, present a summary:

> **Plan Summary for Spec <id>**
>
> **Implementation Beads** (<N> total):
>
> | Bead | Scope | Deps | Verification Steps |
> |:-----|:------|:-----|:-------------------|
> | <bead-1> | <scope> | <deps> | <N steps> |
> | <bead-2> | <scope> | <deps> | <N steps> |
>
> **ADRs Cited**: <list>
>
> **Ready to approve and begin implementation?**

### 5. Request Explicit Approval

Ask the user:

> Do you approve this plan for implementation? (yes/no)

### 6. On Approval

If user approves:

1. Inform user:
   > **Plan approved!**
   >
   > You are now in **Implementation Mode**.
   >
   > **Next steps:**
   > 1. Pick the first bead with no unresolved dependencies
   > 2. Create a worktree for the bead
   > 3. Load the context pack
   > 4. Implement within the bead's scope
   > 5. Verify, update docs, and close the bead

### 7. On Rejection

If user declines:

> Plan remains unapproved.
>
> What changes would you like to make?

Remain in Plan Mode.

---

## Notes

- This is a human gate — the user must explicitly confirm
- Each bead should be implementable independently (respecting dependencies)
- Implementation work must happen in isolated worktrees
