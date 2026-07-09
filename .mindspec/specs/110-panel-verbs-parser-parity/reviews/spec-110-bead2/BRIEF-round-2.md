# spec-110-bead2 — Round 2 (fix re-verification, 8 reviewers)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.worktrees/worktree-mindspec-fbel.2
**Commit under review**: 2b55d1a7 (fix on r1's 62cd6df3; delta = internal/validate/spec.go + spec_test.go only, +72/−6).
**Round-1 tally**: 7 APPROVE / 1 REJECT (G1). `consolidated-round-1.md` in this dir records the disposition: G1's CCR1 ("11 specs flip pass→fail vs the installed binary") applied an ORACLE MIS-SPECIFIED IN THE ORCHESTRATOR'S OWN G1 BRIEF; the correct R5 contract is "validate spec's new rejections ⊆ validate plan's existing rejections" — independently verified by F1 AND S3 (each flipped spec fails main's validate plan byte-identically). G1's CCR2 (five-digit truncation) was REAL and is fixed here.
**Pass = >=7 APPROVE, no REJECT.** READ-ONLY rule unchanged.

## What the fix did

Regex digit-boundary: `\[(ADR-\d{4})([^0-9\]][^\]]*)?\]\([^)]+\)` (RE2-safe structural boundary; capture group 1 unchanged, extraction code untouched). `[ADR-12345]` now produces NO diagnostic (no truncation to ADR-1234); `[ADR-00311]` no longer false-passes via the ADR-0031 prefix; four-digit bare and filename-form anchors behave exactly as before. Three new subtests + comment rewrite. No new error class (parity preserved).

## Round-2 jobs

- **G1 (round-1 REJECT voter) — CORRECTED ORACLE**: your round-1 parity oracle was mis-specified by the orchestrator; the correct contract is: for every spec directory that FAILS the branch binary's `validate spec` but PASSED the installed one, verify main's `validate plan` (or the branch's — plan.go is untouched, zero diff) rejects that same spec with the identical impacted-domains-resolve error — i.e. checks moved EARLIER, never NEW. Re-run your corpus sweep under this oracle, re-test your five-digit fixture against 2b55d1a7 (expect NO diagnostic now), and disposition your two CCRs ADDRESSED/PARTIAL/MISSED/NEW_ISSUE.
- **F1**: your advisory (ADR-00311 prefix false-pass) is now fixed — verify empirically; re-check your other probes hold at 2b55d1a7.
- **O1, O2, O3, S1, S2, S3**: confirm the 2-file delta introduces no regression in your lens (O2: full checklist re-run; others: light confirm).

Verdict → `<slot>-round-2.json`. Keys as round 1; G1 includes per-CCR dispositions.
