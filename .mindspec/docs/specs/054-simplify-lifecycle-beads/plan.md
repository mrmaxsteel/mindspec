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

## Bead 1: State layer — lifecycle.yaml + Focus rename

Add `Lifecycle` type and read/write helpers to `internal/state/`. Rename `ModeCache` → `Focus`, file `mode-cache` → `focus`, functions `ReadModeCache`/`WriteModeCache` → `ReadFocus`/`WriteFocus`. Strip `Mode` from Focus (it's now a cursor only). Update `internal/state/validate.go` to use Focus. Update all tests.

**Files:**
- `internal/state/state.go` — add `Lifecycle{Phase, EpicID}`, `ReadLifecycle(specDir)`, `WriteLifecycle(specDir, lc)`. Rename `ModeCache` → `Focus{ActiveSpec, ActiveBead, ActiveWorktree, SpecBranch, Timestamp}`. Rename read/write functions. Remove `Mode` field from Focus.
- `internal/state/state_test.go` — update all ModeCache references, add lifecycle tests
- `internal/state/validate.go` — `CrossValidate` takes `*Focus` instead of `*ModeCache`, reads phase from lifecycle.yaml via specDir
- `internal/state/validate_test.go` — update accordingly

**Verify:** `go test ./internal/state/...`

**Depends on:** nothing

## Bead 2: Delete specmeta, update templates

Delete `internal/specmeta/` entirely. Remove `molecule_id` and `step_mapping` from the spec template. Remove `SpecLifecycleFormula()` from templates. Delete `.beads/formulas/spec-lifecycle.formula.toml`.

**Files:**
- `internal/specmeta/specmeta.go` — delete
- `internal/specmeta/specmeta_test.go` — delete
- `internal/templates/templates.go` — remove `molecule_id`, `step_mapping` from `specTemplate`; remove `specLifecycleFormula` constant and `SpecLifecycleFormula()` func
- `.beads/formulas/spec-lifecycle.formula.toml` — delete

**Verify:** `go build ./...` (callers will break — fixed in subsequent beads)

**Depends on:** nothing (can parallel with Bead 1)

## Bead 3: Simplify resolve, move ResolveActiveBead

Gut `internal/resolve/resolve.go`: delete `ResolveMode`, `deriveMode`, `fetchStepStatuses`, `IsActive`, `ActiveSpecs`, `SpecStatus`, `molShowResult`, `molIssue`. Keep `ResolveTarget` but rewrite `ActiveSpecs` to scan `lifecycle.yaml` files instead of querying molecules. Move `ResolveActiveBead` to `internal/next/beads.go`. Update `resolve/target.go` if needed. Rewrite all resolve tests.

**Files:**
- `internal/resolve/resolve.go` — delete everything except: `ResolveTarget` (stays in target.go), `ResolveSpecBranch`, `ResolveWorktree`, `FormatActiveList`. Rewrite `ActiveSpecs` to scan lifecycle.yaml. `SpecStatus` keeps `SpecID`, `Mode` (read from lifecycle.yaml `Phase`), `Active` (phase != "done"). Drop `MoleculeID`.
- `internal/resolve/target.go` — no changes needed (already calls `ActiveSpecs`)
- `internal/resolve/resolve_test.go` — rewrite: test `ActiveSpecs` with lifecycle.yaml files instead of molecule mocks
- `internal/resolve/integration_test.go` — rewrite: use lifecycle.yaml instead of spec.md molecule frontmatter
- `internal/next/beads.go` — add `ResolveActiveBead(root, specID)` (moved from resolve, reads `epic_id` from lifecycle.yaml, calls `bd list --status=in_progress --parent <epicID>`)

**Verify:** `go test ./internal/resolve/... ./internal/next/...`

**Depends on:** Bead 1 (needs `ReadLifecycle`)

## Bead 4: Update specinit + approve + complete + next commands

Update all lifecycle commands to use lifecycle.yaml and Focus instead of specmeta/molecule.

**Files:**
- `internal/specinit/specinit.go` — remove `pourFormulaFn`, `writeSpecMeta`, `pourFormula()`, `ensureFormula()`. Instead: write `lifecycle.yaml` with `phase: spec`. Remove specmeta import.
- `internal/specinit/specinit_test.go` — remove molecule mocks, verify lifecycle.yaml written
- `internal/approve/spec.go` — remove `specmeta.EnsureFullyBound`, remove molecule step closure. Write `phase: plan` to lifecycle.yaml. Use `WriteFocus` instead of `WriteModeCache`.
- `internal/approve/spec_test.go` — update
- `internal/approve/plan.go` — remove `specmeta.EnsureFullyBound`. Create standalone epic via `bd create --type epic`. Write `epic_id` + `phase: implement` to lifecycle.yaml. Use `WriteFocus`.
- `internal/approve/plan_test.go` — update
- `internal/approve/impl.go` — remove `specmeta.EnsureFullyBound`, `closeoutTargets`. Read `bead_ids` from plan.md + `epic_id` from lifecycle.yaml. Close impl beads + epic only. Use `WriteFocus`.
- `internal/approve/impl_test.go` — rewrite: no molecule step IDs, just epic + impl beads
- `internal/complete/complete.go` — remove `ensureFullyBoundFn`. `advanceState` reads `epic_id` from lifecycle.yaml, queries `bd ready --parent <epicID>`. Write phase to lifecycle.yaml. Use `WriteFocus`.
- `internal/complete/complete_test.go` — update
- `cmd/mindspec/next.go` — read `epic_id` from lifecycle.yaml instead of `specmeta.EnsureFullyBound`. Use `QueryReadyForMolecule(epicID)` (rename param but same bd call). Use `ReadFocus`/`WriteFocus`.
- `cmd/mindspec/state.go` — `stateShowCmd` reads lifecycle.yaml for phase. `stateSetCmd` writes Focus. Use `ReadFocus`/`WriteFocus`.
- `cmd/mindspec/bead.go` — `beadCreateFromPlanCmd` needs update if it called `approve.CreateBeadsFromPlan` which used specmeta

**Verify:** `go test ./internal/approve/... ./internal/complete/... ./internal/specinit/... ./cmd/mindspec/...`

**Depends on:** Bead 1, Bead 2, Bead 3

## Bead 5: Update validate + hooks + final cleanup

Update validation to not check molecule binding. Update hook dispatch if it references ModeCache. Final integration test pass.

**Files:**
- `internal/validate/spec.go` — remove `checkMoleculeBinding`, `checkSpecApprovalGateConsistency` (or rewrite to check lifecycle.yaml phase). Remove specmeta import.
- `internal/validate/plan.go` — remove `checkPlanApprovalGateConsistency` molecule check. Remove specmeta import.
- `internal/validate/validate_test.go` — update if molecule fixtures exist
- `internal/hook/dispatch.go` — update `HookState` if it references `Mode` (it does — but Mode comes from the caller, which will now read lifecycle.yaml). May need no changes if the caller adapts.
- `internal/hook/dispatch_test.go` — update if needed
- `cmd/mindspec/hook.go` — update to read phase from lifecycle.yaml when populating HookState
- `.gitignore` — ensure `focus` is ignored (was `mode-cache`)

**Verify:** `make test && make build`

**Depends on:** Bead 4
