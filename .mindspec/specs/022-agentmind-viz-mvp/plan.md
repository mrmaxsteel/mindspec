---
adr_citations:
    - id: ADR-0003
      sections:
        - Decision
        - Decision Details
    - id: ADR-0004
      sections:
        - Decision
        - Rationale
approved_at: "2026-02-14T17:15:44Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-14"
spec_id: 022-agentmind-viz-mvp
status: Approved
version: 1
---

# Plan: Spec 022 — AgentMind Viz MVP

## ADR Fitness

- **ADR-0003** (CLI-first tooling): Fit. `mindspec viz` is a CLI command that starts a local HTTP server. All logic is in Go, frontend is embedded static assets. No divergence needed.
- **ADR-0004** (Go implementation): Fit. Backend in Go, single binary embeds all web assets via `embed.FS`. We add `nhooyr.io/websocket` as a single new dependency for correct WebSocket support. This is a minor, justified addition.

## Architecture Overview

```
cmd/mindspec/viz.go          — CLI: viz, viz live, viz replay
internal/viz/
  normalize.go               — OTLP/NDJSON → graph events (NodeUpsert, EdgeEvent)
  graph.go                   — In-memory graph state (nodes, edges, caps, decay)
  hub.go                     — WebSocket hub (broadcast, backpressure, dropped counter)
  server.go                  — HTTP server (embed.FS static files + WS endpoint)
  replay.go                  — NDJSON file replay with speed control
  live.go                    — OTLP/HTTP receiver (reuses bench.Collector patterns)
  viz_test.go                — Tests for normalization, graph state, hub
  web/
    index.html               — SPA entry point
    app.js                   — Three.js 3D rendering, force layout, interaction
    style.css                — Starfield, HUD, detail card styles
    three.min.js             — Vendored Three.js
    OrbitControls.js          — Vendored OrbitControls
```

## Bead 1: Graph State & Normalization

**Scope**: Define in-memory graph data model (`graph.go`) and OTLP-to-graph normalization (`normalize.go`). Core types: Node, Edge, Graph with upsert/tick/snapshot. Normalizer maps `CollectedEvent` to `NodeUpsert`/`EdgeEvent`.

**Steps**
1. Define `Node`, `Edge`, `NodeUpsert`, `EdgeEvent` structs with type enums (agent, tool, mcp_server, data_source, llm_endpoint; tool_call, mcp_call, retrieval, write, model_call)
2. Implement `Graph` struct with thread-safe `UpsertNode` (dedup by ID, merge attrs), `AddEdge` (increment call count for existing combos), and `Snapshot` (copy for serialization)
3. Implement `Graph.Tick()` — apply staleness flags (nodes not seen in last N events), fade flags (edges older than M seconds), evict oldest when over hard caps (500 nodes, 2000 edges), return `Capped` bool
4. Implement `NormalizeEvent(CollectedEvent) → ([]NodeUpsert, []EdgeEvent)` — maps claude_code.api_request → llm_endpoint + model_call; tool events → tool + tool_call; MCP events → mcp_server + mcp_call
5. Write unit tests: node dedup, edge creation, staleness/fade decay, hard cap eviction, normalization of sample events

**Verification**
- [ ] `go test ./internal/viz/... -run TestGraph` passes
- [ ] `go test ./internal/viz/... -run TestNormalize` passes

**Depends on**: none

## Bead 2: WebSocket Hub

**Scope**: Create `hub.go` — concurrent fan-out to multiple browser clients with backpressure handling and dropped event counting.

**Steps**
1. Define `Hub` struct (client set, broadcast channel, register/unregister channels, dropped counter) and `Client` struct (WebSocket conn, buffered send channel)
2. Implement `Hub.Run()` goroutine: register/unregister clients, broadcast messages to all clients via non-blocking send
3. Implement `Client.WritePump()`: drain send channel to WebSocket; on channel-full, drop oldest and increment dropped counter
4. Implement `Client.ReadPump()`: read from WebSocket for disconnect detection, handle cleanup
5. Define JSON message types: `snapshot` (full state), `update` (incremental), `stats` (HUD metrics)
6. Write tests: client registration, broadcast delivery, backpressure drop counting

**Verification**
- [ ] `go test ./internal/viz/... -run TestHub` passes

**Depends on**: none

## Bead 3: HTTP Server & Web UI

**Scope**: Create `server.go` with embed.FS static file serving and WebSocket upgrade endpoint. Build the full Three.js SPA in `internal/viz/web/` (index.html, app.js, style.css, vendored Three.js).

