---
description: Mindspec spec-mode vs implementation-mode enforcement
---

# Mindspec Mode Rules

These rules enforce the spec-driven development workflow where specifications must be approved before any code is written.

## Core Invariant

Before writing any code in `src/` or implementation directories, you MUST verify:

1. **Spec Exists**: A corresponding spec in `docs/specs/<id>/spec.md` exists
2. **Spec Approved**: The spec has `Status: APPROVED` in its Approval section
3. **Acceptance Criteria Defined**: All acceptance criteria are explicitly listed

If these conditions are not met, you are in **Spec Mode**: only create/modify markdown files.

---

## Spec Mode Behavior

When no approved spec exists for the current work:

### Permitted Actions
- Create/update `docs/specs/<id>/spec.md`
- Define acceptance criteria as checkable items
- Build task graph in `docs/specs/<id>/tasks.json`
- Generate context packs
- Update documentation in `docs/core/` or `docs/features/`
- Modify `GLOSSARY.md`
- Request human review when spec is ready

### Forbidden Actions
- Creating or modifying files in `src/`
- Creating or modifying files in `tests/`
- Changing build configuration that affects runtime
- Any implementation code

---

## Implementation Mode Transition

To transition from Spec Mode to Implementation Mode:

1. The user must explicitly approve the spec (via `/spec-approve` or direct confirmation)
2. Update the spec's Approval section:
   ```markdown
   ## Approval
   - **Status**: APPROVED
   - **Approved By**: @username
   - **Approval Date**: YYYY-MM-DD
   ```
3. Only then may code changes begin

---

## Mode Check Protocol

Before any tool call that creates or modifies code:

1. Identify the relevant spec ID from context or ask the user
2. Read `docs/specs/<id>/spec.md`
3. Check the Approval section for `Status: APPROVED`
4. If not approved:
   - Inform the user: "This spec is not yet approved. We're in Spec Mode."
   - Offer to help complete the spec or request approval
   - Do not proceed with code changes

---

## Active Spec Tracking

Check `.mindspec/current-spec.json` for the active spec:

```json
{
  "activeSpec": "<spec-id>",
  "mode": "spec" | "implementation",
  "lastUpdated": "<ISO timestamp>"
}
```

Update this file when:
- A new spec is initialized (`/spec-init`)
- A spec is approved (`/spec-approve`)
- Work on a spec is completed

---

## Divergence Handling

If implementation requires changes not covered by the approved spec:

1. **Stop** code changes immediately
2. **Return to Spec Mode**
3. Update the spec with new requirements
4. Request re-approval before continuing implementation

This ensures the spec remains the source of truth.

---

## Documentation Sync

Even in Implementation Mode, every code change must:

1. Update corresponding documentation
2. Keep acceptance criteria aligned
3. Maintain glossary entries for new concepts

"Done" is not complete until doc-sync passes.

---

## Workflow Commands

Use these workflows for explicit mode management:

| Command | Purpose |
| :------ | :------ |
| `/spec-init` | Initialize a new specification |
| `/spec-approve` | Request transition to Implementation Mode |
| `/spec-status` | Check current mode and active spec |

---

## Reference Documentation

- [MODES.md](docs/core/MODES.md) — Full mode definitions
- [ARCHITECTURE.md](docs/core/ARCHITECTURE.md) — System design
- [policies.yml](architecture/policies.yml) — Machine-checkable policies
