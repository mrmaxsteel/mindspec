<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead9`
- **Round**: 3
- **Branch**: `bead/mindspec-ta30.9`
- **Reviewed commit**: `d59622b3e8c7300aed9e9198f6166fa4f710e527`

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

**Bead implementation review** (gate: bead). Under review: commit `2691189d` on `bead/mindspec-ta30.9` — spec 128 Bead 9: bd stderr passthrough with safe rendering (AC-11). Delivered: internal/bead/bdcli.go's `wrapBDError` folds exec.ExitError.Stderr into tracedOutput's error text (every RunBD caller benefits), %w-preserving so errors.As chains (BeadExists, IsUnsupportedFlagError) are unaffected — regression-tested; excerpt cap `bdStderrExcerptLineCap = 20` with truncation marker; internal/next/beads.go's `renderBDError` per-line termsafe.Escape at the RENDER boundary (the DirtyTreeFailure house pattern) at QueryReady/QueryReadyForEpic's wrapping sites. Tests at BOTH layers: bdcli_test.go (surface/cap/empty-passthrough/errors.As-survival) + new beads_test.go (PATH-shimmed fake-bd through the REAL RunBD chain, both AC-11 legs, renderBDError unit). Four files only — the parallel bead-7 surface (next.go, guard.go) untouched.

Suites: -short 48/48; -race on both packages ok; no-short cmd/mindspec `ok 445.2s`. Deviations: none.

**PARALLEL-TRACK NOTE**: bead 7 is in flight on the same spec branch fork point (bed348c2), touching internal/next/guard.go + cmd/mindspec/next.go. Verify NO file overlap and no export-contract interaction (bead 9 changes beads.go's error wrapping only).

**Schema note (GH #254)**: header citation DANGLING; header field list authoritative; verdict exactly APPROVE / REQUEST_CHANGES / REJECT.

## Files in Scope

`git show 2691189d` in the bead worktree. internal/bead/{bdcli.go,bdcli_test.go}, internal/next/{beads.go,beads_test.go}.

## Acceptance Criteria (verbatim from spec.md)

AC-11 — read in place (the raw-passthrough asymmetry: content leg passes, rendering leg fails on unescaped hostile bytes).

## Prior-Round Asks

Round 1: 4A/2RC → wrapper fix `44690044`. Round 2: **5A / 1RC (R3 ×2: the ready_gate metadata-seam legs ARE armed and in-bounds; red-on-revert render pins missing)** → fixed as ONE commit `d59622b3`: RecordReadinessOverride/RollbackReadinessOverride wrap in bdRenderedError (no added prefix — the raw-error contract preserved); two red-on-revert pins added (TestFetchBeadByID_AC11RenderingLeg via PATH-shimmed control-bearing fake bd; TestGateReadiness_RenderingLegControlBytesEscaped) that FAIL if the wrapper is dropped from their call sites. All three gate lines green (no-short 543.8s). next.go's two legs remain the recorded follow-up (bd bead filed).

Round-3 duty (minimal confirm): R3 classifies its two asks re-deriving (run the two red-on-revert pins; verify the wrap sites); others verify diff confinement (3 files, internal/next only) + confirm standing verdicts; re-verdict at d59622b3.

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.9); leave byte-identical; fixture roots + probes in /tmp, CLEANED; no commits/lifecycle verbs/tracked-file edits; verify-never-confirm; contradiction licensed; write your verdict JSON to the exact path in your prompt BEFORE terminating.
