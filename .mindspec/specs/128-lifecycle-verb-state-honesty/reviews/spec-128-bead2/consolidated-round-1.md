# spec-128-bead2 — consolidated concrete_changes_required, round 1

Tally: 4 APPROVE (R1, R2, R3, R4) / 2 REQUEST_CHANGES (R5 ×1, R6 ×6 with hard_block). Per the every-slot rule, R1's 2 and R3's 3 APPROVE-verdict findings are folded in. 7 RC asks + 5 advisory findings dedupe to 6 groups. Artifact-gate screen: clear.

## G1 — Phase advances must derive fresh evidence INSIDE the lock and never regress fresher state (R6 ×3 + R3-G1, cross-family convergence; the hard-block core) — MUST FIX
1. `advancePhaseOnClaim` writes implement unconditionally-forward: derive fresh child state inside the SAME WithPhaseWriteLock critical section as the write; write implement only while current evidence still derives implement; NEVER overwrite fresher plan/review/done produced by release/complete. (This also dissolves R1's advisory R1-1 — the late-lifecycle-bead review→implement shape stops existing — so the sanction sentence stays absolute.)
2. Same class in plan.go step 4b: its lock fn ignores `stored` — a claim interleaving between preflight and step 4b lands plan over implement. One-line refuse/no-op-backward inside fn (R3-G1).
3. Deterministic REAL-lock interleave tests: delayed next vs release, delayed next vs complete — final stored == fresh child-derived phase; review/done never regressed to implement.
4. Failure semantics for the required first-claim advance: a warning-only failed phase write may NOT leave a successful `next` result with stored plan / derived implement — define the contract (fail the claim? retry? documented divergence with the notice?) and add lock-timeout + merge-failure tests over the real claim path.

## G2 (R5 + R6, convergent) — LegacyPhaseSignature strictness + fixture shapes — MUST FIX
1. The predicate requires every lifecycle child's NORMALIZED status to be exactly `open` (blocked/custom/empty/unknown = non-match), plus stored implement and ≥1 lifecycle child. Table cases for blocked and ≥1 unknown/custom status; an EnsurePhaseSemanticsV2 fixture proving those states get marker-only updates, never a mindspec_phase rewrite.
2. Replace the non-lifecycle fixture with the canonical claimed shape (stored plan, lifecycle tasks all open, non-lifecycle child in_progress): zero phase advance, marker persists, stored==derived. Add an EnsurePhaseSemanticsV2-failure case for that path so a legacy mismatch cannot be silently preserved by the early return.

## G3 (R6) — Marker forward-compatibility — MUST FIX
A present `mindspec_phase_semantics` marker of ANY value is already-discriminated (or unsupported values are explicitly rejected); never overwrite a future/non-v2 value with "v2", never run the legacy rewrite over it. Marker-present future-value and malformed-value fixtures.

## G4 (R6 + R1-R1-2, convergent) — FindEpicBySpecID error discard — MUST FIX
ApprovePlan's semantics-v2 preflight discards the error: propagate it, or document AND TEST the fail-safe contract proving that proceeding cannot skip required reconciliation.

## G5 (R3-G2, from an APPROVE verdict — real usability defect) — Recovery-command dead ends — MUST FIX or disposition explicitly
Both new best-effort lock-timeout recovery commands are dead ends (`mindspec next <bead>` cannot re-select an in_progress bead; `mindspec release <bead>` skips the sync on re-run), and the release-sync failure state has NO unforced copy-pastable fix (AC-5(viii) itself pins the unforced refusal). Either fix the commands to ones that actually converge, or record the explicit disposition routing this to beads 4/5's result-message scope with the beads named (silent dead-end hints are the #248 class this spec exists to kill — dispositioning must be loud).

## G6 (R3-G3, info) — Lexical-vs-semantic sanction fence
completeSyncPhase can land a backward-converging write in failure-residue states; the exactly-one fence is lexical. Record as an accepted residual with rationale in the code/commit (or tighten if cheap). Not gate-blocking; must not be silently dropped.
