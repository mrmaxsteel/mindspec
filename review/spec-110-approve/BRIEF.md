# spec-110-approve — Round 1 Review Panel (spec-approve gate)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity | **Commit under review**: c0fd910e94aac0341abb788bc9d9ab392c49ada7 (drafted + grilled; grill pinned R5's severity-parity claim)
**Target**: the SPEC (.mindspec/docs/specs/110-panel-verbs-parser-parity/spec.md). No implementation.

## What the spec does

Spec 110 = the ratchet in action + the portability contract: (WA1) mindspec panel create/verify/tally verbs — create stamps 109's config resolvers + atomic round/SHA co-bump; verify = read-only completeness/staleness + the SAME PASS/BLOCK preview via PanelGateDecision; tally = the decision from the binary (exit codes + ADR-0035 recovery lines), mechanical concrete_changes aggregation printed, semantic consolidation stays skill judgment; verdict-file/slot schema documented as the agent-neutral contract (ADR-0040 capability tiers). (WA2) spec-approve parser parity — ValidateSpec runs the SAME contextpack.ParseSpec + normalizeImpactedDomains with the SAME severity semantics as downstream (path-like unresolvable → error; bare-name tolerance preserved), ADR touchpoints must resolve to existing files; honest boundary: coverage/intersection stay at plan-approve. Sequencing: 108 (merged ✓) + 109 implementation are hard prerequisites for plan-approve; ADR-0040 referenced in prose, NOT cited (it doesn't exist as a file until 109 B1 — citing it would fail 110's own R6 check).

## Context for judgment

Empirical motivators: ~10 hand-typed panel.jsons this week, a lost-verdict sandbox incident, the decision matrix living in both tally-skill prose and gate.go, the ParseSpec formatting incident detonating two gates late. The three-spec series: 109 (approved) substrate → 110 verbs/parity → 111 workflow runner.

## Your job

Is this SPEC ready for approval? Verdict → JSON to review/spec-110-approve/<slot>-round-1.json (relative to worktree root; keys: reviewer_id, verdict, confidence, rationale <=200w, concrete_changes_required, findings; optional hard_block).

---

# ROUND 2 ADDENDUM (commit: 0c6c56699806f2dbf8299e87b78db67d277c2107)

Round 1: 2 APPROVE (R1, R2) / 4 RC. All 7 consolidated asks claimed applied: A extraction rule pinned to anchored link bullets + self-inconsistency note + boundary AC triple; B BRIEF machine-header co-bump mechanism; C --bead/--target fail-safe note; D instruct-renderer refactor-first onto PanelGateDecision + preview-source AC; E schema-doc consistency AC; F PanelGateDecision contract-pin AC; G placement reconciliation note. NOTE: Codex weekly quota exhausted — R4-R6 are claude-subs this round (slot identities kept).

## Round-2 jobs: RC-voters' asks re-checked ADDRESSED/PARTIAL/MISSED; APPROVE-voters re-verify. Verdict → <slot>-round-2.json.
