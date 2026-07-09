# spec-112-approve — Spec-Approval Panel (Round 1, 9 reviewers, three families)

**Spec worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config
**Spec under review**: `.mindspec/specs/112-per-gate-panel-config/spec.md` @ committed SHA **825f04c572acc88b872bf3f476c539e3799302b9** (read via `git show 825f04c5:.mindspec/specs/112-per-gate-panel-config/spec.md`, or the worktree file — it matches HEAD).
**Panel**: 9 reviewers, three families — F1–F3 Fable, O1–O3 Opus, G1–G3 GPT-5.5 (codex). Pass = **≥8 APPROVE, no REJECT**; any REJECT halts; a hard_block finding blocks regardless of votes.
**This is a SPEC-approval gate** (pre-`mindspec spec approve`): you are reviewing the SPECIFICATION for substance — falsifiability, grounding, internal consistency, completeness, and downstream implementability — NOT code (none exists yet).

## READ-ONLY RULE (mandatory, all 9)

Repo files are READ-ONLY. Write NOTHING except your verdict JSON at the prescribed path. Scratch under /tmp. Prefer reads pinned to the SHA (`git show 825f04c5:<path>`). Do NOT edit spec.md.

## What spec 112 is

Bead `mindspec-kkcw` is the decision record. Spec 109 (JUST MERGED to main) shipped a `panel:` config block with a SINGLE GLOBAL reviewer list `{family: claude|codex, count}` + one `approve_threshold`. It cannot express the operator's standing protocol: per-GATE reviewer mixes with explicit model ids across FOUR families (spec/plan = 3 Fable + 3 Opus + 3 GPT-5.5; bead = 3 Opus + 3 Sonnet; final = 12). Spec 112 extends `panel:` to a per-gate `gates:` map with a generalized reviewer entry `{model, lens, count}` (open model vocabulary), a global `substitutes:` map, a decision-inert recorded `gate` field on panel.json, gate-aware ReviewerCountNote, and `config show --gate`. It ratchets the richer schema on top of 109's minimal foundation per ADR-0040's L2 layer (forward-only, ADR-0023) — it does NOT re-open approved 109, add dispatch/spawning (111's job), or change gate-decision logic.

