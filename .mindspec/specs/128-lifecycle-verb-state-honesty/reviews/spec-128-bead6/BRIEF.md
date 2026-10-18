<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead6`
- **Round**: 5
- **Branch**: `bead/mindspec-ta30.6`
- **Reviewed commit**: `44e6cf2fa79b7d3d83cbdf31c5a03fdfe330981a`

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

**Bead implementation review** (gate: bead) — the spec's BIGGEST bead. Under review: commit `482bcd2f` on `bead/mindspec-ta30.6` — spec 128 Bead 6: the complete gate reorder (OQ-1(a)): tip-seeded prospective-snapshot protocol, the M/R/S/G/T write-ahead marker state machine (single-OID `refs/mindspec/materialized/<bead>`, Mindspec-Stage trailers, per-ref CAS), at-least-once idempotent replay with elision rules and the total re-entry discriminator, the S-exit dirt contract, tree-measured gate legs (ValidateDocsSnapshot etc. beside untouched committed-range functions — reconcile unchanged), eight Executor seam methods (7 named + ChangedFilesTree; internal/complete gains NO gitutil import), the ADR-0041 §1 REPLACEMENT + §3(i) classification (with a red-if-old-sentence-present pin), all TWENTY fixture legs (i)-(xx) as real-git fixtures, and this repo's own ratchets properly extended. Both suites green (-short 48/48; no-short cmd/mindspec 460.5s ok). Judge against the plan's §Bead 6 + OQ-1 protocol text (VERBATIM in `reviews/prep/bead6_impl_prompt.md`) and spec AC-6/AC-7(i) (AC-6's pathspec sentence + AC-7(ii)-(iv) are VACATED under the delivered (a)-reorder).

**Implementer-disclosed deviations (assess A-C explicitly — C is load-bearing)**:
- **A**: MaterializeStageCommit frozen at the literal 4-arg seam → the operator's --commit-msg no longer becomes the materialization commit's subject (fixed stage-named subjects instead). Disclosed audit-trail cosmetic loss — acceptable, or does the message deserve preserving (trailer? body)?
- **B**: D's tree via full re-snapshot (no ninth scoped-refresh seam) — safe ONLY because of C. Judge the coupling.
- **C (design correction forced by the legs)**: the plan's literal S-exit ordering (check AFTER the S commit) let a kill-window interval edit be silently swept into the artifact-sync commit by S's own add -A; legs (viii-a)/(ix) caught it empirically. The classification now runs BEFORE the D snapshot (defensive re-check after). This is the one place landed code differs from the plan's step ordering — judge whether it preserves the plan's INVARIANTS (interval edits preserved-or-refused, never silently committed; the S-exit contract's letter) and whether the ADR §3(i) text describes the delivered ordering.

**Schema note (GH #254)**: header citation DANGLING; header field list authoritative; verdict exactly APPROVE / REQUEST_CHANGES / REJECT.

## Files in Scope

`git show 482bcd2f` in the bead worktree. Key: internal/gitutil/materialize.go(+test), internal/executor/{executor,mindspec_executor,mock}.go(+seam test), internal/validate snapshot legs(+test), internal/complete/{complete,materialize}.go + bead6_fixture_legs_test.go (twenty legs), .mindspec/adr/ADR-0041(+8 pinning tests), ratchet extensions.

## Acceptance Criteria (verbatim from spec.md)

AC-6 (same-gate retry convergence; live legs only) + AC-7(i) (non-tracker commit still trips stale-SHA). The twenty plan fixture legs are the bead's own falsifiability surface.

## Prior-Round Asks

Rounds 1-4: 0/6+2HB → 5A → 5A → 5A (R2's full-radius verification each time). Round-4 fix as ONE commit `44e6cf2f`: gate.go:183 summary label reworded; panel_advisory.go's four twin present-tense narrations reworded; dead `residualDirtyPaths` DELETED (zero callers verified); full-radius grep classified — every survivor is a still-live other-verb primitive or marked history. Disclosed not-acted-on: CompleteBead's now-unreachable `msg != ""` auto-commit branch (dead branch in a shared sensitive executor fn) — flagged as a follow-up-bead candidate, deliberately NOT edited under this dispatch.

Round-5 duty (final confirm): R2 re-runs its grep at 44e6cf2f and classifies; others verify diff confinement + confirm standing verdicts; judge the CompleteBead follow-up disposition (agree it's out-of-bead?); re-verdict.

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.6); leave byte-identical; fixture roots + probes in /tmp, CLEANED before finishing; no commits/lifecycle verbs/tracked-file edits; verify-never-confirm; contradiction licensed; interim statuses on long runs; write your verdict JSON to the exact path in your prompt BEFORE terminating.
