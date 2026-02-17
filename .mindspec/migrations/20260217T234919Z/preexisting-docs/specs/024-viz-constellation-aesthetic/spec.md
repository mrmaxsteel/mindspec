# Spec 024-viz-constellation-aesthetic: Viz Constellation Aesthetic

## Goal

Transform the AgentMind Viz from a clinical "3D graph demo" into an atmospheric constellation star chart. Nodes become glowing star points with diffraction spikes, edges become thin constellation lines, the starfield becomes dense and multi-layered, and bloom post-processing makes active regions of the graph glow with warm nebula-like light — where **bloom intensity is driven by activity** (token throughput, call frequency, cost), so the glow is meaningful, not decorative.

Reference: `mind-constellation.png` in the project root.

## Background

Spec 022 delivered the functional AgentMind Viz MVP: a 3D force-directed graph with colored spheres, particle-animated edges, and a basic starfield. It works, but the aesthetic is flat — uniform node sizes, no glow, sparse starfield, thick edges. The reference image shows a constellation/star-chart look: bright star points with cross-shaped lens flares, thin wiry connector lines, thousands of background stars at varying brightness, and warm nebula regions where activity concentrates.

The key insight from research is that **bloom should encode data, not just look pretty**. The spec ties bloom intensity to meaningful signals:
- **Node bloom** scales with `activityCount` — frequently-used tools and hot LLM endpoints literally glow brighter
- **Edge bloom** pulses on new events — a model call or tool invocation flashes along the edge, then fades
- **Cumulative cost glow** — LLM endpoint nodes shift color temperature from cool blue toward warm amber as their cumulative `cost_usd` grows, creating the warm nebula regions visible in the reference image

This makes the visualization not just beautiful but informative at a glance: the brightest, warmest regions are where your agent is spending the most compute.

## Impacted Domains

- **viz (frontend)**: Major changes to node rendering, edge rendering, starfield, and post-processing in `app.js` + `index.html`
- **viz (backend)**: Minor — need to expose per-node cumulative cost and token totals through the WebSocket payload so the frontend can drive bloom/color temperature

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): CLI-first — all changes are vanilla JS in the embedded SPA; Three.js postprocessing loaded via CDN, no build step
- [ADR-0004](../../adr/ADR-0004.md): Go implementation — backend additions stay in Go

## Requirements

### Node Rendering — Star Points

1. Replace the default sphere geometry with **custom star-point nodes** via `nodeThreeObject()`. Each node is a `THREE.Group` containing:
   - A small bright **core sprite** (circular, white center with type-colored tint) — the star itself
   - A larger, fainter **glow sprite** (soft radial gradient, same color, additive blending) — the halo
   - Optional: subtle **diffraction spikes** (4-point cross) via a second sprite with a cross-shaped texture, for high-activity nodes only (activityCount > 10)

2. **Core sprite size** scales logarithmically with `activityCount`: `baseSize + log2(activityCount + 1) * scaleFactor`. Minimum size ensures all nodes are visible; maximum cap prevents any single node from dominating.

3. **Glow sprite size** is 3-4x the core size, with opacity proportional to `activityCount / maxActivityCount` (normalized 0.1–0.8 range). This creates the variable-brightness star effect visible in the reference.

4. Node colors remain type-coded (agent cyan, tool green, MCP magenta, data source orange, LLM gold) but shift from flat fills to **emissive materials** that interact with bloom.

### Edge Rendering — Constellation Lines

5. Replace thick, high-opacity edges with **thin constellation-style lines**: base width 0.15 (down from 0.3), max width 0.6 (down from uncapped). Line color matches edge type but at reduced saturation (~60% of current) to keep edges subtle against the starfield.

6. Reduce directional particle count from 2 to 1 and shrink particle width from 0.8 to 0.4. Particles should be nearly invisible in idle state — the lines should feel like static constellation connectors, not animated highways.

7. **Edge flash on activity**: when a new event creates or updates an edge, briefly boost that edge's opacity to 1.0 and particle speed to 0.02 for 500ms, then decay back to base opacity. This creates a visible "pulse" along the connection when an event fires.

### Starfield — Dense Multi-Layer Background

8. Replace the single 2,000-point starfield with a **three-layer parallax starfield**:
   - **Far layer**: 8,000 points, spread ±3000 units, size 0.3, color `#333355` (dim blue-grey)
   - **Mid layer**: 3,000 points, spread ±2000 units, size 0.6, color `#555577` (medium)
   - **Near layer**: 500 points, spread ±1000 units, size 1.0–2.0 (random), color `#8888aa` (bright), with slight random tinting toward warm/cool

9. The near layer includes ~20 **"bright stars"** (size 3.0, full white, subtle cross-shaped sprite) scattered randomly to add landmark points in the background, matching the reference image's varied star brightness.

### Bloom Post-Processing — Activity-Driven Glow

10. Add `UnrealBloomPass` via 3d-force-graph's `postProcessingComposer()` API. Parameters:
    - **threshold**: 0.6 — only bright (emissive) objects bloom, preventing the entire scene from glowing
    - **strength**: 1.2 — moderate bloom intensity
    - **radius**: 0.8 — moderately wide glow spread

11. **Node emissive intensity** (which drives bloom) is a function of **activity recency and volume**:
    - `emissive = baseEmissive + (recentActivityBoost * decayFactor)`
    - `recentActivityBoost` spikes to 1.0 each time a node receives an event, then decays over 2 seconds back to `baseEmissive`
    - `baseEmissive` scales with `log2(activityCount)` normalized to 0.1–0.5 range
    - Result: active nodes pulse with bloom on each event, and frequently-used nodes have a steady higher glow

