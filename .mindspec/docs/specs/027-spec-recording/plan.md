---
adr_citations:
    - id: ADR-0009
      sections:
        - Requirements
    - id: ADR-0010
      sections:
        - Requirements
approved_at: "2026-02-16T08:33:00Z"
approved_by: user
bead_ids:
    - mindspec-vtb
    - mindspec-3ry
    - mindspec-5mp
    - mindspec-rci
last_updated: "2026-02-16"
spec_id: 027-spec-recording
status: Approved
version: 1
---

# Plan: 027-spec-recording — Automatic Per-Spec Agent Telemetry Recording

## Summary

Implement ADR-0010: automatic, zero-friction OTLP telemetry recording for every spec lifecycle. Creates `internal/recording/` package (manifest, collector lifecycle, markers, OTLP bootstrap), integrates lifecycle markers into all workflow commands, extends AgentMind replay with `--spec` and `--phase` flags, adds `mindspec record {status,stop,health}` CLI commands, and wires the SessionStart hook to keep collectors alive across sessions.

## ADR Fitness

| ADR | Decision | Fit? | Notes |
|:----|:---------|:-----|:------|
| ADR-0009 | OTLP ingestion, CollectedEvent NDJSON schema, replay | **Yes** | Recording reuses CollectedEvent schema for lifecycle markers. Replay already handles NDJSON streaming; we add `--spec` and `--phase` convenience flags. |
| ADR-0010 | Auto per-spec recording, single file, background collector | **Yes** | Governing ADR. All choices (port 4319, co-located storage, manifest schema, one-time OTLP bootstrap) are directly implementable. Minor discrepancy: ADR says remove OTLP config on impl-approve but spec says permanent config — following spec (later, more refined). |

No ADR divergence — all accepted ADRs remain sound for this work.

## Technical Notes

The bench `Collector` runs in-process (goroutine) but recording needs a process surviving session restarts. We launch `mindspec record collect` as a detached subprocess via `SysProcAttr{Setsid: true}`. For append-mode restart, `bench.Collector` gets a `NewCollectorAppend` constructor that opens with `O_APPEND` instead of `O_CREATE|O_TRUNC`.

## Bead A: Recording Package Core — `mindspec-vtb`

**Scope**: `internal/recording/`, `internal/bench/collector.go`, `cmd/mindspec/record_collect.go`

**Steps**

1. Create `internal/recording/manifest.go` — `Manifest` struct (`SpecID`, `StartedAt`, `CollectorPID`, `CollectorPort`, `Status`, `Phases[]`), `ReadManifest`/`WriteManifest`, path helpers (`RecordingDir`, `ManifestPath`, `EventsPath`)
2. Create `internal/recording/bootstrap.go` — `EnsureOTLP(root)` reads/merges `.claude/settings.local.json` env block with OTLP vars (port 4319), idempotent, warns on endpoint conflict
3. Create `internal/recording/collector.go` — `StartCollector(root, specID)` launches `mindspec record collect` as detached subprocess, writes PID/port to manifest; `StopCollector(root, specID)` sends SIGTERM, updates manifest status; add `NewCollectorAppend` to `bench/collector.go` for append-mode opens
4. Create `internal/recording/health.go` — `HealthCheck(root, specID)` reads manifest, checks PID alive via `syscall.Kill(pid, 0)`; `RestartIfDead(root, specID)` restarts in append mode if manifest says recording but PID is dead
5. Create `internal/recording/markers.go` — `EmitMarker(root, specID, event, data)` appends `bench.CollectedEvent` line to `events.ndjson`; convenience wrappers `EmitPhaseMarker` and `EmitBeadMarker`
6. Create `internal/recording/recording.go` — `StartRecording` orchestrates create-dir + start-collector + emit `lifecycle.start`; `StopRecording` orchestrates emit `lifecycle.end` + stop-collector + finalize manifest
7. Create `internal/recording/recording_test.go` — test manifest round-trip, marker NDJSON format, `EnsureOTLP` idempotency; add `cmd/mindspec/record_collect.go` hidden subcommand running `bench.NewCollectorAppend`

**Verification**

- [ ] `go test ./internal/recording/...` passes
- [ ] `make build` compiles with new package
- [ ] `EnsureOTLP` writes correct JSON to `.claude/settings.local.json` in tests

