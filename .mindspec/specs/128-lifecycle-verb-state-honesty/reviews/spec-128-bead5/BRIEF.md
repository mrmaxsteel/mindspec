<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead5`
- **Round**: 5
- **Branch**: `bead/mindspec-ta30.5`
- **Reviewed commit**: `0f7f2216ba36abe35604e35be63fc57448e75e47`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-5.json` beside this BRIEF
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

**Bead implementation review** (gate: bead). Under review: commit `68ede240` on `bead/mindspec-ta30.5` — spec 128 Bead 5: verb output honesty (R2, AC-3/AC-4/AC-4b/AC-14) + the WRITTEN SCOPE EXTENSION (internal/instruct/run.go's two windows; derive.go de-swallow prerequisite). Delivered: emitInstruct distinguishes derivation failure/ambiguity (new `templates/indeterminate.md`) from idle and consults bead-3's draft discovery before finalizing idle (AC-3); ResolveContextFromDirWithCache's three fabricating fallbacks now set ADDITIVE `Context.DerivationFailed/DerivationError` fields (Phase fallback VALUES unchanged for existing callers) + a `beadsDirMissing` guard (uninitialized workspace = honest idle, not spurious indeterminate); run.go's `handleIdleOrDraft`/`handleIndeterminate` replace the old any-failure→handleNoState; AC-4b's decided seam (completeFetchChildrenFn, advanceState error return, NextModeIndeterminate, step-6.5 skip-on-indeterminate); **ModePlan reworded to "No lifecycle work has started yet. Mode: plan (spec: %s)"**; completeTail routes indeterminate via RenderIndeterminate directly (never a fresh re-derivation); AC-14's total cobra classification over 73 commands with 7-of-9 verbs driven end-to-end.

**Implementer disclosures (assess A-D)**:
- **A**: `beadsDirMissing` guard — out-of-prompt addition, argued required by R1.4/R2.3's own honesty mandate (found via 3 failing tests; a real would-be regression). 
- **B**: root.go's newExecutor seamified to newExecutorFn (outside declared list) so AC-14 could drive four verbs' real RunE hermetically.
- **C**: TestRun_ModeResolutionFailure renamed + assertion flipped (the old one pinned the pre-bead-5 dishonest fallback R2.3 closes).
- **D (scope limitation, documented in a test)**: AC-14's `next` and `impl adopt` rows are classification-present but NOT execution-driven (no reachable bd seams outside their packages; both rows red-if-removed; TestVerbOutput_NextAndImplAdopt_ScopeLimitation documents why). Judge whether this satisfies AC-14's letter or needs an in-bead seam.
- Pin-inventory claim: bead-4's cell pins never went red (no cell MODE changed; prose-only syncs + one unrelated template census bump 14→15). Verify.

**Schema note (GH #254)**: header citation DANGLING; header field list authoritative; verdict exactly APPROVE / REQUEST_CHANGES / REJECT.

## Files in Scope

`git show 68ede240` in the bead worktree. Key: cmd/mindspec/{instruct_tail.go,complete.go,root.go,verb_output_test.go,cwdsafety_test.go}, internal/instruct/{run.go,templates/indeterminate.md}, internal/phase/derive.go, internal/complete/*, prose syncs in agreement_table_test.go + lifecycle-modes.md, ratchet_template_test.go census.

## Acceptance Criteria (verbatim from spec.md)

AC-3, AC-4 (+AC-4b), AC-14 — read in place; R2.1's nine-verb result-first contract; the scope extension's two windows.

## Prior-Round Asks

Rounds 1-4: 1A/5RC → 5A/1RC+HB → 5A/1RC+HB → 3A/3RC+2HB (R3+R5+R6 convergent: the ambiguity nil-route + indeterminate fail-closed on spec branches). Round-4 fix as ONE commit `0f7f2216` — the commit under review: ReadState's tracking bool widened to `ctx.DerivationFailed || ctx.Phase == ""` (instruct_tail.go:30's exact predicate — both honesty surfaces now consistent); the final nil check unreachable post-init BY CONSTRUCTION; runPreCommit explicitly Blocks ModeIndeterminate on spec/* branches with an honest message (escape hatch preserved); four new tests incl. real Run() drives crossed with session variants; invariant comments rewritten. Verified: internal/hook full green; -short suite green.

Round-5 duty (confirm): R3/R5/R6 classify your round-4 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE re-deriving (R5/R6: your hard blocks clear only on your own re-derivation — R5 re-run the two-active-specs probe; R6 re-run the spec-branch indeterminate probe); R1/R2/R4 spot-verify the small diff; re-verdict at 0f7f2216.

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.5); leave byte-identical; fixture roots + probe clones in /tmp and DELETE probe clones before finishing (disk tight); no commits/lifecycle verbs/tracked-file edits; verify-never-confirm; contradiction licensed; write your verdict JSON to the exact path in your prompt BEFORE terminating.
