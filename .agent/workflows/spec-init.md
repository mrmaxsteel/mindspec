---
description: Initialize a new mindspec specification
---

# Initialize Spec Workflow

Use this workflow to start a new specification in Spec Mode.

## Trigger

User invokes `/spec-init` or expresses intent to start a new feature/change.

## Steps

### 1. Gather Information

Ask the user for:
- **Spec ID**: A slug like `002-feature-name` (check `docs/specs/` for next available number)
- **Title**: Brief description of the feature or change
- **Context**: Any relevant background or requirements

### 2. Create Directory Structure

Create the spec directory:

```
docs/specs/<id>/
├── spec.md          # Main specification
├── tasks.json       # Task graph (empty template)
└── context-pack.md  # Generated context (placeholder)
```

### 3. Generate spec.md Template

Create `docs/specs/<id>/spec.md` with this structure:

```markdown
# Spec <ID>: <Title>

## Goal

<Brief description of what this spec achieves>

## Background

<Context, motivation, and any relevant prior decisions>

## Requirements

1. <Requirement 1>
2. <Requirement 2>
3. ...

## Scope

### In Scope
- <File or component 1>
- <File or component 2>

### Out of Scope
- <Explicitly excluded items>

## Acceptance Criteria

- [ ] <Specific, measurable criterion 1>
- [ ] <Specific, measurable criterion 2>
- [ ] <Specific, measurable criterion 3>
- [ ] ...

## Validation Proofs

- `<command 1>`: <Expected outcome>
- `<command 2>`: <Expected outcome>

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: —
```

### 4. Generate tasks.json Template

Create `docs/specs/<id>/tasks.json`:

```json
{
  "specId": "<id>",
  "tasks": []
}
```

### 5. Update Active Spec Tracker

Create or update `.mindspec/current-spec.json`:

```json
{
  "activeSpec": "<id>",
  "mode": "spec",
  "lastUpdated": "<current ISO timestamp>"
}
```

### 6. Inform User

Tell the user:

> ✅ **Spec initialized**: `docs/specs/<id>/spec.md`
> 
> You are now in **Spec Mode**. 
> 
> **Next steps:**
> 1. Fill in the Goal, Requirements, and Scope sections
> 2. Define specific, measurable Acceptance Criteria
> 3. When ready, use `/spec-approve` to request approval

### 7. Open Spec for Editing

If possible, open the spec file in the editor for the user.

---

## Notes

- Spec IDs should be sequential: `001`, `002`, etc.
- The slug should be descriptive: `002-memory-service`, `003-context-pack-generation`
- All work in Spec Mode is markdown-only; no code changes permitted
