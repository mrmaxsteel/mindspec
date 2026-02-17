# AgentMind — AI Agent Observability

AgentMind is a real-time observability dashboard for AI coding agents. It combines a 3D activity graph with token consumption tracking, cost estimation, tool analytics, and session benchmarking — all from standard OpenTelemetry data.

## What You Get

**3D Activity Graph** — Agents, tools, MCP servers, data sources, and LLM endpoints rendered as an interactive force-directed constellation with a starfield aesthetic. Edges animate on activity, nodes scale with usage.

**Token & Cost Tracking** — Input tokens, output tokens, cache reads, and cache creation tokens tracked per model. Estimated USD cost aggregated in real time. Cache hit rate calculated automatically.

**Tool & MCP Analytics** — Every tool call and MCP server interaction counted and categorized. The UI shows frequency histograms so you can see which tools dominate a session.

**Model Statistics** — Per-model breakdown of API calls, token usage, and cost. Supports multi-model sessions (e.g., Opus for planning, Haiku for quick lookups).

**Session Recording & Replay** — Capture full sessions as NDJSON files. Replay at 0.5x to 50x speed, or instant. Filter replay by lifecycle phase.

**Benchmarking** — Compare agentic workflows head-to-head with automated A/B/C testing, pairwise delta reporting, and AI-driven qualitative analysis.

## Node Types

| Node Type | Color | What It Represents |
|:----------|:------|:-------------------|
| `agent` | Teal/Cyan | An AI agent (Claude Code, Codex, custom) |
| `tool` | Green | A tool the agent calls (file read, web search, etc.) |
| `mcp_server` | Purple | An MCP server providing tools |
| `data_source` | Orange | A file, database, or external data source |
| `llm_endpoint` | Yellow | The LLM API being called |

Edges represent calls between nodes: `model_call` (LLM requests with token counts), `tool_call`, `mcp_call`, `retrieval`, `write`, and `spawn` (agent hierarchy).

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

#### Codex

Use the built-in helper to configure `~/.codex/config.toml`:

```bash
./bin/mindspec agentmind setup codex
```

If Codex is already pointed at another OTEL collector, MindSpec prints a warning and leaves it unchanged. To replace an existing endpoint explicitly:

```bash
./bin/mindspec agentmind setup codex --force
```

Equivalent Codex settings:

```toml
[otel]
exporter = { "otlp-http" = { endpoint = "http://localhost:4318/v1/logs", protocol = "json" } }
trace_exporter = "none"
log_user_prompt = false
```

By default, this keeps `otel.log_user_prompt = false` so prompt text is redacted in telemetry unless you explicitly opt in.
Codex expects the full OTLP logs path, so the endpoint includes `/v1/logs`.

#### Any OTLP-Compatible Agent

Point the standard OpenTelemetry environment variables to `http://localhost:4318`. AgentMind accepts OTLP/HTTP JSON on that port.

### 4. Open the UI

