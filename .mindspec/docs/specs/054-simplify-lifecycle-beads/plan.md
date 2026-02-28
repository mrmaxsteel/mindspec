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

Depends on: nothing

1. Add `Lifecycle` struct (`Phase`, `EpicID` fields) and `ReadLifecycle`/`WriteLifecycle` to `internal/state/state.go`
2. Rename `ModeCache` → `Focus`, strip `Mode` field, rename `ReadModeCache`/`WriteModeCache` → `ReadFocus`/`WriteFocus`, rename file from `mode-cache` to `focus`
3. Update `CrossValidate` in `validate.go` to accept `*Focus` and read phase from lifecycle.yaml
4. Update all state tests for new types and file names
5. Update `.gitignore` to ignore `focus` instead of `mode-cache`

### Verification

- `go test ./internal/state/...` passes
- `Lifecycle` round-trip test: write + read preserves Phase and EpicID
- `Focus` round-trip test: write + read preserves ActiveSpec, ActiveBead (no Mode field)

## Bead 2: Delete specmeta, update templates

Depends on: nothing

1. Delete `internal/specmeta/specmeta.go` and `internal/specmeta/specmeta_test.go`
2. Remove `molecule_id`, `step_mapping`, `approved_at`, `approved_by` from `specTemplate` in `internal/templates/templates.go`
3. Remove `specLifecycleFormula` constant and `SpecLifecycleFormula()` function from templates
4. Delete `.beads/formulas/spec-lifecycle.formula.toml`
5. Verify `go build ./internal/templates/...` compiles

### Verification

- `internal/specmeta/` directory does not exist
- `go build ./internal/templates/...` succeeds
- `.beads/formulas/spec-lifecycle.formula.toml` does not exist

## Bead 3: Simplify resolve, move ResolveActiveBead

Depends on: Bead 1

1. Rewrite `internal/resolve/resolve.go`: delete `ResolveMode`, `deriveMode`, `fetchStepStatuses`, `IsActive`, `molShowResult`, `molIssue`. Rewrite `ActiveSpecs` to scan `lifecycle.yaml` files. Drop `MoleculeID` from `SpecStatus`.
2. Move `ResolveActiveBead` to `internal/next/beads.go` — reads `epic_id` from lifecycle.yaml instead of `specmeta.EnsureFullyBound`
3. Rewrite `internal/resolve/resolve_test.go` to test lifecycle.yaml-based `ActiveSpecs`
4. Rewrite `internal/resolve/integration_test.go` to use lifecycle.yaml fixtures instead of molecule frontmatter

### Verification

- `go test ./internal/resolve/...` passes
- `go test ./internal/next/...` passes
- No `specmeta` import in resolve package
- No `bd mol show` calls anywhere in resolve

## Bead 4: Update lifecycle commands

Depends on: Bead 1, Bead 2, Bead 3

1. Update `internal/specinit/specinit.go`: remove `pourFormula`, `ensureFormula`, `writeSpecMeta`. Write `lifecycle.yaml` with `phase: spec` instead. Update tests.
2. Update `internal/approve/spec.go`: remove `EnsureFullyBound` call. Write `phase: plan` to lifecycle.yaml. Use `WriteFocus`. Update tests.
3. Update `internal/approve/plan.go`: remove `EnsureFullyBound`. Create standalone epic via `bd create --type epic`. Write `epic_id` + `phase: implement` to lifecycle.yaml. Use `WriteFocus`. Update tests.
4. Update `internal/approve/impl.go`: remove `EnsureFullyBound`, `closeoutTargets`. Read `epic_id` from lifecycle.yaml, close impl beads + epic only. Use `WriteFocus`. Update tests.
5. Update `internal/complete/complete.go`: remove `ensureFullyBoundFn`. `advanceState` reads lifecycle.yaml for `epic_id`. Write phase transitions. Use `WriteFocus`. Update tests.
6. Update `cmd/mindspec/next.go`: read `epic_id` from lifecycle.yaml. Use `ReadFocus`/`WriteFocus`. Remove specmeta import.
7. Update `cmd/mindspec/state.go`: `stateShowCmd` reads lifecycle.yaml for phase. `stateSetCmd` writes Focus + lifecycle.yaml. Use `ReadFocus`/`WriteFocus`.

### Verification

- `go test ./internal/specinit/... ./internal/approve/... ./internal/complete/...` passes
- `go test ./cmd/mindspec/...` passes
- No `specmeta` import in any of these files
- No `bd mol pour` or `bd mol show` calls

## Bead 5: Validate, hooks, final cleanup

Depends on: Bead 4

1. Update `internal/validate/spec.go`: remove `checkMoleculeBinding` and `checkSpecApprovalGateConsistency` molecule checks. Remove specmeta import.
2. Update `internal/validate/plan.go`: remove `checkPlanApprovalGateConsistency` molecule check. Remove specmeta import.
3. Update `cmd/mindspec/hook.go`: populate `HookState.Mode` by reading lifecycle.yaml phase (via Focus → spec dir → lifecycle.yaml)
4. Update `internal/hook/dispatch.go` and tests if HookState field names changed
5. Accept ADR-0020 status
6. Run `make test && make build` — full green
7. Count lines removed, verify 400+ net reduction

### Verification

- `make test` passes (all packages)
- `make build` succeeds
- `grep -r specmeta internal/ cmd/` returns zero matches
- `grep -r 'bd mol' internal/ cmd/` returns zero matches
- ADR-0020 status is Accepted
