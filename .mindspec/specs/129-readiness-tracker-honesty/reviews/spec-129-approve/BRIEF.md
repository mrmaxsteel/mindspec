<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-129-approve`
- **Round**: 7
- **Branch**: `spec/129-readiness-tracker-honesty`
- **Reviewed commit**: `e7388f71d9325b95ed463169df2a9fc3043234f1`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-7.json` beside this BRIEF
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

This is a **spec-document gate** (spec_approve). The artifact under review is `.mindspec/specs/129-readiness-tracker-honesty/spec.md` (and ADR-0045 plus the ADR-0040/0044 back-pointers) at the reviewed commit in the header. Spec 129 covers GH #255 (spec-scoped `next` selection honesty), bd row caps, spec-linkage reporting, MF-3 false negatives (#239; #232 dispositioned on the #233 mechanism), MF-2 token symmetry, signal-scoped `--allow-not-ready` with evidence digests and ready-check read-back, and the ADR-0045 rider. #234, #244, #250, `merge_strategy` and lineage unification are deferred to a plain-numbered follow-on spec. The S0 rule table and #233's review-to-landing binding go to spec 130, and #251's remaining sub-shapes are bounded. The spec's own Acceptance Criteria section is authoritative for the AC count. Lettered spec IDs are a known defect (bead mindspec-wtjr).

**Verdict schema:** `verdict` must be exactly `APPROVE`, `REQUEST_CHANGES` or `REJECT`. `hard_block` is an optional top-level boolean. Every required change goes in `concrete_changes_required`.

## Files in scope

- The spec and ADR above (the artifacts).
- Grounding, read-only: `internal/lifecycle/reattest.go`, `internal/phase/derive.go` and `phase.FindEpicForBead`, every `FindLandedMerge` consumer (`grep -rn FindLandedMerge --include='*.go' internal cmd`), `plugins/mindspec/skills/ms-bead-impl/SKILL.md`, `cmd/mindspec/next.go`, `internal/next/*.go`, `internal/validate/readiness/*.go`, `internal/lifecycle/landed.go`, `internal/gitutil/gitops.go` (FirstParentMerges), `internal/panel/*.go` (Scan), `internal/approve/plan.go`, `internal/approve/spec.go`, `internal/lifecycle/stale_open.go`, `internal/lifecycle/orphans.go`, `internal/bead/*.go`, `cmd/mindspec/bead_ready.go`; ADRs 0041, 0035, 0030, 0025, 0040, 0044; `bd ready --help`, `bd list --help`.
- GitHub issues: 255, 239, 232. Deferral boundary: 234, 244, 250, 251 (partial), 233.

## Acceptance criteria

The spec's ACs are in spec.md § Acceptance Criteria. Read them in place. For each one ask: is it runnable, is it red-on-revert, and does it pin the requirement it cites?

## Lenses

