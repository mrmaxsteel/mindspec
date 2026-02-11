---
description: Check the current MindSpec mode and active specification
---

# Spec Status Workflow

Use this workflow to check the current operational mode and active specification.

## Trigger

User invokes `/spec-status` or asks about current mode/spec.

## Steps

### 1. Determine Current State

Check for:
- Active spec files in `docs/specs/`
- Spec approval status
- Implementation beads in Beads (if any)
- Plan approval status
- Active worktrees

### 2. Determine Mode

| Condition | Mode |
|:----------|:-----|
| No approved spec | **Spec Mode** |
| Approved spec, no approved plan | **Plan Mode** |
| Approved spec + approved plan + active bead | **Implementation Mode** |

### 3. Report Status

#### If in Spec Mode:

> **Mode**: Spec Mode
>
> **Active Spec**: `<id>` — <goal summary>
>
> **Approval Status**: <DRAFT | PENDING_REVIEW>
>
> **Impacted Domains**: <domain list>
>
> **Acceptance Criteria**: <N> defined
>
> ---
>
> **Next steps**:
> - Complete requirements, domains, and acceptance criteria
> - Use `/spec-approve` when ready for planning

#### If in Plan Mode:

> **Mode**: Plan Mode
>
> **Active Spec**: `<id>` — <goal summary> (APPROVED)
>
> **Implementation Beads**: <N> defined
>
> **ADRs Cited**: <list>
>
> ---
>
> **Next steps**:
> - Define implementation beads with verification steps
> - Use `/plan-approve` when ready for implementation

#### If in Implementation Mode:

> **Mode**: Implementation Mode
>
> **Active Spec**: `<id>` — <goal summary> (APPROVED)
>
> **Active Bead**: <bead-id> — <scope summary>
>
> **Worktree**: <worktree path>
>
> **Verification Steps**: <completed>/<total> complete
>
> ---
>
> **Reminders**:
> - Stay within bead scope
> - Update docs alongside code
> - Mark verification steps as complete when done

### 4. List Recent Specs (Optional)

If user asks, list specs in `docs/specs/`:

| Spec ID | Status | Domains | Criteria |
|:--------|:-------|:--------|:---------|
| 001-skeleton | DRAFT | core | 5 defined |
| 002-glossary | DRAFT | context-system | 5 defined |

---

## Notes

- This workflow is read-only; it doesn't change state
- Use `/spec-init` to start a new spec
- Use `/spec-approve` to transition Spec → Plan
- Use `/plan-approve` to transition Plan → Implementation
