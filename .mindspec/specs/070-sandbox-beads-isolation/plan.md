---
adr_citations: []
approved_at: "2026-03-04T17:05:19Z"
approved_by: user
bead_ids: ["mindspec-frlj.1", "mindspec-frlj.2"]
last_updated: "2026-03-04"
spec_id: 070-sandbox-beads-isolation
status: Approved
version: "1"
---
# Plan: 070-sandbox-beads-isolation

## ADR Fitness

No ADRs are relevant to this work — purely test infrastructure.

## Root Cause

`bd` resolves `.beads/` by walking up from CWD. When the LLM agent `cd`s during a test session, `bd` may find the host project's `.beads/` instead of the sandbox's. Additionally, `bd dolt killall` in sandbox setup kills ALL dolt servers system-wide, including the host's.

## Testing Strategy

- Deterministic unit test: verify sandbox `bd where` resolves to sandbox `.beads/`, not host
- Manual validation: run `TestLLM_SingleBead`, then check host `bd list` for leaked test issues

## Bead 1: Sandbox Beads Isolation

**Goal**: Ensure every `bd` command in a sandbox connects only to the sandbox's own dolt server.

**Steps**

1. Add `DoltPort int` field to `Sandbox` struct to track the sandbox's dolt server port
2. In `initBeads()`:
   - Remove `bd dolt killall` (unsafe — kills host server)
   - After `bd init`, read `.beads/dolt-server.port` from sandbox root, store on struct
3. Register `t.Cleanup()` in `NewSandbox()` to stop the sandbox's dolt server (kill PID from `.beads/dolt-server.pid`)
4. Create a `bd` wrapper shim in the sandbox's shim dir that forces CWD back to the sandbox root before executing the real `bd` binary:
   ```sh
   #!/bin/sh
   cd "<sandbox_root>"
   exec <real_bd_path> "$@"
   ```
   This ensures every agent `bd` call resolves `.beads/` from the sandbox, regardless of where the agent has `cd`d
5. Combine the CWD-forcing logic with the existing recording shim (shims already wrap `bd` for event logging — add the `cd` before the exec)
6. Add a deterministic test that verifies sandbox `bd where` output points to the sandbox `.beads/`, not the host

**Verification**
- [ ] `go test ./internal/harness/ -short -v` passes
- [ ] `bd where` from sandbox shim resolves to sandbox `.beads/`
- [ ] `bd dolt killall` is no longer called in sandbox setup

**Depends on**
None

## Bead 2: Cleanup Leaked Test Issues

**Goal**: Remove the 58+ test-generated issues from prod beads.

**Steps**

1. Query all issues (open + closed) matching test patterns: `test-feature`, `main-feature`, `hotfix-bug`, `artifact-check`, `plan-artifact`
2. Close each open one, then delete via `bd sql` if `bd` supports deletion, or mark with metadata to filter them out
3. Verify with `bd list` and `bd stats` that no test issues remain

**Verification**
- [ ] `bd list --status=open | grep -cE "test-feature|main-feature|hotfix-bug|artifact-check|plan-artifact"` returns 0
- [ ] `bd list --status=closed | grep -cE "test-feature|main-feature|hotfix-bug|artifact-check|plan-artifact"` returns 0
- [ ] `go test ./internal/harness/ -short -v` still passes (no regressions from cleanup)

**Depends on**
None (independent of Bead 1)

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| No test issue leakage into host `bd list` | Bead 1 verification + Bead 2 cleanup |
| Parallel tests without port contention | Bead 1: each sandbox gets `--server-port 0` (random), no `killall` |
| `bd dolt killall` scoped/removed | Bead 1 step 2 |
| Sandbox dolt server stopped in `t.Cleanup()` | Bead 1 step 3 |
| Existing LLM tests pass | Bead 1 verification |
| Leaked test issues purged | Bead 2 verification |