**Depends on**: None (foundation bead)

## Bead B: Workflow Integration — `mindspec-3ry`

**Scope**: `internal/specinit/specinit.go`, `internal/approve/{spec,plan,impl}.go`, `cmd/mindspec/next.go`, `internal/complete/complete.go`

**Steps**

1. Modify `internal/specinit/specinit.go` — after setting state, call `recording.EnsureOTLP(root)` and `recording.StartRecording(root, specID)` with errors as warnings
2. Modify `internal/approve/spec.go` — after state transition to plan, call `recording.EmitPhaseMarker(root, specID, "spec", "plan")` best-effort
3. Modify `internal/approve/plan.go` — after state transition, call `recording.EmitPhaseMarker(root, specID, "plan", "plan-approved")` best-effort
4. Modify `internal/approve/impl.go` — before transitioning to idle, call `recording.StopRecording(root, specID)` best-effort
5. Modify `cmd/mindspec/next.go` — after claiming bead and setting state, call `recording.EmitBeadMarker(root, specID, "start", beadID)` best-effort
6. Modify `internal/complete/complete.go` — after closing bead, call `recording.EmitBeadMarker(root, specID, "complete", beadID)` best-effort
7. Verify graceful no-op: all recording calls check for recording directory existence; pre-existing specs with no recording dir produce zero errors

**Verification**

- [ ] `make test` passes with no regressions
- [ ] Lifecycle markers appear in `events.ndjson` at each workflow transition
- [ ] Pre-existing specs (no recording dir) work with zero errors

**Depends on**: Bead A (uses `internal/recording/` API)

## Bead C: Replay Integration & CLI Surface — `mindspec-5mp`

**Scope**: `cmd/mindspec/viz.go`, `internal/viz/replay.go`, `cmd/mindspec/record.go`

**Steps**

1. Modify `cmd/mindspec/viz.go` — add `--spec` string flag to `agentmindReplayCmd`; when set, resolve path via `workspace.SpecDir` + `/recording/events.ndjson`; make positional arg optional when `--spec` provided
2. Modify `cmd/mindspec/viz.go` — add `--phase` string flag, pass to `viz.RunReplay` as new parameter
3. Modify `internal/viz/replay.go` — add phase filtering: when phase set, skip events until `lifecycle.phase` marker with matching `to` field, stream until next phase marker or `lifecycle.end`
4. Create `cmd/mindspec/record.go` — `recordCmd` parent with `status` (manifest + health + event count + phase) and `stop` (call `recording.StopRecording`) subcommands
5. Register `recordCmd` in command tree and verify `make build` compiles

**Verification**

- [ ] `make build` compiles
- [ ] `mindspec agentmind replay --spec <id>` resolves path correctly
- [ ] `mindspec agentmind replay --spec <id> --phase plan` filters events to plan phase only
- [ ] `mindspec record status` and `mindspec record stop` work correctly

**Depends on**: Bead A (uses `internal/recording/` API for status/stop)

## Bead D: SessionStart Hook & Final Integration — `mindspec-rci`

**Scope**: `.claude/settings.json`, `cmd/mindspec/record.go`, `internal/workspace/workspace.go`

**Steps**

1. Add `record health` hidden subcommand to `cmd/mindspec/record.go` — reads state, checks active spec recording, calls `recording.RestartIfDead`, exits silently
2. Modify `.claude/settings.json` — extend SessionStart hook to chain `./bin/mindspec record health 2>/dev/null` after existing instruct command
3. Add `workspace.RecordingDir(root, specID)` helper to `internal/workspace/workspace.go` for consistency with existing path helpers
4. Verify full lifecycle end-to-end: `spec-init` → collector running → kill → hook restarts → approve through lifecycle → collector stopped
5. Run `make test` to confirm all tests pass with no regressions

**Verification**

- [ ] SessionStart hook runs without errors when no recording exists
- [ ] SessionStart hook restarts a dead collector for an active recording
- [ ] `make test` passes

**Depends on**: Bead B (workflow integration must be wired), Bead C (record commands must exist)

## Dependencies

```
A (Recording Core)       mindspec-vtb
├── B (Workflow)          mindspec-3ry  — depends on A
├── C (Replay & CLI)      mindspec-5mp  — depends on A
└── D (Hook & Final)      mindspec-rci  — depends on B, C
```
