# spec-128-plan-approve — consolidated concrete_changes_required, round 4

Tally: 4 APPROVE (R1, R2, R3, R4 — all zero asks) / 2 REQUEST_CHANGES (R5 ×2, R6 ×2). 4 asks, one convergent core — 2 groups. Artifact-gate screen: clear.

## G1 — The materialization marker must be a full state machine over EVERY tip transition (R5 + R6, tight convergence) — MUST FIX
The round-3 design covers only A→C. A run killed after the artifact-sync commit (tip S/D, marker C) — or during a preserved merge — is indistinguishable from a foreign commit under marker≠tip→delete→normal-panel, and dead-ends at ADR-0037 staleness. Required:
1. Extend the marker protocol across each ref-advancing step (materialization C, artifact-sync D, merge): make each transition durably/atomically recognizable as tool-authored — e.g. construct D with plumbing and transactionally advance bead ref + marker (git update-ref --stdin transactions), or an equivalently fail-closed protocol — so interruption at ANY stage resumes forward before the panel gate; the scoped cannot-launder boundary is retained (a genuinely foreign tip still deletes+blocks).
2. Exact ordering, stated everywhere consistently: commit-tree produces C BEFORE marker(C) can exist — fix bead 6's current contradictory sequence (it says marker write before commit-tree); state whether marker creation is fail-closed; enumerate marker state + cleanup at each step.
3. Terminal ordering: landed-merge reconciliation is detected BEFORE any marker-vs-missing-tip stale decision; ordinary and reconcile completion clear an orphaned marker.
4. Fixtures: red-on-revert kill legs after the artifact-sync commit AND after a preserved merge conflict — plain re-run / --resolve-merge converges without re-paneling or laundering; keep the update-ref-to-reset leg.
5. ADR-0041 amendment draft updated to the exact protocol; preserve the loose-object qualification (R5 confirms it now correct).

## G2 (R6, mechanical) — Remove the stale O_EXCL lead-in
The Windows lock section still opens with the superseded O_EXCL framing; LockFileEx replaced it — sweep the leftover text.
