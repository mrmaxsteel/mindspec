# Spec 030-bench-fanout: AgentMind as Unified OTLP Collector

## Goal

Make AgentMind the single OTLP receiver for all telemetry — recording, benchmarking, and live visualization. Eliminate the separate headless collector processes and extra ports. Operators always have a live visualization URL available, and NDJSON is written to disk as a side effect.

## Background

Today there are three OTLP/HTTP receivers doing essentially the same work:

| Component | Port | Purpose | Code |
|-----------|------|---------|------|
| AgentMind | 4318 | Live 3D visualization | `viz.LiveReceiver` |
| Recording | 4319 | Per-spec NDJSON capture | `bench.Collector` (detached process) |
| Bench | 4320 | Benchmark NDJSON capture | `bench.Collector` (in-process) |

The recording and bench collectors are the same `bench.Collector` on different ports. AgentMind's `LiveReceiver` re-imports `bench.ExtractLogEvents` / `bench.ExtractMetricEvents` for its own parsing. All three accept identical OTLP/HTTP payloads.

During a benchmark run, telemetry goes exclusively to the bench collector on :4320. AgentMind on :4318 sees nothing — you can't watch what agents are doing. This is the immediate pain point.

The deeper issue is that three receivers is two too many. AgentMind already:
- Parses OTLP via `bench.ExtractLogEvents` / `bench.ExtractMetricEvents`
- Buffers all events in memory (`eventBuf`)
- Can export NDJSON via `EventsNDJSON()`

It just needs to **write NDJSON to disk** and be **auto-started** when needed. Then it replaces both headless collectors while also giving operators live visualization for free.

## Impacted Domains

- **viz**: AgentMind gains NDJSON disk output and background auto-start capability
- **bench**: Runner auto-starts AgentMind, removes its own collector, reads NDJSON from AgentMind's output
- **recording**: Replaces detached `record collect` process with detached `agentmind serve`

## ADR Touchpoints

- [ADR-0011](../../adr/ADR-0011.md): AgentMind as unified OTLP collector — the architectural decision this spec implements
- [ADR-0009](../../adr/ADR-0009.md): AgentMind architecture — amended by ADR-0011 to expand from visualization-only to unified collection
- [ADR-0010](../../adr/ADR-0010.md): Recording collector architecture — superseded by ADR-0011

## Requirements

### 1. AgentMind NDJSON Output

1. `agentmind serve` accepts an `--output <path>` flag. When set, every collected event is written to the NDJSON file as it arrives (in addition to feeding the graph and WebSocket broadcast). Append mode, so restarts don't truncate.

2. Without `--output`, AgentMind behaves as today — in-memory only, events available via `EventsNDJSON()` for the save-recording UI action.

