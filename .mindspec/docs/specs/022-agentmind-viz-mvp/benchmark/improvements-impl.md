The viz files from Session C haven't been merged to main yet. Let me check the diff content that was provided in the prompt directly — I already have it. Let me also verify Session B's normalizer approach from the diff.

Based on my thorough analysis of all three session diffs, plans, and the qualitative analysis, here is the improvements report:

---

# Improvements from Non-MindSpec Sessions

## Summary

Sessions A and B surfaced several genuine improvements worth adopting. Session B's simpler server wiring and its use of the `3d-force-graph` library are pragmatically better for an MVP. Session A's file-node classification and gradient edge decay are richer domain modeling that Session C's spec actually called for but the implementation missed.

## Improvements

### 1. Explicit prompt/content safety filtering
**Source**: Session B
**What was better**: Session B's normalizer has an intentional `isSensitive()` function with a blocklist (`prompt`, `content`, `message`, `system_prompt`, `user_message`) and a `safeMeta()` method that strips sensitive keys before populating node metadata. Session C's normalizer uses a whitelist approach in `filterAttrs()` (only passing through named keys like `tool_name`, `duration_ms`), which happens to be safe but is an *accidental* safety mechanism. If someone adds a new key to the whitelist, they might inadvertently leak prompts. Session B's design is *intentionally* defensive.
**Suggestion**: Add an explicit `isSensitive()` blocklist check in `normalize.go` as a secondary defense, even with the whitelist. Defense in depth — the whitelist determines what gets through, the blocklist catches anything that slips past during future whitelist expansion. Add a test that verifies known sensitive fields are never present in normalization output.

### 2. File/data_source node classification
**Source**: Session A
**What was better**: Session A created `file:{path}` nodes when Read/Write/Edit/Glob/Grep tools were used, extracting the file path from tool parameters and creating a `read` or `write` edge from the tool to the file. This makes the graph show actual data flow: `agent → tool:Read → file:src/main.go`. Session C's spec listed `data_source` as a node type and `retrieval`/`write` as edge types, and `normalize.go` has `classifyToolEdge()` that returns `EdgeRetrieval` or `EdgeWrite` — but these edges still point from agent to tool, not from tool to file. The file nodes themselves are never created.
**Suggestion**: When `classifyToolEdge()` identifies a read/write tool, extract the file path from the event's `file_path` or `path` attribute, upsert a `data_source` node with ID `file:<path>`, and emit a second edge from the tool to the file. This completes the data flow chain that the spec envisioned.

### 3. Use `3d-force-graph` library instead of hand-rolled Three.js
**Source**: Session B
**What was better**: Session B uses the `3d-force-graph` npm package (loaded via CDN), which wraps Three.js + d3-force-3d and provides node rendering, force layout, directional particles, node/edge coloring, hover labels, and click handling in ~200 lines of JS. Session C hand-rolls the Three.js scene, force simulation, raycasting, and node rendering in 514 lines of `app.js`. For an MVP, the library approach produces equivalent or better visual results with dramatically less custom code. The `3d-force-graph` library also handles edge cases like node drag, zoom-to-fit, and WebGL context loss that hand-rolled code typically misses.
**Suggestion**: Replace the custom Three.js rendering in `app.js` with `3d-force-graph`. Vendor the library into `internal/viz/web/` alongside Three.js. This would cut `app.js` by ~300 lines while improving visual quality. The starfield and bloom can be added on top of `3d-force-graph`'s renderer via `graph.renderer()`.

### 4. Integrated statistics tracking in the normalizer
**Source**: Session B
**What was better**: Session B's `Normalizer` maintains `StatData` (API calls, total tokens, cost, errors, node/edge counts) under the same mutex as the graph state, emitting `stat_update` events as part of normal normalization flow. This guarantees stats are always consistent with graph state. Session C fragments statistics across `LiveReceiver` atomics (`eventCount`, `errorCount`, `totalLatNs`) — these are eventually consistent but can momentarily disagree with the graph. For example, `eventCount` might be incremented before the corresponding `Graph.UpsertNode()` call.
**Suggestion**: Move cumulative statistics (API call count, total tokens, cost, error count) into `Graph` itself, updated atomically with node/edge mutations under the same lock. Keep the rate-derived stats (events/sec, avg latency) in `LiveReceiver` since they're computed from a time window. This gives a single source of truth for absolute counts.

