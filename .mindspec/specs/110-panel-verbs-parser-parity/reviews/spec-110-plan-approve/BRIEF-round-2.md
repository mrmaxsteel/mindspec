# spec-110-plan-approve — Round 2 (fix re-verification, 9 reviewers, three families)

**Under review**: `.mindspec/specs/110-panel-verbs-parser-parity/plan.md` @ **943cc3105dddb04ec0b41a119bd83af2467cdfd2** (fix commit on top of round-1 e86082d6; plan grew 773 → 1042 lines) in worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity`. Read the APPROVED `spec.md` beside it.
**Fix delta**: `git -C /Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity diff e86082d6..943cc310 -- .mindspec/specs/110-panel-verbs-parser-parity/plan.md`
**Panel**: same 9 slots as round 1 — F1–F3 Fable, O1–O3 Opus, G1–G3 GPT-5.5. Pass = **≥8 APPROVE, no REJECT**.
**Round-1 tally**: 3 APPROVE (F2, O2, O3) / 6 REQUEST_CHANGES (F1, F3, O1, G1, G2, G3) / 0 REJECT. Consolidated change list: `consolidated-round-1.md` in this dir (18 items, all plan-text amendments; design/DAG/citations were approved and are unchanged).
**READ-ONLY RULE**: verdict JSON only; scratch under /tmp; pin reads to the SHA; leave `git status` clean.

## What the fix did (orchestrator-verified present at text level; your job is to judge sufficiency)

1. (F3.1) B2 tests now assert `SevError`/`HasFailures()` on failing cases, `!HasFailures()` on tolerated ones.
2. (F1.1) R6 regex widened to `\[(ADR-\d{4})[^\]]*\]\([^)]+\)` (filename-form anchors, specs 085–094 convention); filename-form test pair added.
3. (F3.2) B2 self-check now builds the branch binary (`go build -o /tmp/ms110b2 …`) instead of `~/.local/bin/mindspec`.
4. (O1.1/O1.2) B3 pins a synthesized non-empty PASS reason for Allow (empty `Decision.Message`) and explicitly reconciles `TestPanelStateEntry_Verdict`'s two GatePass `wantReason` rows.
5. (F3.4) B3 delegation table gains round-mismatch, malformed-panel.json (PanelErr), missing-ref/transient-GitErr Warn rows; the "no residual logic" clause is now a falsifiable source assertion.
6. (F1.2/F3.3) B4 routes RunE exit through a single decision-to-exit helper asserted over a branch-complete GateFacts table (gate.go branches (2)–(10), shared fixture with B3); stale-SHA + hard_block exit rows added; rendered PASS/BLOCK token derived from `d.Action`.
7. (F1.3) `panel tally` Warn semantics pinned: exit 0 + advisory to stderr (parity with complete's panel_advisory), Warn row in exit-code test.
8. (G2.1) Slug validated as single clean path element (empty/`.`/`..`/slashes/absolute/control chars rejected pre-Join); traversal + control/newline test cases.
9. (G2.3) Control-byte/ANSI escaping for user-controlled strings in output + recovery lines, with tests.
10. (G2.4/F3.5) Tally CCR decode policy defined (malformed payloads never affect decision/exit; reported deterministically) + `TestPanelTally_AggregatesConcreteChangesRequired` covers the re-decode wiring with slot attribution.
11. (G1.1) Manual e2e rewritten: git-status before/after comparison, explicit sub-threshold verdict setup.
12. (F2 finding) B4 key_file_paths += `cmd/mindspec/root.go`, `cmd/mindspec/help_golden_test.go`.
13. (G2.2/F3.6) BRIEF marker edge cases specified + tested: legacy no-marker, marker-only-open, duplicated markers, CRLF byte-pass-through; corrupt states fail without touching panel.json/BRIEF.md.
14. (F3.7) `TestPanelSchemaDoc_MatchesConstants` now extracts backtick-quoted example tokens FROM the doc and matches them against the constants/regex, incl. the doc's own marked nonconforming `-round-0` example.
15. (G3.1) R4 schema doc extended to the verdict PAYLOAD contract: `verdict` enum, top-level `hard_block` (sibling of `verdict`, never finding-level), which fields feed the gate vs presentation; drift-test pins.
16. (G3.2) BRIEF stub's "Your job" block names the top-level `hard_block` key; B5's trimmed skills carry one non-conflicting verdict instruction.
17. (G1.2) Leaf assertion in two-step form (`deps=$(go list …) && ! printf … | grep`) in B1 + B4 + Provenance.
18. `mindspec validate plan` passes (WARN decomposition-scope-redundancy R=0.14, advisory — up from 0.11).

## Round-2 jobs

- **F1, F3, O1, G1, G2, G3 (round-1 RC voters)**: evaluate EACH of your own round-1 `concrete_changes_required` items against 943cc310 — disposition every item ADDRESSED / PARTIAL / MISSED / NEW_ISSUE in your `findings`. Judge sufficiency, not just presence: e.g. F3 — is the branch-complete table really branch-complete against gate.go? O1 — does the PASS-reason reconciliation actually keep/knowingly-update the named test rows? G1 — are the rewritten e2e commands now runnable and falsifying? G2 — do the validation/escaping specs close the injection classes or leave gaps? G3 — is the payload contract complete enough for a non-Claude runner?
- **F2, O2, O3 (round-1 approvers)**: confirm the fix delta introduces no regression in your lens. F2 — the shared B3/B4 fixture table and the key_file_paths additions: any new cross-bead coupling or false dep? O2 — step counts still ≤7 per bead, provenance consistent, citations untouched? O3 — no scope creep beyond the spec from the added hardening (slug validation, payload schema — still within R1/R3/R4's remit?), coverage map still exact?

Verdict: APPROVE / REQUEST_CHANGES / REJECT → `<slot>-round-2.json` in this dir. Keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence`, `rationale` (≤160 words), `concrete_changes_required` (empty if APPROVE), `findings` (RC voters: per-item dispositions).
