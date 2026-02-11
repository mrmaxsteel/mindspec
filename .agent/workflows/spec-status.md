---
description: Check the current mindspec mode and active specification
---

# Spec Status Workflow

Use this workflow to check the current operational mode and active specification.

## Trigger

User invokes `/spec-status` or asks about current mode/spec.

## Steps

### 1. Check State File

Read `.mindspec/current-spec.json`.

If file doesn't exist:
> 📭 **No active spec**
> 
> Use `/spec-init <id>` to start a new specification.
> 
> **Current mode**: Spec Mode (no approved spec)

### 2. Parse Current State

Extract from the state file:
- `activeSpec`: The current spec ID
- `mode`: Either "spec" or "implementation"
- `lastUpdated`: Timestamp of last state change

### 3. Read Spec Details

If an active spec exists, read `docs/specs/<activeSpec>/spec.md`:
- Extract the Goal (first line summary)
- Count acceptance criteria (total and completed)
- Get current Approval status

### 4. Report Status

Present the status:

#### If in Spec Mode:

> 📝 **Mode**: Spec Mode
> 
> **Active Spec**: `<id>` — <goal summary>
> 
> **Approval Status**: <DRAFT | PENDING_REVIEW>
> 
> **Acceptance Criteria**: <N> defined
> 
> **Last Updated**: <timestamp>
> 
> ---
> 
> **Next steps**:
> - Complete requirements and acceptance criteria
> - Use `/spec-approve` when ready for implementation

#### If in Implementation Mode:

> 🔨 **Mode**: Implementation Mode
> 
> **Active Spec**: `<id>` — <goal summary>
> 
> **Approval Status**: APPROVED
> 
> **Acceptance Criteria**: <completed>/<total> complete
> 
> **Last Updated**: <timestamp>
> 
> ---
> 
> **Reminders**:
> - Stay within spec scope
> - Update docs alongside code
> - Mark acceptance criteria as complete when done

### 5. List Recent Specs (Optional)

If user asks, list specs in `docs/specs/`:

| Spec ID | Status | Criteria |
| :------ | :----- | :------- |
| 001-skeleton | DRAFT | 5 defined |
| 002-memory | APPROVED | 3/7 complete |

---

## Notes

- This workflow is read-only; it doesn't change state
- Use `/spec-init` to change active spec
- Use `/spec-approve` to transition modes
