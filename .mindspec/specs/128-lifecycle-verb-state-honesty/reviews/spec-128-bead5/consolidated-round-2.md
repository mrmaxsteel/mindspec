# spec-128-bead5 — consolidated concrete_changes_required, round 2

Tally: 5 APPROVE (R1, R2, R3, R5, R6 — all zero asks) / 1 REQUEST_CHANGES with hard_block (R4 slot, claude-sub — the codex run was killed mid-review and the sub completed the empirical sweep). 2 asks.

## G1 (hard block; bisected evidence) — The no-short cmd/mindspec suite is RED at the reviewed tip — MUST FIX
Six real-bd E2E tests fail, bisected to 68ede240 and reproduced at 3c472ac9: TestBeadClarify_* (×4), TestBeadReadyCheck_TemporalFlow_RealPlanApproveAndComplete, TestNextRea… (truncated in verdict — run the suite for the full list). Likely the honest-idle/pre-commit-hook interaction from this bead's instruct/derive changes. Reproduce with `go test ./cmd/mindspec/` (NO -short), diagnose, fix; the no-short suite must be green at the new tip. (Process note: our -short-only gate cadence hid this — the fix report must state the no-short result explicitly.)

## G2 — TestVerbOutput_Next's ordering assertion is vacuous — MUST FIX
next's success guidance is the bead-context prompt, not the instruct tail — none of mindspecGuidanceMarkers occurs in its captured output, so result-before-guidance passes trivially. Add next's REAL guidance markers ('# Bead Context', '## Work Chunk', the '**When done**' completionGuidance line) alongside the tail markers (which cover the fallback legs).
