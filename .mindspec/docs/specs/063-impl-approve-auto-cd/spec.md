---
approved_at: "2026-03-03T23:00:23Z"
approved_by: user
status: Approved
---
# Spec 063-impl-approve-auto-cd: Auto-cd into spec worktree for impl approve

## Goal

`mindspec approve impl <id>` should work from any directory (including main) by automatically cd-ing into the correct spec worktree before running. Currently it fails with "expected review mode" when run from main because phase derivation from the main worktree doesn't find closed epics.

## Background

During spec 062 implementation, `mindspec approve impl` had to be run from the spec worktree to correctly resolve the phase as "review". When run from main, `DiscoverActiveSpecs()` filters out non-open epics, so a closed epic (all beads done) is invisible. The spec worktree path uses `FindEpicBySpecID` which finds all statuses. Rather than fixing the filter (which has broader implications), the command should auto-cd into the spec worktree since it knows the spec ID and the worktree naming convention.

## Impacted Domains

- cmd/mindspec: command-layer auto-cd before calling `ApproveImpl`

## ADR Touchpoints

- None directly relevant — this is a UX convenience in the command layer

## Requirements

1. `mindspec approve impl <spec-id>` auto-detects and cd's into the spec worktree `<root>/.worktrees/worktree-spec-<spec-id>` before calling `ApproveImpl`
2. If already in the spec worktree, no cd happens (idempotent)
3. If the spec worktree doesn't exist, falls through to current behavior (no error from the auto-cd logic)

## Scope

### In Scope
- `cmd/mindspec/impl.go` — add auto-cd logic before `ApproveImpl` call

### Out of Scope
- Changing `DiscoverActiveSpecs` filtering logic
- Changing the `ApproveImpl` library function itself

## Non-Goals

- Fixing the root cause of `DiscoverActiveSpecs` not finding closed epics (that's a broader change)

## Acceptance Criteria

- [ ] `mindspec approve impl <id>` succeeds when run from main (spec worktree exists)
- [ ] `mindspec approve impl <id>` still succeeds when run from the spec worktree
- [ ] `make test` passes

## Validation Proofs

- `make test`: all tests pass

## Open Questions

- None

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-03
- **Notes**: Approved via mindspec approve spec