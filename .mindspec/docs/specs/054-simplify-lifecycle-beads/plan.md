---
spec_id: 054-simplify-lifecycle-beads
status: Draft
version: 1
last_updated: "2026-02-28"
approved_at: ""
approved_by: ""
bead_ids: []
adr_citations:
  - id: ADR-0020
    sections: [Decision, Decision Details]
---

# Plan: 054-simplify-lifecycle-beads

## Summary

Replace molecule-derived lifecycle state with per-spec `lifecycle.yaml` and rename mode-cache to focus file. Delete specmeta package, gut resolve, simplify all lifecycle commands.

## ADR Fitness

- **ADR-0020** (Proposed): This plan implements the decision. ADR-0020 supersedes ADR-0013 (formulas) and ADR-0015 (molecule-derived state). Status will move to Accepted on plan approval.
- **ADR-0008** (Human Gates): Retained unchanged — human gates remain as approve commands, just without Beads issues backing them.

## Testing Strategy

Each bead includes unit tests for the changed code. The approach is:
- **Unit tests** updated in-place for each modified package (`state`, `resolve`, `specinit`, `approve`, `complete`, `next`, `validate`)
- **Integration tests** in `resolve/integration_test.go` rewritten to use `lifecycle.yaml` instead of molecule frontmatter
- **Final validation**: `make test && make build` after all beads

## Provenance

| Acceptance Criterion | Verified in Bead |
|:---|:---|
| `spec-init` creates `lifecycle.yaml`, no molecule | Bead 2, 4 |
| `spec.md` has no `molecule_id`/`step_mapping` | Bead 2 |
| `lifecycle.yaml` committed to git | Bead 4 |
| `approve spec` writes `phase: plan` | Bead 4 |
| `approve plan` creates epic + impl beads | Bead 4 |
| `next` queries `bd ready --parent <epic-id>` | Bead 4 |
| `approve impl` closes only impl beads + epic | Bead 4 |
| `specmeta/` deleted | Bead 2 |
| `make test` passes | Bead 5 |
| `make build` succeeds | Bead 5 |
| ADR-0020 supersedes ADR-0013, ADR-0015 | Bead 5 |
| Net 400+ line reduction | Bead 5 |

## Bead 1: State layer — lifecycle.yaml + Focus rename

**Depends on**: nothing

### Steps

1. Add `Lifecycle` struct with `Phase` and `EpicID` fields plus `ReadLifecycle(specDir)` and `WriteLifecycle(specDir, lc)` to `internal/state/state.go`
2. Rename `ModeCache` to `Focus`, strip `Mode` field, rename `ReadModeCache`/`WriteModeCache` to `ReadFocus`/`WriteFocus`, rename file path from `mode-cache` to `focus`
3. Update `CrossValidate` in `validate.go` to accept `*Focus` and read phase from lifecycle.yaml
4. Update all state tests for new types and file names
5. Update `.gitignore` to ignore `focus` instead of `mode-cache`

### Verification

- [ ] `go test ./internal/state/...` passes
- [ ] `Lifecycle` round-trip test: write + read preserves Phase and EpicID
- [ ] `Focus` round-trip test: write + read preserves ActiveSpec, ActiveBead (no Mode)

## Bead 2: Delete specmeta, update templates

**Depends on**: nothing

### Steps

1. Delete `internal/specmeta/specmeta.go` and `internal/specmeta/specmeta_test.go`
2. Remove `molecule_id`, `step_mapping`, `approved_at`, `approved_by` from `specTemplate` in `internal/templates/templates.go`
3. Remove `specLifecycleFormula` constant and `SpecLifecycleFormula()` function from templates
4. Delete `.beads/formulas/spec-lifecycle.formula.toml`

### Verification

- [ ] `internal/specmeta/` directory does not exist
- [ ] `go build ./internal/templates/...` succeeds
- [ ] `go test ./internal/templates/...` passes (if template tests exist)
- [ ] `.beads/formulas/spec-lifecycle.formula.toml` does not exist

## Bead 3: Simplify resolve, move ResolveActiveBead

**Depends on**: Bead 1

### Steps

1. Rewrite `internal/resolve/resolve.go`: delete `ResolveMode`, `deriveMode`, `fetchStepStatuses`, `IsActive`, `molShowResult`, `molIssue`; rewrite `ActiveSpecs` to scan `lifecycle.yaml` files; drop `MoleculeID` from `SpecStatus`
2. Move `ResolveActiveBead` to `internal/next/beads.go` — reads `epic_id` from lifecycle.yaml instead of `specmeta.EnsureFullyBound`
3. Rewrite `internal/resolve/resolve_test.go` to test lifecycle.yaml-based `ActiveSpecs`
4. Rewrite `internal/resolve/integration_test.go` to use lifecycle.yaml fixtures

### Verification

- [ ] `go test ./internal/resolve/...` passes
- [ ] `go test ./internal/next/...` passes
- [ ] No `specmeta` import in resolve package

## Bead 4: Update lifecycle commands

**Depends on**: Bead 1, Bead 2, Bead 3

### Steps

1. Update `specinit.go`: remove `pourFormula`, `ensureFormula`, `writeSpecMeta`; write `lifecycle.yaml` with `phase: spec`; update tests
2. Update `approve/spec.go`: remove `EnsureFullyBound`; write `phase: plan` to lifecycle.yaml; use `WriteFocus`; update tests
3. Update `approve/plan.go`: remove `EnsureFullyBound`; create standalone epic; write `epic_id` + `phase: implement` to lifecycle.yaml; use `WriteFocus`; update tests
4. Update `approve/impl.go`: remove `EnsureFullyBound`, `closeoutTargets`; read lifecycle.yaml for epic_id; close impl beads + epic only; use `WriteFocus`; update tests
5. Update `complete/complete.go`: remove `ensureFullyBoundFn`; read lifecycle.yaml for epic_id; write phase transitions; use `WriteFocus`; update tests
6. Update `cmd/mindspec/next.go` and `cmd/mindspec/state.go`: use lifecycle.yaml and `ReadFocus`/`WriteFocus`; remove specmeta import

### Verification

- [ ] `go test ./internal/specinit/... ./internal/approve/... ./internal/complete/...` passes
- [ ] `go test ./cmd/mindspec/...` passes
- [ ] No `specmeta` import in any lifecycle command files

## Bead 5: Validate, hooks, final cleanup

**Depends on**: Bead 4

### Steps

1. Update `validate/spec.go`: remove `checkMoleculeBinding` and `checkSpecApprovalGateConsistency`; remove specmeta import
2. Update `validate/plan.go`: remove `checkPlanApprovalGateConsistency` molecule check; remove specmeta import
3. Update `cmd/mindspec/hook.go`: populate `HookState.Mode` by reading lifecycle.yaml phase
4. Accept ADR-0020 status
5. Run `make test && make build` — full green; count lines removed

### Verification

- [ ] `make test` passes (all packages)
- [ ] `make build` succeeds
- [ ] `grep -r specmeta internal/ cmd/` returns zero matches
- [ ] ADR-0020 status is Accepted
