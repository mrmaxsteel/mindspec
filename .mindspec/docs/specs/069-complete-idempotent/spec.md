---
approved_at: "2026-03-04T13:46:10Z"
approved_by: user
status: Approved
---
# Make `mindspec complete` Idempotent and Guard `mindspec next`

## Goal

LLM agents frequently skip `mindspec complete` and use `bd close` directly. This loses the merge, worktree cleanup, and branch deletion. Two fixes:

1. Make `mindspec complete` resilient when the bead is already closed — still perform merge, worktree removal, and branch cleanup
2. Add a guard to `mindspec next` that warns when a predecessor bead was closed without `mindspec complete` (no merge commit on spec branch)

## Background

In LLM test harness runs, 5 of 13 failing tests fail because the agent skips `mindspec complete`. The agent either uses `bd close` directly or jumps to `impl approve`. When `mindspec complete` is skipped, the bead branch never merges into the spec branch, the worktree is orphaned, and the branch lingers.

## Impacted Domains

- complete: Make `Run()` tolerate already-closed beads
- next: Add guard checking for unmerged predecessor beads

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): State derived from beads — complete must query bead status

## Requirements

1. `mindspec complete` succeeds (with warning) when the bead is already closed by `bd close`
2. `mindspec complete` still performs merge, worktree removal, branch deletion for already-closed beads
3. `mindspec next` warns when a closed sibling bead has no merge commit on the spec branch
4. The warning from `mindspec next` suggests running `mindspec complete` to recover

## Scope

### In Scope
- `internal/complete/complete.go` — make `closeBeadFn` failure tolerable when bead already closed
- `cmd/mindspec/next.go` or `internal/next/` — add unmerged-bead guard
- Unit tests for both behaviors

### Out of Scope
- Changing `bd close` behavior
- Preventing agents from calling `bd close` (that's a guidance concern)

## Non-Goals

- Making `mindspec complete` work without any bead context at all
- Auto-running `mindspec complete` from `mindspec next`

## Acceptance Criteria

- [ ] `mindspec complete` succeeds when bead is already closed, performing merge+cleanup
- [ ] `mindspec complete` prints a warning noting the bead was already closed
- [ ] `mindspec next` warns when predecessor bead was closed without merge
- [ ] `make build && go test ./... -short` passes
- [ ] Existing complete tests still pass

## Validation Proofs

- Unit test: close bead via bd, then run `mindspec complete` — should succeed
- Unit test: closed bead with no merge commit → `mindspec next` warns

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-04
- **Notes**: Approved via mindspec approve spec