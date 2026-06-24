# Spec 022: AgentMind Viz MVP

## Goal

Give operators a real-time, visually rich 3D visualization of Claude Code agent activity. A `mindspec viz` command launches a local web server that ingests OpenTelemetry events (live or replayed) and renders them as an interactive "agent activity galaxy" — a force-directed 3D graph with starfield aesthetic, glowing nodes (agents, tools, MCP servers, data sources, LLM endpoints), and constellation-style edges (tool calls, MCP calls, reads, writes, model calls). The operator can observe, filter, and explore agent behavior without reading raw logs.

## Background

Spec 018 established MindSpec's observability plumbing: a structured NDJSON event stream and an OTLP/HTTP collector that captures Claude Code telemetry (`claude_code.api_request`, token metrics, cost data). Spec 019 automated benchmark collection, producing rich NDJSON session files. The `mindspec.md` product spec explicitly calls out a future "Agent Mind Visualization" with nodes (agents, tools, MCP servers, domains, beads, docs, ADRs) and edges (tool calls, context pack injections, bead transitions, verification runs).

Today all this data is consumed only as aggregate reports (`mindspec bench report`, `mindspec trace summary`). Operators cannot see the *shape* of agent activity — which tools cluster together, which MCP servers are hot, how context flows, or where latency concentrations live. A visual, interactive graph fills this gap.

The MVP prioritizes correctness and readability over visual polish. It must work with the data already captured by the existing OTLP collector (Spec 018) and the MindSpec trace stream, requiring no changes to Claude Code itself.

## Impacted Domains

- **core**: New `mindspec viz` subcommand; new `internal/viz/` package
- **workflow**: No changes — viz consumes trace events passively

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): CLI-first tooling — `mindspec viz` is a CLI command that starts a local HTTP server; all logic is in Go, frontend is embedded static assets
- [ADR-0004](../../adr/ADR-0004.md): Go implementation — backend in Go, single binary embeds all web assets via `embed.FS`

## Requirements

### Ingestion & Normalization

1. `mindspec viz` starts a local HTTP server (default `:8420`) that serves the web UI and provides a WebSocket endpoint for real-time event streaming
2. **Live mode**: `mindspec viz live --port <otlp-port>` starts an OTLP/HTTP receiver (reusing `internal/bench.Collector` patterns) that normalizes incoming Claude Code events into graph events and pushes them to connected WebSocket clients
3. **Replay mode**: `mindspec viz replay <file.jsonl>` reads a previously captured NDJSON file (from `mindspec bench collect` or `mindspec trace`) and streams it to the UI at configurable speed (1x, 5x, 10x, max)
4. A normalization layer (`internal/viz/normalize.go`) maps raw telemetry into:
   - **Node upserts**: `{id, type, label, attributes, lastSeen}` — types: `agent`, `tool`, `mcp_server`, `data_source`, `llm_endpoint`
   - **Edge events**: `{id, src, dst, type, status, startTime, endTime, duration, attributes}` — types: `tool_call`, `mcp_call`, `retrieval`, `write`, `model_call`
5. Normalization includes deduplication (same node ID → update, not create) and attribute merging (latest wins)
6. Retention/decay: nodes not seen in the last N events (configurable, default 200) get a `stale` flag; edges older than M seconds (configurable, default 120) get a `faded` flag. The UI uses these for visual decay but does not remove them.
7. Hard caps: max 500 nodes, max 2000 edges. Beyond limits, oldest stale nodes and faded edges are evicted. A `capped` flag in the HUD indicates when limits are active.

### Web UI & 3D Rendering

8. The web UI is a single-page application served from Go via `embed.FS` — no separate build step or npm required at runtime
9. 3D rendering uses Three.js (vendored or CDN-loaded) with a force-directed layout (3D force-graph library or custom force simulation)
10. **Starfield background**: dark canvas with subtle animated star particles
11. **Node rendering**: spheres with type-specific colors and glow effects (bloom/emissive). Size scales with activity count. Types are visually distinct (different colors, optional shape variation via geometry)
12. **Edge rendering**: animated lines between nodes with type-specific colors. Active edges pulse/glow; completed edges dim. Line thickness reflects call frequency.
13. **Force-directed layout**: recent interactions pull connected nodes closer; old/faded edges exert less force, allowing clusters to relax and drift apart. Gentle time-based morphing — no jarring jumps.
14. **Camera**: auto-orbits around center-of-mass by default. User can take manual control (orbit/pan/zoom via OrbitControls). A "reset" button re-centers and resumes auto-orbit.

### Interaction

