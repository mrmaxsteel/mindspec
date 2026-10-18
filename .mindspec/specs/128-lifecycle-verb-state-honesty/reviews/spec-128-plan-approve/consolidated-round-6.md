# spec-128-plan-approve — consolidated concrete_changes_required, round 6

Tally: 3 APPROVE (R1, R2, R3 — zero asks) / 3 REQUEST_CHANGES (R4 ×1, R5 ×1, R6 ×3). 5 asks in 3 groups. Artifact-gate screen: clear.

## G1 — Stage discriminator must be TOTAL under empty tree diffs (R4 + R5 + R6, 3-slot convergence) — MUST FIX
The marker-vs-marker^ artifact-confined-diff rule is ambiguous when C has no tree delta (clean-tree completion) or D is a no-op sync (unchanged artifact). Encode stage kind explicitly and durably in the commit object the marker already covers — canonical commit metadata/message is the named admissible shape (e.g. a pinned trailer) — keeping the single-OID marker and first-parent predecessor property; verify stage kind from the encoding, not the diff. Fixtures: empty C, empty/no-op D, ordinary artifact-changing D — each classified correctly after interruption; R/S neither skipped nor looped; pin when empty stages are elided (if elision is chosen instead, the discriminator must still be total). Carry into the ADR-0041 §3 draft.

## G2 (R6) — Marker publication race safety — MUST FIX
Pin expected-old/CAS semantics for the marker's absent→C and C→D transitions (or an equivalent per-bead serialization — note bead 1's phase lock may be the natural instrument if scoped to cover this; whichever, name it); state the cleanup/refusal result for a failed publication or failed bead-ref advance; add a concurrent-publisher fixture (one invocation cannot overwrite another's recovery evidence).

## G3 (R6) — One authoritative S-resume route — MUST FIX
Every tip==marker-D state, including kill re-entry, must refresh the real index to D and run the genuine-dirt classifier BEFORE merge — remove the "continue at the merge" shortcut; align protocol paragraph, bead steps, ADR draft, fixtures. Split fixture (viii): interval-edit run REFUSES and preserves the worktree; a clean follow-up run converges to the landed merge.
