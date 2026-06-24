# 106-bead3 — Round 1 consolidated changes

**Tally:** 5 APPROVE (R1, R2, R3, R4-sub, R5-sub) · 1 REQUEST_CHANGES (R6-sub, 2 hard_blocks) · 0 REJECT.
Codex usage-limited → R4/R5/R6 were Claude-subs. The mover's core (transactional two-commit, crash-resume, rollback, link-check, precondition, SEC-5 guards) is verified correct + tested + live-tree-safe. The gap is downstream-integration: the mover only does the symmetric flatten, but the plan assigned the asymmetric moves here.

## BLOCKER (R6 — 2 hard_blocks; plan-vs-impl gap that blocks Bead 5)

1. **Add the dogfood-eviction move + asymmetric depth-change rewrite rules.** The plan's Bead 3 step 4 assigns "dogfood eviction depth changes" to this rewriter. Implement the move `.mindspec/docs/{user,installation,research}/` → top-level `project-docs/{user,installation,research}/` (a depth change OUT of `.mindspec/`), plus the rewrite rules so links into/within those docs (and repo-root refs to them) resolve — no 404 under the doctor link-check.
2. **Add the review-co-location move + rules.** The plan's Bead 5 steps 1-2 say "run the mover's review-co-location step." Implement a move generator that, for each root `review/<slug>/`, reads its `panel.json` `spec` field (fallback: infer spec from the slug prefix, e.g. `099-…`→spec 099; skip/record dirs that can't be routed) and moves it to `<spec-dir>/reviews/<slug>/` (spec-dir resolved per the active layout), removing the root `review/` tree. Rewrite any links accordingly.
3. **Make the mover drivable for the full move set.** Either extend `DefaultFlattenPlan`/`DefaultFlattenRules`/`DefaultRootDocs` to include the dogfood + review moves (so Bead 5 just runs `migrate layout`), OR add an injection API (`WithPlan`/`WithRules`/`WithRootDocs` or a builder) the CLI + Bead 5 can use. Prefer extending the defaults so spec-106's flatten is the mover's out-of-box behavior, since Bead 5 "runs the mover" rather than re-specifying it.
4. **Extend `writeLineage`** to record ALL move groups (symmetric + dogfood + review), not just the 5 symmetric groups (R6 minor / R5 minor).
5. Add tests for the new moves: dogfood + review moves apply, their depth-change links resolve (no 404), rollback after a dogfood/review move restores cleanly, lineage records all groups, review routing reads panel.json `spec`.

## MAJOR (R5 — pre-Bead-5-reuse safety)

6. **Scope the rollback `git clean -fd`.** It is currently repo-wide and data-loss-safe ONLY because `RequireCleanTree:true` refuses untracked user files at start. `Run()`/`Abort()` are public and Bead 5 reuses `Run()` on the LIVE tree (which now also writes `project-docs/`). Scope the clean to the mover's touched roots (`.mindspec`, `project-docs`, `review`) OR re-assert clean-tree inside `rollback()` before cleaning, so a future live reuse cannot delete user-untracked files. Document that Bead 5's publishing run must arm `Published` (R5 minor — not a Bead-3 code change, just a note/guard).

## MINOR

7. Fix the self-corrected doc-comment artifact in `internal/layout/rewrite.go` (`applyRewritesInTree`: "Returns the repo-… no, returns the absolute paths…"). (R2)
8. `stageRootRewrite` boundary not separately crash-tested — immaterial (idempotent), optional. (R4-sub)

## Endorsed (do NOT change)
Transactional two-commit protocol, crash-resume at all 6 boundaries, hard-reset rollback, the finite-pattern rewriter preserving `../../adr/…`, the precondition `block ⟺ unmerged AND pre-flatten`, SEC-5 guards, the Executor/gitutil additive surface, deviations A/B/C/D. Live-tree NOT flattened (correct — that's Bead 5).
