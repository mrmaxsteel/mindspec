# spec-115-bead1 — Round 2 consolidated tally

**Reviewed**: `bead/mindspec-fgmg.1` @ `19bc8114` (round-1 impl + round-1 fix). **Panel**: 8 slots. **Threshold**: 8/8 UNANIMOUS.

## Verdicts
R1 Opus APPROVE 0.97 · R2 Opus APPROVE 0.97 · R3 Opus APPROVE 0.97 · R4 Sonnet APPROVE 0.97 · R5 Sonnet APPROVE 0.97 · R6 Sonnet APPROVE 0.95 · R7 Fable APPROVE 0.95 · **R8 codex REQUEST_CHANGES 0.99**.

**7 APPROVE / 1 RC** → fix + re-panel round 3. R8 confirmed the round-1 hole is CLOSED and RED-on-revert; it found a sibling of the same corrupt-shape class.

## The finding (R8, confirmed — a real spec-contract gap, not a taxonomy quibble)

`uncoveredPendingObligations` returns early at `if len(pending) == 0 { return nil, nil }` **before** reading/validating `panel_refuted_entries`. So realizable corrupt metadata passes the fail-closed gate:
- `{"refutation_pending_entries":[], "panel_refuted_entries":null}` → nil (pass)
- `{"panel_refuted_entries":"corrupt"}` (pending absent/empty) → nil (pass)

The spec R3 contract states "a present-but-corrupt `refutation_pending_entries`/`panel_refuted_entries` value REFUSES — never decodes-as-empty" as an **unconditional** rule. The round-2 fix added a present-null check for `panel_refuted_entries` but placed it AFTER the `len(pending)==0` early return, so it only fires when pending is non-empty — a patch, not a structural close.

**Fix ≠ false-refuse:** the "pristine no-op" carve-out is for genuinely ABSENT keys. `{pending:[], refuted:null}` has a PRESENT-but-corrupt key → not pristine → refuse is the contract-correct behavior. A valid present empty array `refuted:[]` and an absent `refuted` key still pass. So the fix does not falsely refuse any valid or pristine bead.

## Fix (STRUCTURAL — round 3, then re-panel)

Restructure `uncoveredPendingObligations` so BOTH metadata keys are fully validated (present-null → error; decode-corrupt → error) **up front, before ANY early return**, order-independent:
1. Validate `refutation_pending_entries`: present-null → error; decode → error (already done).
2. Validate `panel_refuted_entries`: present-null → error; `decodeRefutations` → error — MOVE THIS BEFORE the `len(pending)==0` early return.
3. THEN the `len(pending)==0` no-op return (both keys already validated), then the coverage loop.
This closes the entire "corrupt-shape passes when pending empty" class in one structural move (no early return can skip a validation) — applying the spec-114 "structural, not sibling-by-sibling patch" lesson so no round-4 sibling of this class remains.
4. Add subtests: `{pending:[], refuted:null}` → error; `{pending absent/[], refuted:"corrupt"}` → error; AND confirm no false-refuse — `{}` (both absent) → nil; `{pending:[], refuted:[]}` (valid empties) → nil; absent-refuted with valid-empty-pending → nil.

Confine to `internal/complete/panel_advisory.go` + `panel_advisory_test.go`. One fix commit; clean tree. Re-run full gates; confirm reconcile unchanged on valid inputs (a valid bead with real obligations + valid coverage must still behave identically).

No other findings — all 7 other slots re-confirmed their round-1 APPROVE on the fixed tip (fail-posture, RED-on-revert modeling faithfulness, seam/type, no-regression, scope/grounding all green; R2 additionally confirmed no false-refuse over-correction and that typed-nil-slice is unrealizable from encoding/json).