**Grounding context** (all verified present at draft time; re-verify in your lens): domains `core`/`workflow` exist under `.mindspec/domains/`; ADR-0040/0037/0035/0034/0023/0039 exist under `.mindspec/adr/`; 109's landed surface (`internal/config.Panel`, `PanelExpectedReviewers`/`PanelApproveThresholdExpr`, `internal/panel.ApproveThreshold`, `ReviewerCountNote`, `config show`) is on `main`. Reference: 109's spec is at `../109-orchestration-config-substrate/spec.md` (this worktree's `.mindspec/specs/`) — R2/R5/R6/R7/R8/R10 are the contracts 112 must not weaken.

## Key design points to scrutinize

- **The interleaved global-cursor lens rule (R3)** — the load-bearing new mechanism (this session's finding: lens ≥ model tier). Lens-less slots fill from an INTERLEAVED default ordering `[author-of-record, empirical-prober, codebase-pin, adversarial, contract-stability, integration]` via one cursor that advances only over lens-less slots. Worked example claim: a 9-reviewer 3×3 mix yields Fable{author,empirical,pin}, Opus{adversarial,contract,integration}, GPT{author,empirical,pin} — all six lenses, each entry spanning both structural and sharp. VERIFY this claim holds and the falsification is genuinely falsifiable.
- **Backward-compat (R2)**: absent `gates:` ⇒ behavior byte-identical to 109 (incl. the advisory-skip carve-out in R7 — skip only applies when `gates:` IS configured).
- **Open vocabulary** for model ids (never enum-checked; a curated known-model list drives a `config show` WARNING only, seeded with claude-fable-5/opus-4-8/sonnet-5/gpt-5.5 + legacy claude/codex, non-exhaustive by design).
- **Substitution**: global one-step map; substitute-also-unavailable = runtime HALT surfaced to the runner (out of config scope).
- **Leaf invariant preserved**: `internal/panel` stays config-free; `PanelGateDecision` takes no config; the recorded `gate` field is decision-inert.
- **note:** optional free-text, echoed by config show under 109's control-byte escaping, never consumed.
- **Scope**: no panel.json WRITER (110), no runner/dispatch (111), no machine-config layer (deferred to candidate spec 113 / `mindspec-pev1`).

## Slot lenses (each writes `<slot>-round-1.json`; reviewer_id "<slot> <family>")

- **F1 (Fable) — falsifiability auditor**: every Requirement's "Falsified if …" clause genuinely falsifiable + a concrete test/observable? Any requirement that's aspirational prose, not a checkable behavior? Are the 9 ACs each paired with a runnable proof?
- **F2 (Fable) — 109-compatibility & contract**: does 112 truly EXTEND 109 backward-compatibly? R2 byte-identity (gates-absent ⇒ 109), the leaf invariant (internal/panel no config import), single-interpreter (ApproveThreshold stays the only resolver — 112's config-side resolvers return RAW expressions), the R7 advisory carve-out. Any clause that would force re-opening approved 109?
- **F3 (Fable) — adversarial requirement-refuter**: pick ≥3 load-bearing claims (the lens rule, substitution precedence vs claude_sub_on_quota, unconfigured-adhoc=bead, the note field's inertness, gates-absent=109) and try to construct a config or scenario that REFUTES each. Report refuted/withstood per claim.
- **O1 (Opus) — grounding/repo-accuracy**: every factual claim about the repo (109 requirement refs, ADR ids/domains, `.mindspec/domains/` paths, the `internal/*` symbols 112 cites, the kkcw/pev1 bead refs, commit `ddd955f1` for adhoc routing) verified against the actual tree at this SHA. Flag any claim the tree does not support.
- **O2 (Opus) — contradiction hunter**: pairwise scan requirements + ACs + Non-Goals for mutual contradiction (esp. R7-skip vs R2-byte-identity, per-gate override vs global default precedence, substitutes-supersedes-claude_sub_on_quota vs the legacy meaning, the recorded `gate` field being both parse-lenient AND enum-by-convention). Force-resolve any real pair.
- **O3 (Opus) — completeness & scope**: anything the schema needs but omits (a config an operator would reasonably write that the spec doesn't cover)? Are Scope/Non-Goals/Out-of-Scope coherent and is the 111/110/113 boundary clean? Is the lens rule FULLY specified (deterministic, all edge cases — count=1, single entry, all-explicit-lenses, more entries than lenses)?
- **G1 (codex) — schema & validation implementability**: is the YAML schema + the R4 refusal matrix + the three resolver signatures (`PanelGateExpectedReviewers`/`PanelGateApproveThresholdExpr`/`PanelGateReviewerSlots`) sound and implementable in Go against 109's existing config structs? Any ambiguity a bead implementer would trip on? Do the refusals compose with 109's existing R5 refusals without conflict?
- **G2 (codex) — security/DoS & parse-surface**: the new open-vocab string fields (model, lens, note, substitutes keys/values) — does the spec carry forward 109's control-byte-escaping discipline (the final-review G2 finding) to `config show --gate` and the note field? Any injection/render/size concern the spec should pin (esp. since `config show --gate --json` is machine-consumed by 110/111)? Is the size-cap gap acknowledged?
- **G3 (codex) — downstream-consumer skeptic**: will spec 110 (panel verbs / ParseSpec) and spec 111 (workflow runner) actually consume 112's surface cleanly? Does `config show --gate <name> [--json]` + the resolvers give the 110 writer and the 111 runner what they need to stamp panel.json (the `gate` field) and select reviewer slots? Is the resolver-vs-decision boundary (creation-time defaults only; panel.json is sole gate authority) unambiguous for them?

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `/Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.mindspec/specs/112-per-gate-panel-config/reviews/spec-112-approve/<slot>-round-1.json` with keys: reviewer_id, verdict, confidence, rationale (≤200 words), concrete_changes_required (empty if APPROVE), findings (hard_block: true only for a genuine blocker).
