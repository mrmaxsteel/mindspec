---
approved_at: ""
approved_by: ""
molecule_id: mindspec-mol-q6abv
status: Draft
step_mapping:
    implement: mindspec-mol-mo9qh
    plan: mindspec-mol-aodjs
    plan-approve: mindspec-mol-9wh57
    review: mindspec-mol-qevf7
    spec: mindspec-mol-cbys8
    spec-approve: mindspec-mol-cvw57
    spec-lifecycle: mindspec-mol-q6abv
---

# Spec 054-simplify-lifecycle-beads: Drop Ceremony Molecules, Keep Implementation Beads

## Goal

Eliminate the 6-step spec-lifecycle molecule (spec, spec-approve, plan, plan-approve, implement, review) and replace it with a simple `phase` field in mode-cache. Retain Beads only for implementation work items — the part where structured tracking actually adds value.

This removes ~500-800 lines of molecule ceremony code, deletes the specmeta binding/recovery subsystem, and makes lifecycle transitions a direct write rather than a query-derive-cache pipeline.

## Background

MindSpec uses a Beads molecule (`spec-lifecycle.formula.toml`) to model the full spec lifecycle as 7 issues (1 epic + 6 steps). Each lifecycle command closes a molecule step, then mode is derived by querying which steps are still open.

The review in the previous session identified a fundamental mismatch: the 6 ceremony steps (spec, spec-approve, plan, plan-approve, implement, review) are state machine nodes, not work items. Nobody assigns, estimates, or comments on them. They exist solely to make `deriveMode()` work — a function that walks molecule step statuses to compute a phase string.

Meanwhile, **implementation beads** (created at plan-approve from `## Bead N:` sections) are genuine work items with real dependencies, parallel execution, and tracked completion. These should stay.

The result is a state machine cosplaying as a project tracker for 6 of its 7 steps.

### Prior decisions affected

- **ADR-0013** (Use Beads Formulas for Spec Lifecycle Orchestration) — superseded; the formula is no longer needed for lifecycle orchestration
- **ADR-0015** (Per-Spec Molecule-Derived Lifecycle State) — superseded; phase is stored directly, not derived from molecule queries

## Impacted Domains

- **workflow**: Lifecycle phase tracking moves from molecule-derived to direct
- **beads integration**: Molecule pour eliminated at spec-init; implementation beads created standalone at plan-approve
- **state management**: mode-cache becomes the source of truth for phase, not a cache of derived state

## ADR Touchpoints

- [ADR-0013](../../adr/ADR-0013.md): Superseded by ADR-0020 — formula orchestration removed
- [ADR-0015](../../adr/ADR-0015.md): Superseded by ADR-0020 — molecule-derived state replaced by direct phase tracking
- [ADR-0020](../../adr/ADR-0020.md): New — direct phase tracking, drop ceremony molecules
- [ADR-0008](../../adr/ADR-0008.md): Retained — human gates still exist conceptually as the approve commands; they just don't need Beads issues to represent them

## Requirements

1. **Drop the spec-lifecycle molecule entirely.** `spec-init` no longer calls `bd mol pour`. No molecule is created until plan-approve (if at all — see R3).
2. **Lifecycle phase is a direct field in mode-cache.** Transitions are atomic writes: `approve spec` writes `phase: plan`, `approve plan` writes `phase: implement`, etc. No molecule query needed.
3. **Implementation beads are created standalone at plan-approve.** `bd create --parent <epic-id>` with dependency wiring, same as today. The parent epic is a single Beads issue created at plan-approve time (not a molecule step).
4. **Delete `internal/specmeta/` entirely.** No more `EnsureFullyBound`, `EnsureBound`, `findMoleculeByConvention`, `recoverStepMapping`.
5. **Delete `internal/resolve/resolve.go` molecule-derivation code.** `deriveMode`, `fetchStepStatuses`, `ResolveMode` are replaced by reading mode-cache directly.
6. **Simplify `approve impl` closeout.** Instead of iterating `closeoutTargets()` to close 7+ molecule members, just verify all implementation beads are closed and close the parent epic.
7. **Remove `molecule_id` and `step_mapping` from spec.md frontmatter.** Replace with optional `epic_id` (the parent implementation epic, written at plan-approve).
8. **Update the formula file.** Either delete `.beads/formulas/spec-lifecycle.formula.toml` or retain it as documentation only.
9. **Supersede ADR-0013 and ADR-0015** with a new ADR explaining the change.

