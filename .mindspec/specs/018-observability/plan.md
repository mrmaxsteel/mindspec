---
adr_citations:
    - id: ADR-0003
      sections:
        - instruct
    - id: ADR-0004
      sections:
        - bench
        - trace
approved_at: "2026-02-13T21:38:59Z"
approved_by: user
bead_ids: [mindspec-arh, mindspec-0v5, mindspec-1qi]
last_updated: "2026-02-13"
spec_id: 018-observability
status: Approved
version: 1
---

# Plan: 018-observability — Observability & Benchmarking

## Overview

Three work chunks, layered bottom-up:

1. **Trace infrastructure** (`internal/trace/`) — event emitter, token estimator, NDJSON writer, no-op path
2. **Instrumentation** — wire tracing into contextpack, instruct, glossary, bead CLI, state transitions, command lifecycle; add `--trace` global flag
3. **Bench harness** (`internal/bench/`) — OTLP/HTTP JSON receiver, NDJSON parser, report + compare commands

## ADR Fitness

| ADR | Evaluation |
|:----|:-----------|
| ADR-0003 (Centralized Emission) | **Sound.** Trace hooks wrap `instruct.Render()` externally — no modification to the emission architecture. Adhering. |
| ADR-0004 (Go Language) | **Sound.** JSON OTLP via stdlib `encoding/json` — zero new deps. Adhering. |
| ADR-0001 (DDD + Context Packs) | **Sound.** Token estimation hooks into `packSection.Content` — respects the existing section-based assembly. Adhering. |
| ADR-0002 (Beads as Tracking) | **Sound.** We instrument the existing `bdcli.go` call pattern, not modify Beads itself. Adhering. |

No ADR divergence detected.

## Bead 1: Trace Infrastructure

**Scope**: `internal/trace/` package — event emitter, token estimator, NDJSON writer, no-op path.

**Steps**

1. Create `internal/trace/tracer.go` — `Tracer` interface with `Emit(event Event)` and `Close()`. Global `tracer` var (default no-op). `Init(path string) error` to activate (creates NDJSON writer, generates `RunID` UUID). `SetGlobal(t Tracer)` for testing.
2. Create `internal/trace/event.go` — `Event` struct: `TS` (RFC3339Nano), `Event` (dotted name), `RunID` (UUID), `SpecID` (string), `DurMs` (float64, omitempty), `Tokens` (int, omitempty), `Data` (map[string]any).
3. Create `internal/trace/tokens.go` — `EstimateTokens(s string) int` returning `len([]byte(s)) / 4`. `EstimateTokensBytes(b []byte) int`.
4. Create `internal/trace/noop.go` — `noopTracer` satisfying `Tracer`. Zero allocation. Create `internal/trace/writer.go` — `ndjsonTracer` writing JSON lines to `io.Writer`, thread-safe via mutex.
5. Write `internal/trace/tracer_test.go` — emit events to buffer, verify NDJSON, verify no-op produces zero output, verify token estimation.

**Verification**
- [ ] `Tracer` interface has `Emit` and `Close`
- [ ] NDJSON writer produces valid JSON lines with all Event fields
- [ ] No-op tracer produces zero output and zero allocations
- [ ] `EstimateTokens("hello world")` returns `len("hello world") / 4`
- [ ] `go test ./internal/trace/...` passes

**Depends on**: None

---

## Bead 2: Instrumentation + CLI Wiring

**Scope**: Wire tracing into existing code paths. Add `--trace` global flag. Add `trace summary` command.

**Steps**

