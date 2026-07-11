# spec-115-bead1 — Round 3 consolidated tally

**Reviewed**: `bead/mindspec-fgmg.1` @ `e71a7991` (impl + r1-fix + r2-structural-fix). **Panel**: 8 slots. **Threshold**: 8/8 UNANIMOUS.

## Verdicts
R1 Opus APPROVE · R2 Opus APPROVE · R3 Opus APPROVE · R4 Sonnet APPROVE · R5 Sonnet APPROVE · R6 Sonnet APPROVE · R7 Fable APPROVE · **R8 codex REQUEST_CHANGES 0.99**.

**7 APPROVE / 1 RC** → fix + re-panel round 4. R8 confirmed rounds-1/2 holes closed and the top-level-key validation is now structural/exhaustive; it found a deeper edge INSIDE `panel_refuted_entries` at the element level.

## The finding (R8 round 3) — a real ASYMMETRY, but security-INERT

`panel_refuted_entries` array ELEMENTS are not shape-validated: `encoding/json` decodes a `null` element as a zero-valued `panel.Refutation{Slot:"", Round:0}`, and a covering entry with empty slot / round < 1 is never rejected. `{"panel_refuted_entries":[null]}` with pending absent returns nil.

**Orchestrator assessment — this is defense-in-depth, NOT a live fail-open:** a malformed covering entry produces coverage key `pendingEntryKey("", 0)`. A VALID pending obligation has non-empty slot + round ≥ 1, so its key can never equal `("", 0)`; and a pending entry with empty-slot/round<1 is already rejected (shape-invalid → error) before the coverage check. Therefore a malformed covering entry **cannot** over-cover a valid pending obligation — `{"panel_refuted_entries":[null]}` with pending absent returning nil is behaviorally correct (no obligations to cover). R8 acknowledges this ("even though such an entry cannot cover a valid pending obligation").

**Why fix it anyway (one final, structural pass):**
1. It is a genuine ASYMMETRY — pending elements ARE shape-checked (empty slot / round < 1 → error), refuted elements are NOT. Closing it makes validation EXHAUSTIVE and symmetric across both arrays.
2. An exhaustive per-element validation is a CONVERGENT structural close: a fully-validated structure (every field of every element of both arrays checked) has no unvalidated field left, so it definitively closes the corrupt-shape class (unlike rounds 1–2's patches).
3. The spec R3 contract states present-but-corrupt values REFUSE unconditionally; a `[null]`/shape-invalid covering element is a corrupt value.
4. P1 fail-closed security spec — exhaustive, auditable validation over "trust me, it's inert."
5. No false-refuse: valid covering entries (written by the 114 reconciliation) always carry a real slot + round ≥ 1; only genuinely-corrupt elements are rejected.

## Fix (round 4 — EXHAUSTIVE per-element validation, then re-panel)

In `uncoveredPendingObligations`, after decoding `panel_refuted_entries` and BEFORE the `len(pending)==0` early return, validate EVERY decoded covering element's shape (non-empty slot AND round ≥ 1) — the SAME rule already applied to pending entries — erroring on any shape-invalid covering element (this also rejects the `[null]` zero-valued element). Keep it symmetric with the pending-element check. Add tests: `{refuted:[null]}` pending-absent → error; `{refuted:[{slot:"",round:1}]}` → error; `{refuted:[{slot:"x",round:0}]}` → error; plus no-false-refuse pins (valid covering entries with absent/empty pending still → nil; a valid non-empty pending with a valid covering entry still covers → nil). Confine to `internal/complete/panel_advisory.go` + test; one commit; clean tree; full gates green; reconcile unchanged on valid inputs.

## Spiral guard
This is the 3rd consecutive codex corrupt-shape finding (rounds 1, 2 were live fail-opens; round 3 is inert defense-in-depth). Round-4 fix makes element+field validation EXHAUSTIVE for both arrays. **If round 4 yields ANOTHER corrupt-shape RC after exhaustive validation, escalate to a decision panel (autonomous-grant design-fork clause) to rule whether the gate is sufficiently fail-closed — do NOT patch a fifth time reflexively.**

No other findings — all 7 other slots re-confirmed APPROVE on `e71a7991` (fail-closed completeness + no over-correction, RED-on-revert modeling faithful, seam/type intact, reconcile unchanged, scope/grounding clean).
