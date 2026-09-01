# spec-128-plan-approve — consolidated concrete_changes_required, round 5

Tally: 4 APPROVE (R1, R2, R3, R4 — all zero asks) / 2 REQUEST_CHANGES (R5 ×3 precision, R6 ×2 substantive). 5 asks in 3 groups. Artifact-gate screen: clear.

## G1 (R6) — The two-ref transaction is NOT crash-atomic on the files backend: redesign as marker-as-write-ahead — MUST FIX
`update-ref --stdin` transactions give locked, all-or-nothing VISIBILITY semantics but a crash mid-commit on the default files backend can leave one ref installed. Replace the atomicity assumption with a crash-recoverable protocol. R6's feasible shape (adopt or equal): create C/D → durably publish the marker ENCODING the intended new tip AND expected predecessor (write-ahead) → advance the bead ref with expected-old. On re-entry, the legitimate partial states (marker=new ∧ tip=predecessor; marker=new ∧ tip=new) are distinguishable from a foreign tip, and the run finishes forward. Preserve the no-laundering boundary. Fixtures: kill/fault-injection DURING each A→C and C→D ref-install (every one-ref-installed ordering), plain re-run converges without re-panel, true foreign tip still stale-blocks; test the default files backend explicitly; document/test any git-version floor the mechanism depends on.

## G2 (R6) — S-stage index/dirt contract before merge — MUST FIX
After the artifact-only ref advance to D, refresh the real index to D (or an equivalent clean-index invariant), THEN check for genuine non-artifact worktree changes before any merge path that can force-remove the bead worktree. Interval edits must be preserved via a clean named refusal or an explicitly safe recovery — never both uncommitted and gone after terminal cleanup. Revise fixture (viii) and the post-S kill fixture to assert the executable outcome including no silent loss of an interval edit.

## G3 (R5 ×3) — Precision items — MUST FIX (mechanical)
1. ADR Fitness statement contradiction: §2 forward-only unchanged; §3 IS amended to classify the M→R→S→G→T boundary — align plan.md's Fitness sentence with bead 6's actual disposition.
2. Pin the stage discriminator for "continue from the first incomplete stage": how retry distinguishes C from D (marker/tip equality proves ownership; reviewed-A→C→D parent chain + preserved/landed merge state identify the stage) — carried into the ADR-0041 §3 draft and fixture expectations. (Under G1's write-ahead redesign, restate the discriminator in the write-ahead encoding's terms.)
3. Bead 6 step 7's count label: "all seven" → "all eleven".
