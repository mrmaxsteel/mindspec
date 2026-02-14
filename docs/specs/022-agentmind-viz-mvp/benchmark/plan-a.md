# Agent Activity Galaxy — MVP Implementation Plan

## Context

Build a real-time 3D web visualizer that ingests Claude Code OpenTelemetry events and renders them as a live constellation/galaxy. The existing Go codebase already has an OTLP collector (`internal/bench/collector.go`) demonstrating the parsing pattern. The visualizer will be a standalone Node.js/TypeScript app in a new `viz/` directory.

## Architecture

```
Claude Code ──OTLP/HTTP JSON──► viz server ──WebSocket──► browser (3d-force-graph)
                                    │
                              NDJSON replay ──►
```

Three layers:
1. **OTLP Receiver** — HTTP endpoints `/v1/logs` and `/v1/metrics` parsing OTLP JSON
2. **Normalizer + Graph State** — Maps raw events to node upserts / edge events, manages decay and caps
3. **Browser UI** — `3d-force-graph` with bloom, starfield, HUD controls

## Tech Stack

| Component | Library | Version |
|-----------|---------|---------|
| Server runtime | Node.js + TypeScript | ts 5.7+ |
| HTTP server | express | 4.x |
| WebSocket | ws | 8.x |
| 3D graph | 3d-force-graph | 1.x |
| Rendering | three (peer dep) | 0.170.x |
| Tests | vitest | 2.x |
| Build | tsx (dev), esbuild (prod) | — |

## File Structure

```
viz/
├── package.json
├── tsconfig.json
├── server/
│   ├── index.ts            — Entry point: HTTP + WS server, CLI args
│   ├── otlp-receiver.ts    — Parse OTLP/HTTP JSON → CollectedEvent[]
│   ├── normalizer.ts       — CollectedEvent → GraphOp[] (node upserts, edge events)
│   ├── graph-state.ts      — In-memory graph: dedup, decay, hard caps
│   └── replay.ts           — NDJSON file replay with timing
├── public/
│   ├── index.html          — Single page with canvas + HUD overlay
│   ├── style.css           — Dark theme, HUD layout, detail cards
│   └── app.js              — 3d-force-graph setup, WS client, controls
└── test/
    ├── normalizer.test.ts  — Event→graph mapping tests
    ├── graph-state.test.ts — Decay, cap, dedup tests
    └── fixtures/
        └── sample.ndjson   — Sample events for testing/replay
```

## Implementation Steps

### Step 1: Project scaffold
- Create `viz/package.json` with dependencies
- Create `viz/tsconfig.json`
- Wire up `viz/server/index.ts` with express + ws, serving `public/`

### Step 2: OTLP receiver (`viz/server/otlp-receiver.ts`)
- `parseLogsRequest(body: Buffer): CollectedEvent[]` — parse ExportLogsServiceRequest JSON, extract event name + attributes from logRecords
- `parseMetricsRequest(body: Buffer): CollectedEvent[]` — parse ExportMetricsServiceRequest JSON, extract metric data points
- `CollectedEvent` type: `{ ts: string, event: string, data: Record<string, any> }`
- Register POST handlers for `/v1/logs` and `/v1/metrics` on the express app

### Step 3: Normalizer (`viz/server/normalizer.ts`)
Map each event type to graph operations:

**Node types** (id scheme → shape → color):
| Type | ID | Color |
|------|-----|-------|
| agent | `agent:{session.id}` | Cyan `#00ffff` |
| tool | `tool:{tool_name}` | Lime `#00ff88` |
| mcp_server | `mcp:{mcp_server_name}` | Magenta `#ff00ff` |
| llm | `llm:{model}` | Gold `#ffaa00` |
| file | `file:{path}` | Teal `#00aaff` |

**Edge types** (color):
| Type | From → To | Color |
|------|-----------|-------|
| tool_call | agent → tool | `#00ff88` |
| mcp_call | tool → mcp_server | `#ff00ff` |
| model_call | agent → llm | `#ffaa00` |
| read | tool → file | `#00aaff` |
| write | tool → file | `#ff6600` |

**Mapping rules**:
- `claude_code.tool_result` → upsert agent + tool nodes, emit tool_call edge. If `mcp_server_name` present, upsert mcp node + mcp_call edge. If tool is Read/Write/Edit/Glob/Grep, extract file path → upsert file node + read/write edge.
- `claude_code.api_request` → upsert agent + llm nodes, emit model_call edge with duration_ms, tokens.
- `claude_code.tool_decision` → update tool node last-seen.
- `claude_code.user_prompt` → upsert agent node, update activity timestamp.
- Metrics → update stats counters only (no graph ops).

Function: `normalize(event: CollectedEvent, sessionId: string): GraphOp[]`

### Step 4: Graph state (`viz/server/graph-state.ts`)
- `GraphState` class holding `Map<string, GraphNode>` and `Map<string, GraphEdge>`
- `applyOps(ops: GraphOp[])` — apply node upserts and edge events
- `tick()` — called every second: decay edges older than 5min (reduce opacity), remove edges older than 15min, remove orphan nodes
- Hard caps: max 200 nodes (evict LRU), max 1000 edges (evict oldest)
- `snapshot(): { nodes: GraphNode[], edges: GraphEdge[] }` — for initial WS connection
- `onUpdate(callback)` — notify WS bridge of changes

