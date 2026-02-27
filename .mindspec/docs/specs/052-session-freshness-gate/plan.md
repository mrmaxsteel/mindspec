---
approved_at: "2026-02-27T07:37:36Z"
approved_by: user
bead_ids:
    - mindspec-mol-wrx3o.1
    - mindspec-mol-wrx3o.2
    - mindspec-mol-wrx3o.3
last_updated: 2026-02-27T00:00:00Z
spec_id: 052-session-freshness-gate
status: Approved
version: 1
---

# Plan: 052-session-freshness-gate

## ADR Fitness

- **ADR-0019 (Deterministic Worktree and Branch Enforcement)**: Still sound. This spec operates within the same three-layer enforcement model — the PreToolUse hook (Layer 3) changes its condition from a boolean flag to a timestamp ordering check. No divergence needed.

## Testing Strategy

- Unit tests for state field read/write round-trips
- Unit tests for the freshness gate logic (ordering checks, edge cases)
- Unit tests for the hook dispatch update
- Integration: `make test` must pass with all existing tests unbroken
- Manual validation: the proofs in the spec (fresh session passes, stale session blocks, `--force` bypasses)

## Bead 1: Add session freshness fields to state and write-session command

Replace the `NeedsClear` boolean with timestamp-based session freshness fields. Add a `write-session` CLI command that the SessionStart hook will call to record session metadata.

**Steps**
1. In `internal/state/state.go`: remove `NeedsClear bool` field. Add `SessionSource string` (`json:"sessionSource,omitempty"`), `SessionStartedAt string` (`json:"sessionStartedAt,omitempty"`), and `BeadClaimedAt string` (`json:"beadClaimedAt,omitempty"`) — all RFC3339 timestamps or source strings.
2. Remove the `ClearNeedsClear()` function from `state.go`.
3. In `cmd/mindspec/state.go`: remove `stateClearFlagCmd` and its `init()` registration. Add `stateWriteSessionCmd` that reads `--source` flag and writes `SessionSource` and `SessionStartedAt` (current time) to state. This is a read-modify-write: read existing state, update the two fields, write back (preserving all other fields).
4. Update `internal/state/state_test.go`: remove `NeedsClear` round-trip tests, add tests for new fields.
5. Update `cmd/mindspec/state.go` init: register `stateWriteSessionCmd`, remove `stateClearFlagCmd`.

**Verification**
- [ ] `go test ./internal/state/...` passes
- [ ] `make build` succeeds
- [ ] `./bin/mindspec state write-session --source=startup` writes fields to state.json

**Depends on**
None

## Bead 2: Update SessionStart hook command and setup

Update the SessionStart hook to parse stdin JSON for the `source` field and call `mindspec state write-session`. Update `setup claude` to emit the new hook command.

**Steps**
1. In `internal/setup/claude.go` `wantedHooks()`: update the SessionStart hook command from `"mindspec state clear-flag 2>/dev/null; mindspec instruct 2>/dev/null || ..."` to a script that reads stdin, extracts `source` via `jq`, and calls `mindspec state write-session --source=$source; mindspec instruct 2>/dev/null || ...`.
2. The hook command should be: `source=$(cat | jq -r '.source // "unknown"'); mindspec state write-session --source="$source" 2>/dev/null; mindspec instruct 2>/dev/null || echo 'mindspec instruct unavailable — run make build'`
3. Update `internal/setup/claude_test.go` if it validates hook command strings.

**Verification**
- [ ] `go test ./internal/setup/...` passes
- [ ] `make build` succeeds
- [ ] `./bin/mindspec setup claude --check` shows the updated hook command

**Depends on**
Bead 1

## Bead 3: Implement session freshness gate in `mindspec next` and hook

Replace the `NeedsClear` boolean check in both the CLI gate (`cmd/mindspec/next.go`) and the PreToolUse hook (`internal/hook/dispatch.go`) with the session freshness ordering check.

**Steps**
1. In `cmd/mindspec/next.go` Step 0: replace the `NeedsClear` check with freshness logic:
   - Read state. If `SessionStartedAt` is empty, skip gate (non-Claude-Code environment).
   - If `BeadClaimedAt` is not empty and `BeadClaimedAt >= SessionStartedAt`, block (bead already claimed in this session without `/clear`).
   - If `SessionSource` is `resume`, block (resumed session is not fresh).
   - On `--force`, warn and proceed.
2. After successful bead claim (after Step 5, line 148), write `BeadClaimedAt` to state (read-modify-write, current time RFC3339).
3. In `internal/hook/dispatch.go` `NeedsClear()`: rename to `SessionFreshnessGate()` (update `Run()` dispatch). Apply the same ordering logic: read `SessionStartedAt`, `BeadClaimedAt`, `SessionSource` from state. Block if stale session and command contains `mindspec next` without `--force`.
4. In `internal/complete/complete.go`: remove the `needs_clear` flag-setting block (lines 149-155). The completion path no longer participates in the gate.
5. Update tests in `internal/hook/dispatch_test.go`: replace `NeedsClear` tests with `SessionFreshnessGate` tests covering: fresh startup passes, fresh clear passes, stale session blocks, resume blocks, force bypasses, no session data skips gate.
6. Update tests in `internal/complete/complete_test.go`: remove any tests that assert `NeedsClear` is set after completion.

**Verification**
- [ ] `go test ./internal/hook/...` passes
- [ ] `go test ./internal/complete/...` passes
- [ ] `go test ./cmd/mindspec/...` passes (if cmd tests exist)
- [ ] `make test` passes
- [ ] `make build` succeeds

**Depends on**
Bead 1

## Provenance

| Acceptance Criterion | Bead | Verification |
|:---|:---|:---|
| SessionStart hook writes `sessionSource` and `sessionStartedAt` to state | Bead 1, 2 | `state write-session` unit test; manual validation |
| `mindspec next` writes `beadClaimedAt` on claim | Bead 3 | Freshness gate unit tests |
| `mindspec next` blocks when `beadClaimedAt > sessionStartedAt` | Bead 3 | `dispatch_test.go` + CLI gate |
| `mindspec next` blocks when `sessionSource` is `resume` | Bead 3 | `dispatch_test.go` |
| `mindspec next` passes on `startup` or `clear` with no prior claim | Bead 3 | `dispatch_test.go` |
| `mindspec next --force` bypasses gate | Bead 3 | `dispatch_test.go` + CLI |
| Non-Claude-Code environments skip gate | Bead 3 | `dispatch_test.go` (empty `sessionStartedAt`) |
| `needs_clear` removed from state, complete, clear-flag | Bead 1, 3 | `state_test.go`, `complete_test.go` |
| PreToolUse hook checks session freshness | Bead 3 | `dispatch_test.go` |
| Bead context primer unchanged | N/A | No changes to primer code; `make test` confirms |
| All existing tests pass | All | `make test` |
| New unit tests cover gate logic | Bead 1, 3 | New tests in state, hook, complete packages |
