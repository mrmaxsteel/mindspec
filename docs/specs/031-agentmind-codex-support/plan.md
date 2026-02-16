---
adr_citations:
    - id: ADR-0009
      sections:
        - Decision
        - Decision Details
    - id: ADR-0011
      sections:
        - Decision
        - Auto-Start Behavior
        - Consequences
approved_at: "2026-02-16T13:36:23Z"
approved_by: user
generated:
    bead_ids:
        "1": mindspec-69y.2.2
        "2": mindspec-69y.2.3
        "3": mindspec-69y.2.4
        "4": mindspec-69y.2.5
        "5": mindspec-69y.2.6
    mol_parent_id: mindspec-69y.2
last_updated: "2026-02-16"
spec_id: 031-agentmind-codex-support
status: Approved
version: "0.1"
work_chunks:
    - depends_on: []
      id: 1
      scope: Codex OTEL configuration helper and warnings for endpoint conflicts
      title: Codex OTEL setup path
      verify:
        - Helper output/config update sets otlp-http endpoint to localhost:4318/v1/logs
        - Existing non-AgentMind endpoint is not silently overwritten
        - Unit tests cover merge and warning behavior
    - depends_on:
        - 1
      id: 2
      scope: Map Codex-origin OTEL logs/metrics into existing AgentMind graph semantics
      title: Codex OTEL event normalization
      verify:
        - Codex tool activity emits agent->tool edges
        - Codex token metrics appear in live stats and replay
        - Existing Claude event normalization remains unchanged
    - depends_on:
        - 2
      id: 3
      scope: Extend session aggregation/reporting to count Codex-origin token and call metrics
      title: Bench/report compatibility for Codex events
      verify:
        - bench report includes Codex-derived NDJSON metrics
        - Mixed Claude+Codex NDJSON is aggregated without regressions
        - go test ./internal/bench/... passes
    - depends_on:
        - 2
      id: 4
      scope: Fallback adapter from Codex local session JSONL to CollectedEvent NDJSON
      title: Codex session JSONL fallback ingest
      verify:
        - Fallback parser converts known Codex session records to replayable NDJSON
        - Malformed/unknown lines are skipped without aborting ingest
        - Fallback output replays in AgentMind
    - depends_on:
        - 1
        - 2
        - 3
        - 4
      id: 5
      scope: Update Codex/AgentMind docs and run OTEL-first + fallback validation proofs
      title: Docs and end-to-end validation
      verify:
        - docs/guides/codex.md no longer states AgentMind is unavailable
        - docs/guides/agentmind.md includes Codex setup and privacy notes
        - make test and key integration checks pass
---

# Plan: Spec 031 — AgentMind Support for Codex Agents

**Spec**: [spec.md](spec.md)

## Overview

Deliver Codex support through the existing AgentMind OTEL pipeline first (OTLP/HTTP to port 4318), then add a robust fallback adapter for Codex local session JSONL when OTEL is disabled or unavailable.

## ADR Fitness

### ADR-0009 (AgentMind: Embedded Real-Time Agent Visualization)
**Verdict: Adherent.** ADR-0009 established OTLP ingestion, graph normalization, and replay around a common event model. This plan keeps the same model and extends source compatibility to Codex telemetry. No structural divergence is required.

### ADR-0011 (AgentMind as Unified OTLP Collector)
**Verdict: Adherent with bounded extension.** OTEL-first Codex integration reinforces ADR-0011 by routing Codex into AgentMind on port 4318 as the single live collector. The JSONL fallback path is offline/import-oriented and does not introduce a competing live collector, so ADR-0011 remains sound.

No ADR divergence detected. No superseding ADR is required for this plan.

---

## Bead 1: Codex OTEL Setup Path

**Scope**: Add Codex-facing setup support so users can point Codex OTEL logs to `http://localhost:4318/v1/logs` safely and repeatably.

**Steps**:
1. Define Codex OTEL defaults and expected endpoint for AgentMind (`otlp-http`, `localhost:4318/v1/logs`, prompt logging off by default).
2. Implement a setup helper (command or guided output path) that can create/update Codex config with minimal edits.
3. Add conflict detection when Codex OTEL endpoint is already set to another collector.
4. Implement non-destructive behavior: warn by default; require explicit action to replace existing endpoint.
5. Add unit tests for config merge/update and warning paths.

**Verification**:
- [ ] Codex setup helper emits/applies valid config for AgentMind OTEL ingestion.
- [ ] Existing non-AgentMind endpoint produces explicit warning and no silent overwrite.
- [ ] Tests cover fresh config, merge config, and conflict scenarios.

