# Spec 026-viz-visual-polish: AgentMind Viz Visual Polish

## Goal

Elevate the AgentMind Viz from a functional graph demo into an atmospheric constellation star chart with polished dashboard chrome. This spec supersedes the unimplemented Spec 024 — it incorporates that spec's node/edge/starfield/bloom ideas and adds dashboard, legend, and detail-card refinements that 024 didn't cover.

Reference aesthetic: `mind-constellation.png` in the project root.

## Background

Specs 022 (MVP), 023 (token animation), and 025 (multi-agent identity) delivered a working 3D force-directed visualization. The graph functions well, but the visual presentation is flat: uniform-brightness sprites, thick edges, a sparse 2,000-point starfield, no post-processing glow, and utilitarian dashboard panels.

Spec 024 (Constellation Aesthetic) was drafted to address the 3D scene rendering but never approved or implemented. It also left the dashboard/UI chrome untouched. This spec rolls 024's core ideas (star-point nodes, constellation edges, multi-layer starfield, bloom) into a single implementable unit and adds polish to every UI panel — HUD, legend, detail card, recording dashboard, and control bar.

The design principle: **bloom and brightness encode data**. Active nodes glow brighter, high-cost LLM nodes shift warm, edges flash on activity. The result should be informative at a glance — the brightest, warmest regions are where compute concentrates.

## Impacted Domains

- **viz (frontend)**: Major changes to node/edge rendering, starfield, post-processing, and all UI panel styling in `app.js`, `style.css`, `index.html`
- **viz (backend)**: Minor — expose per-node cumulative cost and token totals through WebSocket payloads

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): CLI-first — all changes are vanilla JS in the embedded SPA; Three.js postprocessing loaded via CDN, no build step
- [ADR-0004](../../adr/ADR-0004.md): Go implementation — backend additions stay in Go

## Requirements

### 1. Colour Palette Refinement

1. **Shift to a deeper, cooler base palette.** The current `#0a0a1a` background stays, but accent colors move from Material Design tones to a more cohesive constellation palette:
   - Agent nodes: `#5eead4` (teal, slightly warmer than current cyan `#4fc3f7`)
   - Tool nodes: `#86efac` (mint green, crisper than current `#81c784`)
   - MCP server nodes: `#c4b5fd` (lavender, softer than current `#ce93d8`)
   - Data source nodes: `#fdba74` (peach-orange, slightly softer than current `#ffb74d`)
   - LLM endpoint nodes: `#fde68a` (warm gold, more cohesive with the constellation warm-glow aesthetic)

2. **Edge colors track their source node type** (same mapping as today, updated to new palette values).

3. **UI accent color** shifts from `#7aa2f7` (periwinkle) to `#93c5fd` (lighter sky blue) for better contrast against the dark panels and bloom glow. Panel border color lightens to `rgba(147, 197, 253, 0.25)`.

### 2. Node Rendering — Star Points

4. Replace the current flat glow sprite with a **two-layer star-point node** via `nodeThreeObject()`. Each node is a `THREE.Group`:
   - **Core sprite**: small, bright, white-center with type-colored tint — the star itself
   - **Glow sprite**: 3-4x core size, soft radial gradient, same type color, additive blending — the halo

5. **Core sprite size** scales logarithmically: `baseSize + log2(activityCount + 1) * scaleFactor`. Min 4, max 20.

6. **Glow sprite opacity** is proportional to `activityCount / maxActivityCount` (clamped 0.15–0.8), creating variable-brightness stars.

7. **Diffraction spikes** (optional 4-point cross sprite) appear on nodes with `activityCount > 10`, adding the lens-flare effect from the reference image.

### 3. Edge Rendering — Constellation Lines

8. Reduce edge width to **0.15 base / 0.6 max** (down from current values). Edge color saturation reduces to ~60% for subtlety.

9. **Edge flash on activity**: when an event fires on an edge, briefly boost opacity to 1.0 for 400ms, then decay back. This replaces the current always-visible glow with a pulsing constellation connector aesthetic.

10. Reduce directional particle size (0.4 width, down from 0.8) and speed. Idle edges should feel like static connectors, not animated highways.

### 4. Starfield — Dense Multi-Layer Background

11. Replace the single 2,000-point starfield with **three parallax layers**:
    - **Far**: 8,000 points, spread ±3000 units, size 0.3, color `#333355`
    - **Mid**: 3,000 points, spread ±2000 units, size 0.6, color `#555577`
    - **Near**: 500 points, spread ±1000 units, size 1.0–2.0 random, color `#8888aa`

12. Near layer includes ~20 **bright landmark stars** (size 3.0, white, subtle cross sprite) for depth anchoring.

### 5. Bloom Post-Processing

13. Add `UnrealBloomPass` via Three.js postprocessing. Parameters: threshold 0.6, strength 1.2, radius 0.8.

14. **Node emissive intensity** drives bloom:
    - Pulses to 1.0 on each event, decays over 2s back to base
    - Base emissive scales with `log2(activityCount)` normalized to 0.1–0.5
    - Result: hot nodes glow steadily; all nodes pulse on activity

15. **Cost-driven color temperature** for LLM nodes: color shifts from base gold → warm amber (`#ff8a65`) as `cumulativeCost` grows, via `lerp(base, warm, min(1.0, cost / $1.00))`.

16. **Selective bloom**: assign bloom-eligible objects (node glows, active edges) to Three.js layer 1; non-bloom objects (starfield, UI) to layer 0. Fallback: high threshold if layer-based separation isn't feasible with `postProcessingComposer()`.

### 6. Always-On Labels for Active Nodes

