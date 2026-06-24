# Spec 031-agentmind-codex-support: AgentMind Support for Codex Agents

## Goal

Enable AgentMind to visualize Codex agents in real time with parity to current Claude support (tool activity, token usage, and replay), using Codex's native OpenTelemetry support as the primary integration path.

## Background

Codex now supports OpenTelemetry configuration in `~/.codex/config.toml` (including `otel.environment`, `otel.exporter`, `otel.log_user_prompt`, and `otel.trace_exporter`) and can export via `otlp-http` or `otlp-grpc`.

AgentMind already operates as MindSpec's unified telemetry collector on port `4318` (ADR-0011) and already ingests OTLP/HTTP logs/metrics. The project docs are outdated where they state AgentMind is unavailable for Codex.

Codex session JSONL files (`$CODEX_HOME/sessions/...`) remain a useful fallback source when OTEL is disabled or unavailable.

## Impacted Domains

- **viz**: Codex OTEL compatibility in live ingestion and replay normalization
- **observability**: Codex setup/bootstrap path for OTEL log export to AgentMind
- **bench**: Aggregation compatibility for Codex-origin token/call events
- **docs**: Update Codex and AgentMind docs to reflect real support and setup

## ADR Touchpoints

- [ADR-0009](../../adr/ADR-0009.md): AgentMind event model and graph semantics remain authoritative
- [ADR-0011](../../adr/ADR-0011.md): AgentMind remains the single collector; Codex joins as another OTEL source

## Requirements

### 1. OTEL-First Codex Integration

1. Define Codex setup for AgentMind using OTLP/HTTP to `localhost:4318/v1/logs`.
2. Provide an explicit Codex OTEL configuration path with sane defaults:
   - `otel.exporter = "otlp-http"`
   - `otel.trace_exporter = "none"` (unless/until AgentMind trace ingestion is implemented)
   - `otel.log_user_prompt = false` by default
3. Add a CLI-assisted setup flow (or equivalent helper output) that guides users to configure Codex OTEL for AgentMind.
4. If Codex OTEL config already exists with a different endpoint, do not override silently; show a warning and remediation.

### 2. Codex Event Normalization

5. Extend normalization and metric aggregation to recognize Codex-origin OTEL event shapes and map them into existing AgentMind graph semantics.
6. Ensure Codex tool activity creates `agent -> tool` edges and tool-result state updates.
7. Ensure Codex token/cost-related events contribute to dashboard and replay metrics (best effort where cost is unavailable).
8. Ensure multi-agent identity works for Codex via resource attributes/session identifiers, consistent with existing identity precedence.

### 3. Runtime and Replay Compatibility

9. `mindspec agentmind serve` must ingest Codex OTEL events without any Codex-specific runtime mode required.
10. Codex-origin events written via `--output` must be replayable with existing `mindspec agentmind replay` flow.
11. Mixed-source sessions (Claude + Codex to same collector) must render as distinct agent nodes.

### 4. JSONL Fallback Path

12. Provide a fallback ingestion path from Codex local session JSONL when OTEL export is unavailable.
13. Fallback path must map session records to the same `bench.CollectedEvent` NDJSON schema used by replay and reporting.
14. Fallback ingestion must tolerate unknown/malformed lines and continue processing.

### 5. Documentation and UX

15. Update `docs/guides/codex.md` to replace the “AgentMind unavailable” limitation with OTEL setup instructions.
16. Update `docs/guides/agentmind.md` with a Codex section covering:
   - OTEL config snippet
   - privacy note on `otel.log_user_prompt`
   - fallback JSONL import workflow
17. Document known differences vs Claude telemetry completeness, if any.

## Scope

### In Scope

- Codex OTEL setup/bootstrap logic and/or helper command surface
- `internal/viz/normalize.go` and related tests for Codex-origin events
- `internal/viz/live.go`/ingestion path updates needed for Codex event compatibility
- `internal/bench/report.go` aggregation compatibility updates
- Optional Codex session JSONL fallback adapter and command wiring
- `docs/guides/codex.md` and `docs/guides/agentmind.md`

### Out of Scope

- Adding OTLP gRPC ingestion to AgentMind in this spec (HTTP path is primary)
- Full OpenTelemetry trace visualization in AgentMind
- Automatic mutation of global user config outside explicit user action/approval

## Non-Goals

- Replacing or regressing existing Claude OTEL support
- Redesigning AgentMind's graph types
- Guaranteeing exact cost accounting when Codex does not emit cost fields

## Acceptance Criteria

- [ ] Codex can stream telemetry into `mindspec agentmind serve` over OTLP/HTTP and appear live in the graph
- [ ] Codex tool calls and tool results render as expected tool edges/events
- [ ] Codex token usage appears in AgentMind metrics/replay outputs
- [ ] `mindspec agentmind serve --output <file>` captures Codex-origin events replayable via `mindspec agentmind replay <file>`
- [ ] Mixed Codex + Claude telemetry can be visualized concurrently with distinct agent identity
- [ ] Fallback JSONL ingestion works when OTEL is disabled
- [ ] Existing Claude OTEL tests and flows remain green
- [ ] `make test` passes

## Validation Proofs

- Configure Codex OTEL to `otlp-http` endpoint `http://localhost:4318/v1/logs` and run an interactive Codex session while `./bin/mindspec agentmind serve` is running: Codex activity appears live
- `./bin/mindspec agentmind serve --output /tmp/codex-live.ndjson` during Codex usage: output file grows with Codex-origin events
- `./bin/mindspec agentmind replay /tmp/codex-live.ndjson`: Codex session replays with graph + metrics
- JSONL fallback command on an existing Codex session file produces NDJSON that replays successfully
- `go test ./internal/viz/... ./internal/bench/...` and `make test` pass

## Open Questions

*None. Resolved decisions:*
- Codex integration is OTEL-first, not JSONL-first
- OTLP/HTTP is the baseline path for AgentMind compatibility
- JSONL ingestion remains fallback, not primary

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-16
- **Notes**: Approved via mindspec approve spec
