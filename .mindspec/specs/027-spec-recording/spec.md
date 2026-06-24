# Spec 027-spec-recording: Automatic Per-Spec Agent Telemetry Recording

## Goal

Automatically capture agent telemetry for the full lifecycle of every spec — from `spec-init` through `impl-approve` — so any feature's development journey can be replayed in AgentMind. Recording is zero-friction: it starts when you create a spec and stops when implementation is approved.

## Background

ADR-0009 established AgentMind as the embedded real-time agent visualization system, and the bench system already collects OTLP telemetry into NDJSON files. However, recording is manual: the operator must run `mindspec bench collect`, configure the OTLP endpoint, and manage files. Recordings have no connection to the spec they belong to.

ADR-0010 decides that recording should be automatic, per-spec, and stored alongside spec artifacts. This spec implements that decision.

The result: after completing any spec, you can run `mindspec agentmind replay --spec 027-spec-recording` and watch the full journey — spec drafting, planning, implementation — as a 3D agent activity graph.

### OTLP Configuration Model

Claude Code reads OTLP environment variables at session startup only — there is no hot-reload. Rather than toggling OTLP config per-spec (which would require a session restart each time), OTLP is configured **once, permanently** during project bootstrap. The collector is the only thing that starts and stops per-spec.

Flow:
1. **One-time setup** (`mindspec init` or first `spec-init`): Write OTLP env vars to `.claude/settings.local.json`. User restarts Claude Code once.
2. **Per-spec**: `spec-init` starts a collector → telemetry immediately flows. `impl-approve` stops the collector → telemetry hits a dead endpoint and is silently dropped by the OTLP exporter (designed for this).
3. **No per-spec OTLP toggling**: The env vars stay set permanently. Zero session restarts after initial setup.

## Impacted Domains

- **workflow**: `spec-init`, `approve`, `next`, `complete` commands all gain recording integration
- **viz**: AgentMind replay gains `--spec` convenience flag and phase filtering
- **observability**: New `internal/recording/` package manages collector lifecycle and markers

## ADR Touchpoints

- [ADR-0009](../../adr/ADR-0009.md): AgentMind ingestion protocol (OTLP/HTTP) and NDJSON replay format — recording produces files in this exact format
- [ADR-0010](../../adr/ADR-0010.md): Governs all architectural decisions — single file per spec, background collector, co-located storage, zero-friction recording

## Requirements

### 1. One-Time OTLP Bootstrap

1. **`EnsureOTLP(root)`**: Check `.claude/settings.local.json` for OTLP env vars. If not present, add them:
   ```json
   {
     "env": {
       "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
       "OTEL_METRICS_EXPORTER": "otlp",
       "OTEL_LOGS_EXPORTER": "otlp",
       "OTEL_EXPORTER_OTLP_PROTOCOL": "http/json",
       "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4319"
     }
   }
   ```
   Merge into existing settings (preserve `permissions`, `hooks`, etc.). If OTLP env vars are already present with a different endpoint, leave them untouched and warn.

2. **First-run detection**: If `EnsureOTLP` wrote new config, print a one-time message: `"OTLP telemetry enabled. Restart Claude Code to begin recording."` Subsequent `spec-init` calls detect the config already exists and skip silently.

3. **Port choice**: Use port **4319** as the recording collector default, leaving 4318 free for `mindspec agentmind serve` (live viz).

### 2. Recording Package (`internal/recording/`)

4. **Manifest management**: Create, read, update `manifest.json` in `docs/specs/<id>/recording/`. Schema: `spec_id`, `started_at`, `collector_pid`, `collector_port`, `status` (recording/stopped/complete), `phases[]` (each with `phase`, `started_at`, `ended_at`, `beads[]`).

5. **Collector lifecycle**: Start a background OTLP collector that writes to `docs/specs/<id>/recording/events.ndjson`. Reuse `bench.Collector` internals (OTLP parsing, NDJSON writing). Write PID and port to manifest. The collector runs as a detached background process that survives Claude Code session boundaries.

6. **Health check**: Given a manifest, determine if the collector is alive (`kill -0 <pid>`). Return status: alive, dead, or no-recording.

7. **Restart**: If the collector is dead but the manifest says `status: recording`, restart it — same port, same output file (append mode), update PID in manifest.

8. **Stop**: Send SIGTERM to collector PID, update manifest `status` to `complete`, record final phase end time.

9. **Lifecycle marker emission**: Append marker events directly to `events.ndjson` (no HTTP round-trip). Marker events use the `CollectedEvent` schema with event names: `lifecycle.start`, `lifecycle.phase`, `lifecycle.bead.start`, `lifecycle.bead.complete`, `lifecycle.end`.

### 3. Workflow Integration

10. **`spec-init`**: After creating the spec directory and setting state: call `EnsureOTLP()`, create the recording directory, start the collector, and emit `lifecycle.start`.

11. **`approve spec`**: Emit `lifecycle.phase` marker (`spec` → `plan`). Update manifest phase tracking.

12. **`approve plan`**: Emit `lifecycle.phase` marker (`plan` → `plan-approved`). Update manifest.

13. **`next`**: Emit `lifecycle.bead.start` marker with bead ID. Update manifest with bead in current phase.

