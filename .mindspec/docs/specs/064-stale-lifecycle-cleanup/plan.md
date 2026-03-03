---
adr_citations:
    - id: ADR-0023
      sections:
        - §3
approved_at: "2026-03-03T23:27:58Z"
approved_by: user
bead_ids:
    - mindspec-3y4t.1
    - mindspec-3y4t.2
last_updated: "2026-03-03"
spec_id: 064-stale-lifecycle-cleanup
status: Approved
version: 1
---
# Plan: 064-stale-lifecycle-cleanup

## Summary

Two beads: (1) delete stale lifecycle.yaml files, (2) add "done" phase for closed epics in `spec list`.

## ADR Fitness

- **ADR-0023** (Eliminate Focus and Lifecycle Files): Sound and directly relevant. §3 defines "Epic closed → done" but the code doesn't implement it. This plan fixes that gap. No divergence needed.

## Testing Strategy

- Unit tests for `DerivePhaseFromChildren` with epic-closed scenario
- Integration: `mindspec doctor` and `mindspec spec list` produce expected output
- Existing tests must continue to pass (`make test`)

## Bead 1: Delete stale lifecycle.yaml files

**Steps**
1. Delete all 12 `lifecycle.yaml` files from `.mindspec/docs/specs/*/`
2. Run `mindspec doctor` to confirm the warning is gone
3. Run `find .mindspec/docs/specs -name lifecycle.yaml` to confirm none remain

**Verification**
- [ ] `find .mindspec/docs/specs -name lifecycle.yaml` returns empty
- [ ] `mindspec doctor` shows no "Stale lifecycle.yaml" warning
- [ ] `make test` passes (no regressions from file deletion)

**Depends on**
None

## Bead 2: Add "done" phase for closed epics

**Steps**
1. Add `ModeDone = "done"` constant to `internal/state/state.go`
2. Update `internal/phase/derive.go`: in `DerivePhase()`, check the epic's own status first — if the epic is closed, return `ModeDone` before querying children
3. Update `internal/speclist/speclist.go`: in `derivePhase()`, pass epic status info through so closed epics get the `done` phase
4. Add unit test in `internal/phase/derive_test.go` for the closed-epic → done case
5. Run `make test` to verify all tests pass
6. Run `mindspec spec list` to confirm completed specs show `done`

**Verification**
- [ ] `go test ./internal/phase/...` passes including new closed-epic test
- [ ] `make test` passes
- [ ] `mindspec spec list` shows `done` for specs with closed epics (e.g. 055, 056, 057)

**Depends on**
None

## Provenance

| Acceptance Criterion | Bead | Verification |
|:----|:----|:----|
| All lifecycle.yaml files removed | Bead 1 | `find` returns empty |
| mindspec doctor passes clean | Bead 1 | doctor output |
| spec list shows `done` for closed epics | Bead 2 | `mindspec spec list` output |
| Existing lifecycle flow unaffected | Bead 2 | `make test` passes |
