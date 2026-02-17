# MindSpec Operational Modes

MindSpec enforces a **gated, three-phase lifecycle** where specification precedes planning, and planning precedes implementation. Each mode controls allowed outputs, required context, and transition gates.

```
Intent → [Spec Mode] → approval → [Plan Mode] → approval → [Implementation Mode] → validation → Done
              ↑                        ↑                            ↑
              └── rejected ────────────┘──── divergence ────────────┘
```

---

## Spec Mode {#spec-mode}

### Objective

Discuss user-facing value and how to validate it. Spec Mode is intentionally **implementation-light**: no deep design unless necessary to define what "done" means.

### Output

A specification containing:

- Problem statement and target user outcome
- Acceptance criteria and validation plan (manual + automated where applicable)
- Non-goals / constraints
- Impacted domains (see [Domains](ARCHITECTURE.md#domains))
- Required architecture touchpoints (ADRs/docs to follow)
- Open questions that must be resolved before planning

### Permitted Artifacts

| Artifact | Location | Template | Purpose |
|:---------|:---------|:---------|:--------|
| Spec files | `docs/specs/<id>/spec.md` | [`docs/templates/spec.md`](../templates/spec.md) | Formal specification |
| Domain docs | `docs/domains/<domain>/` | [`docs/templates/domain/`](../templates/domain/) | Domain documentation |
| Glossary entries | `GLOSSARY.md` | — | New term definitions |
| Architecture docs | `docs/core/` | — | Context/rationale |
| ADR drafts | `docs/adr/` or `docs/domains/<domain>/adr/` | [`docs/templates/adr.md`](../templates/adr.md) | Proposed decisions |

### Forbidden Actions

- Creating or modifying code in `src/` or equivalent implementation directories
- Creating or modifying test code
- Changing build/config that affects runtime behavior

### Exit Gate

To leave Spec Mode, the spec must:

1. Have all acceptance criteria explicitly defined and verifiable
2. Declare impacted domains and ADR touchpoints
3. Have all open questions resolved
4. Receive **explicit human approval**
5. **Working tree must be clean** before transition
6. **Milestone commit**: Commit the spec artifact + bead update (message: `spec(<bead-id>): ...`)

---

## Plan Mode {#plan-mode}

### Objective

Turn an approved spec into bounded, executable work chunks.

### Required Review

Before planning, the agent must review:

- Applicable ADRs (accepted, not superseded) for impacted domains
- Domain docs (`overview.md`, `architecture.md`, `interfaces.md`)
- Context Map for neighboring context contracts
- Existing constraints and invariants

### Plan Artifact

When Plan Mode starts, create `docs/specs/<id>/plan.md` using the template at [`docs/templates/plan.md`](../templates/plan.md). Initialize the YAML frontmatter:

```yaml
status: Draft
spec_id: <id>
version: "0.1"
last_updated: YYYY-MM-DD
```

The plan is iteratively edited during Plan Mode — always readable on disk. On approval, update the frontmatter:

```yaml
status: Approved
approved_at: YYYY-MM-DDTHH:MM:SSZ
approved_by: <human>
bead_ids: [beads-xxx, beads-yyy]
adr_citations: [ADR-NNNN]
```

Frontmatter is the single source of truth for plan status.

### Output

Child beads (**Implementation Beads**) in Beads, each with:

- Small scope ("one slice of value")
- 3-7 step micro-plan
- Explicit verification steps
- Dependencies between beads
- Worktree assignment convention

### ADR Fitness Check

If the planner detects that an accepted ADR blocks progress or is unfit:

1. **Stop** and inform the user
2. Present a divergence option set (continue-as-is vs. propose change)
3. If user accepts divergence, create a **new ADR** that **supersedes** the prior ADR(s)
4. Resume planning with the updated architecture

### Permitted Artifacts

- `docs/specs/<id>/plan.md` (the live plan draft)
- Beads entries (implementation beads, dependency links)
- ADR proposals (if divergence detected)
- Documentation updates (if clarifying scope)

### Forbidden Actions

- Writing implementation code
- Widening scope beyond the spec's defined user value

### Exit Gate

To leave Plan Mode:

1. All implementation beads are defined with verification steps
2. Dependencies are explicit
3. ADRs cited for each bead's architectural assumptions
4. **Explicit human approval** of the plan
5. **Working tree must be clean** before transition
6. **Milestone commit**: Commit plan artifacts + spawned beads (message: `plan(<bead-id>): ...`)

---

## Implementation Mode {#implementation-mode}

### Objective

Execute one implementation bead in an isolated worktree.

### Prerequisites

- An approved plan with implementation beads
- A worktree created for the target bead
- **Working tree must be clean** (`git status` shows no changes)
- Context Pack loaded (mode-specific, budgeted)

### Output

- Code changes within the bead's defined scope
- Evidence / proof (commands, test outputs, screenshots)
- Documentation updates / refactors
- Status progression and closure notes in Beads

### Obligations

| Obligation | Detail |
|:-----------|:-------|
| **Scope discipline** | Changes must stay within the bead's scope. Discovered work becomes new beads. |
| **Doc sync** | Every code change must update corresponding documentation |
| **Proof of done** | Bead closes only when verification steps pass |
| **Worktree isolation** | Work happens in a bead-specific worktree |
| **ADR compliance** | Implementation must follow cited ADRs; divergence triggers the ADR divergence protocol |

### ADR Divergence Protocol

If implementation requires deviation from a cited ADR:

1. **Stop** code changes immediately
2. Inform the user with the specific ADR and the nature of the divergence
3. Present options: continue-as-is, propose new ADR, or revise scope
4. If user approves divergence: create a new ADR superseding the old, then continue
5. The new ADR must be accepted before implementation resumes

### Forbidden Actions

- Widening scope (new work becomes new beads + dependencies)
- Ignoring ADR divergence
- Completing a bead without proof and doc-sync

### Exit Gate

A bead is complete when:

1. All verification steps pass with captured evidence
2. Documentation is updated
3. Bead status is updated in Beads with closure notes
4. **Advance state**: If more beads remain for the spec, set state to the next bead or back to `plan`; if all beads are done, reset state to `idle` (`mindspec state set --mode=idle`)
5. **Milestone commit**: Commit code, tests, docs, bead closure, **and state file** (message: `impl(<bead-id>): ...`)
6. Worktree changes are ready for review

---

## Human-in-the-Loop Gates {#human-gates}

MindSpec requires explicit human confirmation for:

| Gate | Trigger |
|:-----|:--------|
| Spec approval | Spec Mode → Plan Mode transition |
| Plan approval | Plan Mode → Implementation Mode transition |
| ADR divergence | Any mode detects an ADR is unfit or blocking |
| Domain operations | Adding, splitting, or merging domains |
| Scope expansion | Changes to the user value definition in a spec |
| Non-automatable validation | Acceptance of items that cannot be verified automatically |

---

## Mode Enforcement {#mode-enforcement}

### Policy Integration

Mode enforcement policies are defined in [`architecture/policies.yml`](../../architecture/policies.yml):

- `spec-mode-no-code`: Blocks code changes in Spec Mode
- `plan-mode-no-code`: Blocks code changes in Plan Mode
- `implementation-requires-approved-plan`: Blocks implementation without plan approval

### State Tracking

Active mode and bead are tracked via Beads status. The spec file and bead state are the sources of truth for approval status.

---

## See Also

- [ARCHITECTURE.md](ARCHITECTURE.md) — Core system design
- [CONVENTIONS.md](CONVENTIONS.md) — File organization and naming
- [policies.yml](../../architecture/policies.yml) — Machine-checkable policies
- [mindspec-v1-spec.md](../archive/mindspec-v1-spec.md) — Original product specification (archived)