| Slot | Model | Lens |
|:--|:--|:--|
| F1 | Fable | **Scope and split honesty.** Is 129 vs the plain-numbered follow-on spec cut at a real boundary? Is the deliverable count honest? Is anything in #255/#239/#232 silently dropped? Is the S0 deferral justified? |
| F2 | Fable | **AC falsifiability and internal consistency.** Is every AC runnable and red-on-revert? Pairwise contradiction scan across requirements, ACs, Non-Goals and Open Questions. Do the Open Questions pre-decide answers the requirements then contradict? |
| F3 | Fable | **Adversary / completeness.** What is missing? Empty, boundary and concurrency states; a spec bead that is BOTH outside the epic AND capped; the closed-dependency cases; requirements that quietly need out-of-scope machinery. |
| O1 | Opus | **Repo-grounding audit.** Verify every behavioural and symbol claim in Background and the requirements against the tree at the reviewed commit. Flag anything the code does not support. |
| O2 | Opus | **ADR conformance.** ADR-0041 (is R4.3's amendment correctly priced and placed?), ADR-0035 (recovery lines), ADR-0030 (executor/enforcement boundaries for any new git or bd calls), ADR-0025, and ADR-0045 itself (does its text match the repo, and does it conflict with ADR-0040/0044?). |
| O3 | Opus | **Design soundness.** Lineage- and plan-mapped selection; the finder-only fallback scan and reattest acceptance; the panel registration set lookup; the bare-`--allow-not-ready` refusal. Is each the simplest mechanism that fixes the reproduced cause? |
| G1 | codex | **Empirical prober.** Reproduce the spec's live claims at the reviewed commit, using fake bd on PATH and throwaway real-git repos under your scratch dir: the parent-filter drop, the stderr cap notice, the first-parent miss, the lex-first panel contradiction, the MF-2 `R9(a)` failure, and `bd ready --metadata-field`. Run `mindspec validate spec 129-readiness-tracker-honesty` and report its output. |
| G2 | codex | **Trust boundary.** Which new signals are forgeable, and by whom (`metadata.spec_id` is writable by any bd user; merge subjects; an evidence file; a panel registration)? Does any change turn a fail-closed check fail-open? |
| G3 | codex | **Adversary on MF-3 and overrides.** Can a stale, abandoned or forged panel registration or merge now PASS MF-3 that should fail? Can a scoped override be widened? Attack R4 and R6 specifically. |

## Reviewer conduct (binding)

- READ-ONLY on the repo. Do not edit, commit, stash, checkout or run lifecycle verbs other than `mindspec validate spec`, `mindspec state show`, `bd show`, `bd ready`/`bd list` (reads) and `gh issue view`. Do mutation probes only in your own detached checkout (`git worktree add --detach <your scratch>/wt <the reviewed commit in the header>`) and remove it afterwards.
- All scratch goes to the ABSOLUTE scratch path in your prompt.
- Verify, never confirm. Re-derive every claim. You are licensed to contradict this BRIEF and the orchestrator.
- MANDATORY: include a finding titled "Errors in this BRIEF" (say "none found" if so).
- Findings are never out-voted. Every ask you make will be fixed or refuted with evidence.
- Write your verdict JSON to the exact path in your prompt BEFORE you finish.

## Verdict location (overrides the header)

Do not write beside this BRIEF: the repo is read-only for you. Write your verdict to the absolute scratch path given in your prompt. The orchestrator copies it here as `<slot>-round-<N>.json`.

## Round 2 (reviewed commit cb272d50)

Round 1 was 9/9 REQUEST_CHANGES with 95 asks. The fix commit is cb272d50; review `git show cb272d50` and the resulting files. The consolidation is beside this BRIEF in `consolidated-round-1.md`. Its coverage table maps every round-1 ask id to a disposition: 91 fixed, 2 refuted (registerLandedPanel exists in fixtures.go; ready rows carry parent and metadata when set), 0 deferred to this spec.

Main design changes:
- (a) Selection narrowed to plan beads (`<E>.<N>` with parent == E). Every other linked bead is a named near-miss, and lineage-spec ≠ claim-spec refuses before mutation (R1.8). The widen option is rejected in OQ-9.
- (b) First-parent stays primary. A direction-pinned reachable-landing fallback runs only on zero candidates, and only for MF-3 and reattest. Other FindLandedMerge consumers are unchanged (a table plus an AC). It is priced as an ADR-0041 "Amendment (Spec 129)".
- (c) The panel registration set lookup now covers reattest.
- (d) The override marker is schema v2, stores the evidence sha and path but no text, never honours legacy or malformed markers, and allows no flagless re-claim. The ms-bead-impl override prose is removed.
- (e) #250 moves to 129b; the count is stated as 7–10 beads.

ROUND-2 DUTY:
1. Classify EACH of your own round-1 asks as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE, re-deriving against cb272d50, not taking the coverage table's word. Put this in a finding titled "Round-1 ask status".
2. Review the NEW design in your lens for regressions. A narrowed claim must still fix the reported bug (#255, #239). A fallback must not reintroduce a false PASS.
3. Check both refutations if they touch your lens.

## Round 3 (reviewed commit 4c46cf64)

Round 2 was 2 APPROVE (F1, O2) / 7 REQUEST_CHANGES, with 36 asks. Fix commit 4c46cf64; the coverage table is in `consolidated-round-2.md` (36/36 fixed). The round-2 claim that "TODO(skill) placeholders remain in this BRIEF" was REFUTED: `grep -n -i -E 'todo|<!--' BRIEF.md` shows only the two machine header markers. Re-read files fresh from disk and do not rely on memory of earlier rounds.

Main changes:
- Selection = every bd-ready `<E>.<N>` bead whose unchanged FindEpicForBead lineage is E, whether or not a parent edge exists. This makes #255's reporter shape claimable; there is a scratch-bd proof in the consolidation.
- R1.8: every claim must be a plan bead of its lineage spec T, and the dispatch spec is T.
- `Merge bead/X` is now stated as UNDIRECTED. On the fallback path only a surviving tip or a landed binding confirms; a panel-only match gives NoEvidence. A fallback ambiguity is resolved only by a corroborated single group. O3's false PASS is reproduced and closed.
- The override fingerprint covers the complete failure set of each signal (Signal.Failures plus a shared FailureFingerprint).
- Near-misses block only for N1, N2 and lifecycle-type N3.
- A contradictory binding vetoes automatic identification.

ROUND-3 DUTY:
- Slots that requested changes: classify each of your round-2 asks as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE (finding "Round-2 ask status").
- Approving slots: regression-check `git diff cb272d50 4c46cf64` in your lens.
- All slots: attack the new selection rule (can it select a bead of another spec, or a non-claimable bead?), the fallback confirmation rule, and the fingerprint.

## Round 4 (reviewed commit a593792a)

Round 3 was 3 APPROVE (F1, O1, O2) and 6 REQUEST_CHANGES, with 28 asks. The fix commit is a593792a (spec.md only); the coverage table is in `consolidated-round-3.md` (28/28 fixed). RE-READ FROM DISK.

Orchestrator rulings applied:
1. **The fallback never auto-confirms.** FindLandedMergeReachable only finds and names a candidate (a typed off-first-parent result), and MF-3 fails, naming the merge and `mindspec reattest <id>`.
   - reattest accepts a fallback candidate only when there is exactly one owned candidate, no registered reviewed_head_sha lies outside the tip's history (a new refusal state), and the audit is always recorded.
   - Later automatic reads accept a fallback candidate only through a reattest-audited binding.
   - #239 is restated as loud, named, and recoverable by one audited command.
2. **Membership** = plan-form id + FindEpicForBead lineage (no parent shortcut) + N mapped by MF-1's own plan.md mapping. Unmapped plan-form follow-ups are informational class N4.
3. **No lettered spec IDs:** "129b" becomes the follow-on spec (next plain number).
4. **Override recovery** names all currently failing signals.

The reviser scoped Ruling 1's unreached-review refusal to fallback candidates only. Applied globally, it would deadlock a primary-path reattest after a rebased-away round (ADR-0041 §2(i)). The orchestrator accepts that scoping, so attack it if you disagree.

ROUND-4 DUTY:
- Slots that requested changes: classify your round-3 asks as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE (finding "Round-3 ask status").
- Approving slots: regression-check `git diff 4c46cf64 a593792a`.
- All slots: attack the reattest-audited read-back. Can an agent forge a "reattest audit" on a binding, since bd metadata is agent-writable? Also attack the new N4 class and MF-1-mapped membership.

## Round 5 (reviewed commit 7404845f)

Round 4 was 3 APPROVE (F1, O1, O3) and 6 REQUEST_CHANGES, with 27 asks. The fix commit is 7404845f (spec.md only); the coverage table is in `consolidated-round-4.md` (27/27 fixed). RE-READ FROM DISK. (Older round sections above mention "129b"; that name is retired, and the follow-on spec takes the next plain number.)

Main changes:
- (A) The audited tuple. When reattest accepts a fallback candidate, it writes `mindspec_landed_reattest_merge_sha` and `…_second_parent` beside `scanned_branch`. A read-back accepts only if three conditions hold:
  - the audit is VALID (shape-checked);
  - audit tuple == binding pair == the single scan-derived candidate;
  - every git/panel-derived check re-runs and passes: one candidate, the round rule, the surviving-tip contradiction, revert/net-effect at the CURRENT tip, and fail-closed reads.

  A stale audit names a different pair and grants nothing. The residual is stated: a forge reaches only what reattest itself would accept.
- (B) The round rule. An anchor is a registration whose reviewed SHA equals the second parent or is an ancestor of it. reattest refuses only when a non-ancestor-of-tip registration has a round greater than the highest anchor round, and it fails closed on ties, missing rounds or no anchor. The refusal names its exits. The reviser anchored on equal-OR-ancestor, not on equal only: in #233's shape the second parent is complete's tree-identical commit, so equal-only would fail closed on every normal bead. The orchestrator accepts that; attack it.
- (C) `readiness.PlanMapsBead`: Nth `## Bead` heading by position AND work_chunks id N, shared by selection and MF-1. A half-mapped N is N4.
- plan.md is read lazily (checkout, then spec/<S>); N2b is split; the N4 MF-1 override applies only on lineage E; the staged skill line is dated.

ROUND-5 DUTY:
- Slots that requested changes: classify your round-4 asks (finding "Round-4 ask status").
- Approving slots: regression-check `git diff a593792a 7404845f`.
- All slots: attack the audited tuple, the anchor/round rule (can a later unlanded round masquerade as an anchor's ancestor?) and PlanMapsBead.

## Round 6 (reviewed commit 8b3a5535)

Round 5 was 5 APPROVE (F1, F2, O1, O2, O3) and 4 REQUEST_CHANGES (F3, G1, G2, G3), with 21 asks. The fix commit is 8b3a5535 (spec.md only); the coverage table is in `consolidated-round-5.md` (21/21 fixed). RE-READ FROM DISK.

Main change (orchestrator Ruling 5, as implemented): **F2 never compares rounds.** It refuses if ANY visible registration for the bead, abandoned ones included, has a well-formed reviewed_head_sha outside the spec tip's history. Anchors grant nothing. The orchestrator verified the repo convention: 59 registrations on main, zero `-r*` directories, round bumped in place. So a rebased-away earlier round is overwritten and passes, and O3's shape refuses. The refusal names each outside-tip registration (directory, round, SHA) and the remaining exit (the dependent's `--allow-not-ready=MF-3 --evidence`).

Residual, stated exactly: only F1, F3 and F4 are git-derived. F2 reads untracked, agent-writable panel.json, so a forge that also deletes or rewrites a registration defeats F2 with no JSONL trace. This is pinned by AC-11 (ix-g) and carried into the ADR-0041 amendment (AC-18).

Also:
- q9ea is dropped from the fallback exits, and the corroboration string records provenance.
- The no-op requires a VALID audit.
- The lazy plan read covers near-miss classification.
- The spec/<S> plan blob is preferred, with a divergence notice.
- PlanMapsBead counts headings exactly as extractBeadSectionRaw does.
- The estimate is 9–12 beads, with an R6 split expected.
- New OQ-10 (plan-stage): should readiness read the same plan source as selection?

ROUND-6 DUTY:
- Slots that requested changes: classify your round-5 asks (finding "Round-5 ask status").
- Approving slots: regression-check `git diff 7404845f 8b3a5535`.
- All slots: does "any outside-tip registration refuses" ever refuse a NORMAL bead (one review round, complete-landed, or a fix round after the panel)? Does it still let #239 recover through reattest?

## Round 7, confirm (reviewed commit e7388f71)

Round 6 was 9/9 APPROVE (tally PASS) with 10 minor asks from approving slots (F1×1, F2×2, F3×4, O1×2, O2×1). They are fixed in e7388f71 (spec.md only), per `consolidated-round-6.md`:
- reviewed_head_sha value classes, applied the same way in R4.3 and F2: empty is skipped; a non-SHA placeholder is skipped and reported; a well-formed SHA is evaluated; option-like or revision-expression text fails closed.
- A gc'd SHA counts as outside-tip.
- Per-checkout visibility is stated.
- AC-11 (vii-a) covers amend-after-panel.
- A typed ErrPathNotAtRef plan-blob read.
- The repo counts are dropped.
- The residual wording is fixed.

DUTY (short):
- Slots with round-6 asks: confirm each one (finding "Round-6 ask status").
- All slots: regression-check `git diff 8b3a5535 e7388f71` in your lens. Approve unless the diff introduced a defect.
