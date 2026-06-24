---
approved_at: "2026-03-04T17:00:35Z"
approved_by: user
status: Approved
---
# Spec 070-sandbox-beads-isolation: Sandbox Beads Isolation

## Goal

Ensure LLM test harness sandboxes are fully isolated from the host project's beads database, preventing test-created epics/issues from leaking into production and enabling safe parallel test execution.

## Background

The LLM test harness (`internal/harness/`) creates sandbox git repos in `t.TempDir()` and runs `bd init --sandbox --server-port 0` to bootstrap beads. Two problems have been observed:

1. **Epic leakage into prod**: Test scenarios create epics (`[SPEC 001-test-feature]`, `[SPEC 002-main-feature]`, etc.) that appear in the host project's `bd list`. The `--sandbox` flag only disables auto-sync but doesn't prevent the sandbox's dolt server from sharing the host's database. When the agent runs `bd create` during a test, it may resolve to the host's dolt instance (port 14209) rather than the sandbox's ephemeral server.

2. **Parallel port contention**: Running multiple LLM tests in parallel causes `bd init` failures because dolt servers compete for ports/slots. `bd dolt killall` in `initBeads()` is a blunt workaround that can kill the host's own server.

Root cause: `bd` resolves its database connection by walking up directories to find `.beads/` or by connecting to a well-known dolt server. The sandbox's `.beads/` dir exists in a temp directory, but the agent session may inherit or discover the host's dolt connection.

## Impacted Domains

- `internal/harness/sandbox.go`: beads initialization and env isolation
- `internal/harness/session.go`: agent session env vars

## ADR Touchpoints

- None — this is a test infrastructure fix, no architectural decisions affected

## Requirements

1. Sandbox beads must use a completely isolated dolt database — no reads or writes to the host project's beads
2. `bd` commands run by the agent inside a sandbox must connect only to the sandbox's own dolt server
3. Multiple sandboxes must be able to run in parallel without port contention
4. `bd dolt killall` in sandbox setup must not kill the host project's dolt server
5. Sandbox cleanup (`t.Cleanup()`) must stop the sandbox's dolt server and release its port
6. Clean up the 58+ leaked test issues already polluting the host project's beads database (46 open + 12 closed epics/tasks with titles matching `test-feature`, `main-feature`, `hotfix-bug`, `artifact-check`, `plan-artifact`)

## Scope

### In Scope
- `internal/harness/sandbox.go` — `initBeads()`, `runBD()`, `Env()`, `NewSandbox()`
- `internal/harness/session.go` — agent session environment
- One-time cleanup script/command to purge leaked test issues from prod beads

### Out of Scope
- Changes to the `bd` CLI itself (we work within its existing env/flag interface)
- Dolt server management outside the test harness

## Non-Goals

- Making `bd --sandbox` itself provide full isolation (that's a beads feature request)
- Fixing test flakiness unrelated to beads isolation

## Acceptance Criteria

- [ ] Running `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m` does not create any issues in the host project's `bd list`
- [ ] Running two LLM tests in parallel does not cause dolt port contention failures
- [ ] `bd dolt killall` in sandbox setup is scoped to sandbox servers only (or removed)
- [ ] Each sandbox's dolt server is stopped in `t.Cleanup()`
- [ ] Existing LLM test scenarios continue to pass
- [ ] All 58+ leaked test issues (matching `test-feature`, `main-feature`, `hotfix-bug`, `artifact-check`, `plan-artifact`) are purged from the host project's beads

## Validation Proofs

- `bd list --status=open | grep -c "test-feature\|main-feature\|hotfix-bug"`: should return 0 after a test run
- `go test ./internal/harness/ -short -v`: deterministic tests pass
- `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m -count=1`: passes without host pollution

## Open Questions

- [x] Does `bd` support env vars to force `.beads/` resolution? **Yes**: `--db` global flag overrides database path. `BEADS_DB` env var also works but requires the db to exist. The primary isolation mechanism is CWD-based `.beads/` discovery — if the sandbox has its own `.beads/` and `bd` is run from within the sandbox, it should resolve locally. The issue is that `bd` walks up directories and follows redirects.
- [x] Does `bd` support `BEADS_SERVER_PORT` env var? **No env var**, but `--server-port` flag on `init` specifies the port. The chosen port is written to `.beads/dolt-server.port` after startup.
- [x] Can `--server-port 0` output the chosen port? **Yes** — the port is written to `.beads/dolt-server.port` after init. The sandbox can read this file and pass the port to the agent env.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-04
- **Notes**: Approved via mindspec approve spec