<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-plan-approve`
- **Round**: 8
- **Branch**: `spec/128-lifecycle-verb-state-honesty`
- **Reviewed commit**: `2e73a0f3be92c3e787132a425aa52b3d354cbdc0`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-8.json` beside this BRIEF
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

This is a **plan-document gate** (plan_approve). The artifact under review is `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` at the reviewed commit, judged AGAINST the 6/6-approved spec.md beside it (approved at b7deb948 after a six-round panel; its Open Questions section chartered four design decisions to THIS plan). The plan resolves: OQ-1 = (a) gates-before-commit reorder with worktree-inclusive gate measurement (priced ADR-0041 §1 reversal; ADR-0037/internal-panel untouched; the spec's (c)-only AC legs vacate); OQ-2 = on-disk draft carve-out (narrow ADR-0023 draft-discovery amendment; R1.4's either/or resolved to non-zero refusal for state show); OQ-3 = option 1 (plan approve writes `plan`, next's first LIFECYCLE claim advances stored to implement as a NEW locked writer, release takes the sanctioned derived-sync leg — the #248 warning dissolves on both fixture routes; the round-6 trigger-2 advisory discharges by dissolution since a non-lifecycle claim advances nothing); OQ-5 = displayed row 1 in bd-ready priority order (dependency edges own plan order). 10 beads; AC-5's eight legs deliberately split beads 1/2 at the legs' stated grain (flagged in-plan); `mindspec validate plan` exits 0 with three decomposition WARNs answered in an in-plan Decomposition Note.

**Schema note (GH #254 countermeasure)**: the header's schema citation is DANGLING — the field list in the header is authoritative. `verdict` exactly APPROVE / REQUEST_CHANGES / REJECT (never PASS); `hard_block` top-level only. Malformed verdicts silently lose your vote.

## Files in Scope

- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` (under review) against `spec.md` (the contract)
- Grounding: the plan's fresh code claims — complete.go:634 (beadHead late-bound branch name), complete.go:46 (completeMergeMetadataFn pattern), spec.go:73-74 (spec approve idempotence), internal/journal lock idiom, derive.go:568-579, plan.go:675-693 (work_chunks wiring) — plus the ADRs it amends (ADR-0041 §1, ADR-0023, ADR-0025) and the spec's settled facts + AC-1..AC-16.

## Acceptance Criteria (verbatim from spec.md)

The spec's ACs (AC-1..AC-16, incl. lettered legs) are the contract this plan must deliver; the plan's bead sections map them. Judge: does every AC land in a bead whose scope can actually deliver it, and do the OQ resolutions keep every non-vacated AC satisfiable?

## Prior-Round Asks

Rounds 1-6: 1A→2A→2A→4A→4A→3A, fixed `747bf9b3`/`f72803bb`/`6cdac6e5`/`27294e84`/`b98d06c2`/`c15860ab`. Round 7: **3A (R1/R2/R3 zero asks) / 3RC (codex trio, three-way convergent on the elision-evidence hole + R5's ADR-0030 seam item)** → 2 groups (`consolidated-round-7.md`), fixed as ONE commit `2e73a0f3` — the commit under review. Round-8 duty: R4/R5/R6 classify their round-7 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE re-deriving; zero-ask approvers regression-check in their lens territory.

Round-7 fix-author resolutions:

1. **G1 — contract (b) chosen: AT-LEAST-ONCE with per-operation idempotent replay**; the round-6 exactly-once claim is WITHDRAWN in place as unprovable (rationale for rejecting (a): empty trailer commits would stamp a contentless C onto EVERY standard completion — clean subagent-committed trees are the norm). Per-operation proofs enumerated: R's mixed reset is a state-function of (tip, worktree); S's export is ADR-0025's deterministic projection (forward-correct); dirt classification is a pure read; merge rides 127's convergence machinery. Not-skipped by routing; not-looped by convergence. Elided transition graph complete: FIRST publication of a run uses CREATE semantics whatever it points at (absent→C, or absent→D parenting on A when C elides, same refusal); both-elided = no-marker route (kill re-enters as an ordinary run, tip still at reviewed SHA). New fixtures (xviii)/(xix)/(xx); count label "all twenty"; ADR-0041 §3 aligned.
2. **G2 — ADR-0030**: seven NAMED Executor seam methods in bead 6 step 2 (SnapshotWorktreeTree, MaterializeStageCommit, PublishStageMarker (CAS; empty expectedOld = CREATE), ReadStageMarker, ClearStageMarker, AdvanceBranchExpectedOld, ResetIndexToHead); MindspecExecutor thin-wraps the gitutil primitives; MockExecutor stubbed; boundary sentence: internal/complete calls only these seams, no new gitutil import. Chunk 6 already carried the executor files (checked, not assumed).
## Lens

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | **Spec-contract fidelity** — every spec requirement and AC mapped to a bead that can deliver it; the OQ resolutions consistent with the spec's settled facts and carve-outs; the vacated (c)-only AC legs and dead option-2 branches correctly identified; the trigger-2 advisory genuinely discharged under option 1. |
| R2 | Claude | **Repo-grounding of the plan's design claims** — re-derive: complete.go:634 late-bound beadHead and whether the worktree-inclusive gate variant is actually buildable as described; spec.go:73-74 idempotence claim; the 9-writer wrap plan vs the real write sites; the next-first-lifecycle-claim new-writer claim; release's derived-sync leg vs release.go's current shape. |
| R3 | Claude | **Bead decomposition + sequencing** — are the 10 beads independently reviewable with clean seams; deps correct and minimal (chain depth 4 justified?); AC-5's split at the right grain; any bead secretly oversized or trivially mergeable; work_chunks frontmatter consistent with the bead sections. |
| R4 | Codex | **Empirical prober** — run `mindspec validate plan 128-lifecycle-verb-state-honesty` and report; verify the work_chunks frontmatter parses and its positional mapping matches the bead sections; re-derive the OQ-3 dissolution claim mechanically (which derive.go branches fire in the plan-approve window under option 1; does the warning condition at derive.go:179-193 still trigger?). |
| R5 | Codex | **ADR-conformance of the resolutions** — is OQ-1(a)'s ADR-0041 §1 amendment drafted as an honest reversal (both load-bearing directions addressed)? Is the ADR-0023 draft carve-out narrow (no general spec-artifact authority leak)? ADR-0025 amendment for bead 7? Is leaving ADR-0037 untouched actually right under (a)? |
| R6 | Codex | **Adversary / failure modes** — attack the OQ resolutions: what breaks under (a)-reorder (gates measuring uncommitted work — trust boundary? partial-failure states?); option-1's stored-phase migration story for EXISTING epics mid-lifecycle at upgrade time; the lock's failure modes (stale lock file, crashed holder); anything the 10-bead cut leaves homeless. |

## Reviewer conduct (binding)

- READ-ONLY repo; lifecycle verbs limited to `mindspec validate plan` / `state show` / `bd show` / `gh issue view`; absolute /tmp scratch only.
- Verify, never confirm; re-derive inherited claims; you are licensed to contradict this BRIEF and report its errors as findings.
- Write your verdict JSON to the exact path in your prompt BEFORE terminating.
