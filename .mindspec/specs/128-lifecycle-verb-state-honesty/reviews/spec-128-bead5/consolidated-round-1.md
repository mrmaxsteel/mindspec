# spec-128-bead5 — consolidated concrete_changes_required, round 1

Tally: 1 APPROVE (R2) / 5 REQUEST_CHANGES (R1 ×4, R3 ×1, R4 ×1, R5 ×3, R6 ×4). 13 asks in 6 groups.

## G1 (R1 + R3, convergent — a silently-flipped DESTRUCTIVE guard) — MUST FIX, highest priority
The `&& !ctx.DerivationFailed` guard on WorktreeSpec's ModeSpec backfill (derive.go:660) made the "additive, values unchanged" claim FALSE for that arm: internal/cleanup/cleanup.go:50's active-spec refusal (guarding worktree removal + branch deletion) flipped from fail-closed to fail-open under a bd outage in a spec worktree. Fix (both reviewers' preferred form): restore the unconditional ModeSpec backfill alongside DerivationFailed=true (the honest consumers check DerivationFailed BEFORE Phase, so indeterminate rendering is unaffected; WorktreeBead already does exactly this). AND add ≥1 internal/phase-level unit test pinning a DerivationFailed case at the derive level (grep shows ZERO direct test references — all five setting sites covered only transitively).

## G2 (R1 + R4 + R5 + R6 — FOUR-slot convergence) — AC-14's next/impl-adopt limitation is REJECTED — MUST FIX
Drive both verbs' real RunE result-before-guidance hermetically via the in-package fake-bd-on-PATH pattern (next_ready_gate_test.go precedent: canned-JSON bd script in t.TempDir + PATH prepend + t.Cleanup) combined with the newExecutorFn MockExecutor seam — no out-of-scope files, no new exported seams, no real bd. Replace TestVerbOutput_NextAndImplAdopt_ScopeLimitation's presence-only check with the executable assertions. Update the root.go/helper comments to match (R1's item 4 flips direction accordingly).

## G3 (R5) — AC-4 success branch needs a REAL driving test — MUST FIX
Drive the successful context/render path and assert the mode-specific rendered template EQUALS Result.NextMode — red when an implement result pairs with review/plan/idle/indeterminate guidance. The generic callback assertion is insufficient.

## G4 (R5) — impl adopt success lacks its post-result instruct tail — MUST FIX
Add the tail (same non-fatal tail-warning behavior as the other lifecycle verbs); pin both verified and attested success paths red-on-removal/reorder.

## G5 (R6 ×3) — instruct arrival-point gaps — MUST FIX
1. Protected-checkout fast path: prove beads health before rendering idle when .beads exists (zero-candidate case included); red-on-revert test (protected main, .beads present, no candidates, bd down → indeterminate, never idle).
2. Treat beads-derived main-context Phase==idle as a draft-discovery arrival point (not only nil/empty Phase): one-draft → spec guidance; multi-draft → ambiguity listing; none → idle. Coverage from an unprotected checkout.
3. Numeric --spec prefixes for pre-epic drafts: honor or refuse — never silent fall-through to idle (the flag-honoring class, third appearance). Pin numeric-prefix, full-ID, nonexistent, ambiguous-prefix forms.

## G6 (R1 ×2, small) — Rendering + comments — MUST FIX
termsafe.Escape on r.IndeterminateReason in FormatResult's summary line (matching the tail template's own classification, ADR-0042); comment corrections per G2's outcome.
