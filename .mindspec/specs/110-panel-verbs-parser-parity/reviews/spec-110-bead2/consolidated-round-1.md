# spec-110-bead2 — consolidated round-1 changes

Tally at 7 of 8 (S3's resumed verdict pending, superseded by round 2): 6 APPROVE (O1, O2, O3, S1, S2, F1) / 1 REJECT (G1). 

## Disposition of G1's REJECT (orchestrator ruling, recorded for the audit trail)

G1's CCR1 ("11 specs flip pass→fail vs the installed binary — parity break") applied the ORACLE AS SPECIFIED IN THE ORCHESTRATOR'S G1 BRIEF, which was mis-specified: the correct parity contract (spec R5, plan §Bead 2) is "validate spec's NEW rejections ⊆ validate plan's EXISTING rejections" (checks move EARLIER, severity identical) — not "branch validate spec == installed validate spec", which by construction flips every historical spec that only plan-approve used to catch. F1 independently verified the correct oracle on the same corpus: each of the 11 flipped specs fails main's `validate plan` with byte-identical impacted-domains-resolve errors (spot-verified 038/060). CCR1 is therefore DECLASSIFIED as orchestrator-brief error, not a code defect; the empirical facts G1 gathered CONFIRM the intended behavior. G1 re-verifies under the corrected oracle in round 2. (No code change for CCR1.)

## The real fix (ONE item)

1. **(G1 CCR2 + F1 advisory, convergent) Regex digit-boundary — must.** `adrTouchpointLinkRe` = `\[(ADR-\d{4})[^\]]*\]\([^)]+\)` truncates a five-digit anchored id: `[ADR-12345](…)` matches with capture `ADR-1234` and mis-reports "missing ADR-1234" (a diagnostic naming an id the author never wrote). Same boundary defect the other way: `[ADR-00311]` matches via the `ADR-0031` prefix and silently PASSES against an existing ADR-0031. Fix: require a non-digit boundary after the four digits — Go RE2 has no lookahead, so structurally: `\[(ADR-\d{4})([^0-9\]][^\]]*)?\]\([^)]+\)` (after the 4 digits: either `]` immediately, or one non-digit-non-`]` char then any tail). Result: five-or-more-digit anchored ids are cleanly OUTSIDE the extraction shape — neither matched, nor truncated, nor mis-reported (consistent with the four-digit ADR-#### convention and the existence-only remit; do NOT add a new malformed-id error class — that would be stricter than parity). Tests: `[ADR-12345](…)` produces NO adr-touchpoint diagnostic; `[ADR-00311](…)` likewise no match/no false-pass-as-0031; `[ADR-0031](…)` and `[ADR-0031-doc-sync-gate.md](…)` still resolve. Update the regex comment and, if the doc-sync region cites the pattern, keep it consistent.

## Constraints for the fix author

- ONE commit on `bead/mindspec-fbel.2`: `fix(validate): digit-boundary the ADR touchpoint anchor regex (bead panel r1) [mindspec-fbel.2]`.
- Only internal/validate/spec.go + internal/validate/spec_test.go (+ architecture.md only if it cites the pattern).
- Full Bead-2 Verification checklist must pass again (incl. branch-built self-check + `go test ./internal/approve`).
- Note: this deviates from the plan's verbatim regex text in service of the plan's intent (correct existence-only diagnostics); round 2 assesses the deviation explicitly.
- No push, no bd, no `mindspec complete`.
