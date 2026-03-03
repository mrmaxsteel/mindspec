---
approved_at: "2026-03-03T22:46:03Z"
approved_by: user
bead_ids:
    - mindspec-pwa1.4
    - mindspec-pwa1.5
    - mindspec-pwa1.6
spec_id: 062-lifecycle-bugs
status: Approved
version: "1"
---
# Plan: 062-lifecycle-bugs

## ADR Fitness

- **ADR-0023** (accepted): Bug #2 directly violates ADR-0023's mandate that phase derivation queries all beads state. `queryEpics` must return all statuses for `FindEpicBySpecID` and `DerivePhase` to work correctly.

## Testing Strategy

- **Unit tests**: Extend existing tests for `MergeBranch`, `queryEpics`, `createImplementationBeads`, and `ApproveImpl` to cover the fixed behavior.
- **Integration**: `make test` passes; manual full-lifecycle verification.

## Bead 1: Fix bead→spec merge for nested worktrees

`MergeBranch` tries to checkout the target branch in the source worktree. When the bead worktree is nested inside the spec worktree, this fails because the spec branch is already checked out. Fix: merge into the spec worktree directory directly.

**Steps**
1. In `internal/complete/complete.go`, determine the spec worktree path from the bead worktree path (parent `.worktrees/` directory)
2. Change the merge call to target the spec worktree: `git -C <spec-wt> merge <bead-branch>` — no checkout needed since the spec branch is already checked out there
3. Add or update `gitops.MergeInto(targetWorkdir, sourceBranch)` that does a simple `git -C <dir> merge <branch>` without checkout
4. Update tests for complete.go to verify the spec worktree path is used

**Verification**
- [ ] `go test ./internal/complete/...` passes
- [ ] `go test ./internal/gitops/...` passes
- [ ] `make test` passes

**Depends on**
None

## Bead 2: Fix queryEpics to return all statuses

`queryEpics()` runs `bd list --type=epic --json` which defaults to open-only. Same class of bug fixed for `queryChildren` in spec 060.

**Steps**
1. In `internal/phase/derive.go`, update `queryEpics()` to query all three statuses (open, in_progress, closed) — same pattern as the `queryChildren` fix from spec 060
2. Deduplicate results (in case an epic appears in multiple status queries)
3. Update unit tests for `FindEpicBySpecID` to verify closed epics are found

**Verification**
- [ ] `go test ./internal/phase/...` passes
- [ ] `FindEpicBySpecID` finds epics regardless of status
- [ ] `make test` passes

**Depends on**
None

## Bead 3: Auto-commit in impl approve before cleanup

`impl approve` calls `bd worktree remove` which fails if recording artifacts are uncommitted. Fix: auto-commit remaining changes in the spec worktree before merge/cleanup.

**Steps**
1. In `internal/approve/impl.go`, before the merge/worktree-removal step, check for uncommitted changes in the spec worktree
2. If changes exist, run `git -C <spec-wt> add -A && git -C <spec-wt> commit -m "chore: commit remaining spec artifacts"`
3. Update impl_test.go to cover the auto-commit path

**Verification**
- [ ] `go test ./internal/approve/...` passes
- [ ] `make test` passes

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| `mindspec complete` merges bead→spec in nested worktrees | Bead 1 verification |
| `FindEpicBySpecID` finds epics regardless of status | Bead 2 verification |
| Beads created by plan approve have parent_id set | Bead 2 verification (root cause: queryEpics only returned open epics, so parent ID was empty) |
| `impl approve` succeeds without manual commits | Bead 3 verification |
| `make test` passes | All beads |
| Full lifecycle runs without manual intervention | All beads combined |
