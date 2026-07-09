# spec-112-bead2 — Round 1 (bead panel, 8 reviewers, four families)

**Worktree (read here)**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.worktrees/worktree-mindspec-lma4.2
**Branch**: bead/mindspec-lma4.2
**Commit under review**: a2007086 — "feat(panel): recorded decision-inert gate field + ADR-0037 amendment" (sole impl commit; 4 files, +119; branch includes lma4.1's merged config work beneath it)
**Panel**: 8 reviewers — O1–O3 Opus, S1–S3 Sonnet 5, F1 Fable, G1 GPT-5.5 (codex). Pass = **>=7 APPROVE, no REJECT**.
**READ-ONLY RULE**: verdict JSON only; scratch under /tmp; pin reads to the SHA; leave `git status` clean.

## What the work does

Bead 2 of spec 112 (plan §Bead 2; spec R6 + the R9 stability half): adds the recorded, DECISION-INERT `gate` field to `panel.Panel` — name `gate`, type string, `json:"gate,omitempty"`, parse-lenient — with the pinned stability contract, plus the ADR-0037 **§1** amendment note (the second amendment; 109's §3 threshold note untouched) and the workflow doc-sync region. The field records which gate mix a panel was created from; `PanelGateDecision`/`ResolveGateFacts` must NEVER read it (decision-inert — spec falsification: any gate-path consumption fails R6). `TestPanel_GateFieldDecisionInert` pins that. Leaf invariant (no internal/config import) re-asserted.

## Files in scope

- `internal/panel/panel.go` (+16), `internal/panel/panel_test.go` (+76)
- `.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` (+12), `.mindspec/domains/workflow/architecture.md` (+15)

## Known pre-existing failures (do not attribute to this diff)

`internal/instruct TestRun_IdleNoBeads` (z4ps, repo-state leak — verified failing identically at parent a7b432ad) and `internal/harness TestLLM_BlockedBeadTransition` (LLM subprocess test, zero dependency on internal/panel). Verify the decoupling if your lens covers it; don't re-litigate otherwise.

## Slot lenses

| Slot | Family | Lens |
|:-----|:-------|:-----|
| O1 | Opus | Author-of-record — diff vs plan §Bead 2 Steps + ACs exactly. |
| O2 | Opus | Codebase-pin — run the full Bead-2 Verification checklist yourself fresh. |
| O3 | Opus | Contract stability — the R9 pin: field name/type/omitempty/parse-lenient semantics exactly as the spec fixes them; ADR-0037 §1 amendment correctly homed, §3 untouched; old panel.json files (no gate key) still parse. |
| S1 | Sonnet | Empirical prober — scratch programs: write/read panel.json with and without `gate`; a legacy panel.json round-trips; a hostile gate value (control bytes, huge string) never influences any gate outcome and renders safely wherever it surfaces. |
| S2 | Sonnet | Schema/type — omitempty semantics (empty string drops the key — is that the spec'd behavior?), JSON round-trip fidelity, struct-tag hygiene vs the rest of Panel. |
| S3 | Sonnet | Next-bead integration — Bead 3 (gate-aware advisory + config show --gate) consumes this field per the plan: is what it needs present/ergonomic (incl. `PanelGateAdvisoryDefault(recordedGate, isBead)` interplay and the fbel-110 `panel.Create` writer's future stamping noted in 110's plan)? |
| F1 | Fable | Adversarial — attack decision-inertness: find ANY path where the recorded gate value reaches PanelGateDecision/ResolveGateFacts/tally outcomes (incl. via the new helper at panel.go:136); attack parse-leniency (malformed gate types — number, object, null — must not break Scan/gate reads); check TestPanel_GateFieldDecisionInert would actually FAIL if someone wired the field into the decision. |
| G1 | GPT-5.5 | Second empirical prober — build to /tmp; hostile panel.json fixtures (gate: 123, gate: {}, gate with ANSI/control bytes) through Scan + the complete-gate path; verify decision outcomes byte-identical with/without the field across Allow/Warn/Block fixtures. |

## Your job

Verdict: APPROVE / REQUEST_CHANGES / REJECT → `<your-slot>-round-1.json` in this dir (`/Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.mindspec/specs/112-per-gate-panel-config/reviews/spec-112-bead2/`). Keys: `reviewer_id`, `verdict`, `confidence`, `rationale` (<=200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
