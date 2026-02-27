---
approved_at: "2026-02-27T00:22:12Z"
approved_by: user
molecule_id: mindspec-mol-be41w
status: Approved
step_mapping:
    implement: mindspec-mol-a4bf4
    plan: mindspec-mol-2l9pf
    plan-approve: mindspec-mol-syhc0
    review: mindspec-mol-yzrzy
    spec: mindspec-mol-e7fhp
    spec-approve: mindspec-mol-197wm
    spec-lifecycle: mindspec-mol-be41w
---



# Spec 051-worktree-state-propagation: Propagate state to bead worktrees and fix lifecycle command routing

## Goal

Make bead worktrees fully self-contained workspaces by propagating state, and fix lifecycle command routing so that `complete` and `impl-approve` work correctly from the appropriate worktree without hook conflicts.

## Background

Discovered during Spec 050 implementation. Four related bugs:

1. **Bead worktrees have no state.** `mindspec next` → `EnsureWorktree()` creates the bead worktree and updates state in the spec worktree, but doesn't write state into the bead worktree. `mindspec complete` from the bead worktree fails with "no state found." Note: `specinit.Run()` already writes state to both main and spec worktrees (lines 166-173), but `EnsureWorktree` doesn't follow this pattern.

2. **`impl-approve` can't run from the right place.** It needs the spec worktree (where state lives after beads are done), but the worktree-bash hook blocks it because `ActiveWorktree` points to the bead worktree. It only works because `mindspec` is in the allowed commands list — accidental, not intentional.

3. **`mindspec complete` rejects the spec worktree.** It enforces "not main worktree" but doesn't distinguish between main worktree and spec worktree. After all beads are done, `complete` should work from the spec worktree.

4. **Stray binary from `go build`.** Running `go build` without `-o` in a bead worktree creates a `mindspec` binary at repo root that gets swept into `git add -A`. The `.gitignore` doesn't cover it.

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Zero-on-main — worktree isolation model. State propagation extends this by making bead worktrees truly isolated workspaces.

## Impacted Domains

- next: `EnsureWorktree()` needs to copy state into the bead worktree
- complete: needs to accept spec worktree as valid CWD
- hook/dispatch: worktree-bash should recognize lifecycle commands that need spec worktree access
- gitignore: should exclude the built `mindspec` binary at repo root

## Requirements

1. `EnsureWorktree()` in `internal/next/beads.go` must write `.mindspec/state.json` into the newly created bead worktree after creation
2. `mindspec complete` must accept being run from the spec worktree (not just bead worktrees)
3. The worktree-bash hook must allow lifecycle commands (`approve impl`, `complete`) to run from the spec worktree even when `ActiveWorktree` points to a bead worktree
4. Add `/mindspec` (the bare binary name) to `.gitignore` to prevent accidental commits
5. State written to bead worktrees must include `SpecBranch`, `ActiveMolecule`, `StepMapping`, and `ActiveWorktree` (pointing to itself)

## Scope

### In Scope
- `internal/next/beads.go` — state propagation in `EnsureWorktree()`
- `internal/complete/complete.go` — relax CWD validation
- `internal/hook/dispatch.go` — worktree-bash spec-worktree awareness
- `.gitignore` — add `/mindspec`
- Tests for each change

### Out of Scope
- Changing the worktree nesting topology (bead worktrees inside spec worktrees)
- State synchronization between worktrees after initial creation
- Reworking the state model to be branch-aware

## Non-Goals

- Making state shared or synced across worktrees in real-time
- Changing how `specinit.Run()` handles state (it already works correctly)

## Acceptance Criteria

- [ ] `mindspec next` creates a bead worktree that contains `.mindspec/state.json`
- [ ] `mindspec complete` succeeds when run from the spec worktree after all beads are done
- [ ] `impl-approve` can run from the spec worktree without relying on the bash-hook allowed-commands bypass
- [ ] `go build` in a bead worktree doesn't leave a committable `mindspec` binary
- [ ] `make build` succeeds
- [ ] `make test` passes
- [ ] CI passes

## Validation Proofs

- `make build`: clean build
- `make test`: all tests pass
- `cat <bead-worktree>/.mindspec/state.json`: state file exists with correct fields

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-27
- **Notes**: Approved via mindspec approve spec