# spec-113-approve — Spec-Approval Panel (9 reviewers, three families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-113-panel-verb-workflow-followups`
**Branch**: `spec/113-panel-verb-workflow-followups` @ **0b82c1d29151e3a87e498c08622865c1deb0149e**
**Under review**: `.mindspec/specs/113-panel-verb-workflow-followups/spec.md` (the DRAFT spec, before `mindspec spec approve`).
**Panel**: 9 slots — F1–F3 Fable, O1–O3 Opus, **G1–G3 codex/GPT-5.5 (real codex)**. **Pass = ≥8 APPROVE, no REJECT.**
**Base**: current main (has specs 109/110/111/112 — the code these fixes touch is all present; verify claims against it live).

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `0b82c1d2`; leave `git status` clean; scratch under ABSOLUTE /tmp only.

## What this spec is
A **followup-hardening wave** batching 4 bounded fixes from the just-shipped 110/111/112 batch (workflow + core config domains). No new architecture. The two design decisions were resolved interactively with the operator (both to the lower-risk option): **OQ1 → resolve-from-target**, **OQ2 → resolve-to-family** (see the resolved Open Questions).

The 4 requirements (read spec.md for the full falsification clauses):
- **R1 (mindspec-4d9m, P2, load-bearing)**: `panel verify`/`panel tally` mis-resolve staleness for NON-BEAD panels (bead_id null) — they rev-parse `bead/<empty>`, so a non-bead panel keeps reporting PASS after its `--target` advances, with a malformed "references branch bead/" message. Fix: resolve staleness from `panel.json.target` vs `reviewed_head_sha` through the normal `PanelGateDecision` legs (un-shadowing the REJECT/incomplete/threshold legs); fix the message. **Fence**: `PanelGateDecision`/`mindspec complete` semantics unchanged; `internal/instruct` fbel.3 inertness unchanged; only the two read-only/advisory CLI verbs change what they gather+render.
- **R2 (mindspec-lczt, P3)**: `ms-panel.js` `SHELL_METACHAR_RE` matches `$(` but not bare `$` → `$HOME`-expansion survives `validateShellSafe`. Fix: reject bare `$`; keep the `.claude` mirror byte-identical.
- **R3 (mindspec-zw81, P3)**: add `panel create --gate <name>` stamping `panel.Panel.Gate` (112's decision-inert field) from the closed enum, resolving creation-time defaults via 112 R3's gate-scoped resolvers — the writer-side stamping 112 explicitly deferred. Field stays decision-inert.
- **R4 (mindspec-cthj, P3)**: reconcile 112's R4 ("empty-string model" refusal) vs R1 ("family and no model is valid") for `{model:"", family:codex}` → **resolve-to-family** (document superseding R4's phrase + pin with a test; no struct change). `{model:""}` with no family stays refused.

## What to verify (this is a SPEC panel — assess the spec, and GROUND every claim in the real code)
1. **Grounding (verify LIVE — G1/G2/F2)**: the spec cites specific code (`cmd/mindspec/panel.go:~304` IsBead guard; `gate.go:186-189` the "references branch bead/" message; `plugins/mindspec/workflows/ms-panel.js:~127` `SHELL_METACHAR_RE`; `internal/panel` `Panel.Gate`; `internal/config/config.go:~540` `validateReviewerEntries`/`Reviewer.model()`; 112's `PanelGateExpectedReviewers`/`PanelGateApproveThresholdExpr`/`PanelGateAdvisoryDefault`). Read them — does the code actually exhibit the bug/shape the spec claims? A claim the tree doesn't support is a REJECT-worthy grounding failure.
2. **Falsifiability (F1/O1)**: each R has a real "Falsified if …" clause that a test could exercise. The resolutions (resolve-from-target, resolve-to-family) are pinned falsifiably.
3. **Fix soundness (O1/G2)**: R1 — does resolve-from-target actually compose with `PanelGateDecision`'s legs, and does the fbel.3-consistency fence hold (non-bead instruct/complete behavior genuinely unchanged)? R3 — does `--gate` stamping fit `panel.Create`/`CreateInput` + the 112 R9 stable contract, staying decision-inert? R4 — is resolve-to-family consistent with the typed-`string`-can't-distinguish reality? R2 — is rejecting bare `$` monotone (nothing legitimate contains `$`)?
4. **Impacted Domains parity (O3/G3)**: the declared domains (`workflow`, `core`) must be real + OWNERSHIP-resolvable (spec 110 R5's spec-approve parity gate checks this) — verify each against `.mindspec/domains/*/OWNERSHIP.yaml`.
5. **ADR Touchpoints (O2)**: each anchored `[ADR-XXXX](…)` link must resolve to an existing file under `.mindspec/adr/` (spec 110 R6's spec-approve parity gate checks this) — verify each file exists; ADRs cited in prose (not linked) are exempt.
6. **Scope/contradiction (F3/G3)**: the 4 fixes coherent + right-sized (decomposable into ~4 beads); Non-Goals correctly EXCLUDE the lifecycle bugs blp6/zty3; no contradiction with 110/111/112 (which these touch); no synonym-dodge or thinness.

## Per-slot lens defaults
- **F1 Fable** — spec-goal/falsifiability; **F2 Fable** — grounding (claims tied to real code); **F3 Fable** — contradiction/scope/thinness.
- **O1 Opus** — requirement soundness (the 3 fix designs + fences); **O2 Opus** — ADR touchpoints valid+resolvable; **O3 Opus** — impacted-domains parity + decomposition sizing.
- **G1 codex** — grounding empirical (verify the cited line numbers/bug against the real tree); **G2 codex** — fix-design feasibility (does resolve-from-target / --gate stamping / resolve-to-family actually work in the code?); **G3 codex** — completeness (any missing sub-requirement; the ACs runnable; right-sized for beads).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
