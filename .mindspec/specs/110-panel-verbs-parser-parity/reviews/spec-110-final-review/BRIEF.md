# spec-110-final-review — Final Review Panel (12 reviewers, four families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity`
**Branch**: `spec/110-panel-verbs-parser-parity` @ **2da52b094204f01873d4494943faa382fd2a296c**
**Panel**: 12 slots — F1–F3 Fable, O1–O3 Opus, S1–S3 Sonnet, G1–G3 codex/GPT-5.5. **Pass = ≥11 APPROVE, no REJECT.**
**This is the final gate before `mindspec impl approve` merges spec 110 → main.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `2da52b09`; leave `git status` clean. **Any scratch repo/config/binary MUST use ABSOLUTE `/tmp` paths (or `t.TempDir()`) — NEVER a relative `.mindspec/…` write** (a relative-path scratch write + this harness's cwd-reset corrupted a sibling worktree earlier this run).

## The spec's whole changeset (review THIS)
Spec/110 branched from the 109 merge `2f894401`; review the three-dot / merge-base changeset — 110's actual contribution (135 files, +6722/−272 incl. review artifacts):
```
git -C <worktree> diff 2f894401..2da52b09                         # full 110 changeset
git -C <worktree> diff 2f894401..2da52b09 -- ':!*/reviews/*' ':!*/review/*'   # code+docs only
```

## What spec 110 delivers (read `spec.md` in full — Goal + R1–R8, each with a falsification clause)
Two outcomes:
1. **`mindspec panel create | verify | tally`** — the panel lifecycle (today in `ms-panel-run` step 0 + `ms-panel-tally`'s decision matrix + hand-typed `panel.json`) becomes three agent-neutral CLI verbs. `create` atomically writes panel dir + BRIEF stub + `panel.json`, stamping `expected_reviewers`/`approve_threshold` from 109's resolvers and `reviewed_head_sha` co-bumped with `round`. `verify` is a read-only PASS/BLOCK preview **computed by `panel.PanelGateDecision`** (identical to the complete-gate). `tally` renders the decision **from the binary** (verdict table + aggregate + decision + aggregated CCR; exit 0 Allow / non-zero Block). This makes `internal/panel/gate.go` the **single decision home** — the ADR-0040 ratchet.
2. **Spec-approve parser parity** — `mindspec spec approve` now runs the SAME canonical parsers the downstream gates use: `contextpack.ParseSpec` + `normalizeImpactedDomains` (every Impacted-Domain resolvable) and `## ADR Touchpoints` link resolution against the same `adr.Store` (cited ADRs exist). Catches the spec-108 formatting incident at spec-approve, cheaply.

Delivered across 5 merged beads (each bead-panel-passed 8/8 or via fix round): fbel.1 (panel.Create writer + schema doc), fbel.2 (validate spec-approve parser parity R5/R6), fbel.3 (instruct verdict() ratchet onto PanelGateDecision), fbel.4 (cmd panel create/verify/tally verb tree + validators), fbel.5 (skills thinned onto the verbs, judgment kept).

## The load-bearing invariants (R7 — verify hardest)
- **Single decision home**: `panel verify`, `panel tally`, `internal/complete`, AND `internal/instruct --panel-state` ALL route through `panel.PanelGateDecision` — NO second matrix anywhere (grep for a surviving `verdict()` matrix or any re-implementation).
- **Config-free leaf**: `go list -deps ./internal/panel | grep internal/config` MUST be empty; `panel.Create` takes plain values, imports no config/git.
- **No `PanelGateDecision` semantic change**: same inputs/outputs as before.
- **Parser parity is behavior-identical, merely earlier**: spec-approve adds NO stricter rule — a bare-name-no-manifest domain that plan-approve tolerates must still pass spec-approve (R5 falsification); touchpoint check is existence-only, anchored-links-only (a bare prose `ADR-####` is NOT resolved — 110's own `ADR-0040` prose mention must pass).
- **Security**: `validatePanelSlug`+`rejectControlBytes` (now full C0/C1/DEL via `unicode.IsControl`) close path-traversal + terminal-injection before any `filepath.Join`.

## Integration note for G3 (and all): 112 is already on main
Spec 112 (per-gate panel config) merged to main (PR #183) during this run. 110 and 112 both edit `.mindspec/domains/workflow/{architecture,interfaces}.md`, so **spec/110 has a known 3-marker merge conflict against current main** on those two docs — this will be resolved at impl-approve (a content merge of both specs' doc additions). Assess whether the two specs' workflow-doc additions are **semantically compatible** (not contradictory) so the resolve is clean. Also: does spec 111 (workflow-panel-runner, plan-approved) consume 110's verbs + panel-artifact schema cleanly?

## Per-family lens assignments (12 distinct angles)

### Fable (adversarial / falsifiability)
- **F1 — spec-goal fidelity**: does 110 deliver BOTH outcomes and satisfy every R1–R8 falsification clause? Any AC with no implementation/test?
- **F2 — cross-bead coherence**: do the 5 beads compose (writer → parser-parity → instruct ratchet → CLI verbs → skills) with no half-wiring/contradiction? Is the single-home invariant genuinely real across ALL four consumers?
- **F3 — security + honest boundaries (R7)**: the slug/control-byte validators (C0/C1/DEL, traversal) hold on every path; the R7 boundaries (no gate-semantic change, config-free leaf, no plan-work moved) all hold. Use ABSOLUTE /tmp scratch.

### Opus (completeness / architecture)
- **O1 — requirement completeness**: walk R1–R8; each needs implementation AND a test that pins its falsification clause.
- **O2 — ADR compliance**: ADR-0037 (identical decision, single home §3), ADR-0040 (leaf + artifact/CLI contract), ADR-0035 (recovery lines on tally Block + the new spec-approve errors), ADR-0036 (domain-resolution parity, gate-forward doc-sync), ADR-0032 (touchpoint existence-only boundary), ADR-0034 (no new ceremony).
- **O3 — scope discipline / config-free leaf**: `go list -deps ./internal/panel` shows no `internal/config`; `PanelGateDecision` inputs/outputs unchanged; spec-approve emits no `adr-coverage-missing`/`adr-cite-irrelevant`.

### Sonnet (empirical / test / regression)
- **S1 — end-to-end**: build binary (abs /tmp); run `panel create`→`verify`→`tally` against a scratch repo (stamping, co-bump, read-only verify exit 0, tally exit tracks decision); AND `spec approve` parser parity — a mangled path-like Impacted-Domain caught at spec-approve, a bad ADR-touchpoint link caught, a bare-name domain + a bare-prose ADR mention correctly TOLERATED.
- **S2 — test quality**: are the tests falsifiable and pinning the R-clauses (not decorative)? Spot-check by reverting a line.
- **S3 — regression**: full `go test ./...`; confirm no regression; the 2 KNOWN pre-existing failures (`internal/harness` timeout, `internal/instruct` `TestRun_IdleNoBeads` z4ps — the latter env-dependent on the repo's live multi-spec state) reproduce at the merge-base and are NOT introduced by 110.

### codex/GPT-5.5 (empirical injection / schema / integration)
- **G1 — injection/traversal empirical**: hostile slugs (`../`, absolute, C0/C1/NUL/ESC) + `--bead`/`--target` through the built binary; confirm rejection before `filepath.Join`, no file escapes the panel root, no raw control byte in output/`panel.json`/recovery lines.
- **G2 — schema/type + parser-parity**: the `panel.json`/verdict-file schema doc matches `internal/panel` constants (`FileName`/`verdictFileRE`/`ConsolidatedName`); the spec-approve parity uses the IDENTICAL resolver (`normalizeImpactedDomains`) + `contextpack.ParseSpec` + `adr.Store` as the downstream gates, same `impacted-domains-resolve` code, anchored-link-only touchpoint extraction.
- **G3 — integration**: 110 verbs + panel schema are a clean, stable contract for spec 111's ms-panel workflow (read 111's plan); AND assess the 110-vs-112 workflow-doc merge (compatible additions, clean resolve).

## Your job
Evaluate the whole spec end-to-end against Goal + R1–R8. APPROVE means: both outcomes delivered, all requirements implemented + tested to their falsification clauses, the load-bearing invariants hold (single home, config-free leaf, no gate-semantic change, parser-parity behavior-identical-merely-earlier), ADRs honored, security closed, docs synced, no regressions, and 110 composes cleanly with 112-on-main + 111.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
