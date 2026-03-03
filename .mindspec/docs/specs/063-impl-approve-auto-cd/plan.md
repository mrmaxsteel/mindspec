---
adr_citations: []
approved_at: "2026-03-03T23:01:03Z"
approved_by: user
bead_ids:
    - mindspec-l8s9.1
last_updated: "2026-03-03"
spec_id: 063-impl-approve-auto-cd
status: Approved
version: 1
---
# Plan: 063-impl-approve-auto-cd

## ADR Fitness

No ADRs are directly relevant. This is a command-layer UX fix that doesn't change any architectural decisions. The worktree naming convention (`worktree-spec-<id>`) is already established and used throughout the codebase.

## Testing Strategy

- **Unit tests**: The change is in `cmd/mindspec/impl.go` (command wiring). The auto-cd logic is a simple `os.Stat` + `os.Chdir`. Existing `internal/approve/impl_test.go` tests cover the library layer.
- **Integration**: `make test` validates no regressions.

## Bead 1: Add auto-cd to spec worktree in approveImplRunE

**Steps**
1. In `cmd/mindspec/impl.go`, after `findRoot()` and before `approve.ApproveImpl()`, compute the spec worktree path: `filepath.Join(root, ".worktrees", "worktree-spec-"+specID)`
2. If that directory exists (`os.Stat`), `os.Chdir` into it so that `ApproveImpl`'s internal `findLocalRoot` resolves to the spec worktree
3. Add import for `path/filepath`

**Verification**
- [ ] `make test` passes
- [ ] `go build ./cmd/mindspec` succeeds

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---|---|
| `mindspec approve impl <id>` succeeds from main | Bead 1: auto-cd into spec worktree |
| Still succeeds from spec worktree | Bead 1: idempotent — stat finds same dir |
| `make test` passes | Bead 1 verification |
