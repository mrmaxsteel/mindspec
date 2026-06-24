# 106-bead3 — Round 2 Review Panel

**Bead**: `mindspec-3d3i.3` — Bead 3 (mover). **Branch**: `bead/mindspec-3d3i.3`.
**Commit under review**: `99a6bd5a17171a9396104c41daad5797ab9a9af1` (round-2 fix on top of round-1 `52b32b6a`).
**Bead worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-106-layout-flatten/.worktrees/worktree-mindspec-3d3i.3`
**Round-2 diff** (what changed since round 1): `git -C <bead-worktree> show 99a6bd5a` (+636/-25, 11 files; mostly `internal/layout/mover.go` +309 / `mover_test.go` +211).
**Round-1 verdicts**: `<dir>/{R1,R2,R3}-round-1.json`, `<dir>/codex-{R4,R5,R6}-round-1.json`. **Consolidated asks**: `<dir>/consolidated-round-1.md`.

## Round-1 result & what the fix claims

Round 1 was **5 APPROVE / 1 REQUEST_CHANGES**. Core mover (transactional two-commit, crash-resume, rollback, link-check, precondition, SEC-5) was verified correct. The dissent + one major drove this fix:
- **(R6, 2 hard_blocks) ADDRESSED?** The mover now does the FULL spec-106 move set: `DefaultFlattenPlan` extended with dogfood eviction (`.mindspec/docs/{user,installation,research}` → `project-docs/{...}`) + matching rewrite rules; a content-aware `reviewGroups()` review-co-location step (routes each root `review/<slug>/` by `panel.json` `spec`, slug-prefix fallback, skip+record un-routable, removes root `review/` when fully migrated, targets flat `.mindspec/specs/<id>/reviews/<slug>/`); `project-docs` added to the doctor link-check roots; `WithPlan`/`WithRules`/`WithRootDocs` injection. `writeLineage` records all landed groups + `Skipped`.
- **(R5, major) ADDRESSED?** `rollback()` now uses a scoped `CleanForcePaths(workdir, touchedRoots())` instead of repo-wide `git clean -fd`; new `Executor.CleanForcePaths`. Comment added that Bead 5's publishing run must arm `Published`.
- Minors: doc-comment fix; `stageRootRewrite` crash subtest.

## Your job (round 2)

Re-review per your lens (same as round 1 — your round-1 verdict is at your `*-round-1.json`). For each round-1 concrete_changes_required you raised, mark **ADDRESSED / PARTIAL / MISSED**. Then assess the new ~636 lines for NEW correctness/scope/contract issues. The bar for APPROVE: the round-1 dissent (R6 integration hard_blocks; R5 rollback major) is genuinely closed, the mover is now fully drivable by Bead 5 for all three move types via `Run()` with no downstream rework, and the new code introduces no new blocker. Verify against the real code; R4 should run the tests.

**Verdict**: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/review/106-bead3/<your-slot>-round-2.json` with keys:
`reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array; empty if APPROVE), `findings` (array of {severity, area, issue, status?, hard_block?}) where `status` is ADDRESSED/PARTIAL/MISSED/NEW.
