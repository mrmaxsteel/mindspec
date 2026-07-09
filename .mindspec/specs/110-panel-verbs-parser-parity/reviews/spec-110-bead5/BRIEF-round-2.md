# spec-110-bead5 — Round 2 (targeted re-verification: R6, R7)

**Under review**: `bead/mindspec-fbel.5` @ **17eb5afe8fc6da7616f4617765539d57d266f2fc** (fix on round-1 `13788634`; 5 files, +19/−8).
**Pass = ≥7 APPROVE, no REJECT.** Round-1 approvers R1, R2, R3, R4, R5, R8 (6) carry — untouched by these additive doc clauses. Round-2 re-runs: R6 + R7 (the round-1 RC voters).

**READ-ONLY**: verdict JSON only; pin reads to `17eb5afe`; scratch under ABSOLUTE /tmp only. Delta: `git diff 13788634..17eb5afe`. Both mirror pairs stay byte-identical (verified).

## The fixes
- **R7 (safety)**: ms-panel-tally step 1 Allow branch now REQUIRES screening the aggregated `concrete_changes_required` against § Artifact gates before the merge handoff — a CCR item naming a missing measurement artifact / cost projection / drift report / regression baseline HARD-blocks regardless of vote count and regardless of whether any reviewer set `hard_block`. Restores the disjunct the binary (gate.go 9–10) doesn't mechanize.
- **R6 (anti-drift)**: ms-panel-run § Step 0 now instructs the operator to capture `panel create`'s reported `panel directory: <dir>` line and use it directly for the BRIEF/prompts/verdict-checks, noting the non-flat legacy fallback that the prose convention doesn't cover.
- Nits: runbook now names all 5 surviving ms-panel-run sections; stale `expected-reviewers` § Inputs bullet replaced with the config-stamping note; a "read confidence from the verdict JSON files" note added (the CLI's printed table carries verdict/hard_block only — verified against `renderPanelTally`).

## Per-slot jobs
- **R7**: confirm the restored Allow-branch artifact-gate screen genuinely closes the lost-invariant gap you found (a missing-`cost_projection.json` CCR now HARD-blocks a 5/6-Allow even with no flag set), is consistent with the surviving § Artifact gates section, and introduces no contradiction with the mechanized `panel tally` decision. Disposition ADDRESSED/PARTIAL/MISSED/NEW_ISSUE.
- **R6**: confirm ms-panel-run now points the operator at `panel create`'s reported `panel directory:` line as the authoritative path (no longer solely re-deriving from the flat-only prose convention), and the non-flat fallback is acknowledged. Disposition ADDRESSED/PARTIAL/MISSED/NEW_ISSUE.

## Output
Write `<slot>-round-2.json` here. Keys: `reviewer_id` ("R6 sonnet"/"R7 fable"), `verdict`, `confidence`, `rationale` (≤160 words), `concrete_changes_required` (empty if APPROVE), `findings`.