12. **Cost-driven color temperature** for LLM endpoint nodes: as cumulative `cost_usd` grows, the node's color shifts from its base gold (`#ffd54f`) toward warm amber/orange (`#ff8a65`). This creates the warm nebula regions in high-cost areas. The shift is a linear interpolation: `lerp(baseColor, warmColor, min(1.0, costUSD / costCeiling))` where `costCeiling` defaults to $1.00 (adjustable via a constant).

13. **Selective bloom via Three.js layers**: assign bloom-eligible objects (node glow sprites, active edges) to layer 1; non-bloom objects (starfield, HUD, labels) to layer 0. This prevents the background and UI from blooming, keeping the glow focused on data nodes. If `postProcessingComposer()` doesn't support layer-based selective bloom, fall back to setting `threshold` high enough that only emissive node materials exceed it.

### Always-On Labels for Active Nodes

14. Display small, dim text labels (via `three-spritetext`) on the **top 8 most active nodes** (by `activityCount`). Labels show the node's `label` field (e.g. "claude-sonnet-4-5-20250929", "Read", "Bash"). Labels are positioned slightly above the node, font size 3, opacity 0.5, monospace. Non-top-8 nodes show labels on hover only (existing behavior).

15. The active-label list is recalculated every 2 seconds (not every frame) to avoid label flickering. When a node exits the top 8, its label fades out over 500ms rather than disappearing instantly.

### Backend: Per-Node Cost & Token Totals

16. Extend `Node` struct in `graph.go` with two new fields:
    - `CumulativeTokens int64` — sum of `input_tokens + output_tokens` for all events touching this node
    - `CumulativeCost float64` — sum of `cost_usd` for all events touching this node

17. In `UpsertNode()`, when processing a node upsert that carries `input_tokens`, `output_tokens`, or `cost_usd` in its attributes, accumulate these into the node's cumulative fields.

18. Include `cumulativeTokens` and `cumulativeCost` in the JSON serialization of `Node` so the frontend can access them for bloom/color decisions.

### Performance

19. The bloom pass adds a full-screen post-processing step. To maintain >30 FPS:
    - Bloom resolution is set to **half-resolution** (canvas width/2, height/2)
    - Glow sprites use shared materials (one per node type color, reused via a material cache)
    - The diffraction spike sprites are only created for nodes with `activityCount > 10` and removed when nodes go stale

20. Total draw call budget: target <200 draw calls. The starfield layers are 3 `THREE.Points` objects (1 draw call each). Node groups add ~2-3 draw calls per visible node. Edge rendering is handled by 3d-force-graph internally.

## Scope

### In Scope

- `internal/viz/web/app.js`: node rendering overhaul, edge styling, starfield, bloom setup, label system
- `internal/viz/web/index.html`: add Three.js postprocessing scripts (CDN), add `three-spritetext` script
- `internal/viz/web/style.css`: minor — HUD/legend may need slight opacity adjustments to not compete with bloom
- `internal/viz/graph.go`: add `CumulativeTokens` and `CumulativeCost` fields to `Node`, accumulate in `UpsertNode()`
- `internal/viz/normalize.go`: no changes (token/cost data already passes through attributes)

### Out of Scope

- Sound / sonification — future spec
- Timeline / waterfall panel — future spec
- Configurable bloom parameters via CLI flags — hardcoded defaults for now
- Mobile / responsive layout
- Node clustering / community grouping
- Minimap / radar overlay

## Non-Goals

- Not pixel-perfect reproduction of the reference image — the reference is a target *direction*, not a pixel spec
- Not a full shader rewrite — leverage Three.js built-in materials and postprocessing, avoid custom GLSL
- Not a performance benchmark — target is "feels smooth" (>30 FPS at 200 nodes), not a GPU benchmark suite

## Acceptance Criteria

- [ ] Nodes render as glowing star points (bright core + soft halo) rather than solid spheres
- [ ] Node glow intensity varies visibly with `activityCount` — a node with 50 activity is noticeably brighter than one with 2
- [ ] LLM endpoint nodes shift from gold toward warm amber as `cumulativeCost` increases
- [ ] Bloom post-processing is active — bright nodes bleed light into surrounding space
- [ ] Bloom is selective — starfield background and HUD elements do not bloom
- [ ] Edges are thin constellation-style lines (width 0.15 base), not thick bands
- [ ] Edges flash brighter momentarily when a new event fires on them
- [ ] Starfield has 3 visible depth layers totaling ~11,500 stars with varying brightness
- [ ] The near starfield layer includes ~20 bright landmark stars
- [ ] The top 8 most active nodes display always-on text labels; labels fade when a node exits the top 8
- [ ] `Node` JSON payload includes `cumulativeTokens` and `cumulativeCost` fields
- [ ] The visualization maintains >30 FPS with bloom active, 200 nodes, and 1000 edges
- [ ] `make test` passes — no regressions
- [ ] Visual comparison with `mind-constellation.png` shows a recognizable constellation aesthetic (subjective, verified by user)

## Validation Proofs

- `mindspec viz replay <session.jsonl> --speed 5`: observe star-point nodes with glow, thin constellation edges, dense starfield, and bloom effects on active nodes
- Browser DevTools performance tab: confirm >30 FPS sustained with bloom active during replay
- `make test`: all existing and new tests pass
- Visual spot-check: screenshot the viz during active replay and compare side-by-side with `mind-constellation.png`

## Open Questions

*None — all design decisions are captured in the requirements above.*

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: —
