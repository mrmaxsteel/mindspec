# spec-128-plan-approve — consolidated concrete_changes_required, round 7

Tally: 3 APPROVE (R1, R2, R3 — zero asks) / 3 REQUEST_CHANGES (R4 ×2, R5 ×2, R6 ×2). 6 asks in 2 groups. Artifact-gate screen: clear.

## G1 — Elision destroys the evidence that R/S ran: pick the truthful contract (R4 + R5 + R6, full three-way convergence with the SAME two admissible resolutions) — MUST FIX
Eliding empty C/D commits elides the durable stage evidence, making "R/S run exactly once" unprovable after a crash (post-no-op-S kill: tip==marker-C is indistinguishable from S-never-ran). Choose ONE, in both the protocol and the ADR-0041 §3 draft:
(a) retain trailer-bearing EMPTY C/D commits (evidence over elision), or
(b) explicitly replace exactly-once with an at-least-once, idempotent-replay contract — and PROVE each replayed operation idempotent and convergence-preserving (R's index refresh and S's export/dirt-check are plausibly idempotent by construction; the proof obligation must be stated per operation).
Whichever: complete the transition graph for the elided cases — race-safe absent→D marker publication when C is elided (with its failed-publication refusal), the no-marker route when BOTH elide — and add the fixtures: kill after R with C elided pre-D; kill after a no-op S/D-elision pre-merge; both-elided; keep (xvii)'s ordinary-D coverage. Re-entry behavior falsifiable in every leg.

## G2 (R5) — ADR-0030 executor-seam conformance for bead 6 — MUST FIX
Route every new git read/mutation used by internal/complete through an explicitly named Executor seam (granular or high-level; MindspecExecutor wraps gitutil, MockExecutor updated) — or move the git-owned state machine into the execution layer. No direct internal/complete → internal/gitutil dependency and no unplanned boundary crossing; name the seam methods in bead 6's steps and chunk file list.
