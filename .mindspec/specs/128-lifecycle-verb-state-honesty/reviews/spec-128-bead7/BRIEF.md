<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead7`
- **Round**: 2
- **Branch**: `bead/mindspec-ta30.7`
- **Reviewed commit**: `953e9abb265e03d531e6574aa470d07b348f2fc8`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-2.json` beside this BRIEF
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

Round 1: 2A/4RC + R6 hard block — the derived-empty enumeration was FACTUALLY WRONG (R3: recording-marker writes at next.go:373-377; R6: DispatchBead's .gitignore write). Fixed as ONE commit `953e9abb` (12 files, +1537/-183) — the commit under review.

Resolutions: **G1** ComputeEntangleablePaths(repoRoot, cwd, specID) — two real conditional classes (target spec's recording paths when recording.enabled; .gitignore when the entry is absent), BOTH gated on samePath(claimAnchorRoot, cwd) (a write class is entangleable only when its real target tree is the one the guard's porcelain observes — claimAnchorRoot replicates resolveAnchorRoot ADR-0030-safely); HasGitignoreEntry extracted from EnsureGitignoreEntry's own check (can't diverge); spec-target resolution moved ahead of the guard. **G2** two new e2e pin legs (RecordingEnabled; GitignoreEntryAbsent) — the pin can now observe both classes. **G3** -z parsing ONLY in CheckDirtyTreeProportional (old parser byte-identical for release/complete); renames check both sides; untracked-dir ancestors block; notice paths strconv.Quote'd (boundary-forging closed). **G4** presence-bit consumed; present-but-invalid untouched with distinct message + regression test. **G5** wording pinned at unit + real-binary levels.

Disclosed bonus fixes (judge them): anchor-mismatch (the samePath gate); StatusZWithStderr stream-separation (git stderr warnings were corrupting the -z stream — reproduced via symlinked .gitignore); two lint-ratchet table entries (mirroring sibling justifications); mindspecCmdName const (specgate literal constraint).

Round-2 duty: R6's hard block clears ONLY on its own re-derivation (re-probe the .gitignore attack surface at 953e9abb incl. your modified/deleted/renamed/symlinked cases — now real tests); R3 re-derives the recording finding (run the RecordingEnabled pin; verify the samePath gating logic yourself); R1/R5 classify your asks; R2/R4 verify confinement + confirm. Re-verdict at 953e9abb.

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.7); leave byte-identical; fixture roots + probes in /tmp, CLEANED before finishing; no commits/lifecycle verbs/tracked-file edits; NEVER claim/approve in this repo — fixtures build their own; verify-never-confirm; contradiction licensed; write your verdict JSON to the exact path in your prompt BEFORE terminating.
