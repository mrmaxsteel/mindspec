# 106-bead1 — Round 2 Review Panel

**Bead**: `mindspec-3d3i.1` — Bead 1 (resolvers + DetectLayout, CORE). **Branch**: `bead/mindspec-3d3i.1`.
**Commit**: `2f4aee3d18abdf3f35aa3b1919153c99cfec2799` (round-2 fix on top of round-1 `da7eea3e`).
**Bead worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-106-layout-flatten/.worktrees/worktree-mindspec-3d3i.1`
**Round-2 diff**: `git -C <bead-worktree> show 2f4aee3d` (3 files: workspace.go, workspace_test.go, core/interfaces.md).
**Round-1 verdicts**: `<dir>/{R1,R2,R3}-round-1.json`, `<dir>/codex-{R4,R5,R6}-round-1.json`.

## Round-1 result & what the fix claims
Round 1 = **5 APPROVE / 1 REQUEST_CHANGES**. Behavior-preservation, byte-identity, the cycle-free Bead-4 seam, and scope were all verified. Two items drove this fix:
- **(R5, hard_block) ADDRESSED?** `DetectLayout`'s mixed-layout recovery exception was mis-scoped — `migrationRecoveryActive` triggered on ANY `.mindspec/migrations/` dir, so COMPLETED runs (and the 2 stale ones in the repo) silently suppressed `ErrMixedLayout`. Fix: `migrationRunInProgress(runDir)` now reads `state.json` `stage` (Bead-3 runstate schema; terminal = `"applied"`) and the exception activates ONLY for a non-terminal stage. New tests: mixed + completed (`"applied"`) record → STILL `ErrMixedLayout`; mixed + state-less dir → STILL error; recovery subcase uses an explicit in-progress record.
- **(R6, non-blocking major) ADDRESSED?** Added exported `SpecsDir(root)`/`DomainsDir(root)` flat-aware enumerator roots (delegating to `resolveArtifact`), for Bead 2's tier-aware enumerators. Enumerators themselves NOT touched.

## Your job (round 2)
Re-review per your lens (same as round 1 — your round-1 verdict is at your `*-round-1.json`). Mark each round-1 concrete_changes_required ADDRESSED / PARTIAL / MISSED. The bar for APPROVE: the R5 recovery-scoping hard_block is genuinely closed (the in-progress predicate is correct — a completed/`"applied"`/stateless run does NOT suppress the mixed-layout error; an actually-in-progress run does), the SpecsDir/DomainsDir roots are correct + byte-identical on canonical/legacy, and no new issue. Still behavior-preserving (no moves). R4 should run the tests. Verify against real code.

**Verdict**: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/review/106-bead1/<your-slot>-round-2.json`: `reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array), `findings` (array of {severity, area, issue, status?, hard_block?}).
