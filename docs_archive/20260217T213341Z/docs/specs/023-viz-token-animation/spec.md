# Spec 023-viz-token-animation: Viz Real-Time Token Animation

## Goal

Show input and output token counts as floating, fading number animations above the relevant node in the 3D visualization whenever an `api_request` event arrives. This gives operators an immediate, visceral sense of token throughput per model endpoint without reading logs or hovering for detail cards.

## Background

Spec 022 established the AgentMind Viz MVP — a 3D force-directed graph that renders agent activity in real time. Spec 024 upgrades the visual aesthetic to a constellation star chart with bloom post-processing. The normalizer (`normalize.go`) already extracts `input_tokens` and `output_tokens` from `claude_code.api_request` events and records them via `graph.RecordAPIStats()`. However, this data only surfaces as aggregate numbers in the HUD (events/sec, avg latency). The raw per-event token counts are available in edge attributes but invisible unless the user hovers and inspects the detail card.

The animation described here makes token flow **ambient** — each API request triggers a brief floating number that rises and fades above the LLM endpoint node, giving a real-time pulse of how much context is being consumed and generated. This builds on Spec 024's star-point nodes and bloom — the floating numbers inherit the glow aesthetic.

## Impacted Domains

- **viz (frontend)**: New floating-text sprite system in `app.js`; animation loop additions
- **viz (backend)**: Token counts must be forwarded through WebSocket updates so the frontend can trigger animations on arrival

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): CLI-first — no build step; animation code is vanilla JS in the embedded SPA
- [ADR-0004](../../adr/ADR-0004.md): Go implementation — backend changes (if any) stay in Go

## Requirements

### Data Flow

1. When `NormalizeEvent()` processes a `claude_code.api_request` event, the resulting edge (type `model_call`) already carries `input_tokens` and `output_tokens` in its `Attributes` map. No backend changes are required — the frontend reads these from the edge update payload.

### Animation Rendering

2. When the frontend receives an edge update of type `model_call` with `input_tokens` or `output_tokens` attributes, it spawns floating number sprites above the **destination node** (the `llm_endpoint` node, e.g. `llm:claude-sonnet-4-5-20250929`).

3. **Input tokens** are rendered in **cyan** (`#4fc3f7` — matching the existing `agent` node color, since input flows *from* the agent). The label text is the token count (e.g. `"1,024"`).

4. **Output tokens** are rendered in **amber/gold** (`#ffd54f` — matching the existing `llm_endpoint` node color, since output flows *from* the model). The label text is the token count prefixed with a `+` (e.g. `"+512"`).

5. If both `input_tokens` and `output_tokens` are present in the same event (typical), two sprites spawn simultaneously — input on the left, output on the right (offset horizontally ±2 units from the node center to avoid overlap).

6. Each sprite animates over **2.5 seconds** total with an elastic bounce:
   - **0–0.3s (bounce-in)**: opacity ramps 0→1; sprite overshoots upward by ~2 units then settles back (elastic ease-out); scale pops 0.5x→1.2x→1.0x. This gives the "pop" feel that makes floating numbers satisfying rather than lifeless.
   - **0.3–2.0s (float-up)**: sprite drifts upward at ~8 units/sec, opacity holds at 1.0
   - **2.0–2.5s (fade-out)**: opacity ramps 1→0 while continuing to drift up
   - At 2.5s the sprite is removed from the scene.

7. **Text rendering** uses `three-spritetext` (by vasturiano, same author as `3d-force-graph`). This handles billboarding, canvas-to-texture conversion, and material setup natively. Materials use `depthWrite: false` to avoid z-fighting with graph nodes.

8. Token counts are formatted with thousands separators for readability (e.g. `1,024` not `1024`). Use a compact monospace font face for legibility at small sizes.

9. Zero-value token counts (0 or absent) do not spawn a sprite.

### Stacking & Coalescing

10. **Vertical stagger**: when multiple sprites are active on the same node, each new sprite spawns with an additional vertical offset of `+3 units` above the highest active sprite for that node. This prevents overlapping text when events arrive in quick succession.

11. **Coalescing under load**: if more than **3 sprites** are already active on a single node, new events do not spawn additional sprites. Instead, the most recent active sprite's text updates in-place to show the latest token count and its fade timer resets. This prevents visual noise during high-throughput bursts while still conveying "tokens are flowing fast."

### Performance

12. A maximum of **50 active sprites** globally at any time. If the limit is reached, the oldest sprite is immediately removed before spawning a new one.

13. Sprite materials are pooled — one shared `SpriteMaterial` per color (cyan, amber) with `depthWrite: false`, reused across all sprites of that color. Text content is set per-instance via `three-spritetext`'s API. This minimizes GPU draw call overhead.

14. The animation update runs via `requestAnimationFrame` for smooth per-frame interpolation, independent of the 200ms `syncGraphData()` batch interval. The animation must not cause frame drops below 30 FPS with 50 active sprites and 200 nodes.

## Scope

### In Scope

- `internal/viz/web/app.js`: sprite creation, animation loop, sprite pool, coalescing logic
- `internal/viz/web/index.html`: add `three-spritetext` script tag (CDN or vendored)
- `internal/viz/web/style.css`: no changes expected (sprites are 3D, not DOM)

### Out of Scope

- Backend Go changes (token data already flows through edge attributes)
- Aggregated token displays (cumulative counters, charts) — future spec
- Animating other event types (tool calls, MCP calls) — future spec
- Cost ($) animations — future spec
- Configurable colors, timing, or font — hardcoded defaults in this spec

## Non-Goals

- Not a token budget/limit indicator — purely informational visualization
- Not a replacement for the HUD aggregate stats — complementary
- Not configurable colors/timing in this spec — hardcoded defaults; configurability is a future enhancement

## Acceptance Criteria

- [ ] When a `model_call` edge update arrives with `input_tokens: 1024`, a cyan floating number "1,024" appears above the destination LLM node with an elastic bounce-in, drifts upward, and fades out within ~2.5 seconds
- [ ] When a `model_call` edge update arrives with `output_tokens: 512`, an amber floating number "+512" appears above the destination LLM node with an elastic bounce-in, drifts upward, and fades out within ~2.5 seconds
- [ ] When both token counts arrive in the same event, two sprites appear side by side (horizontally offset) without overlapping
- [ ] Zero-value or missing token counts do not produce sprites
- [ ] Token counts display with thousands separators (e.g. `12,345`)
- [ ] Rapid sequential events on the same node stagger vertically (each +3 units higher) rather than overlapping
- [ ] When >3 sprites are active on a single node, new events coalesce into the most recent sprite (text updates, timer resets) rather than spawning more
- [ ] No more than 50 sprites exist globally; oldest are evicted when the cap is hit
- [ ] Sprites use `three-spritetext` with `depthWrite: false` — no z-fighting with graph nodes
- [ ] The visualization maintains >30 FPS with 50 active sprites and 200 nodes
- [ ] Replay mode (`mindspec viz replay`) triggers the same animations as live mode
- [ ] `make test` passes — no regressions

## Validation Proofs

- `mindspec viz replay <session.jsonl> --speed 5`: observe floating token numbers above LLM nodes as `api_request` events replay; cyan numbers for input, amber for output
- Browser DevTools performance tab: confirm no sustained frame drops below 30 FPS during active animation
- `make test`: all existing and new tests pass

## Open Questions

*None — all design decisions are captured in the requirements above.*

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-14
- **Notes**: Approved via mindspec approve spec