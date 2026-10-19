# Spec 129 — spec gate, round 5: consolidation

- **Result:** reviewed commit `7404845f`. 5 APPROVE (F1, F2, O1, O2, O3), 4 REQUEST_CHANGES (F3, G1, G2, G3), no hard block, 21 asks.
- **Reading:** every verdict's asks were read, including those from the APPROVE slots.
- **Ruling 5:** applied as given.
- **This commit:** `spec.md` only. The ADR-0041 amendment text is pinned by AC-18 and lands with the beads.

## Ruling 5 and F2

- **New F2 rule:** refuse (`UnlandedReview`) iff any visible registration for the bead, abandoned ones included, has a well-formed `reviewed_head_sha` that is NOT an ancestor of the spec tip.
- **Rounds are never compared.** panel.json's `round` is a per-directory counter, and the in-place bump means one directory holds ONE record. So the ruling's "compare only within one directory" has no pair left to compare, and any outside-tip registration in any directory fails closed.
- **Under the in-place convention:**
  - a rebased-away earlier round is overwritten, so the landed record passes (AC-11 (i-a));
  - O3's shape `{round 2: B}` refuses (AC-11 (vii-s)).
- **The `-r2` slug convention** (still documented by `ms-panel-run`) fails closed when its predecessor holds a rebased-away SHA, with the exit named. `panel create`'s default round is no longer load-bearing; the plan verifies it.
- **G3r5.2 is moot.** Anchors grant nothing, so an abandoned registration cannot "raise" one. An abandoned registration outside the tip still refuses (AC-11 (x-a)).
- **Ancestor exception, narrowly.** A reviewed SHA that is an ancestor of the second parent is simply in the tip's history. No tree identity is assumed: `complete` adds materialization or JSONL commits on the current binary (O1r5.1).
- **F2's optional bead-side anchor tightening** is not adopted separately. "Anchors grant nothing" is strictly stronger: a forged high-round anchor cannot neutralise anything.
- **Provenance gate:** `wellFormedGitObjectID` runs on each `reviewed_head_sha` before any `merge-base`. A malformed value fails closed (AC-11 (x-m)).
- **Refusal text** names each outside-tip registration's directory, round and SHA, plus the visibility line.
- **New legs:**
  - (x-c) cross-directory: `<X>` at round 3 versus `<X>-r2` at round 2, and an old slug at round 5 versus a new slug at round 1. Both refuse at acceptance AND at read-back with a valid matching audit. Changing only the round does not change the result.
  - (i-c) the #233 descendant positive.

## Residual restated

- Only F1, F3 and F4 are git-derived.
- F2 reads `reviews/*/panel.json`: agent-writable, untracked during the spec's life, with an operator-supplied round.
- A metadata-only forge is visible in the JSONL history.
- A forge that also deletes or rewrites a panel registration defeats F2 and leaves no JSONL trace. It then reaches any single, reachable, source-named, non-reverted fallback merge whose surviving tip, if any, matches, including O3's shape. That is the same class as a forged exact-match `reviewed_head_sha` under Spec-125 item (c). F1, F3 and F4 are unaffected.
- AC-11 (ix-g) pins this reach as documented. The touchpoint's residual bullet and AC-18's amendment pin carry it.

## Other asks

- **O2:**
  - q9ea is dropped from the fallback exits (arm 1b, R4.2 Exits, the AC-11 "refuses" definition). It stays on the primary-path NoEvidence arm.
  - The corroboration string records the fallback provenance: `off-first-parent fallback; F1–F5 passed; panel roots scanned; registrations checked`.
- **G3r5.3:** the no-op now requires a VALID audit. A malformed audit with the same tuple re-runs F1–F5 and rewrites; this is leg (i-m).
- **G1r5.2:**
  - Lazy-read trigger: the plan is read whenever any plan-form row is not already N2a or N2c, which includes rows with no lineage and rows whose title-fallback lineage is E2.
  - Standalone AC-1 fixtures: N1 versus N4, and N2b versus N4.
- **F3:**
  - `spec/<S>`'s committed blob is preferred whenever that branch exists, with a divergence notice; the AC-1 fourth fixture covers it.
  - `PlanMapsBead` counts headings exactly as `extractBeadSectionRaw` does, fenced lines included, and this is pinned.
  - The malformed-SHA gate (above).
- **F2:**
  - The single-directory refuse and pass legs.
  - Q's epic ordering is pinned (E′ newer, list newest-first).
  - N2a's P is non-epic (an epic parent is N2c).
  - OQ-9 is qualified, and a new **OQ-10** (deferred to plan: does readiness read the same plan source?).
- **F1:** the estimate is now **9–12 beads**, and the R6 split is expected.
- **O1:** the #233 wording no longer assumes tree identity (R4.2 F2, AC-12 (iv)).

## Coverage table (21 asks)

| Ask | Disposition |
|---|---|
| F1r5.1 | FIXED — panel-side residual stated; same class as item (c); F1/F3/F4 unaffected |
| F1r5.2 | FIXED — 9–12 beads; split expected (OQ-1, Provenance) |
| F2r5.1 | FIXED — residual; tightening subsumed by "anchors grant nothing" (stated) |
| F2r5.2 | FIXED — AC-11 (vii-s) and (i-a) |
| F2r5.3 | FIXED — Q fixture pinned; N2a non-epic |
| F2r5.4 | FIXED — OQ-9 qualified; OQ-10 deferred to plan |
| F3r5.1 | FIXED — provenance statement; amendment; AC-11 (ix-g) |
| F3r5.2 | FIXED — (a) wellFormedGitObjectID + (x-m); (b) `-r2` convention, default round no longer load-bearing |
| F3r5.3 | FIXED — prefer `spec/<S>` + divergence notice; AC-1 fourth fixture |
| F3r5.4 | FIXED — identical counting, fenced included, pinned |
| O1r5.1 | FIXED — no tree identity assumed |
| O2r5.1 | FIXED — q9ea removed from the fallback exits |
| O2r5.2 | FIXED — residual (round forgery is now moot; deletion and SHA rewrite stated) |
| O2r5.3 | FIXED — fallback provenance in the corroboration string |
| O3r5.1 | FIXED — via ruling 5; AC-11 (x-c) shape 1 |
| G1r5.1 | FIXED — via ruling 5; AC-11 (x-c) shape 2, acceptance and read-back |
| G1r5.2 | FIXED — lazy trigger; standalone fixtures |
| G2r5.1 | FIXED — via ruling 5; round change pinned as irrelevant |
| G3r5.1 | FIXED — via ruling 5; (x-c) |
| G3r5.2 | FIXED — anchors removed; abandoned outside-tip refuses; #233 positive (i-c) |
| G3r5.3 | FIXED — validity-qualified no-op; (i-m) |

**Totals:** 21 FIXED, 0 REFUTED, 0 DEFERRED. OQ-10 is a new plan-stage deferral that no ask required.

## Errors in this brief

- Ruling 5's "compare the anchor and the non-ancestor only within the SAME directory" is vacuous under the in-place bump, because one directory holds one record. Its effect is "any outside-tip registration refuses". I implemented it that way and said so.
- I did not re-measure the "48 registrations at round ≥ 2, zero `-r2` dirs" count; it is cited from F2's verification.
