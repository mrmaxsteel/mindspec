# spec-128-plan-approve — consolidated concrete_changes_required, round 3

Tally: 2 APPROVE (R2, R4 — zero asks) / 4 REQUEST_CHANGES (R1 ×4, R3 ×2, R5 ×3, R6 ×4). 13 asks in 7 groups. Artifact-gate screen: clear.

## G1 — Post-update-ref recovery must ACTUALLY converge (R1 + R5 + R6, 3-slot convergence) — MUST FIX
After update-ref advances the tip to C but reset/index-refresh fails or is interrupted, the promised "idempotent re-run" meets the panel stale-SHA gate (C ≠ reviewed_head_sha) — the hinted command does not work as written (violates goal (b)'s own principle and R4.3's plain-re-run contract). Fix, all faces:
1. Specify RESUME DETECTION that runs before the panel gate: recognize the already-materialized state (tip commit's tree == T, or equivalent durable marker) and safely continue index refresh + artifact sync + merge — or honestly re-scope this leg to the spec-127 post-gate machinery as the plan already does for other post-gate failures (then the recovery line must say that, not promise a plain re-run).
2. Add the update-ref→index-refresh boundary to the ADR-0041 §3 mutation-point classification; pin with a red-on-revert kill fixture: advance ref to C, interrupt before reset, invoke the documented recovery, assert convergence without rollback, re-paneling, phantom staged deletions, or loss of interval edits.

## G2 — Pin the v2-marker state in the AC-5 fixtures; classify the legacy notice (R1 + R6 convergent) — MUST FIX
EnsurePhaseSemanticsV2 at repair's entry would CONSUME the legacy condition before the guard runs, making AC-5(ii)/(viii) unsatisfiable on naively-constructed fixtures. State explicitly: AC-5(ii), (iv), (v), (viii) guard/warning fixtures construct marker-PRESENT states; the marker-ABSENT stored-implement/all-open state is the LEGACY fixture exercising notice + self-heal. And classify the legacy notice as a REPLACEMENT notice, NOT a retained form of the phase-mismatch warning, so AC-5(iv)'s test targets the retained warning only.

## G3 (R6) — Windows lock: mechanically atomic ownership — MUST FIX
Replace the compare-and-delete + post-create-verify dance with a primitive that is atomic by construction: e.g. a retained Windows handle opened WITHOUT FILE_SHARE_DELETE for the whole critical section, or an OS-native named mutex / LockFileEx design; owner-only release. Add a barrier-controlled test for the exact late-delete interleave (both contenders finish stale-identity re-read; winner creates+verifies; loser attempts delete after winner entered → only one inside). Disposition a crash between O_EXCL creation and PID/timestamp publication (empty/partial stale file must not block forever).

## G4 (R3 + R6 convergent) — work_chunks sweep + tightening-clause placement — MUST FIX (mechanical, recurring class)
1. chunk 2: add cmd/mindspec/repair.go (bead 2 step 7's new EnsurePhaseSemanticsV2 call site). This omission class has recurred in all three fix commits — do a FULL final sweep reconciling every chunk's key_file_paths against its bead section's named edit sites, and say the sweep was done.
2. Bead 2 step 3: carry the fence-tightening duty (zero-callers-allowlist → exactly-one) in the bead that executes it, not only in bead 1's step.

## G5 (R1) — Sanction-count classification sentence — MUST FIX (one sentence)
Line ~356's "exactly one sanctioned backward writer" vs EnsurePhaseSemanticsV2's unforced implement→plan relabel: add the classification — the v2 reconciliation is a one-shot semantics TRANSLATION under a durable marker, outside the sanction taxonomy — or recount.

## G6 (R5) — ADR-0041 draft: snapshot side-effect honesty — MUST FIX (small)
A refused content gate may leave unreachable loose objects; replace/qualify the repository-byte-identical language for this exception (preserving the precise guarantee: bead ref/tip, tracker state, real index, worktree unchanged) — or specify an isolated object-store mechanism making the stronger claim true.

## G7 (R1, trivial) — Bead 2 AC parenthetical
Align "retains its safe final recovery line" with the recorded direction-conditional disposition (backward disagreements emit the bd show inspection line).