**Steps**
1. Vendor Three.js and OrbitControls into `internal/viz/web/`, implement `Server` struct with `//go:embed web/*` and HTTP routes (`/` serves SPA, `/ws` upgrades to WebSocket)
2. On WebSocket connect: send full graph `Snapshot()` as initial state, register client with Hub
3. Build `index.html` + `style.css`: dark theme SPA shell with canvas, HUD overlay (legend, stats, search, pause/resume, show-raw toggle), detail card
4. Build `app.js` — Three.js scene: starfield background, force-directed 3D layout, type-colored node spheres with glow/size scaling, type-colored edge lines with pulse/fade animation
5. Build `app.js` — interaction: OrbitControls with auto-orbit, raycasting hover/click for detail cards (pin/unpin), search/filter (fade non-matching), pause/resume buffering
6. Build `app.js` — WebSocket client: receive snapshot/update/stats messages, update graph model, update HUD (events/sec, errors, latency, connection status, capped, dropped, sampling)

**Verification**
- [ ] `go build ./cmd/mindspec` succeeds with embedded assets
- [ ] Browser shows 3D canvas with starfield, nodes/edges render correctly

**Depends on**: Bead 1, Bead 2

## Bead 4: Replay Mode & CLI Wiring

**Scope**: Create `replay.go` for NDJSON file replay with speed control. Wire up `cmd/mindspec/viz.go` with `viz`, `viz replay`, and `viz live` subcommands. Register in root.go.

**Steps**
1. Implement `Replay` struct with file path, speed multiplier (1x/5x/10x/max), Graph and Hub references
2. Implement `Replay.Run(ctx)`: scan NDJSON line by line, normalize events, upsert into Graph, broadcast to Hub; for timed replay compute inter-event delay / speed; for max no sleep
3. Implement sampling: if rate exceeds 100 events/sec, keep every Nth event, set sampling flag; call `Graph.Tick()` after each batch and broadcast stats
4. Create `cmd/mindspec/viz.go`: `vizCmd` parent, `vizReplayCmd` (file arg, --speed, --ui-port), `vizLiveCmd` (--otlp-port, --ui-port); register in root.go
5. Wire replay CLI: create Graph, Hub, Server, Replay; start Hub and Server goroutines, run Replay, handle Ctrl-C gracefully
6. Write tests: replay reads NDJSON correctly, speed=0 means max, sampling triggers at high rate

**Verification**
- [ ] `mindspec viz replay <test.jsonl> --speed max` starts server; `curl localhost:8420` returns HTML with `<canvas>`
- [ ] `mindspec viz --help` prints usage for both subcommands
- [ ] `go test ./internal/viz/... -run TestReplay` passes

**Depends on**: Bead 1, Bead 2, Bead 3

## Bead 5: Live Mode

**Scope**: Create `live.go` — OTLP/HTTP receiver that normalizes incoming events and pushes to Graph + Hub in real time.

**Steps**
1. Implement `LiveReceiver` struct with OTLP port, Graph and Hub references
2. Implement OTLP HTTP handlers `/v1/logs` and `/v1/metrics` (reuse `extractLogEvents`/`extractMetricEvents` patterns from `bench.Collector`)
3. Instead of writing to file, normalize events via `NormalizeEvent()`, upsert into Graph, broadcast to Hub; apply sampling if rate > 100 events/sec
4. Wire up `vizLiveCmd` in `viz.go`: create Graph, Hub, Server, LiveReceiver; start all, handle graceful shutdown
5. Write tests: POST OTLP JSON → verify graph contains expected nodes/edges

**Verification**
- [ ] `mindspec viz live --otlp-port 4318` starts both OTLP receiver and UI server
- [ ] POST to `/v1/logs` creates graph nodes/edges
- [ ] `go test ./internal/viz/... -run TestLive` passes

**Depends on**: Bead 1, Bead 2, Bead 3

## Bead 6: Integration Tests & Validation Proofs

**Scope**: End-to-end verification of all acceptance criteria from the spec.

**Steps**
1. Create test NDJSON fixture with representative events (api_request, tool calls, MCP calls)
2. Write integration test: start replay server on ephemeral port, connect WebSocket client, verify nodes/edges received match expected types
3. Write integration test: start live server, POST OTLP events, verify graph state via WebSocket snapshot
4. Write test: verify hard caps (500 nodes, 2000 edges) with synthetic flood data and capped flag
5. Write test: verify `--help` output covers both `live` and `replay` subcommands
6. Run `make test` — all existing + new tests pass; run validation proofs from spec

**Verification**
- [ ] All acceptance criteria from spec verified
- [ ] `make test` passes
- [ ] `make build` succeeds

**Depends on**: Bead 4, Bead 5

## Dependency Graph

```
Bead 1 (Graph/Normalize) ──┐
                            ├──► Bead 3 (Server/UI) ──┬──► Bead 4 (Replay/CLI) ──► Bead 6 (Tests)
Bead 2 (Hub)           ────┘                          └──► Bead 5 (Live)        ──┘
```

## Notes

- **WebSocket library**: `nhooyr.io/websocket` — single new dependency, justified by correctness.
- **Three.js vendoring**: Downloaded at dev time, committed to `internal/viz/web/`. No runtime CDN.
- **No bundler**: Vanilla JS loaded via `<script>` tags. Build is just `go build`.
