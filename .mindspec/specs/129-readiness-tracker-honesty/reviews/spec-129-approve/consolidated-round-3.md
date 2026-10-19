# Spec 129 — spec gate, round 3: consolidation

- **Panel result.** Reviewed commit `4c46cf64`: 3 APPROVE (F1, O1, O2) and 6 REQUEST_CHANGES (F2, F3, O3, G1, G2, G3). No hard block. 28 asks.
- **Inputs.** I read every verdict's asks, including those from the APPROVE slots. The orchestrator's four rulings are applied as given and are not relitigated.
- **Commit scope.** This round changes `spec.md` only. No ADR file needed a change: the ADR-0041 amendment text lands with the R4/R6 beads, and AC-18 pins it.

## How the rulings were applied

**Ruling 1 — the fallback never auto-confirms (R4.2, touchpoint (i), AC-11).**
- **Finder only.** `FindLandedMergeReachable` only FINDS and NAMES a candidate. It returns a typed `*LandedMergeOffFirstParent` that names every admitted candidate's SHA and the `mindspec reattest <id>` recovery. MF-3 FAILS with a new R4.4 arm (1b).
- **Acceptance path.** `reattest` is the only way a fallback candidate is accepted. It adds three refusals:
  - (i) exactly one owned candidate, with no grouping or tie-break. This kills G3's empty-anchor mask and makes O2r3.1's tie-break departure moot; the touchpoint now says so;
  - (ii) `ReattestStateUnlandedReview` when any registered `reviewed_head_sha` is not an ancestor of the tip. This is O3's fix, and it closes both of O3's round-3 variants;
  - (iii) the audit is always recorded, even on a value no-op.
- **Read-back.** Later automatic reads accept a fallback candidate only through a binding that carries a reattest audit whose scanned branch is this spec branch. The residual is stated: that audit is agent-writable metadata, the class ADR-0037 §8 and Spec 125 already accept. Without this read-back, #239 could not recover even after `reattest`.
- **#239** is restated as "loud, named, recoverable by one audited command" in the Goal (d), the Provenance and Background D.2. The automatic-pass legs are removed.
- **New AC-11 legs:**
  - (i) MF-3 FAILs naming the candidate, then `reattest` PASSes, then MF-3 PASSes;
  - (vii) O3 round 2: refuse, no write (answers F2r3.4);
  - (viii) O3 round 3, stale tip reset: refuse;
  - (ix) O3 round 3, forged binding without audit: refuse;
  - (x) G3's two-candidate anchor shape: refuse;
  - (xi) a merge-time binding with no audit.
- **Deliberate scoping, for the lead to overrule.** Refusal (ii) applies to FALLBACK candidates only. On the primary path a rebased-away review round is normal, and refusing there would create an ADR-0041 §2(i) deadlock. The ruling's text did not scope it.

