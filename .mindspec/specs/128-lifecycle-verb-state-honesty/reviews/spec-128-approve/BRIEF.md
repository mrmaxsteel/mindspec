<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-approve`
- **Round**: 6
- **Branch**: `spec/128-lifecycle-verb-state-honesty`
- **Reviewed commit**: `b7deb948eb69ca9426147fc5fa419de894a1dfd8`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-6.json` beside this BRIEF
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

This is a **spec-document gate** (spec_approve), not an implementation review: the artifact under review is `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` at the reviewed commit above. Spec 128 is the second half of the spec-127 split (charter: bd bead `mindspec-aytt` — read it via `bd show mindspec-aytt`). It covers GH #220/#245 (verbs and `state show` report honest mode; success output never indistinguishable from a no-op), #248 (no self-worsening recovery hints; `repair phase` backward-move guard), #186 real fix (gate-refused `complete` retry converges instead of wedging on its own tracker commit), #179 (proportional dirty-tree claim guard), #187+#146 remainder (deterministic honest bead selection, bd stderr passthrough, explicit strikes of shipped items), #221 (frozen-contract guidance naming `mindspec impl adopt`). It explicitly defers #255/#239/#232/#234/#244/#250 to a spec-129 follow-on, panel-artifact issues to a separate spec, and carries five deliberately-posed-not-resolved Open Questions (OQ-1/2/3/5 deferred to plan stage per the charter; OQ-6 a completeness caveat addressed to YOU, the spec gate). The draft was grilled (self-answer mode) before this panel; the grill record is in the spec's Open Questions section.

