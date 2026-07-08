# spec-109-approve — Round 1 Review Panel (spec-approve gate)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-109-orchestration-config-substrate
**Branch**: spec/109-orchestration-config-substrate | **Commit under review**: b2dd7246686ae47f45a039d7fdbbc6848c5db2d3
**Target**: the SPEC DOCUMENT (.mindspec/docs/specs/109-orchestration-config-substrate/spec.md), pre-approval. No implementation exists.

## What the spec does

Spec 109 = the foundation of a three-spec series (109 substrate → 110 panel verbs/parser parity → 111 workflow runner): (R1) ADR-0040, Accepted-on-landing, codifying the four-layer ratchet (gates/config/instruct/skills) + the portability-via-artifact-contracts principle with capability tiers; (R2-R6, R10) config.yaml gains panel: (reviewer mix 3+3 default, approve_threshold "n-1" validated to [1,sum], substitution policy), models: (free-form runner-specific strings, default inherit), loop: governance skeleton (gate_authority 4 keys default human; halt defaults 3/2/2/halt; budget unset=unlimited; context controller_handoff) with LOAD-TIME REFUSALS for un-weakenable knobs (panel_skip never delegable; on_reject must be halt; threshold-0 rejected as de facto panel skip — defense in depth both config-side and record-side), runner: adapter selector (claude-code-skills default, validated enum), resolvers PanelExpectedReviewers/PanelApproveThresholdExpr; (R7-R8) panel.json gains optional recorded approve_threshold interpreted SOLELY by ApproveThreshold() (absent = byte-identical N−1); PanelGateDecision stays a config-free pure function (internal/panel imports no internal/config — verified true today); ReviewerCountNote advisory never alters Allow/Block; (R9) read-only mindspec config show. Grilled: threshold-0 loophole closed both sides; halt/budget defaults pinned.

## Key invariants to scrutinize

"Identical decision over identical facts" (ADR-0037) preserved verbatim; zero-config round-trip (absent file → all defaults, pre-existing fields unchanged); ADR-0040 lands Accepted (codification ADR, precedent: 0034/0037); the enforcement honesty (models:/loop: parsed+surfaced NOT enforced — future specs, stated explicitly).

## Your job

Is this SPEC ready for approval? Verdict: APPROVE / REQUEST_CHANGES / REJECT → JSON to review/spec-109-approve/<your-slot>-round-1.json (relative to worktree root) with keys: reviewer_id, verdict, confidence (0-1), rationale (<=200 words), concrete_changes_required (empty if APPROVE), findings; optional "hard_block": true.

---

# ROUND 2 ADDENDUM (commit under review: 5435f51aaad2d9d8aa6c39ae0f37705446a78509)

**Prior round**: 3 APPROVE (R2, R4-sub, R6-sub), 3 REQUEST_CHANGES (R1, R3, R5-sub). All 7 consolidated asks claimed applied in 5435f51aaad2d9d8aa6c39ae0f37705446a78509:

A. (R3.1) Config-side reviewer floor: Load refuses reviewers[].count<1 and sum<2 (closes the [{claude,1}]→"n-1"→0 path); added to refusals AC.
B. (R3.2) gate.go:257 threshold>0 guard named as load-bearing THIRD defense in R7/R8, pinned by AC assertions (0-threshold panel still Blocks).
C. (R3.3) ADR-0037 amendment reworded: §3 threshold rule EXTENDED (not "semantically unchanged"); §§6/8 unchanged; absent field byte-identical N−1.
D. (R3.4) Inert-until-consumed annotations on loop:/models:/runner: in R4 + R9 (config show renders "declared, not yet enforced").
E. (R1.1) loop.handoff_log key (default AUTOPILOT-LOG.md) + zero-config AC coverage.
F. (R5.1) Populated round-trip AC (models map + custom reviewers + valid loop block through Load unchanged).
G. (R5.2) ADR-0040 content-anchor greps promoted into the AC checklist (Accepted status, four layer labels, ratchet-direction phrase, portability/artifact-contract phrase).

## Your job (round 2)

R1/R3/R5: mark each of YOUR round-1 asks ADDRESSED/PARTIAL/MISSED against the spec text. R2/R4/R6: re-verify approval holds. Round-2 discipline: new asks outside round-1 scope are advisory findings unless gate-blocking. Verdict → `review/spec-109-approve/<slot>-round-2.json`.
