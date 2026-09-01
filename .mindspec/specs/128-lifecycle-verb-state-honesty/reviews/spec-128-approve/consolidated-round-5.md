# spec-128-approve — consolidated concrete_changes_required, round 5

Tally: 3 APPROVE (R1: 0 asks, R3: 1 advisory, R4: 0) / 3 REQUEST_CHANGES (R2, R5, R6 — 1 ask each). 4 items, no duplicates — 4 groups. All additive precision; no structural defect. Artifact-gate screen: clear.

## G1 (R2) — Third reachable ModePlan trigger — MUST FIX
Add DerivePhaseFromChildren's all-open branch (derive.go:387) to R1.3's complete-row plan leg and R2.3's trigger inventory: reached when the completed bead is a NON-lifecycle child (excluded from the lifecycle count, derive.go:348-352) of an epic whose lifecycle children are all open (claimable via `next`, which applies no issue-type filter). Drop/qualify "reachable only via the empty-lifecycle-subset fallback" and "both actually-reachable triggers"; note this trigger (unlike the swallowed-error one) SURVIVES the R2.3 fix — the plan-time message rewording and AC-2's complete→plan fixture must account for two surviving triggers (:366 and :387).

## G2 (R5) — Extend the ADR-0023 editorial strike to line :108 — MUST FIX
The round-4 re-conditioning makes original edge-case line :108 (mixed closed/open/blocked → plan) as superseded as :107: spec-095's refined rule + derive.go:379-383 derive implement for any lifecycle set with ≥1 closed and none in_progress, blocked included. Extend the strike/supersede-annotation everywhere it is scoped (problem evidence/OQ-4 resolution, Documentation Surfaces, ADR Touchpoints, Scope, R1.3 table-delivery obligation). The delivered agreement table may not coexist with a live ADR row claiming mixed closed/blocked derives plan.

## G3 (R6) — AC-16: done-variant and same-value replay legs — MUST FIX
Add (a) a stored mindspec_phase=done variant: replay leaves done unchanged with honest no-op/refusal output; (b) a stored mindspec_phase=plan same-value replay leg: output identifies no mutation (and does not claim a backfill). Keep the existing implement, review, and empty-phase legs.

## G4 (R3 advisory, folded since the respin is forced) — AC leg for the advanceState clause — MUST FIX
R2.3's new clause ("a failed post-close child read renders the indeterminate form, never a fabricated plan") has no dedicated AC: AC-4's failure branch drives completeTail's context resolution, not advanceState's swallowed FetchChildren error, and Result.NextMode's indeterminate form is asserted nowhere. Add one leg (AC-4 extension or AC-4b) driving a failing FetchChildren via seam through complete's result path, asserting summary+tail render the indeterminate form and no phase sync writes a fabricated value.