Navigate to [http://localhost:8420](http://localhost:8420) in your browser. Activity appears as your agent starts working.

## What the UI Shows

### Live HUD Metrics

The heads-up display in the top-right corner shows:

| Metric | What It Means |
|:-------|:-------------|
| Events/sec | Live telemetry ingestion rate |
| Errors | Parse/processing error count |
| Avg latency | Response time in milliseconds |
| Nodes | Active nodes in the graph |
| Edges | Active connections between nodes |
| Sampling | Whether auto-sampling is active (kicks in at 100+ events/sec) |

### Detail Cards

Click any node or edge to see its detail card:

- **Agent nodes**: API call count, cumulative tokens (in/out), estimated cost
- **Tool nodes**: Call count, category (retrieval/write/generic)
- **MCP server nodes**: Call frequency, connected tools
- **LLM endpoint nodes**: Per-model token breakdown, cost
- **Edges**: Call count, input/output token counts for model calls

### Recording Dashboard

When a session completes or you save a recording, the dashboard shows:

- Session duration
- Total events, nodes, and edges
- Cumulative token usage (input, output, cache read, cache create)
- Estimated cost in USD
- Tool call histogram with relative frequency bars
- MCP server call counts

## Token & Cost Metrics

AgentMind collects token and cost data from OTLP metrics:

| Metric | Source |
|:-------|:-------|
| Input tokens | `claude_code.token.usage` (type: input) |
| Output tokens | `claude_code.token.usage` (type: output) |
| Cache read tokens | `claude_code.token.usage` (type: cacheRead) |
| Cache creation tokens | `claude_code.token.usage` (type: cacheCreation) |
| Cost (USD) | `claude_code.cost.usage` |

All metrics are aggregated per model, so you can see exactly how much each model variant contributes to token usage and cost in a multi-model session.

Codex OTEL aliases such as `codex.api_request`, `codex.token.usage`, and `codex.cost.usage` are normalized into the same model/token pathways used by Claude telemetry.
Codex `codex.sse_event` records with `event.kind=response.web_search_call.completed` are normalized into `WebSearch` tool-call edges.

**Cache hit rate** is calculated as: `cache_read / (input + cache_read + cache_create)`

## Recording Sessions

Capture events to an NDJSON file for later replay or benchmarking:

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

# Filter to a specific lifecycle phase
./bin/mindspec agentmind replay session.ndjson --phase implement
```

Replay accumulates the same metrics as live mode — token counts, cost, tool histograms — so you can analyze completed sessions after the fact.

## Codex JSONL Fallback Import

If Codex OTEL export is unavailable, you can convert a local Codex session JSONL file into AgentMind NDJSON and replay it.

```bash
# Convert a Codex session file
./bin/mindspec agentmind setup codex --session ~/.codex/sessions/2026/02/16/rollout-2026-02-16T13-12-24-019c6694-aa05-76e0-98b1-46390fb71add.jsonl

# Explicit output path
./bin/mindspec agentmind setup codex --session /path/to/rollout.jsonl --output /tmp/codex-session.ndjson

# Replay converted output
./bin/mindspec agentmind replay /tmp/codex-session.ndjson
```

By default, output is written next to the input file as `<input-name>-agentmind.ndjson`.

## Benchmarking

AgentMind includes a benchmarking framework for comparing agentic workflows against each other.

### Setup

```bash
mindspec bench setup --spec 025-my-feature
```

This creates three isolated sessions: `no-docs` (baseline without MindSpec docs), `baseline` (with docs but no workflow), and `mindspec` (full MindSpec workflow). Each session runs in its own git worktree.

### Collect

```bash
mindspec bench collect --spec 025-my-feature
```

Runs all three sessions with configurable timeouts, max turns, and auto-retry. Each session's telemetry is recorded as NDJSON.

### Report

```bash
mindspec bench report --spec 025-my-feature
```

Generates a comparative report with:

| Metric | What's Compared |
|:-------|:---------------|
| API calls | Total LLM requests per session |
| Input/output tokens | Absolute counts and deltas |
| Cache hit rate | How efficiently each workflow uses context caching |
| Cost (USD) | Estimated spend per session, per model |
| Output/input ratio | How "chatty" the agent is relative to context consumed |
| Per-model breakdown | Token and cost deltas for each model variant |

Reports are available in table format (human-readable) and JSON (programmatic). N-way comparison supports 3+ sessions side-by-side.

`mindspec bench report` now aggregates both Claude and Codex NDJSON event aliases in the same session summary pipeline.

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
| **Click** node or edge | Show detail card with metrics |
| **Search** field | Filter nodes by name, type, or ID |
| **Pause/Resume** button | Freeze/unfreeze the graph |
| **Camera Reset** button | Reset the 3D camera position |
| **Save Recording** button | Download events as NDJSON |

## Server Options

```bash
./bin/mindspec agentmind serve \
  --otlp-port 4318 \    # OTLP/HTTP receiver port (default: 4318)
  --ui-port 8420 \       # Web UI port (default: 8420)
  --output events.ndjson # Write events to file
```

## Architecture (Brief)

AgentMind runs as a single process with three components:

1. **OTLP/HTTP receiver** on `:4318` — accepts standard OpenTelemetry log and metric data
2. **WebSocket server** — pushes graph updates and stats to connected browsers (~500ms intervals)
3. **Three.js frontend** — renders a force-directed 3D graph with real-time metric overlays

Performance caps: 500 nodes, 2000 edges, auto-sampling at 100+ events/sec.

### What Gets Collected

| OTLP Endpoint | Events |
|:--------------|:-------|
| `/v1/logs` | API requests, tool calls, tool results, MCP calls |
| `/v1/metrics` | Token usage (input, output, cache read, cache create), cost (USD) |

Events are normalized into graph nodes and edges, with metrics aggregated per node for the detail cards and dashboard.
