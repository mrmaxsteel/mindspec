<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead2`
- **Round**: 2
- **Branch**: `bead/mindspec-ta30.2`
- **Reviewed commit**: `542992b49743c3a31f28e65b7354231da154e0f8`

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

**Bead implementation review** (gate: bead). Under review: commit `798841b9` on `bead/mindspec-ta30.2` — spec 128 Bead 2: OQ-3 option-1 value semantics (plan approve writes `plan`; `next`'s first LIFECYCLE claim advances stored→implement as a new locked writer; release wired as the sole production caller of `phase.SanctionedDerivedSync`, fence tightened zero→exactly-one) + `EnsurePhaseSemanticsV2` legacy reconciliation (new semantics_v2.go: one-shot/idempotent/lock-protected marker `mindspec_phase_semantics: "v2"`; lock-free `LegacyPhaseSignature` reads; legacy NOTICE replacing the obsolete warning) + the direction-conditional AC-5(iv) recovery (forward keeps `repair phase`, backward emits `bd show`) + the #248 dissolution fixtures (both routes + trigger-2 plan-over-plan). Judge against spec AC-5 legs (i), (iv), (viii) and plan §Bead 2. Staged prompt: `reviews/prep/bead2_impl_prompt.md`.

**Implementer-flagged deviations (assess each explicitly — A-F)**:
- **A**: EnsurePhaseSemanticsV2 hardcodes phase's internal mergeMetadataFn instead of an injected per-caller seam — the injected design made an unrelated verb-specific assertion (TestApproveImpl_LifecycleClassificationErrorRefusesPreMutation's "implPhaseMetadataFn not called on preflight refusal") observe this writer; matches EnsureMigrated/SanctionedDerivedSync convention. Judge the design AND whether hardcoding weakens any fault-injection coverage.
- **B**: 13 ApprovePlan tests + 5 harness scenarios gained `phase.SetMergeMetadataForTest` no-op stubs (the v2 marker-write fires until the marker exists — new unconditional first-run write).
- **C**: repair fixtures gained pre-set v2 markers to keep exactly-one-merge-call assertions alive (bead-1's forward-compat pattern).
- **D**: TestApprovePlan_CommitsBeforeWritingImplementPhase retargeted implement→plan (the bead's own OQ-3 change — verify it still pins commit-before-write ordering).
- **E**: lint ratchet — repair.go's new Sprintf idrender-wrapped (no bump); derive.go raw-epicID allowlist 2→4 with rationale (backward branch fully wrapped). Verify honest recount, not gate-weakening.
- **F**: complete.go:1366 untouched beyond the EnsurePhaseSemanticsV2 call added above it.

**Schema note (GH #254 countermeasure)**: the header's schema citation is DANGLING; the header's field list is authoritative. `verdict` exactly APPROVE / REQUEST_CHANGES / REJECT (never PASS); `hard_block` top-level only.

## Files in Scope

`git show 798841b9` in the bead worktree. Key: internal/approve/plan.go:462, cmd/mindspec/next.go (advancePhaseOnClaim), cmd/mindspec/release.go (releaseSyncPhase→SanctionedDerivedSync), internal/phase/{semantics_v2.go,derive.go}, sanctioned_sync_test.go tightening, five new test files.

## Acceptance Criteria (verbatim from spec.md)

AC-5 legs (i), (iv), (viii) — read in place; (ii)/(iii)/(v)/(vi)/(vii) were bead 1's, already merged.

## Prior-Round Asks

Round 1: 4 APPROVE (R1/R2/R3/R4) / 2 RC (R5 ×1; R6 ×6 with hard_block) — the hard-block core was cross-family convergent (R6's fresh-derive-under-lock ask == approve-slot R3's advisory G1). 12 items in 6 groups (`consolidated-round-1.md`; R1/R3's APPROVE-verdict findings were folded in per the every-slot rule). Fixed as ONE commit `542992b4` — the commit under review. Round-2 duty: R5/R6 classify their round-1 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE re-deriving (R6: the hard block clears only on your own re-derivation — the interleave tests and never-regress property are yours to re-prove); approvers regression-check the diff in their lens territory.

Round-1 fix resolutions:
1. **G1**: advancePhaseOnClaim and plan.go step-4b both derive fresh state INSIDE their lock critical sections (fresh Cache children re-fetch; implement written only while fresh derivation says implement; step-4b no-ops when stored is ahead); two REAL-lock interleave tests (next-vs-release via actual SanctionedDerivedSync, next-vs-complete) + lock-timeout/merge-failure tests; failure contract = warning-only failure always leaves a FORWARD-ONLY disagreement with a convergent `repair phase` recovery.
2. **G2**: LegacyPhaseSignature requires exactly-open normalized statuses (blocked/custom/empty non-match) + table/fixture cases; canonical non-lifecycle claimed-shape fixture + read-failure pair.
3. **G3**: marker discrimination is PRESENCE-ONLY (future "v3"/malformed/empty never overwritten or rewritten); four forward-compat fixtures.
4. **G4**: fail-safe contract documented at the discard site + a test proving transient lookup failure neither crashes nor bypasses preflight's refusal.
5. **G5**: dead-end recoveries fixed — next's legs → `repair phase`; release's self-heal leg → `repair phase`, sanctioned-sync leg → `bd show` inspection line with --force-backward named in the body (not the greppable recovery line), matching the dirty-tree-refusal convention. Judge this split treatment.
6. **G6**: lexical-fence residual recorded in completeSyncPhase's doc comment, routed to beads 4/5. Plus one incidental lint-ratchet allowlist entry (release.go arg-hop; callee idrender-wraps) — verify honest.

## Lens

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | **Author-of-record** — plan §Bead 2 steps delivered and only them; deviations A-F each explicitly acceptable-or-not; the OQ-3 value semantics EXACTLY as the plan pinned (plan at plan-approve; first LIFECYCLE claim advances; non-lifecycle advances nothing; release syncs derived). |
| R2 | Claude | **Codebase-pin** — new tests real and green; the #248 route fixtures actually construct the two routes; the fence tightening genuinely exactly-one; deviation E's ratchet recount honest; B/C's stubs don't mask real writes. |
| R3 | Claude | **Contract stability** — the warning-dissolution claim: under option 1, does stored==derived now hold at EVERY point of the plan-approve→first-claim window (trace it); is the legacy-notice/self-heal loop airtight (notice → repair → reconciled, never a loop); does the trigger-2 plan-over-plan case keep the sanction sentence true; is deviation A's hardcoded seam the right long-term shape for bead 4's agreement-table fixtures? |
| R4 | Codex | **Empirical prober** — build/test in the bead worktree (-short always); run the AC-5(viii) twin test and the #248 route fixtures; hand-probe: fresh epic → plan approve → state show (warning ABSENT?), claim → stored advanced, release → stored back via sanctioned sync (warning ABSENT?); legacy epic fixture → notice text → repair self-heals. |
| R5 | Codex | **Schema/correctness** — LegacyPhaseSignature's exact predicate vs the plan's legacy signature (stored implement, ≥1 lifecycle child, all open, none closed/in_progress); the marker write's idempotence/one-shot claim; the direction-conditional recovery's ADR-0035 shape; advancePhaseOnClaim's lifecycle-vs-non-lifecycle classification vs derive.go's own exclusion switch (same rule? — a mismatch would desync stored from derived). |
| R6 | Codex | **Next-bead integration + adversary** — beads 4/5 consume the agreement rows and result messages this bead's semantics create: any state where stored and derived STILL disagree under option 1 (hunt: concurrent claims, release of non-sole bead, adopt/impl paths, failed EnsurePhaseSemanticsV2 mid-flight)? Attack the six semantics_v2 fixtures for a missing shape; error-discarding hazard sweep on every new read (the bead-1 hard-block class). |

## Reviewer conduct (binding)

- Build/test in the bead worktree ONLY (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.2); leave it byte-identical (transient phase.lock must not remain); fixture lock roots in /tmp; no commits/lifecycle verbs/tracked-file edits; absolute /tmp scratch.
- Verify, never confirm; re-derive; contradiction licensed (BRIEF errors are findings).
- Write your verdict JSON to the exact path in your prompt BEFORE terminating.
