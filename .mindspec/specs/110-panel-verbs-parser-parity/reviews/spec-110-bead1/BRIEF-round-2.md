# spec-110-bead1 — Round 2 (fix re-verification, 8 reviewers)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.worktrees/worktree-mindspec-fbel.1
**Commit under review**: c9f54530 (fix on top of r1's 17a2ed28) — `git diff 17a2ed28..c9f54530` is the delta (+430/−76 across the same 3 files).
**Round-1 tally**: 5 APPROVE (O1,O2,O3,S1,S3) / 3 RC (G1, S2, F1). Consolidated: `consolidated-round-1.md` in this dir.
**Pass = >=7 APPROVE, no REJECT.** READ-ONLY rule unchanged (verdict JSON only; scratch in /tmp or job tmp; git status clean).

## What the fix did (verified present; judge sufficiency)

1. (G1.1/S1) Marker detection scoped: only marker comments alone-on-their-line at column 0 count; fence-quoted marker text in the body no longer jams re-panel (new test case).
2. (G1.2) Whitespace-mangled markers now REJECT as corrupt (fail without touching either file) instead of silently becoming "legacy" (new test case).
3. (F1) Drift test section-scoped to the "Panel Artifact Schema" section + F1's exact attack (normative rename to `registration.json`) added as a negative fixture that must FAIL.
4. (G1.3) Doc verdict contract aligned with tally.go's real parsing: enum values are the meaningful ones; any other non-empty string parses as present-but-neither-approving-nor-rejecting; tally.go untouched.
5. (S2.1) "Never hand-edited" claim replaced: steady-state intent + the SANCTIONED abandon-fields hand-edit path acknowledged.
6. (S2.2) Overwrite asymmetry documented (Create docstring + doc) and PINNED by a new test (pre-seeded abandoned panel.json → round-2 Create clears the fields, deliberately).
Non-blocking carried per the consolidated file: sequential-write crash window noted in docstring only; S3's B4/B5 notes deferred to those beads.

## Round-2 jobs

- **G1, S2, F1 (RC voters)**: disposition EACH of your round-1 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE against c9f54530. G1: re-run your hostile marker probes (fence-quoted, whitespace-mangled) + check the verdict-contract wording against tally.go. S2: verify the doc rewording + the abandoned-overwrite pin test. F1: re-run your section-scope attack (and confirm the negative fixture actually fails a wrong doc).
- **O1, O2, O3, S1, S3 (approvers)**: confirm the delta introduces no regression in your lens — O2: the full original Verification checklist still passes at c9f54530; O1: fix stays within Bead-1 scope (no plan-step reinterpretation); S1: your byte-preservation/co-bump probes still hold on the new marker logic.

Verdict → `<slot>-round-2.json` in this dir. Keys as round 1 (reviewer_id, verdict, confidence, rationale, concrete_changes_required, findings with per-item dispositions for RC voters).
