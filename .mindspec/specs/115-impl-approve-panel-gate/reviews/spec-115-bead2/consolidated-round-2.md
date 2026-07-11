# spec-115-bead2 — Round 2 consolidated tally — PASS 8/8

**Reviewed**: `bead/mindspec-fgmg.2` @ `196f07d2` (round-1 gate impl + a comment-only fix). **Panel**: 8 slots. **Threshold**: 8/8 UNANIMOUS.

## Verdicts — 8 APPROVE / 0 REQUEST_CHANGES
R1 Opus APPROVE (0.97) · R2 Opus APPROVE (high) · R3 Opus APPROVE (0.98) · R4 Sonnet APPROVE (0.99) · R5 Sonnet APPROVE (high) · R6 Sonnet APPROVE (high) · **R7 Fable APPROVE (0.95) — FLIPPED from round-1 RC** · R8 codex APPROVE (0.99).

**PASS.** `mindspec complete` will re-verify this tally in-binary before merging bead→spec.

## The two-round arc
- **Round 1 (7/1):** the gate itself (four legs, fail-directions, 11 named tests, AC4 anchor, the 3 import edges, the 5 test-deviations) was cleared by all 8 lenses — including the fail-direction security lens (R2) and the adversarial codex slot (R8, which found NO fail-open on the first round). Deviation C (the `NoCommitsNoBeads` assertion change) was confirmed a correct spec-mandated tightening that actually CLOSES a pre-existing un-gated hole (R6). Sole RC: R7 flagged two comment-level issues — a false reachability claim (the preflight refusal became unreachable once Leg 3 subsumes the missing-plan case — the AC8 misleading-comment class) + a stale MergeBase line citation.
- **Round 2 (8/8):** comment-only fix — rewrote the `NoCommitsNoBeads` comment to truthfully state Leg-3 subsumption, added a truthful preflight code comment, fixed the citation to `:294` (preserving `:249` as the pre-115 Fact-1 pin). All 8 confirmed comment-only + behavior-preserving + accurate. R7 flipped.

## Follow-up filed
`mindspec` P3 issue: remove the vestigial CommitCount preflight refusal now subsumed by Leg 3 (touches the CONSENSUS-revision-9 call-order pin — a considered refactor, out of scope for the comment fix).

## Doc-sync note for `mindspec complete`
Bead 2 changes `internal/approve/**` (adds the gate — a real behavior change). Per the plan's decomposition, all doc updates (ADR-0037 amendment + ms-impl-approve skill) are **Bead 3's** scope. Completing Bead 2 with `--allow-doc-skew` deferring to Bead 3; **Bead 3 must assess whether `.mindspec/docs/domains/` updates are ALSO needed** (not just ADR/skill) so the 12-slot final-review doc-sync passes without a whole-spec skew override.

## Next
`mindspec complete mindspec-fgmg.2` → merges bead→spec. Then Bead 3 (docs + ownership) → 12-slot final review → impl approve → PR → merge.
