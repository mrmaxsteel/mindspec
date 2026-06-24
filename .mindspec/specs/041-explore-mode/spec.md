---
molecule_id: mindspec-mol-ker
step_mapping:
    implement: mindspec-mol-1ux
    plan: mindspec-mol-dnu
    plan-approve: mindspec-mol-0vv
    review: mindspec-mol-sty
    spec: mindspec-mol-5s1
    spec-approve: mindspec-mol-l22
    spec-lifecycle: mindspec-mol-ker
---

# Spec 041-explore-mode: Explore Mode

## Goal

Add an **Explore Mode** to MindSpec that provides a structured, LLM-guided pre-spec phase for evaluating whether an idea is worth pursuing. Users describe a vague idea — a potential feature, refactor, or architectural change — and the LLM helps them assess feasibility, alternatives, and value before committing to the spec-driven workflow. The mode produces one of two durable outputs: an ADR capturing "we decided not to do this" (and why), or a spec draft entering the existing Spec Mode pipeline.

This also serves as the primary entry point for non-engineers (product managers, stakeholders, domain experts) who have ideas but should not be authoring specs directly.

## Background

MindSpec's current lifecycle starts at Spec Mode — the user has already decided to build something and is defining what. There is no structured space for the prior question: "should we even do this?"

Today this pre-decision thinking happens informally — in chat, notes, or hallway conversations — and gets lost. Worse, it leads to two failure modes:
1. **Premature specs**: Users enter Spec Mode for half-baked ideas, waste effort specifying something that gets abandoned during planning.
2. **Lost reasoning**: A team decides *not* to pursue something but captures no record, so the same idea gets re-evaluated months later.

Explore Mode fills this gap by making the "should we?" decision a first-class, tracked activity with durable outputs.

## Impacted Domains

- **workflow**: New mode added to the MindSpec lifecycle, new state transitions
- **core**: `instruct` must emit Explore Mode guidance; state model gains a new mode value
- **adr**: Explore Mode may produce ADRs as a "no" outcome

## ADR Touchpoints

- [ADR-0015](../../adr/ADR-0015.md): Per-spec molecule-derived lifecycle state — Explore Mode needs to integrate with the molecule-based state model. Explore is a pre-molecule phase (no spec exists yet), so it may require its own state tracking approach.
- [ADR-0003](../../adr/ADR-0003.md): Centralized agent instruction emission — `mindspec instruct` must handle Explore Mode with appropriate guidance templates.
- [ADR-0012](../../adr/ADR-0012.md): Compose with external CLIs — Explore Mode's analysis capabilities (codebase scanning, feasibility checks) should compose with existing tools rather than reimplementing.

## Requirements

1. **Enter Explore Mode**: `mindspec explore` (or `mindspec explore "idea description"`) enters Explore Mode from idle state, setting the state to `explore` and emitting LLM-facing guidance.
2. **Structured exploration**: `mindspec instruct` in Explore Mode emits a template that guides the LLM through: problem statement clarification, prior art check (existing specs, ADRs, glossary), feasibility assessment, alternatives enumeration, and a recommendation.
3. **Prior art discovery**: The exploration guidance instructs the LLM to check existing ADRs, specs, and glossary entries for related decisions, preventing re-litigation of settled questions.
4. **"Not worthwhile" exit**: `mindspec explore dismiss` (or similar) exits Explore Mode back to idle. Optionally generates an ADR capturing the decision rationale and the alternatives considered. The ADR is the durable artifact.
5. **"Worthwhile" exit**: `mindspec explore promote` (or similar) exits Explore Mode by running `mindspec spec-init <id>`, carrying forward the exploration context into the new spec's Background section. Optionally generates an ADR if the exploration surfaced an architectural decision.
6. **No exploration artifacts**: Explore Mode is ephemeral — the conversation itself is the exploration. The only durable outputs are the artifacts it produces: an ADR (on dismiss) or a spec (on promote). No `.mindspec/explorations/` directory, no exploration log files, no new artifact type.
7. **No molecule during Explore**: Explore Mode is a pre-spec phase. No molecule is poured until the idea is promoted to a spec. State is tracked via `state.json` with `mode: explore`.
8. **PM-friendly interface**: The `mindspec explore` command accepts plain-language input. The instruct template guides the LLM to ask clarifying questions, present findings conversationally, and avoid jargon. The user does not need to know about specs, ADRs, or beads.

## Scope

### In Scope
- `cmd/mindspec/explore.go` — explore/dismiss/promote subcommands
- `internal/explore/` — exploration logic (dismiss with ADR generation, promote to spec-init)
- `internal/state/state.go` — add `ModeExplore` constant and validation
- `internal/instruct/templates/explore.md` — Explore Mode guidance template

### Out of Scope
- Multi-user collaboration on explorations (single-user for v1)
- Web UI or non-CLI interfaces
- Automated feasibility analysis (the LLM does the thinking; the CLI provides structure)
- Integration with external project management tools

## Non-Goals

- Explore Mode is not a "lite spec" — it should not accumulate acceptance criteria, domain mappings, or other spec machinery. It is deliberately unstructured compared to Spec Mode.
- Explore Mode does not gate Spec Mode — users can still `spec-init` directly if they already know what they want to build.
- No new artifact type — explorations are ephemeral conversations. Only the outputs (ADRs, specs) persist.

## Acceptance Criteria

- [ ] `mindspec explore "short description"` sets state to `mode: explore` and emits Explore Mode guidance via `instruct`
- [ ] `mindspec instruct` in Explore Mode emits guidance covering: problem clarification, prior art check, feasibility, alternatives, recommendation
- [ ] `mindspec explore dismiss` transitions to idle; with `--adr` flag, scaffolds an ADR via `mindspec adr create`
- [ ] `mindspec explore promote <spec-id>` runs `spec-init` and transitions to Spec Mode
- [ ] State validation rejects invalid transitions (e.g., `explore` → `implement`)
- [ ] Existing `spec-init` workflow is unaffected — users can bypass Explore Mode entirely

## Validation Proofs

- `mindspec explore "Add caching to API responses"`: Sets state to explore, emits guidance
- `mindspec instruct`: Emits explore-mode guidance including prior art check instructions
- `mindspec explore dismiss --adr`: Scaffolds ADR, returns to idle
- `mindspec explore promote 042-api-caching`: Runs spec-init for 042, enters Spec Mode
- `mindspec state show`: Shows `mode: explore` during exploration, `mode: idle` after dismiss, `mode: spec` after promote

## Open Questions

- [x] Naming: `mindspec explore` — confirmed.

## Future Considerations

- **Structured scoring rubric**: An effort-vs-impact matrix or similar framework could help consistency across explorations. Deferred until usage patterns emerge from v1.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-20
- **Notes**: Approved via mindspec approve spec