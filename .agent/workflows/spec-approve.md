---
description: Request approval to transition from Spec Mode to Implementation Mode
---

# Spec Approval Workflow

Use this workflow to gate the transition from Spec Mode (markdown-only) to Implementation Mode (code permitted).

## Trigger

User invokes `/spec-approve` or expresses readiness to implement.

## Steps

### 1. Identify Active Spec

Check `.mindspec/current-spec.json` for the active spec ID.

If no active spec:
- Ask user which spec they want to approve
- Or suggest using `/spec-init` first

### 2. Read and Parse Spec

Read `docs/specs/<id>/spec.md` and extract:
- Goal section
- Requirements list
- Scope boundaries
- Acceptance Criteria
- Current Approval status

### 3. Validate Spec Quality

Check each quality criterion:

| Criterion | Check |
| :-------- | :---- |
| **Goal defined** | Goal section is not empty or placeholder |
| **Requirements listed** | At least 2 concrete requirements |
| **Scope bounded** | In Scope and Out of Scope sections filled |
| **Criteria count** | At least 3 acceptance criteria defined |
| **Criteria quality** | Each criterion is specific and measurable |
| **Not vague** | No criteria like "works correctly" or "is fast" |

### 4. Handle Validation Failure

If any checks fail:

> ⚠️ **Spec not ready for approval**
> 
> The following issues need to be addressed:
> - [ ] <Issue 1>
> - [ ] <Issue 2>
> 
> Please update `docs/specs/<id>/spec.md` and try again.

Remain in Spec Mode.

### 5. Present Spec Summary

If validation passes, present a summary:

> 📋 **Spec Summary: <id>**
> 
> **Goal**: <goal summary>
> 
> **Scope**: <key files/components>
> 
> **Acceptance Criteria** (<N> items):
> - <criterion 1>
> - <criterion 2>
> - ...
> 
> **Ready to approve and begin implementation?**

### 6. Request Explicit Approval

Ask the user:

> Do you approve this spec for implementation? (yes/no)

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

2. Update `.mindspec/current-spec.json`:
   ```json
   {
     "activeSpec": "<id>",
     "mode": "implementation",
     "lastUpdated": "<current ISO timestamp>"
   }
   ```

3. Inform user:
   > ✅ **Spec approved!**
   > 
   > You are now in **Implementation Mode**.
   > 
   > You may begin writing code. Remember:
   > - Stay within the defined scope
   > - Update docs alongside code changes
   > - Verify against acceptance criteria when done

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
- The mode state in `.mindspec/` is for local convenience
- Re-approval is required if the spec is modified after approval
