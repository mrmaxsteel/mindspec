<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead7`
- **Round**: 3
- **Branch**: `bead/mindspec-ta30.7`
- **Reviewed commit**: `3018a6591f54d3a37568db9c46dad0ddc0d32d90`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-3.json` beside this BRIEF
(the panel artifact schema — see `.mindspec/domains/workflow/interfaces.md`
§ Panel Artifact Schema). Required and optional top-level fields:

- `verdict` (required): one of `APPROVE`, `REQUEST_CHANGES`, `REJECT`.
- `hard_block` (optional boolean, a top-level sibling of `verdict` — never
  a per-finding field): set `true` only for an evidence-bearing halt that
  no vote count may override.
- `reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`,
  `findings`: reviewer-authored context, read presentation-only by
  `mindspec panel tally` — not consumed by the gate decision.
<!-- /mindspec:panel-header -->

## Summary

**Bead implementation review** (gate: bead). Under review: commit `694b9d46` on `bead/mindspec-ta30.7` — spec 128 Bead 7: proportional claim guard (R5/R5.1, AC-8) + spec-approve safe replay (R3.6, AC-16) + the mandatory ADR-0025 amendment + **MAX'S OPERATOR DIRECTIVE** (binding): the claim prints a one-line informational stderr notice naming ignored unrelated dirt while PROCEEDING. Delivered: internal/next/guard.go gains entangleableClaimPaths (the R5.1 derived enumeration — EMPTY today: session.json gitignored, issues.jsonl artifact-exempt, worktree dir postdates the guard), CheckDirtyTreeProportional (splits blockingDirt/ignoredDirt; CheckDirtyTree/Detail UNTOUCHED so release.go keeps all-blocks semantics), IgnoredDirtNotice ("note: claiming despite unrelated dirty path(s) this claim does not touch (not blocked — ADR-0025 R5.1): %s", per-path termsafe.Escape); next.go Step 1 uses it — blockingDirt refuses via the byte-identical DirtyTreeFailure contract, ignoredDirt notices+proceeds; internal/approve/spec.go extracts backfillExistingEpicPhase (absent-only backfill under bead-1's WithPhaseWriteLock; backfill vs same-value-no-op vs later-phase-untouched distinguished in output; specMergeMetadataFn seam added); ADR-0025 amendment scope-fenced to next's worktree-creating claim path. Tests: AC-8(i) three real bd+git e2e variants mirroring this run's actual #179 hits (untracked file / untracked dir tree / modified tracked file — proceed with notice); AC-8(ii) enumeration pin (real-claim git-status diff vs EntangleableClaimPaths()); AC-8(iii) seam-injected refusal leg (block machinery re-arms); AC-16 five legs (empty backfill, same-value plan, implement/review/done untouched, red-on-revert); next_dirty_test.go rewritten (old test pinned the REMOVED block); M6 regression kept.

Both suites green (-short 48/48; no-short cmd/mindspec `ok 817.049s` with -timeout 30m — first attempt hit go test's DEFAULT 10m ceiling on an untouched pre-existing test, a pre-existing suite-growth finding now filed as a bd bead, not a regression). Deviations: none reported.

**Schema note (GH #254)**: header citation DANGLING; header field list authoritative; verdict exactly APPROVE / REQUEST_CHANGES / REJECT.

## Files in Scope

`git show 694b9d46` in the bead worktree. Key: internal/next/{guard.go,entangle_test.go}, cmd/mindspec/{next.go,next_claim_guard_test.go,next_dirty_test.go}, internal/approve/{spec.go,spec_replay_test.go}, .mindspec/adr/ADR-0025.

## Acceptance Criteria (verbatim from spec.md)

AC-8 (three legs) + AC-16 (five legs) — read in place in spec.md §R5/R5.1/R3.6. Plus the operator directive (notice while proceeding) as an additional binding requirement.

## Prior-Round Asks

Round 1: 2A/4RC+HB (enumeration factually wrong) → fix 953e9abb. Round 2: 3A/3RC (R1 ×2, R3 ×3, R6 ×4 with HB — convergent: lexical samePath's subdir/symlink evasion; custom worktree_root recording divergence; stale ADR/comments). Round-2 fix as ONE commit `3018a659` — the commit under review.

Resolutions: **G1** containment replaces equality — toplevelOf(cwd) (rev-parse --show-toplevel + EvalSymlinks) and containedRel resolving the DIRECTORY chain only; real-claim subdir + symlink-alias regressions refuse byte-identically. **G2** closed BY CONSTRUCTION: the recording class trusts EventsPath/ManifestPath's own resolution and gates purely on containment (correct wherever SpecDir sends it); pinned with R3's exact custom-root decoy fixture. **G3** ADR-0025 amendment rewritten to the shipped mechanism. **G4** header comments honest. **DISCLOSED BONUS BUG (judge it)**: the first containment cut resolved the LEAF symlink too — a dirty symlinked .gitignore resolved to its target's name, missed porcelain's tracked-name match, and wrongly PROCEEDED (caught by the round-1 symlinked-refusal e2e going red); fixed via resolveContainingDirs (dirs resolved, leaf basename kept) + TestContainedRel_SymlinkLeafKeepsOwnTrackedName.

Both suites verifiably green from full logs (-short 48/48; no-short 1054.3s; -race ×4 packages).

Round-3 duty: R6's hard block clears ONLY on its own re-derivation (re-probe subdir + symlink-alias + the leaf-symlink case at 3018a659); R3 re-derives the custom-root fixture + judges the by-construction G2 closure; R1 classifies its ADR/comment asks; R2/R4/R5 verify confinement + confirm. Re-verdict at 3018a659.

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.7); leave byte-identical; fixture roots + probes in /tmp, CLEANED before finishing; no commits/lifecycle verbs/tracked-file edits; NEVER claim/approve in this repo — fixtures build their own; verify-never-confirm; contradiction licensed; write your verdict JSON to the exact path in your prompt BEFORE terminating.
