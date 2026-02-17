---
adr_citations:
    - id: ADR-0009
      sections:
        - Decision Details
        - Claude Code OTLP Field Inventory
    - id: ADR-0010
      sections:
        - Decision Details
        - OTLP Configuration
approved_at: "2026-02-16T11:27:18Z"
approved_by: user
bead_ids:
    - mindspec-s7l
    - mindspec-9gw
last_updated: "2026-02-16"
spec_id: 028-bench-single-collector
status: Approved
version: 1
---

# Plan: 028-bench-single-collector

## Summary

Replace bench's 3-collector architecture with a single shared OTLP collector. Two implementation beads: (1) core architecture change — single collector lifecycle, `bench.label` injection, `Port` removal; (2) filtered report parsing, session ID tracking, and test updates.

## ADR Fitness

### ADR-0009 (AgentMind: Embedded Real-Time Agent Visualization)
**Verdict: Conform.** ADR-0009 confirms that `OTEL_RESOURCE_ATTRIBUTES` flows through Claude Code's OTLP exporter. This spec relies directly on that mechanism for `bench.label` tagging. The ADR also establishes OTLP/HTTP as the ingestion protocol and documents that resource attributes are extracted into `CollectedEvent.Resource` — both of which this plan depends on without modification. No divergence needed.

### ADR-0010 (Automatic Per-Spec Agent Telemetry Recording)
**Verdict: Conform.** ADR-0010 reserves port 4319 for the recording collector. This plan removes bench's use of ports 4318–4320 in favor of a single port (4320, avoiding both 4318/AgentMind and 4319/recording). The single-collector approach also aligns better with ADR-0010's architecture — bench and recording use the same collector code with different ports and output files, no longer competing for port ranges.

## Bead 1: Single Collector + Label Injection

**Scope**: Replace 3 per-session collectors with 1 shared collector. Inject `bench.label` resource attribute per session.

**Depends on**: None

**Steps**

1. Remove port constants `portA`, `portB`, `portC` from `runner.go`. Add single `benchCollectorPort = 4320`.
2. Remove `Port` field from `SessionDef` in `session.go`. Remove port assignments from session definitions in `runner.go`.
3. Move collector lifecycle to `Run()` in `runner.go`: start one `NewCollector(benchCollectorPort, benchEventsPath)` before the session loop, writing to `{WorkDir}/bench-events.jsonl`. Shut down after all sessions complete. Remove per-session collector start/stop from `RunSessionWithRetries()`.
4. Inject `OTEL_RESOURCE_ATTRIBUTES=bench.label=<label>` in `buildSessionEnv()` (`session.go`). Change `OTEL_EXPORTER_OTLP_ENDPOINT` to use the single `benchCollectorPort` for all sessions.
5. Update `RunSessionWithRetries()` to accept collector port as parameter instead of reading from `def.Port`. Remove collector creation/shutdown from this function.
6. Update `CheckPortFree` calls in `Run()` to check only the single bench collector port.

**Verification**

- [ ] `bench run` starts exactly 1 collector process (single port bind)
- [ ] All 3 sessions emit to the same `bench-events.jsonl`
- [ ] Events have correct `Resource["bench.label"]` values (`"a"`, `"b"`, `"c"`)
- [ ] No port 4319 usage (recording port preserved)

**Files**: `internal/bench/runner.go`, `internal/bench/session.go`

---

## Bead 2: Filtered Report Parsing + Tests

**Scope**: Add `ParseSessionByLabel()`, update report generation to use it, add `SessionIDs` field, update tests.

**Depends on**: Bead 1

**Steps**

1. Add `ParseSessionByLabel(path, label string) (*Session, error)` to `report.go`. Filter events by `Resource["bench.label"]` match. Extract shared aggregation logic from `ParseSession()` into a helper; keep `ParseSession()` unchanged for backward compat.
2. Add `SessionIDs []string` to `SessionResult` (`session.go`). After session completes, scan JSONL for unique `Data["session.id"]` values where `Resource["bench.label"]` matches the label.
3. Update `Run()` report generation in `runner.go`: replace `ParseSession(sessionFile, label)` calls with `ParseSessionByLabel(benchEventsPath, label)` against the single JSONL.
4. Update `WriteResults()` in `markdown.go`: write single `bench-events.jsonl` instead of per-session files. Remove per-session port column from report template. Add session ID info.
5. Add tests in `report_test.go`: `ParseSessionByLabel` with mixed-label JSONL, `ParseSession` backward compat on standalone files, `SessionIDs` extraction.
6. Verify `make build` succeeds and `go test ./internal/bench/...` passes.

**Verification**

- [ ] `ParseSessionByLabel("bench-events.jsonl", "a")` returns only session A metrics
- [ ] `ParseSession("legacy.jsonl", "a")` still works on standalone files
- [ ] Quantitative report shows correct per-session breakdowns from single file
- [ ] `SessionResult.SessionIDs` contains correct UUIDs
- [ ] All tests pass

**Files**: `internal/bench/report.go`, `internal/bench/session.go`, `internal/bench/runner.go`, `internal/bench/markdown.go`, `internal/bench/report_test.go` (new)
