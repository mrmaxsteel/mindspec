# spec-112-final-review — Final Review Panel (12 reviewers, four families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config`
**Branch**: `spec/112-per-gate-panel-config` @ **d713a0b79c0073382fd0a181abc57d0193bd2521**
**Panel**: 12 slots — F1–F3 Fable, O1–O3 Opus, S1–S3 Sonnet, G1–G3 codex/GPT-5.5. **Pass = ≥11 APPROVE, no REJECT.**
**This is the final gate before `mindspec impl approve` merges spec 112 → main.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `d713a0b7`; leave `git status` clean. **Any scratch config/binary MUST use ABSOLUTE `/tmp` paths (or `t.TempDir()`) — NEVER a relative `.mindspec/config.yaml` write** (that contaminated a sibling worktree during a bead panel this session).

## The spec's whole changeset (review THIS, not `git diff main`)
Spec/112 branched from the 109 merge `2f894401`; main has advanced since, so a two-dot `git diff main` misreads main's newer files (README/guides) as deletions. Review the **three-dot / merge-base** changeset — 112's actual contribution:
```
git -C <worktree> diff 2f894401..d713a0b7          # full 112 changeset (5469+/54-, 103 files incl. review artifacts)
git -C <worktree> diff 2f894401..d713a0b7 -- ':!*/reviews/*'   # code+docs only, excluding review JSON
```
Mergeability is already confirmed: `git merge-tree` reports 0 conflicts against current main.

## What spec 112 delivers (read `spec.md` in full — Goal + R1–R9 + AC1–AC10)
Extends 109's `panel:` config so the operator's four-family review protocol (spec/plan = 3 Fable+3 Opus+3 GPT = 9 pass≥8; bead = 3 Opus+3 Sonnet = 6 pass≥5; final = +3 Sonnet = 12 pass≥11; quota-wall Sonnet substitution keeping slot id) is **declared, parsed, validated config** instead of skill-prose + memory. One turn of ADR-0040's ratchet (L4 → L2). Backward-compatible: a 109-style global `reviewers` list still works and is the all-gates default; per-gate `gates:` entries override it.

**The decision authority does NOT move** (the load-bearing invariant): the recorded `panel.json` stays the sole input to `PanelGateDecision`; `internal/panel` stays a **config-free leaf**; config supplies creation-time defaults only. 112 adds schema + validation + gate-scoped resolvers + a machine-consumable `config show --gate [--json]` surface + one decision-INERT recorded field. **No dispatch, no spawning, no gate-logic change.** `models:`/`loop:`/`runner:` stay inert as in 109.

Delivered across 3 merged beads (each bead-panel-passed): lma4.1 (config schema: `gates` map + generalized reviewer entry + validation), lma4.2 (decision-inert recorded `gate` field), lma4.3 (gate-aware advisory at both call sites + `config show` gates/substitutes/known-model rendering + `config show --gate [--json]` R9 additive-only contract).

## Per-family lens assignments (12 distinct angles)

### Fable (adversarial / falsifiability)
- **F1 — spec-goal fidelity**: does the whole spec deliver its stated Goal and every AC1–AC10? Is the four-family protocol from the Goal's YAML actually expressible + resolvable in the shipped schema? Any AC with no implementation or no test?
- **F2 — cross-bead coherence**: do the 3 beads compose into ONE coherent feature with no half-wiring, dangling reference, or contradiction between schema (lma4.1), recorded field (lma4.2), and read surface (lma4.3)?
- **F3 — the R9 contract + injection**: is `config show --gate --json`'s 5-member contract genuinely additive-only/forward-compatible and sound for spec 111 to consume? Re-probe the escaping class (hostile config → no raw control bytes on any text path; JSON byte-exact). Use ABSOLUTE /tmp scratch.

### Opus (completeness / architecture)
- **O1 — requirement completeness**: walk R1–R9; each must have implementation AND a test that pins it. Flag any requirement only partially delivered.
- **O2 — ADR compliance**: ADR-0040 (this IS the L4→L2 ratchet — correctly layered? `internal/panel` still config-free leaf?), ADR-0037 (panel gate contract — `panel.json` still sole decision input?), ADR-0035 (guard recovery lines — the unknown-`--gate` 5-key recovery), ADR-0023 (forward-only — 109 not re-opened). Any divergence?
- **O3 — scope discipline / inertness**: does the changeset stay in the workflow domain? Is 112 genuinely INERT-but-declared (no dispatch/spawn/gate-logic activation; `models:`/`loop:`/`runner:` untouched from 109)? Any accidental enforcement activation?

### Sonnet (empirical / test quality / regression)
- **S1 — end-to-end protocol proof**: build the branch binary (absolute /tmp), write the Goal's full four-family protocol YAML, and confirm `config show` renders all gates in enum order with resolved sums (spec/plan 9, bead 6, final 12) and raw thresholds; `--gate <g> --json` gives exactly 5 members with `in_force` flipping; adhoc→bead fallback; unknown-gate + `--json`-without-`--gate` refusals; the 109 backward-compat (global-only config still works).
- **S2 — test quality**: are the ~1400 lines of new tests falsifiable and pinning real contracts (not decorative)? Do they cover the decision surface, the escaping class, the resolver rules, the backward-compat path? Spot-check by reverting a line and seeing a test fail.
- **S3 — regression**: run the full suite. Confirm no regression vs main in touched packages; the 2 KNOWN pre-existing failures (`internal/harness` timeout, `internal/instruct` `TestRun_IdleNoBeads` z4ps) reproduce on the merge-base and are NOT introduced by 112.

### codex/GPT-5.5 (empirical injection / schema / integration)
- **G1 — injection/escaping empirical**: drive a hostile config (ANSI ESC, BEL, embedded newlines forging `recovery:` lines) through note/model/lens/substitutes key+value/known-model warning/`--gate` name; confirm zero raw control bytes on text paths (`| cat -v`), JSON byte-exact round-trip via jq. Absolute /tmp scratch.
- **G2 — schema/type correctness**: the +450-line `internal/config` schema — validation completeness (bad gate names, bad thresholds, malformed reviewer entries, unknown models), resolver correctness (`PanelGateExpectedReviewers`/`PanelGateApproveThresholdExpr`/`PanelGateKeys` enum order/`KnownModels`), no nil-map-marshals-to-null or type-leniency holes.
- **G3 — downstream integration**: does 112's schema + R9 contract give spec 110 (panel writer, `panel create` stamps expected_reviewers/threshold from these resolvers) and spec 111 (workflow runner, consumes `--gate --json`) a clean, stable read surface? Any gap that would make 110/111 unable to consume 112?

## Your job
Evaluate the whole spec end-to-end against its Goal and AC1–AC10. A final-review APPROVE means: the spec delivers its goal, all requirements are implemented + tested, the load-bearing invariants hold (decision authority unmoved, panel leaf config-free, feature inert-but-declared), ADRs are honored, escaping is safe, it merges cleanly, docs are synced, no regressions.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", e.g. "F1 fable" / "O1 opus" / "S1 sonnet" / "G1 gpt-5.5"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
