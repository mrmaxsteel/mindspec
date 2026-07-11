# spec-115-bead3 — Round 2 consolidated tally — PASS 8/8

**Reviewed**: `bead/mindspec-fgmg.3` @ `997e2a9e` (round-1 docs+test + a fix commit). **Panel**: 8 slots (R7 = Opus-sub, Fable quota-walled). **Threshold**: 8/8 UNANIMOUS.

## Verdicts — 8 APPROVE / 0 REQUEST_CHANGES
R1 Opus APPROVE · R2 Opus APPROVE (ADR accuracy — overclaim removed, contradiction gone) · R3 Opus APPROVE (0.97 — proved the broader-glob subtest RED by reverting to the prefix heuristic in a throwaway) · R4 Sonnet APPROVE · R5 Sonnet APPROVE · R6 Sonnet APPROVE (no import cycle — test-only edge) · R7 Opus-sub APPROVE (0.93 — scoped claims resolve 1:1 to the shipped legs) · **R8 codex APPROVE (0.99) — FLIPPED from round-1 RC.**

**PASS.**

## The two-round arc
- **Round 1 (7/1):** the ADR/skill/ownership work was clean on scope, chain position, skill consistency, AC9, grounding. R8 (adversarial) caught two real gaps the 7 Claude slots missed: (1) the opening "REFUSES … closed without complete" overclaimed — it literally included the undetectable raw-merged-no-obligation residual, contradicting the amendment's own §8 disclosure; (2) the AC10 test's string-prefix `claimsLifecyclePackage` missed a broader second-domain glob (`internal/**`), so "exactly one claimant" wasn't truly pinned. Both confirmed legitimate by orchestrator against the artifacts (R2/R7 read the docs holistically; R3 tested narrower globs only).
- **Round 2 (8/8):** doc precision — scoped the ADR + all 3 skill "REFUSES" claims to the DETECTABLE guarantee (unmerged non-ancestor branch / durable uncovered obligation), added a consistent "Disclosed residual" note, softened "impl approve finalizes" → "subject to remaining gates"; AC10 — replaced the prefix heuristic with the PRODUCTION matcher `validate.GlobMatch` against a real probe path (`internal/lifecycle/orphans.go`) + RED subtests (broader-glob/duplicate/removed/sibling). R8 flipped; R3 empirically proved the new subtests RED-on-revert.

## Doc-sync note for `mindspec complete`
Bead 3 changes `internal/setup/claude.go` (the embedded ms-impl-approve skill text — documentation itself). Completing with `--allow-doc-skew`: Bead 3 IS the docs bead; the claude.go change is the embedded skill documentation, and this repo has no `.mindspec/docs/domains/` structure (contract docs live in ADR-0037 + the skill). Whole-spec doc-sync is dispositioned at impl-approve.

## Next
`mindspec complete mindspec-fgmg.3` → merges bead→spec. Then the **12-slot four-family final review** of the whole spec branch vs main (≥11), then `impl approve` (protected-main: push spec branch → PR → CI → merge to main). HELD: v0.11.0 release.
