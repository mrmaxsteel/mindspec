# Spec 025-viz-multi-agent: Multi-Agent Identity in AgentMind Viz

## Goal

Allow multiple agents (and sub-agents) to appear as distinct, identifiable nodes in the AgentMind Viz when they send telemetry to the same OTEL collector. Today every event is attributed to a single hardcoded `agent:claude-code` node, so two agents are indistinguishable.

## Background

The viz pipeline (`live.go` → `collector.go` → `normalize.go` → `graph.go`) currently:

1. **Ignores OTLP resource attributes** — `extractLogEvents()` and `extractMetricEvents()` only parse `logRecords[].attributes` / `dataPoints[].attributes`, discarding the `resource.attributes` array that OTLP provides at the `ResourceLogs` / `ResourceMetrics` level.
2. **Hardcodes agent identity** — `NormalizeEvent()` sets `agentID := "agent:claude-code"` on every event regardless of origin.
3. **Has no sub-agent concept** — the graph model supports a flat `NodeAgent` type but no parent/child relationship.

OTLP already has a standard mechanism for source identity: **resource attributes** such as `service.name` and `service.instance.id`. Agents can also set custom attributes (e.g. `agent.name`, `agent.parent`). We should extract and use these rather than inventing a parallel identity scheme.

## Impacted Domains

- **viz**: normalize, graph, live receiver, frontend rendering
- **bench**: `CollectedEvent` schema gains resource-attribute passthrough

## ADR Touchpoints

None — no existing ADRs are affected.

## Requirements

1. **Extract OTLP resource attributes** — `extractLogEvents()` and `extractMetricEvents()` must parse `resource.attributes` from `ResourceLogs` / `ResourceMetrics` and pass them through to `CollectedEvent`.
2. **Agent identity resolution** — `NormalizeEvent()` must derive a unique agent ID from the event's resource attributes, using a precedence chain: `agent.name` → `service.name` + `service.instance.id` → `service.name` → fallback `"claude-code"`.
3. **Distinct agent nodes** — Each unique agent identity produces its own `NodeAgent` node (e.g. `agent:main-agent`, `agent:sub-agent-1`) with edges from that specific agent to the tools/LLMs it calls.
4. **Sub-agent edges** — If an event carries `agent.parent`, create an edge from the parent agent node to the child agent node (type: new `EdgeSubAgent` or similar), expressing the hierarchy visually.
5. **Frontend agent differentiation** — The legend and node rendering should support multiple agent nodes. Each agent gets a label from its identity. Shared tools/LLMs that both agents call should have edges from each respective agent node (not merged into one).
6. **Backwards compatibility** — Events with no resource attributes (e.g. replayed NDJSON from before this change) must still work, falling back to the current `agent:claude-code` behavior.

## Scope

### In Scope

- `internal/bench/collector.go` — `CollectedEvent` schema, resource attribute extraction
- `internal/viz/normalize.go` — agent identity resolution
- `internal/viz/graph.go` — new edge type for sub-agent relationship
- `internal/viz/web/app.js` — handle multiple agent nodes in rendering
- `internal/viz/web/index.html` — no structural changes expected (legend already dynamic via CSS)
- Tests for all changed files

### Out of Scope

- Configuring how external agents set their OTLP resource attributes (that's the agent's responsibility)
- Agent-scoped recording/dashboard filtering (future spec)
- Cross-session agent tracking or persistence

## Non-Goals

- Per-agent token/cost breakdown in the recording dashboard (separate future work)
- Agent discovery or registration protocol — agents self-identify via OTLP resource attributes
- Changes to the `internal/trace` package (MindSpec's own tracer is not involved)

## Acceptance Criteria

- [ ] Two agents sending OTLP to the same viz instance with different `service.name` values appear as two separate agent nodes in the graph
- [ ] An agent with `agent.parent` set creates a visible parent→child edge in the graph
- [ ] Events with no resource attributes fall back to `agent:claude-code` (backwards compat)
- [ ] `CollectedEvent` includes a `Resource` field containing flattened resource attributes
- [ ] Existing tests pass; new tests cover multi-agent normalization, resource attribute extraction, and sub-agent edge creation
- [ ] Replaying an existing NDJSON session file (pre-multi-agent) still renders correctly

## Validation Proofs

- `make test`: All existing and new tests pass
- Manual: Start viz, point two agents (with different `OTEL_SERVICE_NAME` or `agent.name`) at the collector → two distinct agent nodes appear
- Manual: Replay an existing NDJSON session → single `agent:claude-code` node (backwards compat)

## Open Questions

None — all resolved during research.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-15
- **Notes**: Approved via mindspec approve spec