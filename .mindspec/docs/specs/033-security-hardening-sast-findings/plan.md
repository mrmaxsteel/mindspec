---
adr_citations:
    - id: ADR-0009
      sections:
        - ADR Fitness
    - id: ADR-0011
      sections:
        - ADR Fitness
approved_at: "2026-02-28T08:26:14Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-28"
spec_id: 033-security-hardening-sast-findings
status: Approved
version: 1
---

# Plan: 033-security-hardening-sast-findings — Security Hardening for SAST Findings

## Overview

Six focused changesets addressing the validated SAST findings. Each bead is independently testable. Beads 1-4 are independent and can be implemented in parallel; bead 6 provides shared validation helpers that bead 5 uses.

## ADR Fitness

- **ADR-0009** (AgentMind: Embedded Real-Time Agent Visualization): Sound. This plan hardens the HTTP/WS server established by ADR-0009 (loopback binding, origin checks, body limits, timeouts) without changing its architecture or feature set. No divergence.
- **ADR-0011** (AgentMind as Unified OTLP Collector): Sound. ADR-0011 consolidates collection into AgentMind, meaning security controls applied to AgentMind's server cover all collector paths. The plan applies identical hardening to the standalone bench collector for parity. No divergence.

## Testing Strategy

- **Unit tests**: Each bead adds focused tests in the corresponding `_test.go` files. Table-driven tests for validation, origin checks, and body limits.
- **Integration**: `make test` must pass after each bead. `go test ./internal/viz/... ./internal/bench/... ./internal/recording/... ./internal/approve/...` covers all affected packages.
- **Manual verification**: Validation proofs from the spec (curl commands for 413, origin rejection, loopback binding) exercised post-implementation.

## Provenance

| Spec Acceptance Criterion | Bead | Verification |
|:--------------------------|:-----|:-------------|
| AC1: loopback default, opt-in remote | Bead 1 | Unit test: default Addr contains `127.0.0.1`; `--bind 0.0.0.0` overrides |
| AC2: cross-origin WS rejected | Bead 2 | Unit test: evil.example origin → 403 |
| AC3: oversized POST → 413 | Bead 3 | Unit test: body > limit → 413; body at limit → 200 |
| AC4: timeout fields + max header | Bead 4 | Unit test: server timeout fields non-zero |
| AC5: PID identity mismatch → refuse | Bead 5 | Unit test: wrong process name → error, no signal |
| AC6: traversal spec IDs rejected | Bead 6 | Unit test: `../etc/passwd` → error; `033-slug` → pass |
| AC7: regression tests pass in CI | All | `make test` passes |

## Bead 1: Loopback Binding + Listen Flag

**Steps**
1. Change `http.Server.Addr` in `internal/viz/server.go`, `internal/viz/live.go`, and `internal/bench/collector.go` from `fmt.Sprintf(":%d", port)` to `fmt.Sprintf("127.0.0.1:%d", port)`
2. Add a `BindAddr` field to `Server` and `Live` structs in viz package; default to `"127.0.0.1"`, use in `Addr` formatting
3. Add `--bind` flag to `agentmind serve` in `cmd/mindspec/viz.go` (default `"127.0.0.1"`), thread through to both `Server` and `Live`
4. `bench/collector.go` binds to loopback unconditionally (no flag — always local)
5. Add unit tests: default Addr contains `127.0.0.1:`; explicit `--bind 0.0.0.0` produces `0.0.0.0:port`

**Verification**
- [ ] `go test ./internal/viz/... ./internal/bench/...` passes
- [ ] `make test` passes

**Depends on**
None

## Bead 2: WebSocket Origin Enforcement

**Steps**
1. In `internal/viz/server.go`, remove `InsecureSkipVerify: true` from `websocket.AcceptOptions`
2. Add `OriginPatterns: []string{"localhost", "127.0.0.1", "[::1]"}` to cover loopback origin variations (nhooyr.io/websocket v1.8.17 auto-authorizes request host match; patterns cover cross-representation)
3. When `--bind 0.0.0.0` is set, log a warning that WebSocket origin checks remain local-only
4. Add table-driven test: local origins accepted, `https://evil.example` rejected with 403

**Verification**
- [ ] `go test ./internal/viz/...` passes with origin tests
- [ ] `make test` passes

