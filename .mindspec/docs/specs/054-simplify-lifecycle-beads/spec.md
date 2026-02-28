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

Eliminate the 6-step spec-lifecycle molecule and replace it with a per-spec `lifecycle.yaml` file committed to git. Retain Beads only for implementation work items — the part where structured tracking actually adds value.

This removes ~500-800 lines of molecule ceremony code, deletes the specmeta binding/recovery subsystem, and makes lifecycle transitions a direct file write — durable, versioned, and co-located with the spec.

## Background

MindSpec uses a Beads molecule (`spec-lifecycle.formula.toml`) to model the full spec lifecycle as 7 issues (1 epic + 6 steps). Each lifecycle command closes a molecule step, then mode is derived by querying which steps are still open.

The review in the previous session identified a fundamental mismatch: the 6 ceremony steps (spec, spec-approve, plan, plan-approve, implement, review) are state machine nodes, not work items. Nobody assigns, estimates, or comments on them. They exist solely to make `deriveMode()` work — a function that walks molecule step statuses to compute a phase string.

Meanwhile, **implementation beads** (created at plan-approve from `## Bead N:` sections) are genuine work items with real dependencies, parallel execution, and tracked completion. These should stay.

### State management evolution

The state architecture has gone through three iterations:

1. **state.json (ADR-0005)**: Global, committed to git. Durable but caused merge conflicts, was noisy in diffs, and couldn't support multiple specs.
2. **mode-cache (Spec 053, ADR-0015)**: Global, gitignored. Ephemeral but safe because the molecule served as backup — lose mode-cache, re-derive from `bd mol show`.
3. **This spec**: Per-spec `lifecycle.yaml`, committed to git. Durable, versioned, no merge conflicts (lives on the spec branch where only one agent works), and no molecule needed.

The per-spec approach avoids the problems of both predecessors: it's durable like state.json but per-spec like the molecule, and it doesn't require a tracking system to encode what is fundamentally a phase field.

### Prior decisions affected

- **ADR-0005** (State Tracking via Committed State File) — already superseded by ADR-0015; this spec completes the arc by returning to committed state but per-spec
- **ADR-0013** (Use Beads Formulas for Spec Lifecycle Orchestration) — superseded; the formula is no longer needed
- **ADR-0015** (Per-Spec Molecule-Derived Lifecycle State) — superseded; phase is stored directly, not derived from molecule queries

## Impacted Domains

- **workflow**: Lifecycle phase tracking moves from molecule-derived to per-spec file
- **beads integration**: Molecule pour eliminated at spec-init; implementation beads created standalone at plan-approve
- **state management**: Per-spec `lifecycle.yaml` is the source of truth; global focus file (`.mindspec/focus`) becomes a cursor for "which spec am I focused on"

## ADR Touchpoints

- [ADR-0013](../../adr/ADR-0013.md): Superseded by ADR-0020 — formula orchestration removed
- [ADR-0015](../../adr/ADR-0015.md): Superseded by ADR-0020 — molecule-derived state replaced by per-spec lifecycle file
- [ADR-0020](../../adr/ADR-0020.md): New — per-spec lifecycle file, drop ceremony molecules
- [ADR-0008](../../adr/ADR-0008.md): Retained — human gates still exist as the approve commands; they just don't need Beads issues

## Requirements

1. **Introduce per-spec `lifecycle.yaml`.** Each spec gets a `docs/specs/<id>/lifecycle.yaml` file containing:
   ```yaml
   phase: implement    # spec | plan | implement | review | done
   epic_id: mindspec-xyz  # set at plan-approve, optional before then
   ```
   This file is committed to git on the spec branch. Lifecycle commands update it atomically.

2. **Drop the spec-lifecycle molecule entirely.** `spec-init` no longer calls `bd mol pour`. No molecule is created at any point in the lifecycle.

3. **Rename mode-cache to focus file (`.mindspec/focus`).** The focus file retains `activeSpec` (which spec the user is working on) and `activeBead` (which bead is claimed). Phase is read from the spec's `lifecycle.yaml`, not stored in the focus file. Hooks that need the phase read `lifecycle.yaml` from the active spec's directory.

4. **Implementation beads are created standalone at plan-approve.** `bd create --parent <epic-id>` with dependency wiring, same as today. The parent epic is a single Beads issue created at plan-approve time. `epic_id` is written to `lifecycle.yaml`.

5. **Delete `internal/specmeta/` entirely.** No more `EnsureFullyBound`, `EnsureBound`, `findMoleculeByConvention`, `recoverStepMapping`.

6. **Delete `internal/resolve/resolve.go` molecule-derivation code.** `deriveMode`, `fetchStepStatuses`, `ResolveMode` are replaced by reading `lifecycle.yaml`. Keep `ResolveTarget` and `ResolveActiveBead`.

7. **Simplify `approve impl` closeout.** Instead of iterating `closeoutTargets()` to close 7+ molecule members, just verify all implementation beads are closed and close the parent epic.

