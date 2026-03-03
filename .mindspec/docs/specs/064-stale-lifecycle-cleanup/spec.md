---
approved_at: "2026-03-03T23:25:59Z"
approved_by: user
status: Approved
---
# Spec 064-stale-lifecycle-cleanup: Stale Lifecycle Cleanup

## Goal

Clean up stale `lifecycle.yaml` files and fix `mindspec spec list` so completed specs show a "done" phase instead of "—".

## Background

1. **Stale lifecycle.yaml**: ADR-0023 moved lifecycle state tracking from per-spec `lifecycle.yaml` files to beads. 12 `lifecycle.yaml` files remain as vestigial artifacts, causing a `mindspec doctor` warning on every run.

2. **Missing "done" phase in spec list**: `mindspec spec list` shows PHASE "—" for fully completed specs. The root cause is twofold:
   - `DerivePhaseFromChildren` maps "all children closed" → `review`, but specs that have been fully merged via `impl approve` should show `done`
   - The epic itself may be closed after `impl approve`, and the phase derivation or epic lookup may not handle closed epics properly for display purposes
   - Pre-beads specs (000-031) have no epic at all and show "unknown" status + "—" phase

## Impacted Domains

- core: removal of deprecated lifecycle artifacts, addition of "done" phase constant

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): lifecycle state derived from beads, not lifecycle.yaml

## Requirements

1. Delete all 12 stale `lifecycle.yaml` files
2. Add a `"done"` mode/phase constant
3. When a spec's epic is closed, `spec list` should show phase `done` (not `review` or `—`)
4. `mindspec doctor` should no longer emit the stale lifecycle warning

## Scope

### In Scope

- `.mindspec/docs/specs/*/lifecycle.yaml` — delete all instances
- `internal/state/state.go` — add `ModeDone` constant
- `internal/phase/derive.go` — handle closed epics → `done` phase
- `internal/speclist/speclist.go` — ensure closed epics are included in phase derivation

### Out of Scope

- Pre-beads specs (000-031) that have no epic — they will continue to show "—" (retroactively creating epics is a separate effort)
- Changes to the doctor check logic itself

## Non-Goals

- Retroactively creating beads epics for old specs
- Changing the active lifecycle flow (spec → plan → implement → review → idle)

## Acceptance Criteria

- [ ] All `lifecycle.yaml` files removed from spec directories
- [ ] `mindspec doctor` passes with no stale lifecycle warning
- [ ] `mindspec spec list` shows `done` for specs whose epic is closed
- [ ] Existing lifecycle flow (idle → spec → plan → implement → review) is unaffected

## Validation Proofs

- `mindspec doctor`: No "Stale lifecycle.yaml" warning
- `find .mindspec/docs/specs -name lifecycle.yaml`: Returns empty
- `mindspec spec list`: Completed specs (e.g. 055, 056, 057) show `done` phase

## Open Questions

_None._

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-03
- **Notes**: Approved via mindspec approve spec