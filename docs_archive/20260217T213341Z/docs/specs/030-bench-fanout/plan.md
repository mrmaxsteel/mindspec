---
adr_citations:
    - id: ADR-0009
      sections:
        - Decision
        - Decision Details
    - id: ADR-0010
      sections:
        - Decision
        - Decision Details
        - Collector Lifecycle
    - id: ADR-0011
      sections:
        - Decision
        - Key Changes from ADR-0010
approved_at: "2026-02-16T12:10:08Z"
approved_by: user
bead_ids:
    - mindspec-tga
    - mindspec-dnj
    - mindspec-z6x
    - mindspec-x0w
    - mindspec-rnk
last_updated: "2026-02-16"
spec_id: 030-bench-fanout
status: Approved
version: 1
---

# Plan: Spec 030 — AgentMind as Unified OTLP Collector

## Overview

Collapse three OTLP receivers (AgentMind :4318, recording :4319, bench :4320) into one: AgentMind gains NDJSON disk output, and both recording and bench auto-start it instead of running headless collectors. Port 4318 becomes the single telemetry endpoint.

## Dependency Graph

```
mindspec-tga  AgentMind --output NDJSON disk writer
    ↓
mindspec-dnj  AgentMind auto-start utility
    ↓               ↓
mindspec-z6x      mindspec-x0w
Recording         Bench integration
integration
    ↓               ↓
        mindspec-rnk
    Port cleanup & deprecation
```

## ADR Fitness

### ADR-0009 (AgentMind: Embedded Real-Time Agent Visualization)
**Verdict: Adherent, expanded scope.** ADR-0009 established AgentMind as an OTLP/HTTP receiver for live visualization. This plan expands its role to include durable NDJSON recording — a natural extension. The OTLP ingestion, event parsing, and port allocation (4318) are unchanged. ADR-0009 remains sound; ADR-0011 documents the expanded scope.

### ADR-0010 (Automatic Per-Spec Agent Telemetry Recording)
**Verdict: Superseded by ADR-0011.** ADR-0010's core decision — a separate headless collector on port 4319 — is eliminated. The recording model (spec-scoped NDJSON, phase markers, manifest, auto-start lifecycle) is preserved; only the underlying process changes from `record collect` to `agentmind serve --output`. ADR-0011 already supersedes ADR-0010.

No ADR divergence detected. No new ADRs needed.

---

## Bead 1: AgentMind --output NDJSON disk writer (`mindspec-tga`)

**Scope**: `internal/viz/live.go`, `internal/viz/run.go`, `cmd/mindspec/viz.go`

**Depends on**: None (first bead)

**Steps**

1. Add `outputPath string` and `outputFile *os.File` fields to `LiveReceiver` struct in `internal/viz/live.go`
2. Add a `SetOutput(path string)` method on `LiveReceiver` that stores the path; in `Run()`, open the file with `O_APPEND|O_CREATE|O_WRONLY` and close on shutdown
3. In `processEvents()`, after buffering to `eventBuf`, write each event as NDJSON to `outputFile` using `json.Marshal` + newline (same pattern as `bench.Collector.writeEvents()`) under a write mutex
4. Update `RunLive()` in `internal/viz/run.go` to accept `outputPath string` parameter and call `receiver.SetOutput(outputPath)` when non-empty
5. Add `--output` flag to `agentmindServeCmd` in `cmd/mindspec/viz.go` and pass it to `RunLive()`

**Verification**

- [ ] `mindspec agentmind serve --output /tmp/test.ndjson` writes NDJSON lines to disk as OTLP events arrive
- [ ] Events are still processed by graph + WebSocket (viz unaffected)
- [ ] Restart with same output path appends rather than truncates
- [ ] `go test ./internal/viz/...` passes

---

## Bead 2: AgentMind auto-start utility (`mindspec-dnj`)

**Scope**: new `internal/viz/autostart.go`

**Depends on**: mindspec-tga

**Steps**

1. Create `internal/viz/autostart.go` with `IsRunning(port int) bool` that probes `http://localhost:<port>/v1/logs` with a short timeout; returns true if connection succeeds
2. Add `AutoStart(root string, otlpPort, uiPort int, outputPath string) (pid int, err error)` that checks `IsRunning()`, finds the mindspec binary, execs `mindspec agentmind serve --otlp-port <port> --ui-port <uiPort> --output <outputPath>` as a detached process (`SysProcAttr{Setsid: true}`, nil stdin/stdout/stderr), waits for port ready, and returns PID
3. Add `WaitForPort(port int, timeout time.Duration) error` helper that polls until connection succeeds or timeout expires
4. Extract `findMindspecBinary(root)` from `internal/recording/collector.go` into a shared location (or duplicate the 10-line function)
5. Print `AgentMind started — watch live at http://localhost:<uiPort>` to stderr on successful start

