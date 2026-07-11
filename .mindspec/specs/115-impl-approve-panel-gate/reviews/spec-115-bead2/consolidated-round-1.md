# spec-115-bead2 — Round 1 consolidated tally

**Reviewed**: `bead/mindspec-fgmg.2` @ `8b664359`. **Panel**: 8 slots. **Threshold**: 8/8 UNANIMOUS.

## Verdicts — 7 APPROVE / 1 REQUEST_CHANGES
R1 Opus APPROVE · R2 Opus APPROVE (fail-direction security core — no leg fails open) · R3 Opus APPROVE (RED-on-revert, throwaway-worktree mutants) · R4 Sonnet APPROVE (empirical, all 11 tests + AC4 green) · R5 Sonnet APPROVE (7 seams + 3 import edges acyclic) · R6 Sonnet APPROVE (deviations A–E legitimate; deviation C actually CLOSES a pre-existing hole) · **R7 Fable REQUEST_CHANGES (0.9)** · R8 codex APPROVE (0.99 — adversarial, cleared every leg first round).

**Deviations A–E all ruled legitimate by R1/R2/R4/R6/R8** — most notably C (the `NoCommitsNoBeads` assertion change) is a correct spec-mandated tightening: R6 traced that pre-Bead-2 a missing plan.md WITH commits sailed through un-gated; Leg 3 closes exactly that. R8 confirmed no fail-open in any leg.

## The finding (R7 — confirmed correct by orchestrator code trace; findings never out-voted)

Two comment-level issues in the Bead-2 diff:

**1. False reachability claim + now-dead refusal (the AC8 anti-pattern, ironically in this very spec).** The preflight `impl.go:424` condition is `count == 0 && (planErr != nil || len(beadIDs) == 0)`. Because `readPlanBeadIDs` ERRORS on empty `bead_ids`, `len(beadIDs)==0 ⟺ planErr != nil`, so the disjunction reduces to `planErr != nil`. But Leg 3 (`runOrphanObligationGate`, :369) already refuses whenever `planErr != nil`, BEFORE the preflight. So past Leg 3, `planErr==nil ∧ len(beadIDs)>0` always hold, the disjunction is always false, and the `:425` "no commits beyond main" refusal is **unreachable**. The `NoCommitsNoBeads` test comment's claim that this message is "reachable via a spec that DOES have a valid plan" is therefore FALSE (a valid-plan-zero-commit spec is the cleanup path and PASSES; `TestApproveImpl_NoCommitsButClosedBeads_AllowsCleanup` confirms). Verified airtight by the orchestrator.

**2. Stale line citation** (`orphan_gate_test.go:600`): cites `exec.MergeBase ... at impl.go:249`; this commit's own insertions moved it to `:294` (`:249` is the spec's Fact-1 pin against the pre-115 tree). The semantic ordering claim (MergeBase before the gate) is verified TRUE — only the absolute line is stale.

## Fix (comment-only — round 2, then re-panel)
1. Rewrite the `TestApproveImpl_NoCommitsNoBeads` comment to be truthful: the "no commits beyond main" refusal's degenerate-plan condition is now fully SUBSUMED by Leg 3 (which refuses every missing/empty-plan case pre-mutation); remove the false "reachable via a valid plan" clause. The test pins that Leg 3 intercepts the missing-plan state before the preflight.
2. Add a truthful code comment at the preflight (`impl.go:~424`) noting the `(planErr != nil || len(beadIDs) == 0)` disjunction is subsumed by Leg 3 and unreachable in normal flow (retained as a defensive backstop / for the CONSENSUS-revision-9 call-order pin); a valid-plan zero-commit spec is the legitimate cleanup path and passes.
3. Fix the stale `orphan_gate_test.go:600` citation (`:249`→`:294`, or reword to reference the semantic ordering without a brittle absolute line — `:249` remains correct only as the spec's pre-115 Fact-1 pin, so distinguish the two).
No behavior change, no test-assertion change. **Follow-up filed** for the larger question of removing the vestigial preflight entirely (touches the call-order pin — a considered refactor, out of scope for this comment fix).

No other findings — all 7 other slots APPROVED (fail-directions, RED-on-revert, seams/imports, empirical, deviations, adversarial all clean).
