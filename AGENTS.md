# Mindspec Agent Instructions

This repository uses **mindspec** for spec-driven development. All agents working in this repository must follow the mode system and workflows defined here.

## Mode System

All work follows a two-phase approach:

### Spec Mode (Default)
- **Permitted**: Markdown files only (`docs/`, `GLOSSARY.md`, specs)
- **Focus**: Requirements, acceptance criteria, documentation
- **Exit**: Explicit user approval via `/spec-approve`

### Implementation Mode
- **Permitted**: Code changes in `src/`, tests, configuration
- **Requires**: Approved spec with all acceptance criteria defined
- **Obligations**: Doc-sync, scope discipline, proof-of-done

> **Rule**: Never create or modify code in `src/` without an approved spec.

---

## Required Workflows

| Command | Purpose |
| :------ | :------ |
| `/spec-init` | Initialize a new specification |
| `/spec-approve` | Request transition to Implementation Mode |
| `/spec-status` | Check current mode and active spec |

---

## Before Writing Code

1. Check if an approved spec exists for the work
2. Verify the spec has `Status: APPROVED` in its Approval section
3. Confirm the proposed changes are within the spec's scope
4. If any check fails → remain in Spec Mode, complete the spec first

---

## Documentation Sync

Every code change must:
- Update corresponding documentation
- Keep acceptance criteria aligned
- Add glossary entries for new concepts

**"Done" includes doc-sync.**

---

## Architecture Divergence

If implementation requires changes that diverge from documented architecture:

1. **Stop** code changes immediately
2. **Assess**: Is this a scope change or an architecture divergence?
   - **Scope change**: Minor additions within existing patterns
   - **Architecture divergence**: Changes to invariants, patterns, or system boundaries
3. **For architecture divergence**:
   - Create an **ACP** (Architecture Change Proposal) in `docs/architecture/proposals/`
   - ACP must include: summary, motivation, options, impact, required doc updates
   - **Await explicit human approval** before proceeding
4. **After approval**: Update spec with new requirements, request re-approval
5. **For scope changes**: Update spec directly, request re-approval

> **Rule**: Architecture divergence always triggers an ACP. The ACP is the decision artifact.

---

## Key Documentation

| Document | Purpose |
| :------- | :------ |
| [MODES.md](docs/core/MODES.md) | Mode definitions and transitions |
| [ARCHITECTURE.md](docs/core/ARCHITECTURE.md) | System design and invariants |
| [CONVENTIONS.md](docs/core/CONVENTIONS.md) | File organization and naming |
| [GLOSSARY.md](GLOSSARY.md) | Term definitions for context injection |
| [policies.yml](architecture/policies.yml) | Machine-checkable policies |

---

## State Tracking

Active spec and mode are tracked in `.mindspec/current-spec.json`:

```json
{
  "activeSpec": "<spec-id>",
  "mode": "spec" | "implementation",
  "lastUpdated": "<ISO timestamp>"
}
```

This file is gitignored (local state only). The spec file itself is the source of truth for approval status.