**Verification**

- [ ] `AutoStart` with AgentMind not running starts process and returns PID > 0
- [ ] `AutoStart` with AgentMind already running returns 0 with no new process
- [ ] `IsRunning()` returns correct bool based on port state
- [ ] `go test ./internal/viz/...` passes

---

## Bead 3: Recording integration (`mindspec-z6x`)

**Scope**: `internal/recording/collector.go`, `internal/recording/bootstrap.go`, `internal/recording/health.go`

**Depends on**: mindspec-dnj

**Steps**

1. Update `StartCollector()` in `collector.go` to call `viz.AutoStart(root, 4318, 8420, eventsPath)` instead of execing `mindspec record collect`; update manifest with returned AgentMind PID
2. Update `bootstrap.go`: replace `defaultRecordingPort = 4319` with 4318 and update `EnsureOTLP()` so the expected endpoint is `http://localhost:4318`
3. Update `RestartIfDead()` in `health.go` — it already calls `StartCollector()` which now starts AgentMind; verify no other changes needed
4. Verify marker code (`markers.go`) continues to work — markers append directly to events.ndjson via `O_APPEND`, which is atomic for small POSIX writes alongside AgentMind's append writes

**Verification**

- [ ] `mindspec spec-init <test-id>` starts AgentMind (not `record collect`), manifest shows correct PID
- [ ] Events appear in `docs/specs/<id>/recording/events.ndjson` via AgentMind
- [ ] `http://localhost:8420` shows live viz during spec work
- [ ] Health check detects dead AgentMind and restarts it
- [ ] `go test ./internal/recording/...` passes

---

## Bead 4: Bench integration (`mindspec-x0w`)

**Scope**: `internal/bench/runner.go`, `internal/bench/session.go`, `cmd/mindspec/bench.go`

**Depends on**: mindspec-dnj

**Steps**

1. Remove `benchCollectorPort = 4320` constant from `runner.go`
2. In `Run()`, replace in-process collector startup: if AgentMind is already running (`viz.IsRunning(4318)`), reuse it; otherwise call `viz.AutoStart(root, 4318, 8420, benchEventsPath)`. Print `Watch live at http://localhost:8420` to stdout.
3. Update `buildSessionEnv()` in `session.go`: change OTLP endpoint from `localhost:4320` to `localhost:4318`
4. Remove collector shutdown logic after sessions complete — AgentMind persists (may serve recording). Bench just stops reading the NDJSON file.
5. Update `cmd/mindspec/bench.go` to remove any port flags referencing 4320

**Verification**

- [ ] `bench run` auto-starts AgentMind if not running, prints viz URL
- [ ] AgentMind UI shows live agent activity during bench sessions
- [ ] Quantitative report generates correctly from NDJSON filtered by `bench.label`
- [ ] Bench with already-running AgentMind (active recording) reuses it without port conflict
- [ ] `go test ./internal/bench/...` passes

---

## Bead 5: Port cleanup and deprecation (`mindspec-rnk`)

**Scope**: `cmd/mindspec/record.go`, `cmd/mindspec/bench.go`, `internal/recording/bootstrap.go`, docs

**Depends on**: mindspec-z6x, mindspec-x0w

**Steps**

1. Add deprecation notice to `record collect` command in `cmd/mindspec/record.go`: print `Deprecated: use 'mindspec agentmind serve --output <path>' instead` to stderr before proceeding
2. Add deprecation notice to `bench collect` command in `cmd/mindspec/bench.go`: same pattern
3. Search codebase for remaining references to ports 4319 and 4320; remove or update each occurrence
4. Update `docs/core/BENCHMARKING.md` and any other docs referencing port 4319 or 4320

**Verification**

- [ ] `mindspec record collect` prints deprecation warning but still works
- [ ] `mindspec bench collect` prints deprecation warning but still works
- [ ] No references to port 4319 or 4320 remain in `internal/` or `cmd/` (except deprecation messages)
- [ ] `make build` succeeds
- [ ] Full test suite passes: `make test`
