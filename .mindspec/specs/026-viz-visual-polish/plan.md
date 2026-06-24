---
adr_citations:
    - id: ADR-0003
      sections:
        - Requirements
    - id: ADR-0004
      sections:
        - Requirements
approved_at: "2026-02-15T15:43:55Z"
approved_by: user
bead_ids:
    - mindspec-hhv
    - mindspec-9np
    - mindspec-ea3
    - mindspec-yii
    - mindspec-787
    - mindspec-cv7
    - mindspec-b71
    - mindspec-13u
last_updated: "2026-02-15"
spec_id: 026-viz-visual-polish
status: Approved
version: 1
---

# Plan: 026-viz-visual-polish — AgentMind Viz Visual Polish

## Summary

Transform AgentMind Viz from a functional graph into an atmospheric constellation star chart with polished UI chrome. Eight beads covering backend data, colour palette, starfield, node rendering, edge rendering, bloom post-processing, labels, and dashboard polish. Supersedes unimplemented Spec 024.

## ADR Fitness

| ADR | Decision | Fit? | Notes |
|:----|:---------|:-----|:------|
| ADR-0003 | CLI-first, vanilla JS SPA | **Yes** | All frontend changes are vanilla JS/CSS. Three.js postprocessing loaded via ES module import map (CDN), no build step. |
| ADR-0004 | Go implementation language | **Yes** | Backend additions (`CumulativeTokens`, `CumulativeCost`) are Go-only. |
| ADR-0001 | DDD-informed context packs | **N/A** | No new domain boundaries. Viz remains a single subsystem. |
| ADR-0005 | Explicit state tracking | **Yes** | State transitions follow existing patterns. |

No ADR divergence — all accepted ADRs remain sound for this work.

## Technical Notes

Three.js 0.160.0 ships postprocessing as ES modules only. We use an **import map** in `index.html` to resolve module specifiers against CDN URLs (no build step, ADR-0003 compliant). Selective bloom uses the dual-render pattern with Three.js layers. Labels use `three-spritetext` UMD from CDN.

## Bead 1: Backend — Per-Node Cumulative Cost & Tokens — `mindspec-hhv`

**Scope**: `internal/viz/graph.go`, `internal/viz/graph_test.go`

**Steps**

1. Add `CumulativeTokens int64` and `CumulativeCost float64` fields to `Node` struct with JSON tags `json:"cumulativeTokens,omitempty"` and `json:"cumulativeCost,omitempty"`
2. In `UpsertNode()`, extract `input_tokens`, `output_tokens`, `cost_usd` from `u.Attributes` and accumulate into the node's cumulative fields
3. In `Reset()`, cumulative fields are implicitly zeroed by new `Node` creation
4. Add test: upsert a node with token/cost attributes twice, verify cumulative values sum correctly
5. Run `make test` to verify no regressions

**Verification**

- [ ] `make test` passes
- [ ] `Snapshot()` JSON includes `cumulativeTokens` and `cumulativeCost` fields

**Depends on**: None (first bead, independent)

## Bead 2: Colour Palette & UI Accent Update — `mindspec-9np`

**Scope**: `internal/viz/web/app.js`, `internal/viz/web/style.css`

**Steps**

1. Update `NODE_COLORS` map to constellation palette: agent `#5eead4`, tool `#86efac`, mcp_server `#c4b5fd`, data_source `#fdba74`, llm_endpoint `#fde68a`
2. Update `EDGE_COLORS` to match new node type colors
3. Update `NODE_COLORS_THREE` pre-computed `THREE.Color` instances
4. In `style.css`, replace accent color `#7aa2f7` with `#93c5fd` globally
5. Update panel border color to `rgba(147, 197, 253, 0.25)`

**Verification**

- [ ] Load viz, confirm node/edge colors match new palette
- [ ] UI accent and panel borders use updated colors

**Depends on**: None (independent)

## Bead 3: Multi-Layer Starfield — `mindspec-ea3`

**Scope**: `internal/viz/web/app.js`

**Steps**

1. Remove existing single-layer starfield creation code
2. Create `buildStarfield()` function returning 3 `THREE.Points` objects: far (8000 pts, ±3000, size 0.3, `#333355`), mid (3000 pts, ±2000, size 0.6, `#555577`), near (500 pts, ±1000, size 1.0–2.0, `#8888aa`)
3. Add ~20 bright landmark stars in the near layer (size 3.0, white, cross-shaped sprite texture)
4. Add all layers to scene via `graph.scene().add()`
5. Assign starfield layers to Three.js layer 0 (non-bloom)

**Verification**

- [ ] Load viz, observe dense multi-layer background with varied brightness
- [ ] Near layer includes visible bright landmark stars

**Depends on**: None (independent)

## Bead 4: Star-Point Node Rendering — `mindspec-yii`

**Scope**: `internal/viz/web/app.js`

**Steps**

1. Create `buildNodeGlowTexture()` — core sprite texture with bright center and type-colored tint
2. Create `buildNodeHaloTexture()` — larger, softer glow for halo (additive blending)
3. Create `buildDiffractionTexture()` — 4-point cross sprite for high-activity nodes
4. Replace `nodeThreeObject()` callback to return `THREE.Group` with core + glow sprites, plus optional diffraction sprite when `activityCount > 10`
5. Implement logarithmic sizing: core `4 + log2(activityCount + 1) * 2`, glow 3-4x core, max 20
6. Glow opacity scales with `activityCount / maxActivityCount` (clamped 0.15–0.8)
7. Assign node glow sprites to Three.js layer 1 (bloom-eligible)