3. NDJSON format is identical to the existing `bench.CollectedEvent` schema (same as today's `bench.Collector` output).

### 2. Auto-Start

4. `bench run` checks whether AgentMind is already running on :4318. If not, it starts `mindspec agentmind serve --output <bench-events.jsonl>` as a detached background process, waits for it to be ready, and prints: `AgentMind started — watch live at http://localhost:8420`

5. `record start` (per-spec recording) does the same: starts AgentMind as a detached process with `--output <spec-recording-dir>/events.ndjson` if not already running. If AgentMind is already running, it updates the output path (or appends to the existing output).

6. Detection is a simple HTTP probe to `http://localhost:4318/v1/logs` (or a health endpoint). If reachable, AgentMind is running.

### 3. Port Simplification

7. Claude Code's `OTEL_EXPORTER_OTLP_ENDPOINT` is always `http://localhost:4318`. One port, one receiver.

8. Remove port 4319 (recording collector) and port 4320 (bench collector) from the codebase. Remove `defaultRecordingPort` constant and `benchCollectorPort` constant.

9. `.claude/settings.json` keeps `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` (already the case). Remove the recording bootstrap's attempt to override it to :4319.

### 4. Bench Integration

10. `bench run` sets `OTEL_RESOURCE_ATTRIBUTES=bench.label=<a|b|c>` on sessions (unchanged from spec 028). Events are differentiated by label in the shared NDJSON.

11. `bench run` reads the NDJSON file that AgentMind is writing to for the quantitative report. `ParseSessionByLabel` filters by `bench.label` as before.

12. When bench completes, it does NOT stop AgentMind (it may be serving other purposes). It just stops reading the NDJSON file.

### 5. Recording Integration

13. `record start` starts AgentMind with `--output` pointing to the spec's recording directory. The manifest records AgentMind's PID instead of the headless collector's PID.

14. `record stop` sends SIGTERM to AgentMind (same lifecycle as current recording collector). If the user is also viewing the viz, they see it shut down — acceptable since recording end = spec lifecycle complete.

15. Phase markers and manifest management remain unchanged — they're orthogonal to which process is doing the collecting.

### 6. Standalone Use

16. `agentmind serve` continues to work standalone (without bench or recording). Without `--output`, it's a pure visualization tool. With `--output`, it doubles as a recorder.

17. `bench collect` and `record collect` are deprecated. They continue to work (for backward compat with scripts) but print a deprecation notice pointing to `agentmind serve --output`.

## Scope

### In Scope

- `internal/viz/live.go` — Add NDJSON disk writer alongside in-memory buffer
- `cmd/mindspec/viz.go` — Add `--output` flag to `agentmind serve`
- `internal/bench/runner.go` — Auto-start AgentMind, remove in-process collector, use AgentMind's NDJSON output
- `internal/recording/collector.go` — Start AgentMind instead of `record collect`
- `internal/recording/bootstrap.go` — Remove port 4319 endpoint override
- `cmd/mindspec/bench.go` — Remove `benchCollectorPort`, add AgentMind auto-start
- `cmd/mindspec/record.go` — Deprecation notice on `record collect`

### Out of Scope

- Changing AgentMind's graph normalization, WebSocket protocol, or UI
- Changing the recording manifest/phase system
- Multi-target fan-out (AgentMind is the single target)
- Merging `bench.Collector` type into `viz.LiveReceiver` (can be cleaned up later; the collector type still works for `bench collect` backward compat)

## Non-Goals

- Not adding a general-purpose OTel Collector pipeline
- Not changing how AgentMind replay works (it already reads NDJSON)
- Not requiring AgentMind to be running for Claude Code to function (if nothing is listening on :4318, telemetry is silently dropped by the OTLP exporter — this is standard OTEL behavior)

## Acceptance Criteria

- [ ] `agentmind serve --output events.ndjson` writes NDJSON to disk as events arrive
- [ ] `bench run` auto-starts AgentMind if not running, prints the viz URL
- [ ] AgentMind live viz shows agent activity during benchmark sessions
- [ ] Bench quantitative report reads from AgentMind's NDJSON output, correctly filtered by `bench.label`
- [ ] `record start` starts AgentMind as the collector (not a headless collector on :4319)
- [ ] Port 4319 and 4320 are no longer used anywhere in the codebase
- [ ] Claude Code always targets `http://localhost:4318`
- [ ] AgentMind not running does not break anything (OTLP exporter drops events silently)
- [ ] `make build` succeeds
- [ ] `go test ./internal/viz/... ./internal/bench/... ./internal/recording/...` passes

## Validation Proofs

- `mindspec agentmind serve --output /tmp/test-events.ndjson` — verify NDJSON appears on disk as Claude Code runs
- `mindspec bench run --spec-id <id> --prompt "..." --skip-qualitative --skip-commit --skip-cleanup` — verify AgentMind auto-starts, viz URL printed, live activity visible, quantitative report generated from NDJSON
- `mindspec record start --spec-id <id>` then do spec work — verify AgentMind is the collector process, events written to spec recording dir
- Stop AgentMind, run bench again — verify AgentMind auto-starts fresh

## Open Questions

*None.*

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-16
- **Notes**: Approved via mindspec approve spec