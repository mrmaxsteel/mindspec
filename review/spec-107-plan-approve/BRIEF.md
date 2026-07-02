# spec-107-plan-approve — Round 1 Review Panel (plan-approve gate)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-107-cleanup-deadcode-dry-wave1
**Branch**: spec/107-cleanup-deadcode-dry-wave1
**Commit under review**: 474a38097da8675a71482ac59cfc1f99337b6bcc — docs(spec-107): draft plan for round-1 approval panel
**Target**: the PLAN DOCUMENT (.mindspec/docs/specs/107-cleanup-deadcode-dry-wave1/plan.md) against the APPROVED spec (spec.md, approved round-2 6/6). No implementation exists yet.

## What the work does

The plan decomposes spec 107 (cleanup wave 1) into FOUR independent beads (no dependency edges): B1 dead-code sweep (all R1 deletions except the two that ride B2/B3, plus dangling-comment fixes), B2 setup managed-block unification through safeio + codex symlink-refusal test + full-equality content test + chainBeads fold, B3 complete/phase perf pair (exported phase.FetchChildren, queryAllChildren replacement, hoisted epic resolution, FindActiveBeadForEpic deletion, stubChildrenByStatus re-point), B4 AGENTS.md guardrails section + spec-init alias RunE dedup. adr_citations: 0030/0033/0034/0035/0036/0037 covering all four impacted domains. Validator: clean, one advisory (scope redundancy R=0.08 below 0.15 — intentional, ownership-aligned cut).

KNOWN ENV NOTE: the PATH mindspec binary was stale (pre-flatten); it has been rebuilt from main. Validate with the current binary from the worktree root.

## Files in scope (final state at 474a38097da8675a71482ac59cfc1f99337b6bcc)

- .mindspec/docs/specs/107-cleanup-deadcode-dry-wave1/plan.md (under review)
- .mindspec/docs/specs/107-cleanup-deadcode-dry-wave1/spec.md (the approved contract)
- review/spec-107-approve/source-report.md (evidence base)

## Your job

Evaluate whether this PLAN is ready for `mindspec plan approve` (which auto-creates the four beads and moves to Implementation Mode). Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `review/spec-107-plan-approve/<your-slot>-round-1.json` (relative to the worktree root) with keys: reviewer_id, verdict, confidence (0-1), rationale (<=200 words), concrete_changes_required (empty if APPROVE), findings. An artifact-gate finding may set "hard_block": true.

---

# ROUND 2 ADDENDUM (commit under review: 93143d92f8470ede23132fc01fc8696b70460dc0)

**Prior round**: 4 APPROVE (R1, R2, R4, R6), 2 REQUEST_CHANGES (R3, R5), 0 REJECT.

## Round-1 concrete_changes_required (consolidated — all claimed applied in 93143d92f8470ede23132fc01fc8696b70460dc0)

A. (R3) Per-bead doc-sync satisfaction: each bead now has a Steps item + runnable Verification checkbox updating the domain doc(s) of every domain whose source it touches (B1→all four; B2→workflow only, internal/safeio untouched; B3→workflow+core; B4→workflow). Strategy: dated cleanup notes appended to the touched domains' docs, in the same bead diff.
B. (R5) Every Verification '- [ ]' item rewritten as a concrete shell command whose exit status is the pass/fail (incl. B3 subprocess-count go test selectors, B4 fence/cross-ref/RunE/help checks, inverted grep-negatives).
C. (R3) Testing Strategy + AC map acknowledge the complete-time doc-sync gate as part of bead exit.

## Your job (round 2)

R3/R5: mark each of YOUR round-1 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE in findings (R3: re-walk each bead's file set incl. the new domain-doc touches against OWNERSHIP globs + the docsync gate; R5: actually sanity-check 3 of the rewritten commands are runnable as written). R1/R2/R4/R6: re-verify your approval holds at 93143d92f8470ede23132fc01fc8696b70460dc0 (Steps counts changed; doc-touch steps added). Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output to `review/spec-107-plan-approve/<slot>-round-2.json` (same schema).

---

# ROUND 3 ADDENDUM (commit under review: 07eb467e2017e31bcc187d57f3b3c52fcb74705a — DECISIVE ROUND)

**Prior round**: 3 APPROVE (R1, R2, R3 claude), 3 REQUEST_CHANGES (R4, R5, R6 codex). All nine round-2 asks were mechanical/convergent; all claimed applied in 07eb467e2017e31bcc187d57f3b3c52fcb74705a:

- (R6) Bead-unique domain-doc targets — zero shared key_file_paths across the four chunks (workflow: B1 architecture / B2 overview / B3 interfaces / B4 runbook; core: B1 architecture / B3 overview; execution+context-system: B1 only).
- (R4) B1 deadcode Verification split: tool exit status checked before the inverted grep.
- (R4+R5) All named-new-test Verifications use the PASS-line pattern (fail when the test is absent).
- (R5) Explicit per-agent full-equality managed-doc test Verifications added.
- (R5) B4 fence check scoped to the extracted section body.

## Your job (round 3)

R4/R5/R6: mark each of YOUR round-2 asks ADDRESSED / PARTIAL / MISSED — verify empirically (parse work_chunks for duplicate paths; dry-read the rewritten commands for the false-pass modes you flagged). R1/R2/R3: re-verify approval holds (doc targets moved to bead-unique files). This is round 3 of 3: a below-threshold tally halts the track for human review, so judge on gate-forwardness and falsifiability, not stylistic preference — new asks outside your round-2 scope belong in findings as advisory notes, not in concrete_changes_required unless they are genuine gate-blockers. Verdict: APPROVE / REQUEST_CHANGES / REJECT → `review/spec-107-plan-approve/<slot>-round-3.json` (same schema).
