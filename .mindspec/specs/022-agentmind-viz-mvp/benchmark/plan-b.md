# Plan: Agent Activity Galaxy — MVP Web Visualizer

## Context

MindSpec already has an OTLP/HTTP collector (`internal/bench/collector.go`) that receives Claude Code telemetry (`claude_code.api_request`, token/cost metrics). This plan adds a real-time 3D "galaxy" visualization that ingests the same OTLP stream and renders agents, models, tools, and MCP servers as glowing nodes in a force-directed constellation, connected by animated edges.

## Architecture

```
Claude Code (OTLP/HTTP)           Recorded NDJSON file
        │                                │
        ▼                                ▼
  OTLP receiver (:4318)           Replay reader
        │                                │
        ▼                                ▼
   Normalizer (OTLP event → GraphEvent[])
        │
        ▼
   WebSocket Hub → broadcasts to all browser clients
        │
        ▼
   Browser (3d-force-graph on Three.js, single HTML file)
```

## Files to Create

### 1. `internal/viz/types.go` (~80 lines)
- Duplicate `CollectedEvent`, `otlpKeyValue`, `otlpValue`, `flattenAttributes`, `parseOTLPTimestamp`, `extractLogEvents`, `extractMetricEvents` from `internal/bench/collector.go`
- Avoids import cycle; these are ~60 lines of stable code

### 2. `internal/viz/normalizer.go` (~180 lines)
- Types: `NodeType` (agent/model/tool/mcp), `GraphEvent`, `NodeData`, `EdgeData`, `StatData`
- `Normalizer` struct: maintains known nodes map, active edges, cumulative stats
- `NewNormalizer()` pre-seeds the root `agent-1` node
- `Normalize(CollectedEvent) []GraphEvent` mapping:
  - `claude_code.api_request` → model node upsert + model_call edge_start + stat_update
  - `claude_code.token.usage` → stat_update
  - `claude_code.cost.usage` → stat_update
  - Events with `tool_name` → tool node upsert + tool_call edge
  - Events with `mcp_server` → mcp node upsert + mcp_call edge
- Limits: 200 nodes / 500 edges max; oldest pruned beyond cap
- Safety: strips `prompt`, `content`, `message` keys from meta

### 3. `internal/viz/normalizer_test.go` (~200 lines)
- api_request → correct nodes/edges/stats
- token/cost → stats update
- safety: prompt fields stripped
- limits: >200 nodes triggers pruning

### 4. `internal/viz/hub.go` (~120 lines)
- `Hub` struct with clients map, broadcast channel
- `Client` struct wrapping `nhooyr.io/websocket` conn + send channel
- `Register`: sends full graph snapshot (all nodes + edges + stats) on connect
- `Broadcast`: non-blocking fan-out; drops slow clients
- `Run`: main loop reading from broadcast channel

### 5. `internal/viz/server.go` (~150 lines)
- `Server` struct with httpPort, otlpPort, normalizer, hub
- Two `http.Server` instances:
  - HTTP `:8080`: `GET /` (embedded HTML), `GET /ws` (WebSocket upgrade), `GET /health`
  - OTLP `:4318`: `POST /v1/logs`, `POST /v1/metrics`
- OTLP handlers: parse body → `extractLogEvents`/`extractMetricEvents` → normalize → broadcast
- `//go:embed static/index.html` for the UI (same pattern as `internal/instruct/instruct.go:15`)
- Graceful shutdown via context

### 6. `internal/viz/replay.go` (~80 lines)
- `Replay` struct: filePath, speed multiplier, normalizer ref, hub ref
- `Run(ctx)`: reads NDJSON line-by-line, sleeps `delta/speed` between events, normalizes, broadcasts
- Keeps server alive after file exhausted

### 7. `internal/viz/replay_test.go` (~80 lines)
- Replays multi-line NDJSON, verifies events produced
- Speed multiplier halves duration
- Context cancellation stops replay

### 8. `internal/viz/static/index.html` (~700 lines)
Single self-contained HTML/CSS/JS file. CDN dependency: `3d-force-graph@1`.

**Visual design:**
- Dark starfield background (`#0a0a1a`) with Three.js particle field (2000 stars)
- Node types by shape + color:
  - Agent: large blue-white sphere (`#aaccff`) with glow sprite
  - Model: golden icosahedron (`#ffcc44`)
  - Tool: green octahedron (`#44ff88`)
  - MCP: purple dodecahedron (`#bb66ff`)
- Edges colored by type with directional particle animation during active calls
- Emissive materials for neon glow (no bloom pass needed for perf)

**Interaction:**
- Camera auto-orbits; disabled on user interaction; reset button restores
- Hover: highlight node + connected edges, show label tooltip
- Click: pin detail card (type, label, meta attributes; no prompts)
- Search input: dims non-matching nodes
- Pause/Resume button: buffers events while paused, flushes on resume

**HUD (top-right):**
- API calls count, total tokens, cost USD, errors, node/edge counts

**State management:**
- `Map<id, NodeData>` for nodes, `Map<id, EdgeData>` for edges
- WebSocket auto-reconnects every 2s; receives full snapshot on reconnect

### 9. `cmd/mindspec/viz.go` (~80 lines)
Following `cmd/mindspec/bench.go` pattern:

- `vizCmd` (parent): `mindspec viz`
- `vizServeCmd`: `mindspec viz serve --port 8080 --otlp-port 4318`
  - Creates Server, signal handling, runs until Ctrl-C
- `vizReplayCmd`: `mindspec viz replay <session.jsonl> --port 8080 --speed 1.0`
  - Creates Server (no OTLP listener), starts Replay goroutine

### 10. `cmd/mindspec/root.go` (modify)
- Add `rootCmd.AddCommand(vizCmd)` in `init()` at line ~65

### 11. `go.mod` / `go.sum` (modify)
- Add `nhooyr.io/websocket` dependency via `go get`

## Implementation Order

1. `internal/viz/types.go` — OTLP parsing foundation
2. `internal/viz/normalizer.go` + `normalizer_test.go` — core mapping logic
3. `internal/viz/hub.go` — WebSocket broadcast
4. `internal/viz/server.go` — HTTP + OTLP + WS wiring
5. `internal/viz/replay.go` + `replay_test.go` — file replay
6. `internal/viz/static/index.html` — browser UI
7. `cmd/mindspec/viz.go` + root.go modification — CLI integration
8. `go get nhooyr.io/websocket` — dependency
9. Build and manual test with replay file

## Verification

1. **Unit tests**: `go test ./internal/viz/...`
2. **Replay test**: `go run . viz replay /tmp/bench-session.jsonl` → open `http://localhost:8080` → verify nodes appear, edges animate, HUD updates, camera orbits
3. **Live test**: `go run . viz serve` → configure Claude Code with `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` → run a short session → verify real-time updates
4. **Safety**: confirm no prompt/content fields visible in detail cards
5. **Limits**: stress test with >200 synthetic nodes; verify pruning
6. **Filters**: type text in search; verify dimming
7. **Pause/resume**: pause stream, trigger events, resume; verify buffered events flush