15. **Hover**: shows a compact detail card (tooltip overlay) with: source node, destination node, edge type, duration, status, key attributes. For nodes: type, label, connection count, last seen time.
16. **Click**: pins the detail card. Click elsewhere or press Escape to unpin.
17. **Search/filter**: text input that filters nodes/edges by name, type, or attribute value. Matching nodes highlight; non-matching fade. Dropdown presets for: agent, tool, MCP server, type.
18. **Pause/resume**: toggle button pauses the event stream (events buffer but don't render). Resume replays buffered events.
19. **Legend/HUD**: small overlay showing: node type color legend, throughput (events/sec), error count, average latency, connection status (live/replay/paused), cap indicator.

### Noise & Safety

20. Raw prompts and completions are NOT displayed by default. The detail card shows event metadata only (type, duration, token counts, status). An explicit "show raw" toggle (off by default) can reveal full attributes.
21. Sampling: if inbound event rate exceeds 100 events/sec, the normalizer samples (keeps every Nth event to stay under 100/sec) and shows a "sampling active" indicator in the HUD.
22. Backpressure: if a WebSocket client cannot keep up, the server drops oldest undelivered events and increments a "dropped" counter visible in the HUD.

### CLI Interface

23. `mindspec viz live [--otlp-port 4318] [--ui-port 8420]` — starts OTLP receiver + web UI
24. `mindspec viz replay <file.jsonl> [--speed 1] [--ui-port 8420]` — replays recorded session
25. `mindspec viz --help` prints usage for both subcommands

## Scope

### In Scope

- `cmd/mindspec/viz.go`: `viz live` and `viz replay` subcommands
- `internal/viz/`: normalization, graph state, WebSocket hub, HTTP server
- `internal/viz/web/`: embedded static assets (HTML, JS, CSS — Three.js-based SPA)
- Reuse of OTLP parsing from `internal/bench/collector.go` (shared or extracted)

### Out of Scope

- Modifications to Claude Code itself
- Persistent storage of graph state (in-memory only)
- Multi-user collaboration or shared sessions
- Historical trending or analytics dashboards
- npm/webpack/bundler build step — web assets are vanilla JS + vendored Three.js
- Full OTel span correlation (MVP uses event-level mapping, not distributed traces)
- Mobile or responsive layout (desktop-only is fine for MVP)

## Non-Goals

- Not a monitoring/alerting system — no thresholds, no notifications
- Not a replacement for `mindspec bench report` — this is complementary visual exploration
- Not a production deployment target — localhost only, no auth, no TLS
- Not pixel-perfect design — functional correctness and readability over polish

## Acceptance Criteria

- [ ] `mindspec viz replay <file.jsonl>` opens a browser-ready page at `localhost:8420` showing a 3D graph that populates as events are replayed
- [ ] Nodes are created for each unique agent, tool, MCP server, data source, and LLM endpoint observed in the event stream
- [ ] Edges represent tool calls / MCP calls / reads / writes / model calls with correct source and destination nodes
- [ ] Edge durations reflect actual tool execution time from the telemetry (start → end delta)
- [ ] Hovering a node or edge shows a detail card with type, label, duration, status, and key attributes
- [ ] The search/filter input filters visible nodes by name or type; non-matching nodes fade
- [ ] Pause/resume stops and resumes event consumption without losing events
- [ ] The view remains responsive (>30 FPS) with 200 nodes and 1000 edges
- [ ] Raw prompts are not shown by default; the "show raw" toggle is off by default
- [ ] `mindspec viz live --otlp-port 4318` accepts OTLP events from a Claude Code session configured with `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` and renders them in real time
- [ ] Hard caps (500 nodes / 2000 edges) are enforced; the HUD shows a "capped" indicator when active
- [ ] Replay mode supports speed control (1x, 5x, 10x, max)

## Validation Proofs

- `mindspec viz replay docs/specs/018-observability/benchmark/session-a.jsonl --speed max &`: starts server; `curl -s http://localhost:8420/ | grep -c '<canvas'`: returns 1 (canvas element present)
- `mindspec viz live --otlp-port 4318 &` then `curl -X POST http://localhost:4318/v1/logs -H 'Content-Type: application/json' -d '{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"timeUnixNano":"1700000000000000000","body":{"stringValue":"claude_code.api_request"},"attributes":[{"key":"model","value":{"stringValue":"claude-sonnet-4-5-20250929"}},{"key":"input_tokens","value":{"intValue":"1000"}},{"key":"output_tokens","value":{"intValue":"500"}}]}]}]}]}'`: WebSocket clients receive a node upsert for the LLM endpoint and an edge event for the model call
- `mindspec viz --help`: prints usage covering both `live` and `replay` subcommands
- `make test`: all existing tests pass, new viz tests pass

## Open Questions

*None — all design decisions are captured in the requirements above.*

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-14
- **Notes**: Approved via mindspec approve spec