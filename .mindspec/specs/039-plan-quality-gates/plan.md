---
adr_citations:
    - id: ADR-0003
      sections:
        - instruct emission
    - id: ADR-0008
      sections:
        - human gates
    - id: ADR-0013
      sections:
        - formula lifecycle
approved_at: "2026-02-20T10:45:04Z"
approved_by: user
last_updated: "2026-02-20"
spec_id: 039-plan-quality-gates
status: Approved
version: "1.0"
---

# Plan: 039-plan-quality-gates

## ADR Fitness

**ADR-0003** (Centralized Agent Instruction Emission): Sound. This spec updates the plan-mode instruct template to communicate new required sections — this is exactly how ADR-0003 intended guidance to evolve (dynamic emission from templates, not static docs). No divergence.

**ADR-0008** (Human Gates as Dependency Markers): Sound. This spec strengthens what `mindspec validate plan` checks *before* the human gate fires. The human gate remains the judgment layer; validation provides structural scaffolding. This is complementary, not competing. No divergence.

**ADR-0013** (Beads Formulas for Lifecycle): Sound. The plan-approve formula step calls `mindspec approve plan`, which runs `mindspec validate plan`. Tighter validation means the formula gate is more meaningful — the structural checks happen automatically when the gate fires. No divergence.

No ADR supersession needed.

## Testing Strategy

All changes are in `internal/validate/plan.go` and `internal/instruct/templates/plan.md`. Testing approach:

- **Unit tests** in `internal/validate/plan_test.go` — one test per new/promoted check, using temporary directories with synthetic plan files
- **Existing test patterns** — the file already has `makePlanWithCitations()` and `writeTestADR()` helpers; new tests follow the same pattern
- **Regression** — existing tests must continue to pass unchanged (backwards compatibility for approved plans)
- **Integration** — `make test` full suite, plus manual `mindspec validate plan 039-plan-quality-gates` dogfooding

## Bead 039-A: Promote ADR checks and add ADR Fitness error

**Scope**: Change ADR-related checks in `internal/validate/plan.go` from warnings to errors, with the conditional logic for empty citations when ADR Fitness is present.

**Steps**:
1. Read current `checkADRFitnessSection()` and `checkADRCitations()` in `plan.go`
2. Change `checkADRFitnessSection()` to emit `AddError` instead of `AddWarning`
3. Update ADR citations check: if `adr_citations` is empty AND `## ADR Fitness` section exists → warning; if both are missing → error
4. Add backwards-compatibility guard: skip new checks when frontmatter `status == "Approved"`
5. Write unit tests for all three cases: (a) missing ADR Fitness → error, (b) empty citations with ADR Fitness present → warning, (c) empty citations without ADR Fitness → error
6. Write unit test for backwards compat: approved plan with missing sections still passes

**Verification**:
- [ ] `go test ./internal/validate/...` passes with new ADR promotion tests
- [ ] Existing `TestValidatePlan_ADR*` tests still pass (no regression)
- [ ] Approved plan fixture skips new checks

**Depends on**: nothing

## Bead 039-B: Add Testing Strategy section check

**Scope**: Add a new validation check requiring `## Testing Strategy` section in plan.md.

**Steps**:
1. Add `checkTestingStrategySection()` function in `plan.go` — scan for `## Testing Strategy` heading
2. Emit `AddError("testing-strategy-missing", ...)` when absent
3. Wire into `ValidatePlan()` after ADR checks, before bead section checks
4. Guard with backwards-compatibility check (skip for `status == "Approved"`)
5. Write unit tests: (a) present → no error, (b) missing → error, (c) approved plan missing it → no error

**Verification**:
- [ ] `go test ./internal/validate/...` passes with new testing strategy tests
- [ ] `mindspec validate plan` on a plan without `## Testing Strategy` emits an error

**Depends on**: nothing

## Bead 039-C: Require testable bead verification

**Scope**: Validate that each bead's `**Verification**` items reference concrete test artifacts.

**Steps**:
1. Define `testArtifactPatterns` list in `plan.go`: `_test.go`, `.test.ts`, `.test.js`, `.spec.ts`, `make test`, `go test`, `pytest`, `npm test`, `mindspec validate`
2. Add `checkVerificationTestability()` function that scans each verification checkbox item for at least one pattern match
3. If a bead's verification section has zero items matching any pattern, emit `AddError("bead-verification-testability", ...)`
4. Wire into `checkBeadSection()` — pass the raw verification lines to the testability checker
5. Guard with backwards-compatibility check
6. Write unit tests: (a) bead with `_test.go` reference → passes, (b) bead with `make test` reference → passes, (c) bead with only "Confirm it works" → error, (d) mixed items where at least one matches → passes

**Verification**:
- [ ] `go test ./internal/validate/...` passes with testability tests
- [ ] Plan with vague-only verification items produces errors per-bead

**Depends on**: nothing

## Bead 039-D: Add Provenance section warning

**Scope**: Add a validation warning when the plan lacks a `## Provenance` section for output provenance (AC → bead mapping).

**Steps**:
1. Add `checkProvenanceSection()` function in `plan.go` — scan for `## Provenance` heading
2. Emit `AddWarning("provenance-missing", ...)` when absent (warning, not error)
3. Wire into `ValidatePlan()` after testing strategy check
4. Guard with backwards-compatibility check
5. Write unit tests: (a) present → no warning, (b) missing → warning, (c) approved plan → no warning

**Verification**:
- [ ] `go test ./internal/validate/...` passes with provenance tests
- [ ] `mindspec validate plan` on a plan without `## Provenance` emits a warning

**Depends on**: nothing

## Bead 039-E: Update plan-mode instruct template

**Scope**: Update `internal/instruct/templates/plan.md` to communicate the new required sections and testable verification expectations.

**Steps**:
1. Read current `plan.md` template
2. Add `## Testing Strategy` and `## Provenance` to the "Required Output" section
3. Add testable verification guidance: each bead's verification must reference test files or test commands
4. Add note about ADR Fitness being a hard requirement (not optional)
5. Verify template renders correctly via `mindspec instruct` in plan mode

**Verification**:
- [ ] `mindspec instruct --spec=039-plan-quality-gates` output includes new required sections
- [ ] `make test` passes (template changes don't break instruct rendering)

**Depends on**: nothing

## Bead 039-F: Dogfood — validate this plan

**Scope**: Run the strengthened validator against this plan itself to prove it works end-to-end.

**Steps**:
1. Build the updated binary: `make build`
2. Run `mindspec validate plan 039-plan-quality-gates`
3. Fix any validation errors in this plan.md
4. Run `make test` to confirm full suite passes
5. Document the validation output as proof

**Verification**:
- [ ] `mindspec validate plan 039-plan-quality-gates` passes with no errors
- [ ] `make test` passes with no regressions

**Depends on**: 039-A, 039-B, 039-C, 039-D, 039-E

## Provenance

| Spec Acceptance Criterion | Bead Verification |
|:--------------------------|:------------------|
| ADR Fitness section → error | 039-A: ADR promotion tests |
| Empty citations + ADR Fitness → warning | 039-A: conditional citation tests |
| Empty citations + no ADR Fitness → error | 039-A: conditional citation tests |
| Testing Strategy section → error | 039-B: testing strategy tests |
| Bead verification testability → error | 039-C: testability tests |
| Provenance section → warning | 039-D: provenance tests |
| Approved plans skip new checks | 039-A/B/C/D: backwards compat tests |
| Instruct template updated | 039-E: instruct output check |
| All new checks have unit tests | 039-A/B/C/D: each bead adds tests |
| `make test` passes | 039-F: full suite run |
