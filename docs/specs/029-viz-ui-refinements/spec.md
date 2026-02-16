# Spec 029-viz-ui-refinements: AgentMind Visual UI Refinements

## Goal

Three targeted refinements to the AgentMind visualization: a keyboard-togglable immersive mode that hides all UI chrome, more prominent node activity growth so hot nodes stand out visually, and slower edge glow decay so constellation lines linger after firing.

## Background

Spec 026 (Visual Polish) delivered star-point nodes, constellation edges, bloom, and polished dashboard panels. The visualization works well, but three usability gaps remain:

1. **No immersive mode** — the HUD, legend, controls, and detail card are always visible. When presenting or just watching the graph, users want a clean view showing only the 3D scene. The only keyboard shortcut today is Escape (close detail card / dashboard).

2. **Node activity growth is subtle** — the current core size formula `4 + log₂(activity+1) * 2` produces a narrow range (4–20px). A node with 100 events is only ~3.3x the size of a fresh node. Combined with the halo opacity range (0.15–0.8), high-activity nodes don't pop enough against medium-activity ones.

3. **Edge glow decays too fast and only fires once** — the current decay rate of `0.985` per 50ms tick gives a half-life of ~2.3 seconds. Constellation lines flash and fade before the eye can trace them, especially during bursts of activity. A longer linger would make the graph feel more alive. Also edge glow appears to only first first traversing pulse, subsequent pulses dont appear to make the edge glow again. I really wanted glow to be cumulative (to a point) so you can visually see the hot spot edges.

## Impacted Domains

- **viz (frontend)**: Changes to `app.js` (keyboard handler, node size/glow formulas, edge decay constants) and `style.css` (immersive mode class)

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): CLI-first — changes are vanilla JS in the embedded SPA, no build step

## Requirements

### 1. Immersive Mode Toggle

1. Add a keyboard shortcut (`H` key, case-insensitive) that toggles all UI overlays on/off. When toggled off ("immersive mode"), the following elements are hidden: HUD panel, legend, control bar, detail card, and recording dashboard.

2. Implement via a single CSS class `immersive` on `<body>`. When present, all fixed-position UI panels get `display: none` (or `opacity: 0` with `pointer-events: none` for a fade transition).

3. Pressing `H` again restores all panels to their previous state. If a detail card was pinned before entering immersive mode, it reappears on exit.

4. The `H` shortcut is suppressed when the search input is focused (to avoid accidental toggles while typing).

5. A brief toast notification ("UI hidden — press H to restore") appears for 2 seconds on entering immersive mode, then fades. The toast itself is exempt from the immersive hide rule.

### 2. More Prominent Node Activity Growth

6. Increase the node core size scale factor from `2` to `3`, changing the formula to `4 + log₂(activity+1) * 3`. This widens the effective range: a 100-event node goes from ~17px to ~24px (capped at max 28, up from 20).

7. Raise the max core size cap from `20` to `28`.

8. Increase the halo size multiplier from `3.5x` to `4x` the core size, making the glow envelope larger on active nodes.

9. Steepen the halo opacity curve by using a square-root scale: `sqrt(activity / maxActivity)` instead of linear `activity / maxActivity`. This makes mid-activity nodes noticeably brighter while keeping low-activity nodes dim. Clamp range stays 0.15–0.8.

10. Increase the bloom emissive base range from `0.1–0.5` to `0.15–0.65`, so active nodes push more light into bloom.

### 3. Slower Edge Glow Decay

11. Change `EDGE_GLOW.DECAY_RATE` from `0.985` to `0.993`, increasing the half-life from ~2.3s to ~5.0s. Edges linger noticeably longer after firing.

12. Increase `PARTICLE_DURATION` from `1.5s` to `2.5s` so glow particles traverse edges more slowly, matching the longer linger.

13. Reduce `EDGE_GLOW.FIRE_BOOST` from `0.8` to `0.6` to compensate for slower decay — prevents energy from stacking excessively during rapid bursts.

## Scope

### In Scope

- `internal/viz/web/app.js`: keyboard handler, immersive toggle state, node size/glow formulas, edge decay constants, toast notification
- `internal/viz/web/style.css`: `.immersive` class rules, toast styling

### Out of Scope

- Backend changes (no Go modifications)
- New UI panels or controls
- Changes to color palette or bloom parameters (addressed in Spec 026)

## Non-Goals

- Not adding a full keyboard shortcut system or keybinding config — just the single `H` toggle
- Not changing the force-graph layout or physics
- Not adding animation transitions to nodes on size change (would conflict with the 500ms glow update throttle)

## Acceptance Criteria

- [ ] Pressing `H` hides all UI panels (HUD, legend, controls, detail card, recording dashboard); pressing `H` again restores them
- [ ] `H` is suppressed when the search input has focus
- [ ] When UI panels are hidden, there is small subtle text in footer that lets the user know they can press `H` again to re-show the UI 
- [ ] A toast notification appears for 2 seconds when entering immersive mode
- [ ] A node with 100 events is visibly larger and brighter than a node with 5 events (core size ratio > 2x, halo opacity ratio > 1.5x)
- [ ] Node max core size is 28px (up from 20px)
- [ ] Edge glow persists for ~5 seconds after firing (vs ~2.3s previously)
- [ ] Edge glow increases for every firing pulse for ~5
- [ ] Glow particles traverse edges in ~2.5 seconds (vs 1.5s previously)
- [ ] Rapid edge firing does not cause runaway energy stacking (FIRE_BOOST reduced to 0.6)
- [ ] `make test` passes with no regressions
- [ ] Visual verification: replay a session and confirm nodes pop more, edges linger longer, H toggle works cleanly

## Validation Proofs

- `mindspec viz replay <session.jsonl> --speed 5`: observe larger/brighter active nodes, longer-lingering edge glow, press H to toggle immersive mode
- `make test`: all tests pass
- Manual: during replay, press H → all chrome disappears, toast shows briefly; press H → chrome returns

## Open Questions

*None — all design decisions are captured above.*

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: —
