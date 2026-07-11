# spec-115-bead1 — Round 4 consolidated tally — PASS 8/8

**Reviewed**: `bead/mindspec-fgmg.1` @ `a8da895a` (impl + r1/r2/r3 fixes). **Panel**: 8 slots. **Threshold**: 8/8 UNANIMOUS.

## Verdicts — 8 APPROVE / 0 REQUEST_CHANGES / 0 REJECT
R1 Opus APPROVE · R2 Opus APPROVE (0.97) · R3 Opus APPROVE · R4 Sonnet APPROVE · R5 Sonnet APPROVE · R6 Sonnet APPROVE · R7 Fable APPROVE · **R8 codex APPROVE (0.99) — FLIPPED from 3 rounds of RC.**

**PASS.** The bead panel is unanimous. `mindspec complete` will re-verify this tally in-binary before merging bead→spec.

## The four-round hardening arc (all in the fail-closed obligation predicate `uncoveredPendingObligations`)
- **R1** (live fail-open): present-null `refutation_pending_entries` decoded-as-empty → passed the gate. Fixed: comma-ok present-null → error.
- **R2** (live fail-open, sibling): `panel_refuted_entries` corrupt/null validated only AFTER the `len(pending)==0` early return → passed when pending empty. Fixed: structural reorder, both keys validated up front.
- **R3** (defense-in-depth asymmetry, security-inert): `panel_refuted_entries` array ELEMENTS not shape-validated (a `null` element → zero-valued Refutation); pending elements were. Assessed inert (a malformed covering entry can't key-match a valid pending obligation — `pendingEntryKey` is NUL-separated, so `("",0)` never matches a valid `(slot,round≥1)`), but fixed for exhaustive fail-closed completeness + pending/refuted symmetry + the spec's unconditional present-but-corrupt-REFUSES contract. Fixed: per-element shape validation for refuted elements, symmetric with pending, before the early return.
- **R4**: 8/8. Validation is now EXHAUSTIVE — every present-null/decode/element-shape check for BOTH `refutation_pending_entries` and `panel_refuted_entries` runs before any early return. No realizable corrupt metadata shape decodes-as-empty. No false-refuse (pristine-absent and valid-empty-array still pass). No regression (reconcile byte-identical on valid inputs — covering entries are always written from already-validated pending entries).

## Spiral discipline
The spec-114 "codex finds a deeper edge every round" pattern was recognized: rounds 1-2 were live holes (fix), round 3 was inert defense-in-depth (fix once more, structurally + exhaustively, with an explicit spiral guard to escalate to a decision panel if round 4 produced another corrupt-shape RC). It did not — the exhaustive per-element validation converged, R8 flipped. The class is definitively closed.

## Non-blocking note (R7, info-level — no action)
"Every field of every element is validated" slightly overstates: only `Slot`/`Round` are shape-checked (not `Reason`/`Evidence`), which mirrors the pending-entry rule exactly and `(slot, round)` is the only coverage-load-bearing pair. Symmetry holds; no change needed.

## Next
`mindspec complete mindspec-fgmg.1 "<summary>"` → merges bead→spec, closes the bead, removes the worktree, advances state. Then Bead 2 (the gate).