**Verification**

- [ ] Load viz with replay data, observe star-point nodes with variable brightness
- [ ] High-activity nodes show diffraction spikes
- [ ] Node sizes scale logarithmically with activity

**Depends on**: mindspec-9np (Bead 2 — uses new color palette)

## Bead 5: Constellation Edge Rendering — `mindspec-787`

**Scope**: `internal/viz/web/app.js`

**Steps**

1. Reduce base edge width to 0.15, max to 0.6
2. Apply ~60% saturation reduction to `EDGE_COLORS` via desaturate helper
3. Reduce directional particle size to 0.4 width and slow speed
4. Add edge flash: on new event boost opacity to 1.0 for 400ms, then exponential decay
5. Assign recently-flashed edges to Three.js layer 1 for bloom interaction

**Verification**

- [ ] Load viz with replay, edges are thin constellation lines
- [ ] Edges pulse visibly on activity then fade back
- [ ] Idle edges feel like static connectors, not animated highways

**Depends on**: mindspec-9np (Bead 2 — uses new edge colors)

## Bead 6: Bloom Post-Processing — `mindspec-cv7`

**Scope**: `internal/viz/web/index.html`, `internal/viz/web/app.js`

**Steps**

1. Add import map to `index.html` for Three.js module resolution (three, three/addons/)
2. After graph init, set up `EffectComposer` with `RenderPass` + `UnrealBloomPass` (threshold 0.6, strength 1.2, radius 0.8) via dynamic import
3. Implement dual-render selective bloom: render bloom layer → bloom pass → composite with full scene
4. Set node emissive intensity: base scales with `log2(activityCount)` (0.1–0.5), pulses to 1.0 on events, decays over 2s
5. Implement cost-driven color temperature for LLM nodes: `lerp(gold, amber, min(1.0, cumulativeCost / 1.0))`
6. Set bloom to half resolution for performance
7. Profile at 200 nodes to verify >30 FPS

**Verification**

- [ ] Bright nodes bleed light into surrounding space
- [ ] LLM nodes shift from gold toward amber as cost grows
- [ ] Starfield and HUD do not bloom (selective via layers)
- [ ] >30 FPS sustained with bloom at 200 nodes

**Depends on**: mindspec-hhv (Bead 1 — `cumulativeCost`), mindspec-yii (Bead 4 — node emissive materials), mindspec-787 (Bead 5 — edge layer assignment)

## Bead 7: Always-On Node Labels — `mindspec-b71`

**Scope**: `internal/viz/web/index.html`, `internal/viz/web/app.js`

**Steps**

1. Add `three-spritetext` CDN script to `index.html`
2. Track top-8 active nodes by `activityCount` (recalculate every 2 seconds)
3. For top-8 nodes, add `SpriteText` label to their `THREE.Group` (font size 3, opacity 0.5, monospace, above node)
4. When a node exits top-8, fade label opacity to 0 over 500ms, then remove
5. Non-top-8 nodes keep existing hover-only label behavior

**Verification**

- [ ] Load replay, labels appear on most active nodes
- [ ] Labels fade smoothly when ranking changes
- [ ] Non-top-8 nodes show labels on hover only

**Depends on**: mindspec-yii (Bead 4 — labels attach to node THREE.Group objects)

## Bead 8: Dashboard & UI Chrome Polish — `mindspec-13u`

**Scope**: `internal/viz/web/style.css`, `internal/viz/web/index.html`

**Steps**

1. HUD panel: add `backdrop-filter: blur(12px)`, top accent border (2px gradient), `font-variant-numeric: tabular-nums`, error color `#f87171`
2. Legend panel: add glow `box-shadow` to dots; add edge-type subsection with dashed-line indicators
3. Detail card: add colored type pip, alternating row shading, color-coded token/cost values
4. Recording dashboard: gradient section dividers, bar chart inner glow, green cost `text-shadow`, scale-up transition
5. Control bar: glow `box-shadow` on hover/active, pulsing pause button, focus glow on search
6. Update legend HTML in `index.html` to include edge-type entries

**Verification**

- [ ] HUD has backdrop blur and accent border
- [ ] Legend shows both node and edge types with glow dots
- [ ] Detail card has colored pips and alternating rows
- [ ] Recording dashboard has gradient dividers and glowing bars
- [ ] Control buttons have glow hover states

**Depends on**: None (independent — CSS/HTML only)

## Dependency Graph

```
Bead 2 (Palette) ──┬──→ Bead 4 (Nodes) ──┬──→ Bead 6 (Bloom)
                    │                      └──→ Bead 7 (Labels)
                    └──→ Bead 5 (Edges) ───────→ Bead 6 (Bloom)

Bead 1 (Backend) ─────────────────────────────→ Bead 6 (Bloom)

Bead 3 (Starfield) ───── independent
Bead 8 (Dashboard) ───── independent
```

## Risk & Mitigation

| Risk | Mitigation |
|:-----|:-----------|
| Import map + postprocessing CDN compat | Verify in Bead 6 step 1; fallback: vendor 3 postprocessing files locally |
| Bloom performance regression | Half-res bloom; profile at 200 nodes; reduce strength if needed |
| `3d-force-graph` renderer access | Library exposes `.renderer()` and `.scene()` — verified in existing code |
| `three-spritetext` UMD compat | Widely used with global THREE; test in Bead 7 step 1 |
