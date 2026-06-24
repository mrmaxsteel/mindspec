---
approved_at: "2026-03-08T21:33:36Z"
approved_by: user
status: Approved
---
# Spec 078-per-bead-acceptance-criteria: Per-Bead Acceptance Criteria

## Goal

Each implementation bead receives acceptance criteria scoped to its own work, not the full spec-level acceptance criteria. Agents working on a bead see only the criteria they are responsible for satisfying, reducing noise and improving focus.

## Background

### Current behavior (Spec 074)

Spec 074 made beads self-contained by populating fields at plan approval. Requirement #2 specified that each bead's `acceptance_criteria` field receives the **spec-level** acceptance criteria from `spec.md`. This means every bead — regardless of its scope — carries the full spec AC.

In `internal/approve/plan.go:230-261`:
```go
acceptanceCriteria := contextpack.ExtractSection(specContent, "Acceptance Criteria")
// ... same value passed to every bead
"--acceptance", acceptanceCriteria,
```

### Problem

Spec-level AC covers the entire feature surface. A bead scoped to "add a config field" receives AC about end-to-end behavior, UI rendering, and other concerns it cannot satisfy alone. This creates two issues:

1. **Noise**: The agent sees criteria outside its scope, wasting context and potentially causing confusion about what "done" means for this bead
2. **False completeness signal**: An agent cannot mark spec-level AC as satisfied from a single bead, so the criteria are structurally unverifiable at the bead level

### Desired behavior

The plan author decomposes spec-level AC into bead-specific criteria during planning. Each bead section includes an `**Acceptance Criteria**` subsection. At plan approval, each bead receives only its own criteria. The existing Provenance section continues to serve as the traceability matrix from spec AC → bead AC.

## Impacted Domains

- templates: Plan template gains per-bead `**Acceptance Criteria**` subsection
- instruct: Plan mode guidance updated to instruct agents to write per-bead AC
- validate: Bead section parser extracts per-bead AC; optional validation that all spec AC is covered
- approve: Plan approval reads per-bead AC instead of spec-level AC

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): Beads as single state/context store — per-bead AC improves the quality of snapshotted context

## Requirements

1. The plan template (`planTemplate` in `internal/templates/templates.go`) must include an `**Acceptance Criteria**` subsection in each bead section
2. The instruct plan template (`internal/instruct/templates/plan.md`) must include `**Acceptance Criteria**` in the bead section format example and instruct agents to decompose spec AC into per-bead criteria
3. `ParseBeadSections()` in `internal/validate/plan.go` must parse an `**Acceptance Criteria**` subsection from each bead and expose it on the `BeadSection` struct
4. `createImplementationBeads()` in `internal/approve/plan.go` must read per-bead AC from the parsed section content (falling back to spec-level AC if no per-bead AC is found, for backwards compatibility)
5. The plan validator should warn (not error) if a bead section lacks `**Acceptance Criteria**`
6. Existing Provenance section remains the mechanism for tracing spec AC → bead coverage

## Scope

### In Scope
- `internal/templates/templates.go` — `planTemplate` constant
- `internal/instruct/templates/plan.md` — plan mode guidance
- `internal/validate/plan.go` — `BeadSection` struct, `ParseBeadSections()`, `checkBeadSection()`
- `internal/approve/plan.go` — `createImplementationBeads()`
- Tests for all of the above

### Out of Scope
- Changes to spec.md template or spec-level AC format
- Automated validation that per-bead AC fully covers spec AC (Provenance already handles this manually)
- Changes to `bd show` rendering or `internal/contextpack/beadctx.go` — the field name stays `acceptance_criteria`, only the content changes
- Retroactive update of existing beads

## Non-Goals

- Enforcing 1:1 mapping between spec AC items and bead AC items — the decomposition is the plan author's judgment call
- Eliminating the Provenance section — it remains as the human-readable traceability matrix
- Changing the beads CLI `--acceptance` flag or field name

## Acceptance Criteria

- [ ] Plan template includes `**Acceptance Criteria**` subsection in the bead section format
- [ ] Plan mode instruct template shows per-bead AC in the example and instructs agents to decompose spec AC
- [ ] `ParseBeadSections()` populates a new `AcceptanceCriteria` field on `BeadSection` with per-bead AC text
- [ ] `createImplementationBeads()` passes per-bead AC (from plan) to `--acceptance` instead of spec-level AC
- [ ] When a bead section has no `**Acceptance Criteria**`, falls back to spec-level AC
- [ ] Plan validator warns when a bead section lacks `**Acceptance Criteria**`
- [ ] `go test ./internal/validate/ -run TestParseBeadSections` verifies per-bead AC parsing
- [ ] `go test ./internal/approve/ -run TestCreateImplementationBeads` verifies per-bead AC is passed to `bd create`
- [ ] Existing plan validation tests continue to pass

## Validation Proofs

- `go test ./internal/validate/ -v -run TestParseBeadSections`: BeadSection.AcceptanceCriteria populated
- `go test ./internal/approve/ -v -run TestCreateImplementationBeads`: `--acceptance` flag receives per-bead AC
- `go test ./internal/validate/ -v -run TestValidatePlan`: existing checks still pass
- `mindspec validate plan <id>`: warns on missing per-bead AC in a test plan

## Open Questions

- [x] Should missing per-bead AC be an error or warning? **Resolved**: Warning for backwards compatibility — old plans without per-bead AC should still validate. The fallback to spec-level AC preserves current behavior.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-08
- **Notes**: Approved via mindspec approve spec