<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead4`
- **Round**: 3
- **Branch**: `bead/mindspec-ta30.4`
- **Reviewed commit**: `8abb8e065ab2eacd505418c7ed0415fd22b73999`

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

**Bead implementation review** (gate: bead). Under review: commit `6f75235b` on `bead/mindspec-ta30.4` — spec 128 Bead 4: the per-verb agreement table delivered as `project-docs/user/guides/lifecycle-modes.md` (+ ADR-0023 reference), the row fixture suite `internal/phase/agreement_table_test.go` (734 lines — every row cross-checked against the shipped status-aware derivation so table and code cannot silently drift, AC-2), and the ADR-0023 supersede-annotations for the stale :107/:108 edge-case rows (bead-3's draft-discovery amendment paragraph untouched). Fixtures assert LIVE arms only (OQ-3 option 1 + release redesign leg are decided; option-2 arms dead). The commit trailer states NO third-surface (instruct-tail) assertion was deferred to bead 5 — all rows assert all their contracted surfaces. Judge against spec R1.3 (the row/carve-out contract) + AC-2 and plan §Bead 4. The bead-4 carry-forward file (reviews/prep/bead4_carryforward.md) was contract input: its four items must appear as guide deliverables (legacy-shapes subsection incl. the blocked-child shape; advancePhaseOnClaim counted as evidence-fresh FORWARD-by-intent; the preserve-invalid-stored shape; the sanction-sentence truth check).

**Schema note (GH #254)**: header citation DANGLING; header field list authoritative; verdict exactly APPROVE / REQUEST_CHANGES / REJECT.

## Files in Scope

`git show 6f75235b` in the bead worktree: project-docs/user/guides/lifecycle-modes.md (new, 125 lines), internal/phase/agreement_table_test.go (new, 734), .mindspec/adr/ADR-0023.md (strike annotations, +13/−3).

## Acceptance Criteria (verbatim from spec.md)

AC-2 (each agreement-table row has a fixture asserting the row's contracted surfaces against the shipped derivation; carve-outs per R1.3's text; editing table without fixture or vice versa is red). Read R1.3 + AC-2 in place.

## Prior-Round Asks

Round 1: 0/6+HB → 6 groups, fixed `f490fd26`. Round 2: **4A (HB cleared) / 2RC (R2+R5 convergent: stored-echo circularity in seven fixture rows; R5's row-1 contract point)** → 2 groups (`consolidated-round-2.md`), fixed as ONE commit `8abb8e06` — the commit under review: fixture inputs made independent of expected cells (production writers executed/asserted where feasible), the non-circular DerivePhaseFromChildren leg added per row, file-header claim corrected, and (tally ruling) the independent status-aware derivation assertion added for row 1 over the canonical draft fixture (no self-authorized carve-out). ORCHESTRATOR PRE-VERIFIED: suite green at HEAD; doc-cell edit goes RED. Round-3 duty (confirm): R2/R5 classify your round-2 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE re-deriving (R2: run your joint doc+record edit probe); approvers spot-check the diff in your lens territory; re-verdict.
## Lens

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | **Author-of-record** — plan §Bead 4 steps and only them; the four carry-forward items genuinely delivered as guide content (not paraphrased away); the sanction-sentence truth check correct; ADR strike annotations exactly :107/:108-by-content, bead-3 paragraph untouched. |
| R2 | Claude | **Codebase-pin** — the 734-line fixture suite: every R1.3 row present with the RIGHT carve-out shape (plan-approve three-surface; release two-surface + redesign-leg semantics; complete's implement/plan(two triggers)/review legs; terminal rows' two-layer done+idle); the anti-drift property real (edit a table row in a /tmp copy → red?); suite green. |
| R3 | Claude | **Contract stability** — the no-deferral claim: bead 5 will CHANGE tail selection (indeterminate form, spec-create tail) — do bead-4's tail-level assertions pin the CURRENT (pre-bead-5) tails in a way bead 5 must break, or the contracted POST-spec semantics? If the former, that is a sequencing defect to flag now. Guide text vs bead-2/3 delivered semantics (no contradiction with the legacy notice, draft discovery, claim-advance). |
| R4 | Codex | **Empirical prober** — run the agreement suite + the anti-drift red-on-revert probe; hand-walk two rows end-to-end in a scratch fixture (spec create → state show; release with a closed sibling → implement row); verify the guide renders accurate command output examples. |
| R5 | Codex | **Schema/correctness** — the ADR-0023 strike annotations (do the superseded rows now UNAMBIGUOUSLY point at :140/:141's governing rows; is bead-3's amendment paragraph byte-identical); the guide's row wording vs spec R1.3 verbatim contract (no drift in the carved rows). |
| R6 | Codex | **Adversary** — hunt a REACHABLE verb-success state the table omits (adopt paths, forced repair, release with mixed sibling states, EnsurePhaseSemanticsV2-triggering entries); attack the no-deferral claim (find a tail assertion that would pass today but contradict bead 5's contracted indeterminate/spec-tail semantics); marker-fixture discipline. |

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.4); leave byte-identical; fixture roots in /tmp; no commits/lifecycle verbs/tracked-file edits; verify-never-confirm; contradiction licensed; write your verdict JSON to the exact path in your prompt BEFORE terminating.