**Ruling 2 — membership requires a plan mapping (R1.1/R1.2, AC-1, AC-4 (v)).**
- **Membership.** A member needs a plan-form ID, lineage E, AND an N mapped by `plan.md` (MF-1's own mapping, read once).
- **New N4.** Unmapped follow-up children are a new class N4. It is informational, exit 0, never default-picked, and offers no claimable recovery.
- **N4 positional.** An N4 positional goes to `GateReadiness`, which refuses it on MF-1. A scoped, evidenced MF-1 override is the deliberate route.
- **Rejected alternative, in one sentence:** F2's lifecycle-first default pick, because MF-1 refuses unmapped beads.
- **AC-1** adds O (`<E>.7` bug, unmapped) and X (a conflicting epic dependency). AC-4 (v) is fixed.
- **Evidence.** `mindspec-cter.8` and `mindspec-jfgj.4` are both type bug in the tracked `.beads/issues.jsonl`.

**Ruling 3 — no lettered IDs.**
- Every "129b" is now "the follow-on spec (next free plain number)", and "129c" is now "a further plain-numbered spec".
- New Background K covers the `parseSpecID`/`Sscanf` mechanism, cites mindspec-wtjr, and notes the duplicate-spec-number-epic nondeterminism (newest epic wins, fail-safe).

**Ruling 4 — the override recovery names all failing signals.**
- R6.4's re-claim line is now `--allow-not-ready=<all currently failing ids>`. The uncovered list is diagnostic only.
- AC-17 gains the mixed-coverage legs: MF-2 covered and MF-3 uncovered; MF-3 covered and MF-4 uncovered; coverage lost because the digest list grew.

## Remaining items

- **O1r3.1.** Background A and the Validation Proof now state that the full-ID reporter shape is selected today on bd 1.1.0. AC-1's fake bd is edge-only, and the numeric-tag G′ is added.
- **O1r3.2.** Precedence for aggregated MF-3 recovery: `indeterminate > not-closed > no-candidate > off-first-parent > no-evidence > contradiction > reverted`. There is one `recovery:` line per signal, and every record is listed in Detail. An AC-13 leg is added.
- **F2r3.3, F3r3.6.** A named signal carrying an `indeterminate` record refuses before mutation. An AC-15 leg is added.
- **F3r3.3.** N2 is split into N2a (named parent → inspect), N2b (title-fallback lineage to E2, no parent → `bd update --parent`, safe because an epic dependency wins over the title in `FindEpicForBeadWithCache`) and N2c (lineage through a dependency on epic E2 → inspect). An AC-2 leg is added for N2b.
- **F3r3.4. REFUTED in part.** A bug bead with no epic lineage is ALREADY refused by `next` today. Evidence: `GateReadiness` → `EvaluateReadiness` returns the lineage error (`internal/validate/readiness/readiness.go` returns `resolving owning spec for %s` on `findEpicForBeadFn` error, and `internal/next/ready_gate.go` returns `evaluating readiness for …`) before `advancePhaseOnClaim` runs, so that comment is stale. Still fixed:
  - R1.8 states the behaviour change for lineage-but-non-plan-form beads, and the alternatives (`bd update --claim` plus a manual branch, or folding the work into a plan bead);
  - the `advancePhaseOnClaim` comment is added to the doc surfaces;
  - AC-6 (d) and (f) are added.
- **F3r3.5.** `spec/<T>` must resolve before the claim (R1.8 (d), AC-6 (e)).
- **F3r3.7.** Per-record digests: growth uncovers and shrinkage stays covered. The validity contract is updated, and AC-16 adds a shrink leg.
- **G3r3.2.** An MF-4 record is `<kind>:<source>:<section>:<k>:<hash>`, keeping multiplicity. AC-16 adds a description-vs-plan `TBD` leg and a repeated-checklist leg.
- **G3r3.4.** The parent-E shortcut is removed, and `FindEpicForBead` runs for every plan-form row. An AC-1 X row is added (N2c).
- **O2r3.2, O3r3.5.** The false "executor writes it only" sentence is gone with the round-2 confirmation rule. The binding is described as agent-writable, written by the executor and by `reattest`. Only a reattest-audited binding confirms a fallback candidate, and the residual is stated.
- **O2r3.3.** The `ms-bead-impl` line is marked as a staged rule under ADR-0045 Decision 5's transition. Its target is `next-action` (spec 133); its date is set in the rule table (spec 130).
- **F1r3.1.** Resolved by Ruling 2, with a different outcome than F1 proposed: the follow-up `<E>.7` bug is an N4 near-miss, not a member. AC-1 row O pins it.

## Coverage table (28 asks)

| Ask | Group | Disposition |
|---|---|---|
| F1r3.1 | R2 | FIXED — via Ruling 2: N4, not a member; AC-1 O; OQ-7 reworded |
| F2r3.1 | R2 | FIXED — via Ruling 2; F2's lifecycle-first alternative rejected with its reason; AC-1 O, AC-4 (v) |
| F2r3.2 | O1 | FIXED — edge-only fake plus numeric-tag G′ |
| F2r3.3 | R6 | FIXED — indeterminate refuses before mutation; AC-15 |
| F2r3.4 | R1 | FIXED — reattest REFUSES and writes nothing in O3's shape (AC-11 vii) |
| F3r3.1 | R2 | FIXED — option (a) via a plan mapping; N4; AC-1 O |
| F3r3.2 | R3 | FIXED — plain numbers; Background K; mindspec-wtjr |
| F3r3.3 | R1 | FIXED — N2a/N2b/N2c; AC-2 N2b |
| F3r3.4 | R1 | FIXED; REFUTED in part (no-lineage bugs are already refused today; see above) |
| F3r3.5 | R1 | FIXED — R1.8 (d); AC-6 (e) |
| F3r3.6 | R6 | FIXED — same as F2r3.3 |
| F3r3.7 | R6 | FIXED — per-record digests |
| O1r3.1 | O1 | FIXED |
| O1r3.2 | R4 | FIXED — precedence; AC-13 |
| O2r3.1 | R1 | FIXED — tie-break withdrawn under Ruling 1, stated in touchpoint (i) |
| O2r3.2 | R1 | FIXED — binding wording; reattest-audited read-back only |
| O2r3.3 | R6 | FIXED — staged rule under the Decision 5 transition |
| O3r3.1 | R2 | FIXED — plan mapping |
| O3r3.2 | R2 | FIXED — AC-1 O (N4), AC-4 (v) |
| O3r3.3 | R2 | FIXED — N1/N2b recovery only when N is mapped |
| O3r3.4 | R1 | FIXED — via Ruling 1: the automatic path never accepts; the unreached-review refusal is in reattest; AC-11 (viii)/(ix) |
| O3r3.5 | R1 | FIXED |
| G1r3.1 | R4 | FIXED — all failing IDs; AC-17 mixed leg |
| G2r3.1 | R4 | FIXED — AC-17 mixed legs, including the narrower re-claim refusal |
| G3r3.1 | R1 | FIXED — exactly one candidate in reattest, no grouping; AC-11 (x) |
| G3r3.2 | R6 | FIXED — MF-4 record identity; AC-16 |
| G3r3.3 | R4 | FIXED — AC-17 MF-3/MF-4 leg |
| G3r3.4 | R2 | FIXED — shortcut removed; AC-1 X |

**Totals:** 28 asks. FIXED 28. One of them (F3r3.4) is also partly REFUTED. 0 DEFERRED.

## Errors in this brief

- Ruling 1 says "strengthen reattest to REFUSE when any registered `reviewed_head_sha` … is not an ancestor". Applied unscoped, this would refuse primary-path reattests whenever a review round was rebased away (an ADR-0041 §2(i) deadlock). I scoped it to fallback candidates and flagged that for the lead.
- Ruling 1 left one thing unsaid: once the automatic path never accepts a fallback candidate, a reattest-written binding must be READ BACK, or #239 stays unrecoverable. I added the reattest-audited read-back and stated its residual.
- mindspec-wtjr is not in the tracked `.beads/issues.jsonl` at this commit; it exists only in the live database. I cite it on the lead's word.
- F3r3.4's premise that `next` claims a no-epic bug today is false (see above).