**Schema note (GH #254 countermeasure)**: the header's citation of `.mindspec/domains/workflow/interfaces.md § Panel Artifact Schema` is DANGLING — that file does not exist. The authoritative verdict contract is the field list in the header itself: `verdict` must be exactly one of `APPROVE` / `REQUEST_CHANGES` / `REJECT` (never `PASS`, never lowercase); `hard_block` is a TOP-LEVEL optional boolean, never per-finding. A malformed `verdict` value silently loses your vote.

## Files in Scope

- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` (the document under review — the whole artifact)
- Read-only grounding surfaces you should consult to verify the spec's claims: `bd show mindspec-aytt`; `.mindspec/adr/ADR-0023.md`, `ADR-0041-gate-before-mutate.md`, `ADR-0037-panel-gate-enforced-contract.md`, `ADR-0025-jsonl-as-build-artifact.md`, `ADR-0035-agent-error-contract.md`, `ADR-0042-render-derivation-provenance.md`; `internal/resolve/resolve.go`, `internal/phase/derive.go`, `cmd/mindspec/state.go`, `cmd/mindspec/next.go`, `cmd/mindspec/repair.go`, `cmd/mindspec/spec.go`, `cmd/mindspec/complete.go`, `internal/complete/complete.go`, `internal/panel/gate.go`, `internal/next/{guard,select,beads}.go`, `internal/bead/bdcli.go`, `internal/approve/plan.go`, `internal/doctor/beads.go`, `internal/instruct/templates/*.md`, `cmd/mindspec/impl.go`, `project-docs/user/guides/merge-safety.md`
- GitHub issues (via `gh issue view N`): 220, 245, 248, 186, 179, 187, 146, 221; deferral boundary: 255, 239, 232, 234, 244, 250

## Acceptance Criteria (verbatim from spec.md)

The spec's 13 ACs are in `spec.md` § Acceptance Criteria (AC-1 … AC-13), reproduced there verbatim at the reviewed commit — read them in place; they are the falsifiability surface you are judging. Key gate question per AC: is it runnable, red-on-revert, and does it actually pin the requirement it cites?

## Prior-Round Asks

Rounds 1-4: 1A→2A→3A→4A, fixed at `8a13b898`/`9ac301ee`/`ad66efbc`/`3a321066`. Round 5: **3A (R1, R4 zero asks; R3 one advisory) / 3RC (R2, R5, R6 — one ask each, all additive precision)** → 4 groups (`consolidated-round-5.md`; R3's advisory folded as G4), fixed at `b7deb948` — the commit under review. Round-6 duty: R2/R5/R6 classify their round-5 ask ADDRESSED / PARTIAL / MISSED / NEW_ISSUE re-deriving; R3 verifies its folded advisory (AC-4b); zero-ask approvers regression-check the small diff in their own lens territory. Review `git show b7deb948` plus the resulting spec.md.

Round-5 fix-author resolutions:

1. **G1 (R2's third trigger)**: R1.3's plan leg now names TWO surviving triggers (:366 empty-lifecycle fallback AND :387 all-open via completing a non-lifecycle child — the exclusion switch derive.go:347-352 drops it from the count; `next` filters only epic-typed items, beads.go:101-103, so bug-typed children are claimable); AC-2's fixture covers both; R2.2's message clause lists the full inventory (two survivors + the swallowed-error one R2.3 removes).
2. **G2 (R5)**: the ADR-0023 strike now covers :107 AND :108 at all five scoped sites, with the explicit sentence that the delivered table may not coexist with a live ADR row claiming mixed closed/blocked derives plan.
3. **G3 (R6)**: AC-16 gains the done-variant leg and the same-value (already-`plan`) no-op leg; implement/review/empty-backfill legs kept.
4. **G4 (R3's folded advisory)**: new AC-4b drives a failing post-close child read via a named child-read seam (today's direct call complete.go:1434) through complete's real result path — summary+tail render the indeterminate form, no fabricated `plan`, no phase sync writing a value derived from the failed read (completeMergeMetadataFn at complete.go:1366).

## Lens

Document-review lenses (this is a spec gate; no implementation exists yet):

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | **Charter fidelity + split integrity** — does the spec faithfully inherit the `mindspec-aytt` charter: all six settled facts carried without relitigation, deferrals honored, the OQ-1/2/3/5 questions genuinely posed-not-resolved, nothing chartered silently dropped? Is the 129-deferral boundary (esp. R6.3 vs #255) honest and precise? |
| R2 | Claude | **Repo-grounding audit** — verify the spec's file:line and behavioral claims against the tree at the reviewed commit: the #220 mechanism chain (resolve.go:57-67, state.go:80-97, derive.go:568-579, instruct_tail.go), the #186 ordering chain (complete.go:750/:710-715/:815, gate.go:280-286), the R6 residues (next.go:403-445, :178-186, beads.go:38/54, bdcli.go:443-455), the shipped-item strikes (select.go:42-98, plan.go:675-693, doctor/beads.go:282-317/:394-400, release.go). Flag any claim the tree does not support. |
| R3 | Claude | **AC falsifiability + internal consistency** — every AC runnable and red-on-revert as written? Pairwise contradiction scan across R1–R7, ACs, Non-Goals, and the OQ deferrals (e.g. can R1.3's table row set, R3's fixture, and OQ-3's open resolution all coexist? Do AC-6/AC-7 jointly pin the exemption narrowly?). Are the scope fences (R3.4, R3.5, R6.3's #255 boundary) falsifiable or weasel-worded? |
| R4 | Codex | **Empirical prober** — reproduce the spec's live-evidence claims by hand at the reviewed commit: `mindspec state show` from the spec worktree and from a main checkout; the emit-only `--pick` claim (read `runEmitOnly`); the `containsSpecID` filter; the plan.go:455 write; `grep -rn adopt internal/instruct/`. Report any claim that does not reproduce. Run `mindspec validate spec 128-lifecycle-verb-state-honesty` from the worktree and report its output. |
| R5 | Codex | **ADR-conformance** — read the six touchpoint ADRs in full and judge the spec's characterizations: is ADR-0041 §1 really reversed by OQ-1(a) and intact under (b)/(c)? Is the ADR-0037 §4 no-warn-downgrade constraint carried correctly into R4.2? Is the ADR-0025 conditional amendment priced correctly for R5.1 vs R5.3? Is the OQ-4 editorial-strike resolution (ADR-0023:107 vs :142) correct — verify both rows yourself. Any touchpoint mischaracterized or missing? |
| R6 | Codex | **Adversary / completeness** — what is MISSING: unprobed edge cases in R1–R7 (failure/concurrency/empty/boundary), reachable states the six-verb agreement table omits, OQ-6's warning that some of the nine inherited questions are unrecovered (can you identify a lifecycle-verb trust defect in the covered issue set the spec fails to require anything about?), scope leaks where a requirement quietly needs out-of-scope machinery. Attack the deferral rationales. |

## Reviewer conduct (binding)

- READ-ONLY on the repo: do not edit, commit, or run lifecycle verbs other than `mindspec validate spec` / `mindspec state show` / `bd show` / `gh issue view`. All scratch files go under `/tmp` ABSOLUTE paths (never relative paths, never inside any worktree).
- Verify, never confirm: your job is to find what is wrong or missing, not to endorse. Re-derive claims; do not inherit them (the dominant 127 defect class was claims asserted without re-derivation).
- You are explicitly licensed to contradict the orchestrator's framing and this BRIEF. If you find an error IN THIS BRIEF, report it as a finding.
- Write your verdict JSON to the exact path given in your prompt BEFORE terminating.
