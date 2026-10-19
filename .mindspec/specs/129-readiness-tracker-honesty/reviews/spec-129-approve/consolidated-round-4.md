# Spec 129 — spec gate, round 4: consolidation

- **Panel result:** reviewed commit `a593792a`. 3 APPROVE (F1, O1, O3), 6 REQUEST_CHANGES (F2, F3, O2, G1, G2, G3), no hard block, 27 asks.
- **Scope of reading:** every verdict's asks were read, including those from the APPROVE slots.
- **This commit:** `spec.md` only. ADR-0041's amendment lands with the R4/R6 beads, and AC-18 pins its text.

## Code facts I verified before applying the groups

- **Reattest audit keys.** `internal/lifecycle/reattest.go` defines the audit keys `…_actor`, `…_at`, `…_op`, `…_corroboration`, `…_prior_merge_sha`, `…_prior_second_parent` and `…_scanned_branch`. None of them records the ACCEPTED pair. The op constant is `"mindspec reattest"`. The executor's merge-time write and reattest's write both go through `bead.MergeMetadata`.
- **MF-1 structure.** `evaluateMF1` requires BOTH `sectionFound` AND `chunk != nil`. `extractBeadSectionRaw` counts `## Bead ` headings positionally (`count == n`) and ignores the number written in the heading.
- **Panel rounds.** `internal/panel/panel.go`: "On every re-panel, Round and ReviewedHeadSHA are bumped IN THE SAME WRITE". So a single registration carries only its latest round. Several rounds are visible only as separate registration dirs, and that is the shape F2 orders by `round`.
- **Ambiguity state.** `ReattestStateAmbiguous = "ambiguous-owned-second-parents"`.
- **Plan examples.** `mindspec-jfgj.4` is typed bug and titled `[073-llm-test-coverage] …`, and 073's plan has 5 `## Bead` sections, so it IS a mapped Bead 4 (O1 is right). 093's plan has 7, so `mindspec-cter.8` is out of range.
- **Primary-path no-op pin.** `TestReattest_HappyPathFleetState` contains the re-run no-op leg ("re-run must be a no-op (Wrote=false)"). This is the primary-path no-op test that stays green.

## Groups