**Depends on**: nothing

---

## Bead 2: Codex OTEL Event Normalization

**Scope**: Normalize Codex-origin OTEL logs/metrics into existing AgentMind node/edge and live stats behavior.

**Steps**:
1. Capture representative Codex OTEL fixtures (logs + metrics) and add them as test fixtures.
2. Extend normalization logic to recognize Codex-origin event names and attribute shapes for tool use/results and API activity.
3. Map Codex token metrics into existing token stat pathways used by live HUD and replay.
4. Ensure agent identity resolution works for Codex resource/session attributes without clobbering Claude behavior.
5. Add/extend tests in `internal/viz/normalize_test.go` for Codex fixtures.

**Verification**:
- [ ] Codex tool events produce visible `agent -> tool` edges.
- [ ] Codex token metrics contribute to live and replay metrics.
- [ ] Existing Claude normalization tests still pass.

**Depends on**: Bead 1

---

## Bead 3: Bench/Report Compatibility

**Scope**: Make bench/session reporting count Codex-origin events in `internal/bench/report.go`.

**Steps**:
1. Extend aggregation switch logic to include Codex event aliases for API-call and token metrics.
2. Preserve existing Claude event aggregation semantics and avoid double-counting mixed-source files.
3. Add report fixtures for Codex-only and mixed Claude+Codex NDJSON.
4. Update tests to assert expected totals/cost handling behavior for Codex records.

**Verification**:
- [ ] `ParseSession` and report outputs include Codex token and call metrics.
- [ ] Mixed-source fixture reports are correct and stable.
- [ ] `go test ./internal/bench/...` passes.

**Depends on**: Bead 2

---

## Bead 4: Codex Session JSONL Fallback Ingest

**Scope**: Provide a fallback path that converts Codex local session JSONL to `CollectedEvent` NDJSON for replay/analysis.

**Steps**:
1. Implement Codex session discovery and JSONL parser that reads records incrementally.
2. Map supported session records (tool calls/results, token snapshots, metadata) into `bench.CollectedEvent`.
3. Implement tolerant parsing: malformed/unknown records increment counters and continue.
4. Wire fallback ingest into `agentmind` CLI surface without interfering with OTEL live mode.
5. Add parser and mapping tests using real-world sample lines.

**Verification**:
- [ ] Fallback ingest produces replayable NDJSON from Codex session files.
- [ ] Unknown/malformed lines do not crash the ingest flow.
- [ ] `mindspec agentmind replay` works on generated fallback NDJSON.

**Depends on**: Bead 2

---

## Bead 5: Docs and End-to-End Validation

**Scope**: Update documentation and prove OTEL-first Codex support plus fallback behavior end-to-end.

**Steps**:
1. Update `docs/guides/codex.md` to document AgentMind support via OTEL and remove outdated limitation text.
2. Update `docs/guides/agentmind.md` with Codex setup snippet, privacy guidance (`otel.log_user_prompt`), and fallback flow.
3. Add validation commands to verify live OTEL ingest, NDJSON output, replay, and fallback conversion.
4. Run full test and targeted integration checks; record outcomes in plan/spec notes.

**Verification**:
- [x] Codex guide and AgentMind guide are consistent and actionable.
- [x] OTEL-first live flow and fallback flow both reproduce expected visualization/replay behavior.
- [x] `make test` passes.

**Validation Notes (2026-02-16)**:
- `go run ./cmd/mindspec agentmind setup codex --help` confirms single Codex setup command includes both OTEL setup and `--session` fallback conversion flags.
- `go test ./internal/viz -run TestLiveReceiverCodexMetricsCreateModelEdge -count=1` passed (OTEL-first Codex live ingest path).
- `go test ./internal/viz -run TestConvertCodexSessionFileProducesReplayableNDJSON -count=1` passed (fallback JSONL conversion + replayability).
- `go test ./internal/recording -run Codex -count=1` passed (Codex setup helper coverage).
- `go test ./internal/bench -run Codex -count=1` passed (Codex bench/report aggregation aliases).
- `make test` passed.

**Depends on**: Bead 1, Bead 2, Bead 3, Bead 4

---

## Dependency Graph

```text
Bead 1 (Codex OTEL setup)
   |
   v
Bead 2 (Codex OTEL normalization)
  / \
 v   v
Bead 3 (Bench/report)   Bead 4 (JSONL fallback ingest)
  \                       /
   \                     /
    v                   v
      Bead 5 (Docs + validation)
```
