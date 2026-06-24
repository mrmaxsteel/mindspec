---
adr_citations:
    - id: ADR-0023
      sections:
        - Decision §5
approved_at: "2026-03-08T21:47:53Z"
approved_by: user
bead_ids:
    - mindspec-bkkz.1
    - mindspec-bkkz.2
last_updated: 2026-03-08T00:00:00Z
spec_id: 078-per-bead-acceptance-criteria
status: Approved
version: "1.0"
---

# Plan: Spec 078 — Per-Bead Acceptance Criteria

**Spec**: [spec.md](spec.md)

---

## ADR Fitness

- **ADR-0023** (Beads as single state authority): Sound. This change improves the quality of data snapshotted into beads at plan approval — per-bead AC is more useful than spec-level AC repeated across all beads. No divergence.

No other ADRs are impacted. The change is internal to the plan→bead data flow.

---

## Testing Strategy

- **Unit tests**: New test cases in `validate_test.go` for `ParseBeadSections` AC parsing; updated test in `plan_test.go` for per-bead AC in `createImplementationBeads`
- **Regression**: All existing tests in `internal/validate/` and `internal/approve/` must continue to pass
- **Build**: `make build` verifies template changes compile

No integration or LLM harness tests needed — this is a data-flow change within plan approval, and existing harness tests don't exercise plan approval (they start from already-created beads).

---

## Bead 1: Update templates and plan guidance

**Scope**: Plan template (`internal/templates/templates.go`) and instruct plan template (`internal/instruct/templates/plan.md`)

**Steps**
1. In `internal/templates/templates.go`, add `**Acceptance Criteria**` subsection to the `planTemplate` bead section format, between `**Verification**` and `**Depends on**`
2. In `internal/instruct/templates/plan.md`, update the "Bead section format" example to include `**Acceptance Criteria**` with placeholder items
3. Add guidance text in the instruct plan template's "Required Output" section instructing agents to decompose spec-level AC into per-bead criteria
4. Verify `make build` succeeds (template is a Go string constant)
5. Run `go test ./internal/instruct/...` to verify template rendering

**Acceptance Criteria**
- [ ] `planTemplate` constant includes `**Acceptance Criteria**` subsection in the bead section
- [ ] Instruct plan template example shows `**Acceptance Criteria**` in the bead format
- [ ] Instruct plan template instructs agents to decompose spec AC into per-bead AC

**Verification**
- [ ] `make build` succeeds
- [ ] `go test ./internal/instruct/...` passes

**Depends on**
None

---

## Bead 2: Parser, approval logic, and tests

**Scope**: `internal/validate/plan.go` (parser + validator), `internal/approve/plan.go` (bead creation), and their tests

**Steps**
1. Add `AcceptanceCriteria string` field to `BeadSection` struct in `internal/validate/plan.go`
2. Update `ParseBeadSections()` to detect `**Acceptance Criteria**` (and `### Acceptance Criteria`) as a subsection marker, accumulating lines into the new field
3. In `checkBeadSection()`, add a warning (not error) when `AcceptanceCriteria` is empty on a bead section (skip for already-approved plans, matching the `isApproved` pattern)
4. In `internal/approve/plan.go` `createImplementationBeads()`: parse bead sections via `ParseBeadSections()`, match by heading, and use per-bead `AcceptanceCriteria` for `--acceptance` when non-empty; fall back to spec-level AC otherwise
5. Add test cases in `internal/validate/plan_test.go` for `ParseBeadSections` with per-bead AC (present and absent)
6. Update `TestCreateImplementationBeads_PopulatesFields` in `internal/approve/plan_test.go` to include per-bead AC in the plan fixture and assert it is passed to `--acceptance` instead of spec-level AC
7. Add a test case for the fallback: bead section without `**Acceptance Criteria**` falls back to spec-level AC

**Acceptance Criteria**
- [ ] `BeadSection.AcceptanceCriteria` is populated when `**Acceptance Criteria**` subsection exists
- [ ] `createImplementationBeads()` passes per-bead AC to `--acceptance` when available
- [ ] Missing per-bead AC falls back to spec-level AC
- [ ] Validator warns on missing per-bead AC for non-approved plans

**Verification**
- [ ] `go test ./internal/validate/ -run TestParseBeadSections` passes with per-bead AC cases
- [ ] `go test ./internal/approve/ -run TestCreateImplementationBeads` passes with per-bead AC assertions
- [ ] `go test ./internal/validate/ -v` — all existing tests still pass
- [ ] `go test ./internal/approve/ -v` — all existing tests still pass

**Depends on**
None

---

## Dependency Graph

078-A (templates + guidance)
078-B (parser + approval + tests)

No dependencies — both beads can run in parallel.

---

## Provenance

| Spec Acceptance Criterion | Satisfied By |
|:---|:---|
| Plan template includes `**Acceptance Criteria**` subsection | Bead 1 step 1 |
| Plan mode instruct template shows per-bead AC in example | Bead 1 steps 2, 3 |
| `ParseBeadSections()` populates `AcceptanceCriteria` field | Bead 2 steps 1, 2, 5 |
| `createImplementationBeads()` passes per-bead AC to `--acceptance` | Bead 2 steps 4, 6 |
| Falls back to spec-level AC when per-bead AC missing | Bead 2 steps 4, 7 |
| Validator warns on missing per-bead AC | Bead 2 steps 3, 5 |
| `go test ./internal/validate/ -run TestParseBeadSections` passes | Bead 2 step 5 |
| `go test ./internal/approve/ -run TestCreateImplementationBeads` passes | Bead 2 step 6 |
| Existing plan validation tests pass | Bead 2 verification |