1. Add `--trace` persistent flag to `cmd/mindspec/root.go`. In `PersistentPreRunE`, check flag and `MINDSPEC_TRACE` env var; call `trace.Init(path)`. In `PersistentPostRunE`, call `trace.Close()`. Emit `command.start` / `command.end` events (with duration, command name).
2. Instrument context-pack: after `Build()` returns in `cmd/mindspec/context.go`, emit `contextpack.build` event with `tokens_total`, per-section breakdown, section count. Instrument instruct: after `Render()` returns in `cmd/mindspec/instruct.go`, emit `instruct.render` with `tokens_total`, `mode`, `template`.
3. Instrument glossary: after `Match()` returns in `cmd/mindspec/glossary.go`, emit `glossary.match` with `query`, `hit_count`, `tokens_matched`. Instrument state: in `internal/state/state.go` `SetMode()`, emit `state.transition` with `from`, `to`, `spec_id`.
4. Instrument bead CLI: add `tracedRun(op string, args []string) ([]byte, error)` helper in `internal/bead/bdcli.go` wrapping `execCommand` with timing. Emit `bead.cli` event with `op`, `args`, `dur_ms`, `ok`. Refactor existing functions to use it.
5. Add `cmd/mindspec/trace.go` with `trace summary <file>` subcommand: reads NDJSON, aggregates by event type, prints total duration, total tokens, per-type breakdown. Write tests with fixture NDJSON data.

**Verification**
- [ ] `mindspec context-pack --trace /tmp/t.jsonl 018-observability` writes NDJSON with `command.start`, `contextpack.build`, `command.end`
- [ ] `contextpack.build` event has `tokens_total` (int > 0) and per-section `sections` map
- [ ] `instruct.render` event has `tokens_total`, `mode`, `template`
- [ ] Running without `--trace` produces zero trace output
- [ ] `mindspec trace summary /tmp/t.jsonl` prints readable aggregate
- [ ] `make test` passes

**Depends on**: Bead 1

---

## Bead 3: Bench Harness

**Scope**: OTLP/HTTP JSON receiver, report generation, A/B comparison. Zero new dependencies — stdlib `encoding/json` + `net/http`.

**Steps**

1. Create `internal/bench/collector.go` — HTTP server accepting `POST /v1/logs` and `POST /v1/metrics` (OTLP/HTTP JSON). Parse JSON payloads, extract `claude_code.api_request` events and `claude_code.token.usage` metrics. Write normalized NDJSON: `ts`, `event`, `data` (input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cost_usd, duration_ms, model). Graceful shutdown on context cancellation.
2. Create `internal/bench/report.go` — `ParseSession(path string) (*Session, error)` reads NDJSON. `Session` struct: API call count, total input/output/cache tokens, total cost, wall-clock duration, per-model breakdown. Optional MindSpec trace enrichment if trace events present.
3. Create `internal/bench/compare.go` — `Compare(a, b *Session, labels [2]string) *Report`. `FormatTable(r *Report) string` for human output. `FormatJSON(r *Report) string` for machine output. Deltas show absolute and percentage differences.
4. Add `cmd/mindspec/bench.go`: `bench setup` prints env var blocks for two sessions (ports 4318/4319, includes `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`); `bench collect --port <port> --output <path>` starts collector with SIGINT shutdown; `bench report <file1> <file2> --labels "mindspec,baseline" [--format table|json]` produces comparison.
5. Write tests: OTLP JSON parsing with fixture data, report aggregation, comparison with known deltas.

**Verification**
- [ ] `mindspec bench setup` prints two env var blocks with different ports and `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`
- [ ] `mindspec bench collect --port 4318 --output /tmp/test.jsonl` accepts `POST /v1/logs` with JSON body and writes NDJSON
- [ ] `mindspec bench report` with two fixture files produces table with token/cost/time columns and delta row
- [ ] `--format json` produces valid JSON output
- [ ] `make test` passes

**Depends on**: Bead 2

---

## Implementation Order

```
Bead 1: Trace Infrastructure (no deps)
  ↓
Bead 2: Instrumentation + CLI Wiring (depends on Bead 1)
  ↓
Bead 3: Bench Harness (depends on Bead 2)
```

## Risk Notes

- **Claude Code JSON OTLP support**: We depend on `OTEL_EXPORTER_OTLP_PROTOCOL=http/json` working in Claude Code's OTel JS SDK. The OTel spec mandates it, but should be verified with a real session early in Bead 3. If unsupported, fallback is adding the protobuf dep.
- **Claude Code OTel event schema**: The exact event field names depend on Claude Code's implementation. If the schema differs from what we expect, the collector will need adjustment. Mitigated by testing with a real Claude Code session early in Bead 3.