### 5. Simpler server factory pattern
**Source**: Session B
**What was better**: Session B's `NewServer(httpPort, otlpPort)` creates the normalizer, hub, and both HTTP servers internally. The caller in `cmd/mindspec/viz.go` needs only 3 lines: create server, handle signals, run. Session C requires the caller to create `Graph`, `Hub`, `Server`, and `LiveReceiver` separately, start `hub.Run()` in a goroutine, start `server.Run()` in a goroutine, then call `receiver.Run()`. This is 15+ lines of wiring in `vizLiveCmd`. While Session C's dependency injection is more testable and flexible, the wiring boilerplate is unnecessarily verbose for the CLI layer.
**Suggestion**: Add a `viz.RunLive(ctx, otlpPort, uiPort)` convenience function that creates all components, starts goroutines, and blocks until ctx is cancelled. Keep the individual constructors for testing. The CLI layer calls the convenience function; tests construct components individually.

### 6. Gradient edge opacity decay
**Source**: Session A
**What was better**: Session A implemented smooth linear interpolation for edge opacity: `opacity = max(0, 1 - (age - fadeStart) / (fadeEnd - fadeStart))`, with edges gradually becoming transparent between 5 and 15 minutes. Session C's `Graph.Tick()` sets a binary `Faded` boolean — edges are either fully visible or faded, with no gradation. The gradient approach produces a much more visually informative display: recently-active edges glow brightly, older edges dim smoothly, giving the user an intuitive sense of recency.
**Suggestion**: Replace the `Faded bool` field on `Edge` with `Opacity float64` (0.0-1.0). In `Graph.Tick()`, compute opacity as a linear interpolation based on time since last activity. The frontend can use this opacity value directly for edge rendering. This is a small change (~10 lines in `graph.go`) with significant visual impact.

### 7. MCP edges routed through tools, not directly from agent
**Source**: Session A
**What was better**: Session A routes MCP calls as `tool → mcp_server` (the tool is the MCP client that invokes the server), which is architecturally accurate. Both Session B and Session C route MCP calls as `agent → mcp_server`, which skips the tool intermediary. In reality, Claude Code calls a tool, and that tool happens to use an MCP server — the agent doesn't directly talk to MCP.
**Suggestion**: When normalizing an event with both `tool_name` and `mcp_server_name`, create the edge from the tool node to the MCP node instead of from the agent to the MCP node. This requires the tool node to already exist (upsert it first if needed). This makes the graph topology more accurate.

### 8. CLI subcommand naming: `serve` vs `live`
**Source**: Session B
**What was better**: Session B named the live ingestion subcommand `viz serve`, which is more conventional (matches `http.ListenAndServe`, Docker `serve`, etc.). Session C named it `viz live`, which is descriptive but non-standard. However, this is debatable — `live` communicates the real-time aspect more clearly than `serve`. This is a minor stylistic difference.
**Suggestion**: No change needed. `viz live` is fine and matches the spec's terminology. Noted for completeness.

## Conclusion

The most actionable improvements are the `3d-force-graph` library adoption (#3, which would cut ~300 lines of frontend code), the file-node classification (#2, which completes a feature the spec called for but the implementation missed), and the gradient edge opacity (#6, a small change with outsized visual impact). These suggest that MindSpec's workflow excels at architectural correctness and testing discipline but can over-engineer foundational layers (custom Three.js rendering) and under-deliver on domain richness (missing data_source nodes despite specifying them). The workflow's bead decomposition correctly identified the *components* but didn't catch that the normalization bead's *domain model* was incomplete relative to the spec's node type enumeration.

