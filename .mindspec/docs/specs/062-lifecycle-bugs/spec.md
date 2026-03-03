---
approved_at: "2026-03-03T22:42:41Z"
approved_by: user
status: Approved
---
# Spec 062-lifecycle-bugs: Fix Lifecycle Workflow Bugs

## Goal

Fix four bugs in the mindspec lifecycle that require manual intervention during normal spec workflows. These were discovered during the spec 060 and 061 implementations.

## Background

During the implementation of specs 060 (eliminate focus/lifecycle) and 061 (spec list), the following lifecycle workflow bugs required manual workarounds:

1. **Bead→spec merge fails in nested worktrees**: `mindspec complete` calls `MergeBranch(beadWorktree, beadBranch, specBranch)` which runs `git -C <bead-wt> checkout spec-branch`. This fails because the spec branch is already checked out in the parent spec worktree. The fix is to run the merge in the spec worktree directory instead (`git -C <spec-wt> merge bead-branch`), since the spec branch is already checked out there.

2. **`queryEpics()` only returns open epics**: `phase.queryEpics()` runs `bd list --type=epic --json` without a `--status` flag. `bd list` defaults to open-only. When an epic auto-closes (e.g. when its last bead closes), `FindEpicBySpecID` can't find it, causing `ResolveContextFromDir` to fall through to "spec worktree without epic → spec mode" instead of deriving the correct phase. This is the same class of bug fixed for `queryChildren` in spec 060.

3. **`plan approve` doesn't set `parent_id` on created beads**: Beads created by `mindspec plan approve` have no `parent_id`. This means `bd list --parent=<epic>` returns nothing, breaking phase derivation (`DerivePhase` sees no children → returns "plan" instead of the correct phase). Required manual `bd update <bead> --parent=<epic>` for each bead.

4. **Recording artifacts uncommitted before worktree removal**: Lifecycle commands (`spec approve`, `plan approve`, `complete`) write recording files to `.mindspec/docs/specs/<id>/recording/` in the spec worktree but never commit them. When `impl approve` calls `bd worktree remove`, it fails with "uncommitted changes". Fix: `impl approve` should auto-commit remaining changes in the spec worktree before merge/cleanup.

## Impacted Domains

- complete: merge strategy for nested bead worktrees
- phase: epic query should include all statuses
- approve: plan approve should set parent_id on beads; impl approve should auto-commit
- gitops: MergeBranch needs a variant that merges into an already-checked-out branch

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): Phase derivation from beads requires queryEpics to return all statuses, and beads must have parent_id set for DerivePhase to work

## Requirements

1. `mindspec complete` successfully merges bead→spec when the bead worktree is nested inside the spec worktree
2. `queryEpics()` returns epics of all statuses (open, in_progress, closed)
3. `mindspec plan approve` sets `parent_id` on each created bead, linking it to the spec epic
4. `mindspec impl approve` auto-commits any remaining changes in the spec worktree before attempting cleanup

## Scope

### In Scope
- `internal/gitops/gitops.go` — fix or add merge function that targets the spec worktree
- `internal/complete/complete.go` — pass spec worktree path to merge function
- `internal/phase/derive.go` — fix `queryEpics()` to query all statuses
- `internal/approve/plan.go` — set parent_id when creating beads
- `internal/approve/impl.go` — auto-commit before worktree removal

### Out of Scope
- Worktree directory layout changes (nesting is intentional)
- Beads CLI changes

## Non-Goals

- Changing the nested worktree structure
- Adding new CLI commands

## Acceptance Criteria

- [ ] `mindspec complete` merges bead branch into spec branch without manual intervention when bead worktree is nested inside spec worktree
- [ ] `FindEpicBySpecID` finds epics regardless of their status (open/closed)
- [ ] Beads created by `mindspec plan approve` have `parent_id` set to the spec epic ID
- [ ] `mindspec impl approve` succeeds without manual commits of recording artifacts
- [ ] `make test` passes
- [ ] Full lifecycle (spec create → spec approve → plan approve → next → complete → impl approve) runs without manual intervention

## Validation Proofs

- `make test`: All tests pass
- Manual full-lifecycle test: run the complete spec lifecycle on a test spec and verify no manual steps needed

## Open Questions

None.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-03
- **Notes**: Approved via mindspec approve spec