8. **Remove `molecule_id` and `step_mapping` from spec.md frontmatter.** These fields are no longer needed — `lifecycle.yaml` carries the phase and `epic_id`.

9. **Delete the formula file.** Remove `.beads/formulas/spec-lifecycle.formula.toml` and the embedded copy in `internal/templates/`.

10. **Supersede ADR-0013 and ADR-0015** with ADR-0020.

11. **Auto-detect spec from branch.** When on branch `spec/054-foo`, commands can infer `specID = 054-foo` and read `lifecycle.yaml` from the spec directory without consulting the focus file. The focus file's `activeSpec` serves as fallback when branch detection isn't possible.

## Scope

### In Scope

- `internal/specmeta/specmeta.go` — delete entirely
- `internal/specmeta/specmeta_test.go` — delete entirely
- `internal/resolve/resolve.go` — remove `ResolveMode`, `deriveMode`, `fetchStepStatuses`, `IsActive`, `ActiveSpecs` molecule-query logic; keep `ResolveTarget`, `ResolveActiveBead`
- `internal/resolve/resolve_test.go`, `integration_test.go` — update accordingly
- `internal/approve/impl.go` — simplify `closeoutTargets`, remove molecule step closure
- `internal/approve/plan.go` — create standalone epic + impl beads, write `epic_id` to `lifecycle.yaml`
- `internal/approve/spec.go` — remove `EnsureFullyBound` call, write phase to `lifecycle.yaml`
- `internal/specinit/specinit.go` — remove `pourFormula`, write `lifecycle.yaml` with `phase: spec`
- `internal/complete/complete.go` — simplify `advanceState`, update `lifecycle.yaml`
- `internal/state/state.go` — add `lifecycle.yaml` read/write helpers; rename `ModeCache` to `Focus`, simplify to cursor only
- `internal/state/validate.go` — remove molecule cross-validation
- `internal/validate/spec.go`, `plan.go` — remove molecule-related validation
- `cmd/mindspec/next.go` — read `epic_id` from `lifecycle.yaml`, use `bd ready --parent`
- `cmd/mindspec/bead.go` — remove molecule references
- `cmd/mindspec/state.go` — `state show` reads `lifecycle.yaml` for phase
- `internal/templates/templates.go` — remove embedded formula
- `.beads/formulas/spec-lifecycle.formula.toml` — delete
- `internal/hook/dispatch.go` — update to read phase from `lifecycle.yaml` instead of focus file
- ADR-0020 superseding ADR-0013 and ADR-0015

### Out of Scope

- Implementation bead creation/tracking (stays exactly as is)
- Recording system (continues to emit lifecycle markers)
- Branch/worktree lifecycle (unchanged)
- Session freshness gate (unchanged — still uses `session.json`)

## Non-Goals

- Changing how implementation beads work (dependencies, ready queries, claiming, closing)
- Removing Beads from the project (Beads stays for implementation tracking)
- Changing the user-facing lifecycle commands (`approve spec`, `approve plan`, `next`, `complete`, `approve impl`)
- Multi-spec parallelism (per-spec state enables it naturally, but the UX for working on multiple specs simultaneously is out of scope)

## Acceptance Criteria

- [ ] `mindspec spec-init` creates `lifecycle.yaml` with `phase: spec`, no molecule
- [ ] `spec.md` frontmatter has no `molecule_id` or `step_mapping` fields
- [ ] `lifecycle.yaml` is committed to git on the spec branch
- [ ] `mindspec approve spec` writes `phase: plan` to `lifecycle.yaml`
- [ ] `mindspec approve plan` creates a parent epic + impl beads, writes `epic_id` + `phase: implement` to `lifecycle.yaml`
- [ ] `mindspec next` queries `bd ready --parent <epic-id>` (not `--mol`)
- [ ] `mindspec approve impl` closes only the impl beads + parent epic
- [ ] `internal/specmeta/` package is deleted
- [ ] `make test` passes
- [ ] `make build` succeeds
- [ ] ADR-0020 supersedes ADR-0013 and ADR-0015
- [ ] Net reduction of 400+ lines of Go code

## Validation Proofs

- `make test`: All tests pass
- `make build`: Binary compiles
- `./bin/mindspec spec-init 999-test && cat .mindspec/docs/specs/999-test/lifecycle.yaml`: Shows `phase: spec`, no molecule_id
- `grep -r molecule_id .mindspec/docs/specs/999-test/`: No matches
- `./bin/mindspec doctor`: No errors

## Open Questions

- [ ] Should hooks read `lifecycle.yaml` directly, or should the focus file mirror the phase for hook performance? (If `lifecycle.yaml` is in the worktree, reads are fast — probably no need for mirroring.)
- [ ] Should `ResolveActiveBead` stay in `internal/resolve/` or move to `internal/next/`?

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
