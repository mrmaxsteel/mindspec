# AgentMind — Real-Time AI Agent Visualization

AgentMind renders your AI agent's activity as an interactive 3D force-directed graph with a starfield aesthetic. Agents, tools, MCP servers, data sources, and LLM endpoints appear as nodes; calls between them appear as animated edges — all updating in real time.

## What You'll See

A 3D constellation where each node type has a distinct color:

| Node Type | What It Represents |
|:----------|:-------------------|
| `agent` | An AI agent (Claude Code, Codex, custom) |
| `tool` | A tool the agent calls (file read, web search, etc.) |
| `mcp_server` | An MCP server providing tools |
| `data_source` | A file, database, or external data source |
| `llm_endpoint` | The LLM API being called |

Edges represent calls between nodes. Thicker edges indicate higher call frequency. The HUD shows live metrics: events/sec, active nodes, total edges.

## Quick Start (< 2 minutes)

### 1. Build

```bash
make build
```

### 2. Start AgentMind

```bash
./bin/mindspec agentmind serve
# OTLP receiver listening on :4318
# Web UI at http://localhost:8420
```

### 3. Configure Your Agent

#### Claude Code

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
```

For persistent configuration, add to `.claude/settings.local.json`:

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/json"
  }
}
```

#### Any OTLP-Compatible Agent

Point the standard OpenTelemetry environment variables to `http://localhost:4318`. AgentMind accepts OTLP/HTTP JSON on that port.

### 4. Open the UI

Navigate to [http://localhost:8420](http://localhost:8420) in your browser. Activity appears as your agent starts working.

## Recording Sessions

Capture events to an NDJSON file for later replay:

```bash
./bin/mindspec agentmind serve --output session.ndjson
```

Events are appended to the file in real time. You can also save directly from the UI using the save-recording button.

## Replay

Replay a recorded session:

```bash
./bin/mindspec agentmind replay session.ndjson
# UI at http://localhost:8420

# Speed up playback
./bin/mindspec agentmind replay session.ndjson --speed 5

# Max speed (no delays)
./bin/mindspec agentmind replay session.ndjson --speed 0

# Replay a specific spec's recording
./bin/mindspec agentmind replay --spec 022-agentmind-viz-mvp
```

## Customizing Agent Labels

Set the `agent.name` resource attribute to label your agent in the graph:

```bash
export OTEL_RESOURCE_ATTRIBUTES="agent.name=MyBot"
```

## Multi-Agent Visualization

Multiple agents can send telemetry to the same AgentMind instance. Each agent appears as a distinct node. Point multiple agent processes at the same `OTEL_EXPORTER_OTLP_ENDPOINT`.

## UI Controls

| Control | Action |
|:--------|:-------|
| **Click** node or edge | Show detail card |
| **Search** field | Filter nodes by name |
| **Pause/Resume** button | Freeze/unfreeze the graph |
| **Camera Reset** button | Reset the 3D camera position |
| **Save Recording** button | Download events as NDJSON |

The HUD in the top-right shows live metrics: events per second, active node count, and total edge count.

## Server Options

```bash
./bin/mindspec agentmind serve \
  --otlp-port 4318 \    # OTLP/HTTP receiver port (default: 4318)
  --ui-port 8420 \       # Web UI port (default: 8420)
  --output events.ndjson # Write events to file
```

## Architecture (Brief)

AgentMind runs as a single process with three components:

1. **OTLP/HTTP receiver** on `:4318` — accepts standard OpenTelemetry trace/metric/log data
2. **WebSocket server** — pushes events to connected browsers in real time
3. **Three.js frontend** — renders a force-directed 3D graph in the browser

Performance caps: 500 nodes, 2000 edges, auto-sampling kicks in at 100+ events/sec.
