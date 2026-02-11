---
description: MindSpec three-mode enforcement rules for agents
---

# MindSpec Mode Rules

These rules enforce the spec-driven development workflow. MindSpec uses three modes: **Spec**, **Plan**, and **Implementation**. Each mode gates what artifacts you may create or modify.

## Core Invariant

Before writing any code, you MUST verify:

1. **Spec exists and is approved**: A spec in `docs/specs/<id>/spec.md` with `Status: APPROVED`
2. **Plan exists and is approved**: Implementation beads are defined in Beads with explicit verification steps
3. **You are working on a specific bead**: A bead ID is active with a worktree assigned

If conditions 1-3 are not met, you are in **Spec Mode** or **Plan Mode** respectively. Only proceed to Implementation Mode when all three hold.

---

## Spec Mode

**When**: No approved spec exists for the current work.

### Permitted Actions
- Create/update `docs/specs/<id>/spec.md`
- Define acceptance criteria
- Declare impacted domains and ADR touchpoints
- Document open questions
- Update documentation in `docs/core/`, `docs/domains/`, or `docs/features/`
- Modify `GLOSSARY.md`
- Draft ADR proposals in `docs/adr/`
- Request human review when spec is ready

### Forbidden Actions
- Creating or modifying files in `src/` or test directories
- Changing build configuration that affects runtime
- Creating implementation beads in Beads (that's Plan Mode)

### Transition
User must explicitly approve the spec via `/spec-approve` or direct confirmation. The spec's Approval section must be updated to `Status: APPROVED`.

---

## Plan Mode

**When**: An approved spec exists but implementation beads are not yet approved.

### Permitted Actions
- Create implementation beads (child beads) in Beads with:
  - Small scope (one slice of value)
  - 3-7 step micro-plan
  - Explicit verification steps
  - Dependencies between beads
- Review and cite applicable ADRs for each bead
- Review domain docs and Context Map
- Propose new ADRs if divergence is detected
- Update documentation to clarify scope

### Required Review (before planning)
1. Read accepted ADRs for impacted domains
2. Read domain docs (`overview.md`, `architecture.md`, `interfaces.md`)
3. Check Context Map for neighboring context contracts
4. Verify existing constraints and invariants

### Forbidden Actions
- Writing implementation code in `src/` or test directories
- Widening scope beyond the spec's defined user value
- Skipping ADR review

### ADR Fitness Check
If an accepted ADR blocks progress or is unfit:
1. **Stop** and inform the user
2. Present options: continue-as-is vs. propose superseding ADR
3. If user accepts divergence, create a new ADR that supersedes the prior one
4. Resume planning with updated architecture

### Transition
User must explicitly approve the plan. All implementation beads must have verification steps and ADR citations.

---

## Implementation Mode

**When**: An approved spec and approved plan exist; you are executing a specific implementation bead.

### Permitted Actions
- Code changes within the bead's defined scope
- Test creation for the bead's scope
- Documentation updates (doc-sync is mandatory)
- Capturing proof/evidence (command outputs, test results)
- Updating bead status in Beads

### Obligations
1. **Worktree isolation**: Work in a bead-specific worktree
2. **Scope discipline**: Stay within the bead's scope. Discovered work becomes new beads + dependencies.
3. **Doc sync**: Update corresponding documentation with every code change
4. **Proof of done**: Bead closes only when verification steps pass with captured evidence
5. **ADR compliance**: Follow cited ADRs; divergence triggers the divergence protocol

### Forbidden Actions
- Widening scope beyond the bead definition
- Ignoring ADR divergence
- Completing a bead without proof and doc-sync
- Making changes outside the assigned worktree

### ADR Divergence Protocol
If implementation requires deviation from a cited ADR:
1. **Stop** code changes immediately
2. Inform the user: specify the ADR and the nature of divergence
3. Present options: continue-as-is, propose new superseding ADR, or revise scope
4. A new ADR must be accepted before implementation resumes

### Completion
A bead is complete when:
1. All verification steps pass with captured evidence
2. Documentation is updated
3. Bead status is updated in Beads with closure notes
4. Worktree changes are ready for review

---

## Human-in-the-Loop Gates

Always stop and request explicit human confirmation for:

- **Spec approval**: Spec Mode → Plan Mode
- **Plan approval**: Plan Mode → Implementation Mode
- **ADR divergence**: any mode detects an ADR is unfit
- **Domain operations**: adding, splitting, or merging domains
- **Scope expansion**: changes to user value definition

---

## Workflow Commands

| Command | Purpose |
|:--------|:--------|
| `/spec-init` | Initialize a new specification (enters Spec Mode) |
| `/spec-approve` | Request Spec Mode → Plan Mode transition |
| `/plan-approve` | Request Plan Mode → Implementation Mode transition |
| `/spec-status` | Check current mode, active spec, and bead state |

---

## Reference Documentation

- [MODES.md](docs/core/MODES.md) — Full mode definitions
- [ARCHITECTURE.md](docs/core/ARCHITECTURE.md) — System design
- [policies.yml](architecture/policies.yml) — Machine-checkable policies
- [mindspec.md](mindspec.md) — Product specification
