# spec-112-approve — Round 2 (fix re-verification, 9 reviewers, three families)

**Spec worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config
**Spec under review**: `.mindspec/specs/112-per-gate-panel-config/spec.md` @ **39b5724423eeba385b6b2080c5f32b5dcfc7a2a6** (fix commit on top of round-1 825f04c5; the branch was ALSO rebased forward onto post-109 main, so 109's surface is now present in the worktree — see below).
**Panel**: 9 reviewers, three families — F1–F3 Fable, O1–O3 Opus, G1–G3 GPT-5.5 (codex). Pass = **≥8 APPROVE, no REJECT**.
**Round-1 tally**: 6 APPROVE (G1,O1,O2,O3,F1,F2) / 3 REQUEST_CHANGES (G2,G3,F3) / 0 REJECT. Change list: `consolidated-round-1.md` in this dir.

## GROUNDING CORRECTION vs round 1 (important)

The round-1 BRIEF wrongly said "109's surface is in this worktree" — it wasn't (the 112 branch predated the 109 merge). That is now FIXED: the branch has been rebased forward (merged post-109 origin/main), so 109's ADR-0040, its spec at `.mindspec/specs/109-orchestration-config-substrate/spec.md`, and its landed code (`internal/config.Panel`, `PanelExpectedReviewers`/`PanelApproveThresholdExpr`, `internal/panel.ApproveThreshold`/`ReviewerCountNote`, `cmd/mindspec config show`) are ALL present in this worktree at the reviewed SHA. Read them directly (no `git show main:` needed). The spec's own rebase-before-planning prerequisite is thereby satisfied.

## READ-ONLY RULE (mandatory, all 9)

Repo files READ-ONLY; write only your verdict JSON; scratch under /tmp; pin reads to the SHA.

## What the fix commit changed (round-1 → round-2 delta: `git diff 825f04c5..39b5724`)

Twelve consolidated tightenings, all one-sentence-scale, one new requirement (R9). Highlights:
1. **Cursor start = 0 PINNED (the convergent F3/F1/O3 finding)** — R3 now has a falsification predicate + an AC asserting the exact 9-slot worked example (R1=author-of-record … R9=codebase-pin); the worked example is normative, not illustrative. An off-by-k cursor start now fails a clause.
2. **Unknown recorded `gate` value** (R7) — treated by the advisory as "no known gate": note skipped, R3 resolver never called with it, no error surfaces. Falsification + AC added.
3. **Escaping REQUIRED** (R8: "Escaping is required, not optional") — 109 escapeConfigValue-equivalent for note/model/lens/substitutes in config show + --gate text; `--gate --json` pinned to a real JSON encoder; new `TestConfigShow_HostileStringsEscaped` AC.
4. **Map render order pinned** — gates → R1 enum order, substitutes → sorted keys (R8 + AC), so `--json` is reproducible for 110/111.
5. **New R9** — the exposed machine contract (`--gate --json` schema + recorded `gate` field) is the STABLE documented thing 110/111 consume (does NOT add requirements owned by 110/111).
6. R7 skip carve-out gets a 4th `TestPanelAdvisory_GateAwareCompare` AC case.
7. **adhoc per-field inheritance** + R4 cross-field range check now covers the adhoc→bead chain (closes the "bead integer inherited by smaller adhoc = loadable but never-passable" hole).
8. Size caps: deferred WITH rationale (not silent).
9. Count-less-entry relaxation named in R2/AC1 (109 refuses count<1; 112 loads count=1).
10. `gates: {}` behaves as absent (len>0 keying). 11. unknown-gate recovery line names all five valid keys. 12. AC assertions for note-inertness, supersession-precedence, seeded-model negative control.

## Round-2 jobs

- **G2, G3, F3 (round-1 RC voters)**: evaluate EACH of your round-1 concrete_changes_required as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE against 39b5724. G2 → the escaping requirement + --json encoder + size-cap resolution; G3 → the stable downstream contract (R9) + does it stay within 112's scope (not stealing 110/111's requirements?); F3 → cursor-start pin, adhoc partial-fallback + the range-inheritance hole, unknown-recorded-gate.
- **G1, O1, O2, O3, F1, F2 (round-1 approvers)**: confirm the fix delta introduces no regression in your lens and that the new/edited requirements + ACs stay falsifiable and internally consistent (esp. F1: are the new falsification clauses genuinely falsifiable? O2: does R9 or the R7 unknown-gate text create any new contradiction? O1: is the now-present 109 surface consistent with 112's citations?). Read `consolidated-round-1.md` for what was asked.

Verdict: APPROVE / REQUEST_CHANGES / REJECT → `<slot>-round-2.json` (reviewer_id "<slot> <family>", verdict, confidence, rationale ≤180 words, concrete_changes_required, findings with per-item dispositions for the RC voters).