17. Display dim text labels (via `three-spritetext` or canvas-textured sprites) on the **top 8 most active nodes**. Labels show the node's `label` field, positioned above the node, font size 3, opacity 0.5, monospace.

18. Active-label list recalculates every 2 seconds. Exiting nodes fade out over 500ms.

### 7. Dashboard & UI Chrome Polish

19. **HUD panel** (top-left):
    - Add a subtle top-border accent line (2px, gradient from accent color to transparent)
    - Value text uses tabular-nums for stable alignment as numbers change
    - Error count uses `#f87171` (red-400) instead of current `#f7768e`
    - Add a subtle background blur (`backdrop-filter: blur(12px)`) for depth

20. **Legend panel** (bottom-left):
    - Legend dots gain a subtle outer glow matching their color (`box-shadow: 0 0 6px <color>`)
    - Add edge type sub-section beneath node types (thin line separator), showing edge type → color mapping with small dashed-line indicators

21. **Detail card** (right panel):
    - Header row shows a colored pip (matching node/edge type) beside the type label
    - Key-value rows alternate subtle background shading for readability (`rgba(147, 197, 253, 0.03)` on odd rows)
    - Token counts and cost values in the detail card use the same color coding as the dashboard (cyan for input tokens, gold for output, green for cost)

22. **Recording dashboard**:
    - Section dividers use a gradient line (accent → transparent) instead of a flat border
    - Bar chart fills gain a subtle inner glow (box-shadow inset)
    - Cost value uses a green glow effect (`text-shadow: 0 0 8px rgba(134, 239, 172, 0.4)`)
    - Overview stat values gain a scale-up micro-animation on change (CSS `transition: transform 0.2s`)

23. **Control bar** (top-right):
    - Active/hover button states gain a subtle glow (`box-shadow: 0 0 8px rgba(147, 197, 253, 0.3)`)
    - Pause button when active shows a pulsing border (like the recording button but in accent blue)
    - Search input gains a faint inner glow on focus

### 8. Backend: Per-Node Cost & Token Totals

24. Extend `Node` struct in `graph.go` with:
    - `CumulativeTokens int64` — sum of input + output tokens for all events on this node
    - `CumulativeCost float64` — sum of `cost_usd` for all events on this node

25. In `UpsertNode()`, accumulate `input_tokens`, `output_tokens`, and `cost_usd` from event attributes into cumulative fields.

26. Include `cumulativeTokens` and `cumulativeCost` in Node JSON serialization.

## Scope

### In Scope

- `internal/viz/web/app.js`: node rendering overhaul, edge styling, starfield layers, bloom setup, label system, color palette update
- `internal/viz/web/style.css`: full UI panel polish (HUD, legend, detail card, dashboard, controls)
- `internal/viz/web/index.html`: add Three.js postprocessing + sprite-text CDN scripts
- `internal/viz/graph.go`: add `CumulativeTokens` and `CumulativeCost` fields, accumulate in upserts
- Spec 024 status update: mark as superseded by 026

### Out of Scope

- Sound / sonification
- Timeline / waterfall panel
- Configurable bloom via CLI flags (hardcoded defaults)
- Mobile / responsive layout
- Node clustering / community detection
- Minimap / radar overlay
- Custom GLSL shaders

## Non-Goals

- Not a pixel-perfect reproduction of `mind-constellation.png` — the reference is a direction, not a pixel spec
- Not a full shader rewrite — leverage Three.js built-ins, avoid custom GLSL
- Not a performance benchmark — target is ">30 FPS with bloom at 200 nodes"

## Acceptance Criteria

- [ ] Nodes render as glowing star points (bright core + soft halo), not solid flat sprites
- [ ] Node glow intensity visibly varies with `activityCount` — 50-activity node is noticeably brighter than 2-activity
- [ ] High-activity nodes (>10 events) display diffraction spike sprites
- [ ] LLM endpoint nodes shift from gold toward warm amber as `cumulativeCost` increases
- [ ] Bloom post-processing is active — bright nodes bleed light into surrounding space
- [ ] Bloom is selective — starfield and HUD do not bloom
- [ ] Edges are thin constellation lines (0.15 base width), flash on activity
- [ ] Starfield has 3 visible depth layers (~11,500 stars) with ~20 bright landmarks
- [ ] Top 8 most active nodes display always-on text labels; labels fade on exit
- [ ] Updated colour palette is applied to nodes, edges, and UI accent elements
- [ ] HUD panel has backdrop blur, accent border, and tabular-nums alignment
- [ ] Legend panel shows both node types and edge types with glow dots
- [ ] Detail card has colored type pips, alternating row shading, and color-coded values
- [ ] Recording dashboard has gradient dividers, glowing bar fills, and green cost glow
- [ ] Control bar buttons have glow hover states; pause button pulses when active
- [ ] `Node` JSON includes `cumulativeTokens` and `cumulativeCost` fields
- [ ] Visualization maintains >30 FPS with bloom, 200 nodes, 1000 edges
- [ ] `make test` passes with no regressions
- [ ] Visual comparison with `mind-constellation.png` shows constellation aesthetic (subjective, user verified)

## Validation Proofs

- `mindspec viz replay <session.jsonl> --speed 5`: observe star-point nodes, constellation edges, dense starfield, bloom on active nodes, polished dashboard panels
- Browser DevTools performance tab: confirm >30 FPS sustained with bloom during replay
- `make test`: all existing and new tests pass
- Visual spot-check: screenshot during active replay, compare with `mind-constellation.png`

## Open Questions

*None — all design decisions are captured above.*

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-15
- **Notes**: Approved via mindspec approve spec