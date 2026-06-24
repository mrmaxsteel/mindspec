---
adr_citations: []
approved_at: "2026-02-15T15:19:28Z"
approved_by: user
bead_ids:
    - mindspec-zsl
    - mindspec-lwm
    - mindspec-twj
    - mindspec-6xa
last_updated: "2026-02-15"
spec_id: 025-viz-multi-agent
status: Approved
version: 1
---

# Plan: Multi-Agent Identity in AgentMind Viz

## Summary

Thread OTLP resource attributes through the pipeline so each agent gets its own graph node. Four beads, executed in dependency order.

## ADR Fitness

No accepted ADRs constrain the viz subsystem. ADR-0001 (DDD context packs) and ADR-0003 (centralized instruction emission) are unrelated to the telemetry visualization pipeline. No divergence proposed.

## Bead 1: Extract OTLP resource attributes — `mindspec-zsl`

**Depends on**: None (first bead)

**Scope**: `internal/bench/collector.go`

**Steps**

1. Add `Resource map[string]any` field to `CollectedEvent` struct with JSON tag `"resource,omitempty"`
2. In `extractLogEvents()`, extend the `ResourceLogs` struct to include `Resource struct { Attributes []otlpKeyValue }` and call `flattenAttributes()` on it
3. Attach the flattened resource attributes to each `CollectedEvent.Resource` emitted from that `ResourceLogs` entry
4. In `extractMetricEvents()`, do the same for `ResourceMetrics[].Resource.Attributes`
5. Verify nil `Resource` when OTLP payload has no resource block (backwards compat)

**Verification**

- [ ] `go test ./internal/bench/...` passes
- [ ] Existing NDJSON files still parse correctly (Resource field omitted = nil)

## Bead 2: Agent identity resolution — `mindspec-lwm`

**Depends on**: mindspec-zsl

**Scope**: `internal/viz/normalize.go`

**Steps**

1. Add `resolveAgentID(resource map[string]any) (id string, label string)` with precedence: `agent.name` > `service.name`+`service.instance.id` > `service.name` > fallback `"claude-code"`
2. Replace all four hardcoded `agentID := "agent:claude-code"` sites in `NormalizeEvent()` with calls to `resolveAgentID(e.Resource)`
3. Replace all hardcoded `Label: "Claude Code"` with the resolved label
4. If `e.Resource["agent.parent"]` is a non-empty string, emit an additional parent agent node and an `EdgeSpawn` edge from parent to child
5. Handle edge case: if `agent.parent` equals the resolved agent name, skip the spawn edge (no self-loops)

**Verification**

- [ ] `go test ./internal/viz/...` passes
- [ ] Events with no Resource field produce `agent:claude-code` (backwards compat)
- [ ] Events with `service.name=my-agent` produce `agent:my-agent`

## Bead 3: EdgeSpawn type + frontend — `mindspec-twj`

**Depends on**: mindspec-lwm

**Scope**: `internal/viz/graph.go`, `internal/viz/web/app.js`

**Steps**

1. Add `EdgeSpawn EdgeType = "spawn"` constant to `graph.go` alongside existing edge types
2. Add `spawn: '#4fc3f7'` entry to `EDGE_COLORS` in `app.js` (agent blue, visually links parent↔child)
3. Verify multiple agent nodes render with correct labels via the existing `nodeLabel` callback (already uses `node.label`)

**Verification**

- [ ] `go build ./...` compiles without errors
- [ ] `app.js` references new spawn color
- [ ] No regressions in existing edge rendering (visual check via replay)

## Bead 4: Tests — `mindspec-6xa`

**Depends on**: mindspec-lwm, mindspec-twj

**Scope**: `internal/bench/collector_test.go`, `internal/viz/normalize_test.go`

**Steps**

1. Add collector test: OTLP payload with `resource.attributes` containing `service.name` → `CollectedEvent.Resource["service.name"]` populated
2. Add collector test: OTLP payload without `resource` key → `CollectedEvent.Resource` is nil
3. Add normalize test: event with `Resource: {"agent.name": "main"}` → agent node ID is `agent:main`, label is `main`
4. Add normalize test: event with `Resource: {"agent.name": "sub-1", "agent.parent": "main"}` → two agent nodes + spawn edge
5. Add normalize test: event with nil Resource → fallback `agent:claude-code` with label `Claude Code`
6. Add normalize test: event with `Resource: {"service.name": "foo", "service.instance.id": "bar"}` → agent node ID is `agent:foo:bar`

**Verification**

- [ ] `make test` passes with all new test cases
- [ ] No test flakiness on repeated runs

## Dependency Graph

```
mindspec-zsl (collector)
    |
    v
mindspec-lwm (normalizer)
    |
    v
mindspec-twj (graph + frontend)
    |
    v
mindspec-6xa (tests)
```
