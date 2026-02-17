---
adr_citations:
    - id: ADR-0003
      sections:
        - Architecture Notes
        - ADR Fitness
    - id: ADR-0004
      sections:
        - Architecture Notes
        - ADR Fitness
approved_at: "2026-02-14T20:18:29Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-14"
spec_id: 023-viz-token-animation
status: Approved
version: 1
---

# Plan: 023-viz-token-animation

## Summary

Add floating token-count animations to the 3D viz. When `model_call` edge updates arrive with token counts, spawn cyan (input) and amber (output) number sprites that bounce in, drift upward, and fade out over 2.5 seconds. All work is frontend-only — token data already flows through edge attributes via WebSocket.

## Architecture Notes

**Data flow (already complete):**
`normalize.go` extracts `input_tokens`/`output_tokens` into edge `Attributes` → graph stores them → WebSocket broadcasts edge updates with full attributes → frontend `handleUpdate()` receives them.

**No backend changes needed.** The frontend's `handleUpdate()` is the sole extension point.

**Library addition:** `three-spritetext` (by vasturiano, same author as `3d-force-graph`) for billboard text sprites. Loaded via CDN `<script>` tag — consistent with existing vendoring pattern (no npm/build step per ADR-0003/0004).

## ADR Fitness

### ADR-0003 (CLI-First Architecture)
**Verdict: Sound.** The viz SPA is an embedded asset served by the Go binary. Token animations are purely client-side JS — no new CLI commands, no coupling to the instruction system. Adherence confirmed.

### ADR-0004 (Go as CLI Language)
**Verdict: Sound.** No new Go code required for this spec. All changes are in the embedded `web/` assets (JS, HTML). The `embed.FS` pattern continues unchanged. Adherence confirmed.

No ADR divergence detected. No new ADR needed.

## Bead 1: Sprite System & Core Animation

**Scope**: Add `three-spritetext`, create the sprite manager, wire into `handleUpdate()`, and implement the full animation lifecycle.

**Steps**
1. Add `three-spritetext` CDN `<script>` tag to `internal/viz/web/index.html` (after the 3d-force-graph script)
2. Create sprite manager in `app.js` — `spritePool` Map (nodeId → active sprites), `allSprites` array (global list), two shared materials (cyan `#4fc3f7` and amber `#ffd54f`, both `depthWrite: false`), `formatTokenCount(n)` helper for thousands separators
3. Hook `handleUpdate()`: when an edge has `type === 'model_call'` and non-zero `input_tokens` or `output_tokens` in attributes, call `spawnTokenSprite(nodeId, count, isOutput)`
4. Implement `spawnTokenSprite()` — create `SpriteText` with formatted count (prefix `"+"` for output), set color/font/material, position at destination node with horizontal offset ±2 units, record spawn time, add to pools, add to `Graph.scene()`
5. Implement animation loop via `requestAnimationFrame` — 0–0.3s elastic bounce-in (opacity 0→1, scale pop 0.5→1.2→1.0, Y overshoot +2 then settle); 0.3–2.0s float up at ~8 units/sec; 2.0–2.5s fade out (opacity 1→0); at 2.5s remove from scene and pools
6. Add zero-value guard: skip sprite creation when token count is 0, null, or undefined

**Verification**
- [ ] `mindspec viz replay <session.jsonl>`: cyan and amber numbers appear above LLM nodes on `api_request` events
- [ ] Numbers display with thousands separators (e.g. `1,024`)
- [ ] Sprites animate: bounce-in → float → fade → gone in ~2.5s
- [ ] Paired input/output sprites appear side-by-side (horizontally offset), not overlapping
- [ ] `make test` passes — no regressions

**Depends on**: none

## Bead 2: Stacking, Coalescing & Performance

**Scope**: Handle rapid-fire events gracefully — vertical stagger, coalescing under load, global sprite cap, and replay mode verification.

**Steps**
1. Implement vertical stagger: before spawning, scan `spritePool[nodeId]` for active sprites; set Y offset = highest active sprite's current Y + 3 units above the node to prevent text overlap on rapid events
2. Implement coalescing: if `spritePool[nodeId].length >= 3`, do not spawn a new sprite; instead update the most recently spawned sprite's text to the new token count and reset its animation timer to 0 (restart the 2.5s lifecycle)
3. Implement global cap: if `allSprites.length >= 50`, remove the oldest sprite (earliest spawn time) from scene and arrays before spawning a new one
4. Verify replay mode: confirm `mindspec viz replay` triggers the same `handleUpdate()` path — replay events should produce identical animations with no separate code path needed
5. Performance validation: with 50 active sprites and 200 nodes, confirm >30 FPS in browser DevTools performance tab

**Verification**
- [ ] Rapid sequential events on same node: sprites stack vertically (each +3 units higher)
- [ ] >3 rapid events on same node: 4th event coalesces into 3rd sprite (text updates, timer resets)
- [ ] Never more than 50 sprites visible globally; oldest evicted smoothly
- [ ] `mindspec viz replay <session.jsonl> --speed 5`: animations fire correctly in replay mode
- [ ] DevTools performance: no sustained drops below 30 FPS with 50 sprites + 200 nodes
- [ ] `make test` passes — no regressions

**Depends on**: Bead 1