**(A) Audited read-back** (O2, O3, F1, F3, G1, G2, G3).
- **Audited tuple.** On acceptance of a fallback candidate, reattest writes NEW keys `mindspec_landed_reattest_merge_sha` and `mindspec_landed_reattest_second_parent`, alongside `scanned_branch`, in the same `MergeMetadata` call as the binding. It writes them whenever the tuple is absent or differs, even when the binding values are unchanged.
- **Read-back conditions.** A read accepts the candidate only when all of these hold:
  - the audit is VALID: `op` equals the constant, `at` is RFC-3339, `actor` is non-empty, `scanned_branch` equals this spec branch, and both SHA keys are well-formed (this is O2's shape check);
  - the audit tuple equals the current binding pair and the single scan-derived candidate;
  - the fallback safety checks F1–F5 pass AGAIN at read time:
    - F1: exactly one candidate;
    - F2: the round-ordered unlanded-review check;
    - F3: the surviving-tip contradiction;
    - F4: the net-effect/revert test at the current tip;
    - F5: read errors fail closed.
- **Stale audits:** an executor rewrite under `MergeMetadata` leaves a stale audit in place, but that audit names a different pair, so it grants nothing. No executor change is needed.
- **Residual, stated honestly:** a metadata-only forge can reach only a candidate that reattest would itself accept. It skips the operator's vouch and nothing else. The spec calls this a new, bounded reach, not "the same class" (O3r4.1).
- **New AC-11 legs:**
  - (ix-f) a full forged audit;
  - (xii) a stale audit after an M1→M2 executor rewrite, then a fresh reattest refuses on multiple candidates;
  - (xiii) malformed audits;
  - (xiv) accepted, then reverted;
  - (xv) accepted, then a later review;
  - (xvi) accepted, then a second candidate;
  - (i-r) a same-tuple no-op.

**(B) Rebased-away round** (O2, F2).
- **Rule:** F2 refuses only when a registration whose `reviewed_head_sha` is NOT an ancestor of the tip has a `round` GREATER than the highest anchor round.
- **Anchor:** a registration whose SHA is the second parent or an ancestor of it. Ancestors are included so #233's tree-identical `complete` commit still anchors.
- **Fail closed** on a tie, a missing or non-positive round, or no anchor while a non-ancestor exists.
- **Pinned legs:** (i-a) an earlier rebased-away round passes; (vii) and (viii) a later unlanded round refuses; (x-t) a tie or a missing round refuses.
- **Exits named:** the dependent's `--allow-not-ready=MF-3 --evidence …`, and the q9ea attested restore. They appear in reattest's refusal and in MF-3 arm 1b.
- **Visibility line:** "no later unlanded review registration found in <roots>" (O3r4.2).

**(C) Plan mapping uses AND** (O1, O3, F3).
- **One helper,** `readiness.PlanMapsBead(planContent, n)`: at least N `## Bead ` headings counted positionally, AND a `work_chunks` id N. Selection and `evaluateMF1` both call it.
- A half-mapped N is N4, and the N4 message gives the truthful reason.
- **AC-1 rows:** H is half-mapped (a chunk but no section); the plan's headings are non-contiguous, so the mapping is pinned by count.
- **Examples corrected:** cter.8 stays as the out-of-range follow-up, and jfgj.4 becomes the counter-example (a bug-typed MAPPED bead).

**(D) N4 positional** (G1). The MF-1 override promise now covers only N4 beads whose lineage resolves to E. A lineage-none N4 refuses at selection with "no readiness override applies", and `GateReadiness` is never called. AC-3 has a leg for each.

**(E) Minors**
- **F2 items:**
  - lazy `plan.md` read; AC-4 (iv) runs with no `plan.md` anywhere and records zero reads;
  - AC-1 O and X titles pinned to `[S] …`;
  - AC-3 N4 legs;
  - AC-11 (x) topology written out in full, with no probe-file reference.
- **Staged line** (O2, F2): a `SKILL.md` marker `target=next-action (spec 133); remove-by=2027-06-30 (provisional)`. It is new staged content, so the marking duty applies now. AC-18 pins the marker.
- **F3 items:**
  - N2b is split by whether E2's spec number matches S's (reparent first) or not (inspect first, with a conditional reparent);
  - plan source: the invoking checkout, then `spec/<S>` via git when absent, then absent everywhere means all N4. `--emit-only` from main keeps working (AC-1 second fixture);
  - N2c/row X is flagged for real-bd confirmation. My own round-1 scratch `bd dep add <bead> <epic>` produced no dependency; the fallback fixture uses `parent: E2`.
- **F1 items:** R4.1 hint wording now says "plan-form", and there is a hint for a closed unmapped N4 follow-up dependency. F4 runs at read time, which answers F1r4.1.
- **O1 nits:**
  - Instead of renaming `ReattestStateAmbiguous`, I added a NEW `ReattestStateMultipleFallbackCandidates` (`"multiple-fallback-candidates"`). The existing value keeps its exact primary-path meaning, and nothing is renamed.
  - The primary-path no-op test is named: `TestReattest_HappyPathFleetState`.
- **Classification order** added (first match wins): N2a, N2c, N4 (any unmapped), N2b, N1, N3.

## Coverage table (27 asks)

| Ask | Group | Disposition |
|---|---|---|
| F1r4.1 | A | FIXED — F4 runs at read time; AC-11 (xiv) |
| F1r4.2 | E | FIXED — "plan-form" wording; N4 dependency hint |
| F2r4.1 | B | FIXED — option (b) round ordering; (i-a) passes, (vii)/(viii) refuse; exits named |
| F2r4.2 | E | FIXED — lazy read; plan source stated; AC-4 (iv) has no plan.md |
| F2r4.3 | E | FIXED — O and X titles contain the full spec ID |
| F2r4.4 | D | FIXED — AC-3 N4 legs with exact red-on-revert |
| F2r4.5 | E | FIXED — AC-11 (x) topology written out |
| F2r4.6 | E | FIXED — provisional remove-by date in the SKILL.md marker; AC-18 |
| F3r4.1 | A | FIXED — audited tuple; AC-11 (xii) |
| F3r4.2 | A | FIXED — F1–F5 re-run on read-back; AC-11 (ix-f) |
| F3r4.3 | C | FIXED — PlanMapsBead AND + positional; AC-1 H and non-contiguous headings |
| F3r4.4 | E | FIXED — N2b same-number vs other-number split; AC-2 |
| F3r4.5 | E | FIXED — checkout, then spec branch, then all-N4; AC-1 second and third fixtures |
| O1r4.1 | C | FIXED — one helper; half-mapped is N4 |
| O1r4.2 | C | FIXED — jfgj.4 is the counter-example; cter.8 is the example |
| O2r4.1 | B | FIXED — exits in the reattest refusal and MF-3 arm 1b; AC-11 "refuses" definition |
| O2r4.2 | A | FIXED — audited tuple must equal the binding |
| O2r4.3 | A | FIXED — audit validity contract; AC-11 (xiii) |
| O2r4.4 | E | FIXED — marker with target and date; AC-18 |
| O3r4.1 | A | FIXED — F1–F5 re-run; residual restated as a new bounded reach |
| O3r4.2 | B | FIXED — visibility line in the output |
| O3r4.3 | C | FIXED — AND |
| G1r4.1 | A | FIXED — audited tuple; AC-11 (xii) |
| G1r4.2 | D | FIXED — lineage-none N4 refuses; AC-3 |
| G2r4.1 | A | FIXED — tuple; AC-11 (xii), (i-r) |
| G3r4.1 | A | FIXED — read-back never bypasses F1–F5; AC-11 (xiv)–(xvi); scoped to fallback |
| G3r4.2 | A | FIXED — tuple; malformed refusal; same-pair no-op preserved |

**Totals:** 27 FIXED, 0 REFUTED, 0 DEFERRED.

## Errors in this brief

- **(B)** says to refuse when the non-ancestor is a later round than "the registration equal to the accepted second parent". In #233's shape no registration equals the second parent, because `complete` adds a tree-identical commit. Taken literally, the rule would therefore fail closed on every normal `complete`-landed bead whose landing later lands off the first-parent chain. I defined the anchor as "equal to, or an ancestor of, the second parent".
- **(B)** "Define round ordering from the panel.json `round` field per registration": `round` is bumped in place, so multiple rounds exist only as separate registration dirs. This is stated, and the rule works on that shape.
- **(C)** cter.8 being "correctly out of range" is confirmed. The brief otherwise matches the code.
