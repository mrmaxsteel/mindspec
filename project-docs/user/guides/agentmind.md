# AgentMind — AI Agent Observability

AgentMind is a real-time observability dashboard for AI coding agents: a 3D activity graph with token consumption tracking, cost estimation, tool analytics, and session recording/replay — all from standard OpenTelemetry data.

**AgentMind is a standalone companion product, not a mindspec feature.** It lives in its own repository at [`github.com/mrmaxsteel/agentmind`](https://github.com/mrmaxsteel/agentmind), with its own install, releases, and documentation — that repo's docs are authoritative for the product itself. It was extracted from mindspec by specs 083/084 (ADR-0026, ADR-0027): the mindspec binary has no agentmind dependency and never spawns it, and a permanent CI gate (`internal/specgate/verify_no_agentmind_dep_test.go`) keeps it that way. The two integrate one way, over OTLP/HTTP.

> **If you remember the old verbs:** `mindspec agentmind serve|replay|setup`, `mindspec viz`, and `mindspec bench …` were removed by spec 084. For one release they survive as hidden deprecation stubs that print a one-line pointer and exit 2. The replacements are `agentmind …` (installed from its own repo) and `mindspec otel setup`.

## Quick start

### 1. Install and start AgentMind

Install AgentMind from [its repository](https://github.com/mrmaxsteel/agentmind), then:

```bash
agentmind serve
# OTLP receiver listening on :4318
# Web UI at http://localhost:8420
```

### 2. Point your agent at it — mindspec's side of the integration

MindSpec's entire role is writing the OTLP exporter configuration (it performs zero network I/O and never validates the endpoint):

```bash
mindspec otel setup --endpoint http://localhost:4318            # Claude Code (default target)
mindspec otel setup --endpoint http://localhost:4318 --codex    # Codex (~/.codex/config.toml)
mindspec otel setup --endpoint http://localhost:4318 --target env  # print POSIX export lines
mindspec otel status                                            # read-only: show what's configured
```

For Claude Code this writes the telemetry env block into `.claude/settings.local.json`, overwriting the OTEL keys and preserving every other setting; for Codex the entire `[otel]` table — including a nested `[otel.exporter]` — is replaced wholesale with mindspec's canonical block, not merged key-by-key. That means any non-mindspec key co-located under `[otel]` (for example a hand-added `environment = "..."`) is destroyed, and `log_user_prompt` is always reset to `false` on every run: an explicit `log_user_prompt = true` you set previously does **not** survive a re-run. Tables outside the `[otel]` namespace keep their contents, but their order and surrounding blank lines are not preserved byte-for-byte. (Whether this is the intended behavior or the code owes a fix to match its own documented merge contract is an open question — see `mindspec-tnf6`.) It does **not** detect or warn about an existing endpoint: re-running `otel setup` against a config that already points at another OTEL collector silently replaces that endpoint. Check with `mindspec otel status` first if you're not sure what's configured.

Any OTLP-compatible agent works without mindspec's help: point the standard OpenTelemetry environment variables (`OTEL_EXPORTER_OTLP_ENDPOINT` etc.) at `http://localhost:4318`. And the receiver doesn't have to be AgentMind — anything that speaks OTLP/HTTP works (Honeycomb, Tempo, Jaeger, opentelemetry-collector-contrib).

### 3. Open the UI

Navigate to [http://localhost:8420](http://localhost:8420). Activity appears as your agent starts working.

## What AgentMind shows

The feature descriptions below are as of the extraction (specs 083/084); the [agentmind repo](https://github.com/mrmaxsteel/agentmind) is authoritative for the current product.

**3D Activity Graph** — Agents, tools, MCP servers, data sources, and LLM endpoints rendered as an interactive force-directed constellation. Edges animate on activity, nodes scale with usage. Node types: `agent`, `tool`, `mcp_server`, `data_source`, `llm_endpoint`; edges represent `model_call`, `tool_call`, `mcp_call`, `retrieval`, `write`, and `spawn` (agent hierarchy).

**Token & Cost Tracking** — Input, output, cache-read, and cache-creation tokens tracked per model from OTLP metrics (`claude_code.token.usage`, `claude_code.cost.usage`; Codex aliases like `codex.token.usage` are normalized into the same pathways). Estimated USD cost aggregated in real time; cache hit rate calculated as `cache_read / (input + cache_read + cache_create)`.

**Tool & MCP Analytics** — Every tool call and MCP server interaction counted and categorized, with frequency histograms.

**Session Recording & Replay** — Capture full sessions as NDJSON (`agentmind serve --output session.ndjson` or the UI's save button), then `agentmind replay session.ndjson` at 0.5x–50x speed or instant, with lifecycle-phase filtering. Replay accumulates the same metrics as live mode.

**Benchmarking** — The A/B/C workflow-comparison framework (formerly `mindspec bench`) moved to the agentmind repo with the rest of the subsystem; ADR-0028 records the move. The old `mindspec bench setup|collect|report` verbs are deprecation stubs.

To label your agent in the graph, set the `agent.name` resource attribute (`OTEL_RESOURCE_ATTRIBUTES="agent.name=MyBot"`). Multiple agents can send telemetry to the same AgentMind instance; each appears as a distinct node.

## Related

- [Installing AgentMind alongside MindSpec](../../installation/agentmind.md) — the integration contract in full
- ADR-0026 (AgentMind extracted to standalone repo) · ADR-0027 (mindspec is OTEL-only) · ADR-0028 (bench rescue procedure)