14. **`complete`**: Emit `lifecycle.bead.complete` marker with bead ID. Update manifest.

15. **`approve impl`**: Emit `lifecycle.end` marker. Stop collector. Update manifest `status` to `complete`. (OTLP env vars stay in settings — they're permanent.)

16. **SessionStart hook**: Add recording health check to the existing hook script. If active spec has a recording with `status: recording` and the collector is dead, restart it.

### 4. Replay Integration

17. **`--spec` flag on replay**: `mindspec agentmind replay --spec <id>` resolves to `docs/specs/<id>/recording/events.ndjson` and starts replay.

18. **`--phase` filter on replay**: `mindspec agentmind replay --spec <id> --phase plan` filters the NDJSON stream to events between the matching `lifecycle.phase` markers.

### 5. CLI Surface

19. **`mindspec record status`**: Show recording status for the active spec (or a given `--spec`). Outputs: spec ID, collector alive/dead, event count, current phase, elapsed time.

20. **`mindspec record stop`**: Manually stop recording for the active spec. For cases where the user wants to stop early or the lifecycle didn't complete cleanly.

## Scope

### In Scope

- New `internal/recording/` package (manifest, collector lifecycle, markers, health check, OTLP bootstrap)
- New `cmd/mindspec/record.go` (status, stop subcommands)
- Modifications to `cmd/mindspec/spec_init.go` (OTLP bootstrap + start recording)
- Modifications to `internal/approve/spec.go`, `internal/approve/plan.go`, `internal/approve/impl.go` (emit markers, stop on impl-approve)
- Modifications to `cmd/mindspec/next.go` and `internal/complete/complete.go` (emit markers)
- Modifications to `cmd/mindspec/viz.go` (add `--spec` and `--phase` flags to replay)
- Modifications to `.claude/hooks.json` or SessionStart hook script (health check)
- Tests for `internal/recording/`

### Out of Scope

- Fan-out (proxying to multiple OTLP endpoints simultaneously) — deferred per ADR-0010
- Recording aggregation / analytics (e.g., "total tokens across all specs")
- Recording pruning / cleanup / retention policies
- Modifying the AgentMind graph visualization itself (no new node types for lifecycle events)
- Recording during non-spec workflows (ad-hoc bench collect remains separate)

## Non-Goals

- Not replacing the existing `bench collect` / `bench report` workflow — that remains for manual benchmarking
- Not providing a recording UI/dashboard — replay through AgentMind is sufficient
- Not recording sub-agent telemetry beyond what Claude Code already emits via OTLP
- Not guaranteeing gap-free recording — if the collector dies between sessions and the hook doesn't fire, events are lost; partial recordings are fine

## Acceptance Criteria

- [ ] First `spec-init` writes OTLP env vars to `.claude/settings.local.json` (idempotent, preserves existing settings)
- [ ] `mindspec spec-init <id>` creates `docs/specs/<id>/recording/manifest.json` and starts a background collector on port 4319
- [ ] OTLP events from Claude Code appear in `docs/specs/<id>/recording/events.ndjson` during normal spec work (after session restart on first setup)
- [ ] `lifecycle.start` marker is the first event in every new recording
- [ ] `approve spec` emits a `lifecycle.phase` marker visible in the NDJSON
- [ ] `approve plan` emits a `lifecycle.phase` marker
- [ ] `next` emits a `lifecycle.bead.start` marker with the bead ID
- [ ] `complete` emits a `lifecycle.bead.complete` marker with the bead ID
- [ ] `approve impl` emits `lifecycle.end` and stops the collector (OTLP config stays)
- [ ] SessionStart hook restarts a dead collector for an active recording without data loss
- [ ] `mindspec record status` shows collector health, event count, and current phase
- [ ] `mindspec record stop` gracefully terminates the collector and updates manifest
- [ ] `mindspec agentmind replay --spec <id>` replays the full recording
- [ ] `mindspec agentmind replay --spec <id> --phase plan` replays only the planning phase
- [ ] Pre-existing specs (no recording directory) work with no errors — all recording code gracefully no-ops
- [ ] If OTLP env vars are already set to a different endpoint, `EnsureOTLP` warns and does not override
- [ ] `make test` passes with no regressions

## Validation Proofs

- `mindspec spec-init test-recording && cat docs/specs/test-recording/recording/manifest.json`: manifest exists with PID and port
- `kill -0 $(jq .collector_pid docs/specs/test-recording/recording/manifest.json)`: collector process is alive
- `jq .env .claude/settings.local.json`: OTLP env vars present
- `wc -l docs/specs/test-recording/recording/events.ndjson`: event count grows as agent works
- `grep lifecycle docs/specs/test-recording/recording/events.ndjson`: lifecycle markers present
- `mindspec record status`: shows recording active
- `mindspec agentmind replay --spec test-recording --speed 5`: full replay works in AgentMind
- `make test`: all tests pass

## Open Questions

*None — all design decisions are resolved.*

- Port 4319 for recording collector (4318 reserved for live AgentMind viz) — decided
- Always-on OTLP with one-time bootstrap — decided per discussion, documented in Background section
- Single file with phase markers vs per-phase files — decided per ADR-0010

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-16
- **Notes**: Approved via mindspec approve spec