### Step 5: WebSocket bridge (in `viz/server/index.ts`)
- On WS connect: send `{ type: "snapshot", ...graphState.snapshot() }`
- On graph update: broadcast `{ type: "node_upsert", node }` or `{ type: "edge_event", edge }`
- Periodic stats: every 2s send `{ type: "stats", data: { throughput, errors, latency, nodeCount, edgeCount } }`
- Backpressure: if WS send buffer > 64KB, drop non-critical messages

### Step 6: Browser app — 3D graph (`viz/public/app.js`)
- Initialize `ForceGraph3D()` on `#graph` container
- **Node rendering**: Custom Three.js meshes per node type:
  - agent: IcosahedronGeometry, cyan emissive material
  - tool: OctahedronGeometry, lime emissive material
  - mcp_server: DodecahedronGeometry, magenta emissive material
  - llm: SphereGeometry, gold emissive material
  - file: BoxGeometry, teal emissive material
- **Edge rendering**: Animated directional particles, colored by edge type
- **Bloom**: Access Three.js renderer via `graph.renderer()`, add EffectComposer + UnrealBloomPass (strength=1.5, radius=0.7, threshold=0.2)
- **Starfield**: Add THREE.Points with 3000 random positions, PointsMaterial with white color, size attenuation
- **Force config**: d3-force charge=-120, link distance=80, center force, gentle warmup
- **Camera**: autoRotate=true, speed=0.5, enableDamping=true. User interaction pauses auto-rotate, reset button restores it.

### Step 7: HUD and controls (`viz/public/index.html` + `style.css` + `app.js`)
- **Legend panel** (bottom-left): node type colors/shapes, edge type colors
- **Stats panel** (top-right): events/s, error count, avg latency, node/edge counts
- **Controls** (top-left):
  - Pause/Resume button (pauses WS consumption)
  - Search input (highlight matching nodes, dim others)
  - Filter checkboxes per node type and edge type
- **Detail card** (floating):
  - On hover: show card with node/edge metadata (type, id, duration, status, key attrs)
  - On click: pin card; click elsewhere to dismiss
  - Never show raw prompt content (redacted by default)

### Step 8: Replay mode (`viz/server/replay.ts`)
- CLI flag: `--replay <path.ndjson>`
- Read NDJSON file line by line
- Parse timestamps, replay with original inter-event delays (capped at 2s max gap)
- `--speed <multiplier>` flag (default 1.0)
- Feed events through same normalizer → graph state pipeline
- On file end, keep server running so user can inspect final state

### Step 9: Tests (`viz/test/`)
- `normalizer.test.ts`:
  - tool_result → creates agent + tool nodes + tool_call edge
  - api_request → creates agent + llm nodes + model_call edge
  - MCP tool → creates mcp_server node + mcp_call edge
  - Read tool with path → creates file node + read edge
  - Prompt events don't leak content
- `graph-state.test.ts`:
  - Node deduplication (same id = update, not duplicate)
  - Edge decay after timeout
  - Hard cap enforcement (evict oldest beyond limit)
  - Snapshot correctness
- `fixtures/sample.ndjson`: 20-30 representative events covering all types

### Step 10: Sample fixture and validation
Create `viz/test/fixtures/sample.ndjson` with realistic events:
- Session start, user prompt, tool calls (Read, Edit, Bash, Grep), MCP tool call, API requests, errors
- Use this for both tests and replay demo

## Key Design Decisions

1. **Node.js not Go** for the viz server — faster iteration for web UI, native npm ecosystem for 3d-force-graph
2. **No React/bundler** — vanilla JS in public/ keeps it simple, no build step for frontend
3. **3d-force-graph** — all-in-one solution wrapping Three.js + d3-force-3d, avoids manual WebGL
4. **Session-based agent nodes** — one agent node per session.id, since Claude Code uses session-based (not span-based) telemetry
5. **Edge decay not scroll** — old edges fade and disappear rather than scrolling off, keeping the view clean

## Privacy/Safety

- Raw prompts never displayed (prompt field not included in detail cards)
- Tool parameters shown only if OTEL_LOG_TOOL_DETAILS was enabled upstream
- No persistence of telemetry data beyond in-memory graph state

## How to Run (end state)

```bash
# Live mode — point Claude Code at the viz server
cd viz && npm install && npm run dev
# Then in another terminal:
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_LOGS_EXPORTER=otlp
export OTEL_METRICS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
claude  # use Claude Code normally, watch the galaxy

# Replay mode
cd viz && npm run dev -- --replay test/fixtures/sample.ndjson --speed 2

# Tests
cd viz && npm test
```

## Verification

1. `npm test` passes — normalizer and graph-state unit tests
2. Start server, POST sample OTLP to `/v1/logs`, verify WS client receives graph events
3. Open `http://localhost:4318` in browser — see starfield, bloom, legend
4. Run replay — nodes and edges appear with correct types/colors/timing
5. Hover a node — detail card shows metadata, no raw prompts
6. Use search/filter — nodes highlight/hide correctly
7. 200 nodes render at 30+ FPS
8. Configure live Claude Code session → real-time visualization works
