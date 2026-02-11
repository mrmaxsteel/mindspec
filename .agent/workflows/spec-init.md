---
description: Initialize a new MindSpec specification
---

# Initialize Spec Workflow

Use this workflow to start a new specification in Spec Mode.

## Trigger

User invokes `/spec-init` or expresses intent to start a new feature/change.

## Steps

### 1. Gather Information

Ask the user for:
- **Spec ID**: A slug like `004-feature-name` (check `docs/specs/` for next available number)
- **Title**: Brief description of the feature or change
- **Context**: Any relevant background or requirements

### 2. Create Directory Structure

Create the spec directory:

```
docs/specs/<id>/
├── spec.md          # Main specification
└── context-pack.md  # Generated context (placeholder)
```

### 3. Generate spec.md from Template

Copy `docs/templates/spec.md` to `docs/specs/<id>/spec.md` and fill in the `<ID>` and `<Title>` placeholders.

### 4. Inform User

Tell the user:

> Spec initialized: `docs/specs/<id>/spec.md`
>
> You are now in **Spec Mode**.
>
> **Next steps:**
> 1. Fill in the Goal, Requirements, and Scope sections
> 2. Declare impacted domains and ADR touchpoints
> 3. Define specific, measurable Acceptance Criteria
> 4. Resolve all Open Questions
> 5. When ready, use `/spec-approve` to request approval

### 5. Open Spec for Editing

If possible, open the spec file in the editor for the user.

---

## Notes

- Spec IDs should be sequential: `001`, `002`, etc.
- The slug should be descriptive: `004-beads-integration`, `005-worktree-lifecycle`
- All work in Spec Mode is markdown-only; no code changes permitted
- Specs must declare impacted domains and ADR touchpoints before approval