**Depends on**
None

## Bead 3: Request Body Limits

**Steps**
1. Define package-level constants: `maxReplayBodySize = 64 << 20` (64 MB) in `internal/viz/server.go`, `maxOTLPBodySize = 4 << 20` (4 MB) in `internal/viz/live.go` and `internal/bench/collector.go`
2. In `handleReplayUpload` (server.go), wrap body: `r.Body = http.MaxBytesReader(w, r.Body, maxReplayBodySize)` before `io.ReadAll`
3. In `handleLogs` and `handleMetrics` (live.go and collector.go), wrap body with `http.MaxBytesReader(w, r.Body, maxOTLPBodySize)`
4. After `io.ReadAll`, check for `*http.MaxBytesError` and return `413 Payload Too Large`
5. Add tests: body exceeding limit → 413; body at limit → 200; zero-length body → 200

**Verification**
- [ ] `go test ./internal/viz/... ./internal/bench/...` passes with body limit tests
- [ ] `make test` passes

**Depends on**
None

## Bead 4: HTTP Timeout Hardening

**Steps**
1. In all three `http.Server` initializations (server.go, live.go, collector.go), add timeout fields: `ReadHeaderTimeout: 10 * time.Second`, `ReadTimeout: 30 * time.Second`, `WriteTimeout: 60 * time.Second`, `IdleTimeout: 120 * time.Second`
2. Set `MaxHeaderBytes: 1 << 20` (1 MB) on all three servers
3. Add unit tests asserting each server's timeout fields and MaxHeaderBytes are non-zero after construction

**Verification**
- [ ] `go test ./internal/viz/... ./internal/bench/...` passes
- [ ] `make test` passes

**Depends on**
None

## Bead 5: PID Verification in Recording Stop

**Steps**
1. Add `ProcessName` field to `Manifest` struct in `internal/recording/manifest.go`; populate at collector start with the expected binary name
2. In `internal/recording/collector.go` `StopCollector()`, before signaling: validate PID > 0, send signal 0 for existence check, then verify process identity via command name
3. In `internal/recording/proc_unix.go`, add `processName(pid int) (string, error)` using `exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")` (portable across Linux and macOS)
4. Compare returned command name against `Manifest.ProcessName`; if mismatch → set manifest status to `stale`, return error (fail closed)
5. Update `signalTerminate` to return error instead of discarding with `_`
6. Add tests: wrong process name → error, no signal; matching process → signal sent; PID 0 / negative → immediate error

**Verification**
- [ ] `go test ./internal/recording/...` passes with PID verification tests
- [ ] `make test` passes

**Depends on**
Bead 6

## Bead 6: Spec ID Validation + Path Containment

**Steps**
1. Create `internal/validate/specid.go` with `SpecID(id string) error` using regex `^[0-9]{3}-[a-z0-9]+(?:-[a-z0-9]+)*$`; rejects empty, `.`, `..`, `/`, `\`, and non-matching input
2. Add `SafePath(root, path string) error` in same file: resolves to absolute via `filepath.EvalSymlinks`, confirms prefix under `root` with `strings.HasPrefix`
3. Add `validate.SpecID()` calls at entry points: `cmd/mindspec/viz.go` (replay `--spec` flag), `internal/approve/impl.go` (specID param), `internal/recording/*.go` (manifest SpecID on load)
4. Add `validate.SpecID()` as secondary guard inside `workspace.SpecDir()`
5. Add tests: valid IDs (`033-security-hardening`, `001-init`) pass; invalid IDs (`../etc/passwd`, `foo/bar`, `.`, empty) error; path containment rejects resolved paths outside root

**Verification**
- [ ] `go test ./internal/validate/... ./internal/workspace/...` passes
- [ ] `make test` passes

**Depends on**
None

## Risk Notes

- **nhooyr.io/websocket `OriginPatterns`**: confirmed supported in v1.8.17. Uses `filepath.Match` on origin host; request host is always auto-authorized.
- **Process inspection on macOS**: no `/proc` filesystem. `ps -p <pid> -o comm=` works on both Linux and macOS without build tags.
- **Body limit on replay**: 64 MB may be too small for very large sessions. Make it configurable later if needed (out of scope for this spec).