## Scope

### In Scope

- `internal/specmeta/specmeta.go` — delete entirely
- `internal/specmeta/specmeta_test.go` — delete entirely
- `internal/resolve/resolve.go` — remove `ResolveMode`, `deriveMode`, `fetchStepStatuses`, `IsActive`, `ActiveSpecs` molecule-query logic; keep `ResolveTarget` and `ResolveActiveBead`
- `internal/resolve/resolve_test.go`, `integration_test.go` — update accordingly
- `internal/approve/impl.go` — simplify `closeoutTargets`, remove molecule step closure
- `internal/approve/plan.go` — create standalone epic + impl beads instead of closing molecule step
- `internal/approve/spec.go` — remove `EnsureFullyBound` call, write phase directly
- `internal/specinit/specinit.go` — remove `pourFormula`, molecule ID/step mapping writes
- `internal/complete/complete.go` — simplify `advanceState` to not query molecule
- `internal/state/state.go` — ensure `ModeCache.Mode` is authoritative (already is)
- `internal/state/validate.go` — remove molecule cross-validation
- `internal/validate/spec.go`, `plan.go` — remove molecule-related validation
- `cmd/mindspec/next.go` — simplify molecule-scoped ready query to epic-scoped
- `cmd/mindspec/bead.go` — remove molecule references
- `cmd/mindspec/state.go` — `state show` reads mode-cache directly instead of deriving from molecule
- `internal/templates/templates.go` — remove embedded formula
- `.beads/formulas/spec-lifecycle.formula.toml` — delete or demote to docs
- New ADR superseding ADR-0013 and ADR-0015

### Out of Scope

- Implementation bead creation/tracking (stays exactly as is)
- Recording system (continues to emit lifecycle markers, just tied to phase writes instead of molecule step closures)
- Branch/worktree lifecycle (unchanged)
- Hook system (unchanged — still reads mode-cache)

## Non-Goals

- Changing how implementation beads work (dependencies, ready queries, claiming, closing)
- Removing Beads from the project (Beads stays for implementation tracking)
- Changing the user-facing lifecycle commands (`approve spec`, `approve plan`, `next`, `complete`, `approve impl`)

## Acceptance Criteria

- [ ] `mindspec spec-init` creates no molecule (no `bd mol pour` call)
- [ ] `spec.md` frontmatter has no `molecule_id` or `step_mapping` fields
- [ ] `mindspec approve spec` writes `phase: plan` to mode-cache directly
- [ ] `mindspec approve plan` creates a parent epic + impl beads via `bd create`, writes `epic_id` to plan.md
- [ ] `mindspec next` queries `bd ready --parent <epic-id>` (not `--mol`)
- [ ] `mindspec approve impl` closes only the impl beads + parent epic, not 7 molecule members
- [ ] `internal/specmeta/` package is deleted
- [ ] `make test` passes with no molecule-related test failures
- [ ] `make build` succeeds
- [ ] New ADR supersedes ADR-0013 and ADR-0015
- [ ] Net reduction of 400+ lines of Go code

## Validation Proofs

- `make test`: All tests pass
- `make build`: Binary compiles
- `./bin/mindspec spec-init 999-test && grep molecule_id .mindspec/docs/specs/999-test/spec.md`: No molecule_id in frontmatter
- `./bin/mindspec doctor`: No errors

## Open Questions

- [ ] Should the formula file be deleted or retained as historical documentation?
- [ ] Should `ResolveActiveBead` stay in `internal/resolve/` or move to `internal/next/`?

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
