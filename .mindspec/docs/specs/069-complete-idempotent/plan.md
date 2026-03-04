---
approved_at: "2026-03-04T13:46:38Z"
approved_by: user
spec_id: 069-complete-idempotent
status: Approved
version: "1"
---
# Plan: Make `mindspec complete` Idempotent and Guard `mindspec next`

## ADR Fitness

- ADR-0023: State derived from beads — complete queries bead status to determine if already closed

## Testing Strategy

Unit tests for both behaviors using stubbed bd/git functions.

## Bead 1: Make `mindspec complete` idempotent for already-closed beads

**Steps**
1. In `complete.Run()`, after `closeBeadFn(beadID)` fails, check if the bead is already closed via `bd show <beadID> --json`
2. If already closed, print a warning ("bead already closed, performing cleanup") and continue with merge/worktree/branch cleanup
3. If the error is something else (not "already closed"), return the error as before
4. Also handle the case where `resolveActiveBeadFn` returns empty because the bead is closed — look for recently closed beads
5. Add unit test: stub closeBeadFn to return error, stub bd show to return closed status, verify complete succeeds

**Verification**
- [ ] `go test ./internal/complete/ -v -run TestAlreadyClosed` passes
- [ ] Existing complete tests still pass

**Depends on**
None

## Bead 2: Add unmerged-bead guard to `mindspec next`

**Steps**
1. In `cmd/mindspec/next.go` or `internal/next/`, after resolving the epic, query closed sibling beads
2. For each closed bead, check if its `bead/<id>` branch still exists (unmerged indicator)
3. If an unmerged closed bead is found, emit a warning: "Warning: bead <id> was closed without `mindspec complete`. Run `mindspec complete --spec=<spec>` to recover merge topology."
4. This is a warning, not a blocker — next should still proceed
5. Add unit test

**Verification**
- [ ] `go test ./internal/next/ -v -run TestUnmergedBead` passes
- [ ] `mindspec next` still works normally when no unmerged beads exist

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| complete succeeds when bead already closed | Bead 1 unit test |
| complete prints warning for already-closed bead | Bead 1 unit test |
| next warns about unmerged predecessor beads | Bead 2 unit test |
| All existing tests pass | Both beads verification |
