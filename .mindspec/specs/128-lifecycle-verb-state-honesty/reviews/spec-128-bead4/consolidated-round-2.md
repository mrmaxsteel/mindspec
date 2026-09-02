# spec-128-bead4 — consolidated concrete_changes_required, round 2

Tally: 4 APPROVE (R1, R3, R4, R6 — hard block CLEARED) / 2 REQUEST_CHANGES (R2 ×2, R5 ×2, convergent). 2 groups.

## G1 (R2 + R5, convergent) — Break the stored-echo circularity — MUST FIX
Seven wireEpicFixture three-surface rows seed storedPhase FROM row.derivation — the fixture follows the record, so a wrong doc row + wrong record still passes. Fix: (a) make canonical fixture inputs independent of the expected cells (pin each verb's canonical post-state independently — by executing/asserting its production writer where feasible, else independent construction); (b) add the non-circular code leg: assert phase.DerivePhaseFromChildren(children)==row.derivation per row (spec-approve's nil-children empty-set fallback yields plan correctly). Acceptance probe: editing BOTH the doc row and the record together goes red. (c) Correct the file-header claim (:19-27) to what the mechanism actually guarantees.

## G2 (R5) — Row-1 disposition: TALLY RULING — add the independent derivation assertion — MUST FIX
No self-authorized two-surface carve-out for spec-create: the spec's R1.3 carve-out list does not include row 1, and amending the contract is not the fix author's call. Add the independent status-aware derivation assertion over the canonical fixture (the bead-3-delivered draft semantics make this constructible: draft dir + branch, no epic → derivation surface must yield spec independently of the tail path). Keep the WorktreeSpec-proxy disclosure as-is.
