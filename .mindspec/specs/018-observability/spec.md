# Spec 018-observability: Observability & Benchmarking

## Goal

Enable A/B comparison of MindSpec-assisted vs freestyle Claude Code sessions. Capture token cost, cycle time, and per-phase breakdown so operators can quantify MindSpec's impact on development efficiency.

## Background

MindSpec claims that spec-driven development produces better outcomes — less token waste, tighter iterations, higher-quality output. Today there is no way to measure this. The operator has to take it on faith.

The target experiment:
1. Two VSCode sessions, same repo, same commit (different worktrees)
2. One uses MindSpec (spec → plan → implement). One goes freestyle.
3. Both receive an identical feature description.
4. At the end, compare: **token cost**, **cycle time**, and **quality** (qualitative).

Claude Code already emits OpenTelemetry metrics including `claude_code.api_request` events with full token breakdowns (input, output, cache read, cache creation, cost, duration). MindSpec needs to:
1. Capture that OTel data from both sessions
2. Add its own enrichment (context pack sizes, phase timing)
3. Produce a clear comparison report

## Impacted Domains

- **cli**: New `mindspec bench` subcommand tree; `--trace` flag on root command
- **contextpack**: Emit token estimates on build
- **instruct**: Emit token estimates on render
- **glossary**: Emit match stats

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): Centralized instruction emission — trace hooks into the instruct pipeline
- [ADR-0004](../../adr/ADR-0004.md): Go language — OTLP receiver and trace infra implemented in Go

## Requirements

### Benchmark Harness

1. `mindspec bench collect --port <port> --output <path>` starts a lightweight OTLP/HTTP receiver that writes Claude Code's OTel events to NDJSON
2. Each VSCode session configures `CLAUDE_CODE_ENABLE_TELEMETRY=1` and `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:<port>` to send telemetry to the collector
3. `mindspec bench report <session-a.jsonl> <session-b.jsonl> --labels "mindspec,baseline"` produces a comparison table:
   - Total input tokens, output tokens, cache read tokens, cache creation tokens
   - Total estimated cost (USD)
   - Wall-clock duration (first event → last event)
   - API call count
4. `mindspec bench setup` prints the env vars and instructions for configuring both VSCode sessions (copy-pasteable)

### MindSpec-Side Tracing (Enrichment)

5. Structured event log emitted as NDJSON via `--trace <path>` flag or `MINDSPEC_TRACE` env var
6. Zero overhead when tracing is disabled (no-op path)
7. Events carry: `ts` (RFC3339Nano), `event` (dotted name), `run_id` (UUID), `spec_id`, and optionally `dur_ms` and `tokens`
8. Token estimation: `tokens ≈ bytes / 4` (Claude-family approximation, sufficient for comparison)
9. Instrumented events:
   - `command.start` / `command.end`: command name, duration, exit status
   - `contextpack.build`: total token estimate, per-section breakdown (spec, glossary, domains, ADRs, policies, context-map)
   - `instruct.render`: total token estimate, template name, mode
   - `glossary.match`: query term, hit count, matched content token estimate
   - `bead.cli`: operation name, raw command + args, duration, success/failure
   - `state.transition`: from-mode, to-mode, spec-id
10. `mindspec trace summary <file>` prints aggregate stats: total duration, total tokens, per-event-type breakdown

### Comparison Report

11. `mindspec bench report` merges OTel data (actual API tokens) with MindSpec trace data (context sizes, phases) when both are available for a session
12. Report output includes:
    - **Cost**: total tokens (in/out/cache), estimated USD, tokens per API call
    - **Time**: wall-clock duration, time per phase (spec/plan/implement — MindSpec session only)
    - **Efficiency**: output tokens per input token ratio, cache hit rate
    - **Context**: MindSpec context-pack token overhead (MindSpec session only)
13. Report supports `--format table` (default, human-readable) and `--format json` (machine-readable)

## Scope

### In Scope

- `internal/trace/` package: event emitter, no-op implementation, NDJSON writer, token estimator
- `internal/bench/` package: OTLP/HTTP receiver, NDJSON parser, report generator, comparison logic
- `cmd/mindspec/bench.go`: `bench collect`, `bench setup`, `bench report` subcommands
- `cmd/mindspec/trace.go`: `trace summary` subcommand
- Root command `--trace` flag and `MINDSPEC_TRACE` env var wiring
- Instrumentation of: command lifecycle, context-pack build, instruct render, glossary match, bead CLI calls, state transitions

### Out of Scope

- Full OTel SDK in MindSpec (we receive OTLP, we don't export it)
- Automated quality scoring (quality remains qualitative / human-judged)
- Persistent benchmark database or historical trending
- Network exporters (no forwarding to Jaeger/Datadog/etc.)
- Exact tokenizer (byte-based estimation is sufficient for MindSpec-side; Claude Code OTel provides exact counts for API usage)

## Non-Goals

- Not a logging framework — existing `fmt.Fprintf` warnings stay as-is
- Not a cost management tool — this is for comparative benchmarking, not billing
- No automated "MindSpec is better" verdict — the operator interprets the data
- No modification to Claude Code itself

## Acceptance Criteria

- [ ] `mindspec bench setup` prints copy-pasteable env var blocks for two sessions (with different ports)
- [ ] `mindspec bench collect --port 4318 --output /tmp/session-a.jsonl` starts an OTLP/HTTP receiver that captures `claude_code.api_request` events and writes NDJSON
- [ ] A Claude Code session configured with `CLAUDE_CODE_ENABLE_TELEMETRY=1` and `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` successfully sends events to the collector
- [ ] `mindspec bench report /tmp/session-a.jsonl /tmp/session-b.jsonl --labels "mindspec,baseline"` outputs a table with: total tokens (in/out/cache), cost, duration, API call count for each session, with deltas
- [ ] `mindspec context-pack --trace /tmp/ms-trace.jsonl 018-observability` writes NDJSON including `contextpack.build` with per-section `tokens` breakdown
- [ ] `mindspec trace summary /tmp/ms-trace.jsonl` prints total duration, total estimated tokens, per-event-type breakdown
- [ ] Running any command without `--trace` produces zero trace output and no measurable overhead
- [ ] `--format json` on `bench report` outputs machine-readable JSON for downstream processing

## Validation Proofs

- `mindspec bench setup`: Expected output includes two env var blocks with different ports and labels
- `mindspec bench collect --port 4318 --output /tmp/test.jsonl &` then `curl -X POST http://localhost:4318/v1/logs -H 'Content-Type: application/json' -d '{"resourceLogs":[]}' && kill %1`: Expected: collector accepts POST, writes to file, shuts down cleanly
- `mindspec context-pack --trace /tmp/t.jsonl 018-observability && jq 'select(.event=="contextpack.build") | .data.tokens_total' /tmp/t.jsonl`: Expected output is an integer > 0
- `mindspec trace summary /tmp/t.jsonl`: Expected output includes total duration and token breakdown

## Open Questions

- [x] ~~Should `--trace` accept a path or enable stderr?~~ Resolved: path to file, `--trace=-` for stderr.
- [x] ~~Should trace events include a correlation ID?~~ Resolved: yes, `run_id` UUID per invocation.
- [x] ~~Bead CLI call granularity?~~ Resolved: one event per subprocess, include raw command + args.
- [x] ~~Should `bench collect` also capture cumulative metrics?~~ Resolved: yes, capture both `claude_code.api_request` events and `claude_code.token.usage` cumulative counters.
- [x] ~~Quality annotations in `bench report`?~~ Resolved: out of scope. Quality is assessed qualitatively outside the tool.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via mindspec approve spec