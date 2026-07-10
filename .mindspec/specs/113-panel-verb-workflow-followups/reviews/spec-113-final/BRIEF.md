# spec-113-final — Final Review Panel (12 reviewers, four families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-113-panel-verb-workflow-followups`
**Branch**: `spec/113-panel-verb-workflow-followups` @ **12c02de1b58dd931880afd5810f39dcb8a4092f6**
**Base**: `origin/main` — merge-base **45ae75772490db5fc59b65a0196b90c9a566e5c1**. Review the **three-dot diff** `git diff 45ae7577 12c02de1` (spec branch vs main).
**Panel**: 12 slots — F1–F3 Fable, O1–O3 Opus, S1–S3 Sonnet, **G1–G3 codex (real codex)**. **Pass = ≥11 APPROVE, no REJECT. Every raised finding is adjudicated (fixed or evidenced-refuted), NEVER out-voted by the APPROVE count.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `12c02de1`; scratch under ABSOLUTE /tmp only; leave `git status` clean. Write your verdict ONLY to the exact absolute path at the bottom.

## What spec 113 is
A followup-hardening wave from the just-shipped 110/111/112 batch — **4 bounded fixes**, each shipped as its own bead + 8-slot panel (all passed, findings adjudicated):
- **R1 (bead r6hk.1, LOAD-BEARING P2)** — truthful non-bead panel staleness in `panel verify`/`panel tally`. Was: non-bead panels (bead_id null) rev-parsed the empty `bead/` ref → false PASS after `--target` advanced + malformed `references branch bead/` message. Fix lives ENTIRELY in `cmd/mindspec` (a `nonBeadTargetRevParse` closure rev-parses `panel.json.target` via the `revParseForPanelFn` seam; a CLI-layer `sanitizeNonBeadDecision` rewrites the message; `tallyExitActionNonBead` in the RunE handler for the non-bead recovery) → **ZERO diff to `internal/panel`/`internal/instruct`/`internal/complete`** (the consistency fence). `tallyExitAction` stays 2-arg.
- **R2 (bead r6hk.2, P3)** — `SHELL_METACHAR_RE` in `ms-panel.js` folds a bare `$` into the char class (`/[\x60;|&\n$]/`), closing `$HOME`/`${x}`/`$x` expansion that survived `validateShellSafe`; `.claude/workflows/ms-panel.js` mirror byte-identical.
- **R3 (bead r6hk.3, P3)** — `panel create --gate <name>`: stamps the decision-inert `Panel.Gate` field (112's) from the single `config.PanelGateKeys` enum + resolves creation-time defaults via 112's gate-scoped resolvers (the writer-side stamping 112 deferred). Invalid gate → reject-before-write + 5-key recovery. Byte-identical panel.json when omitted.
- **R4 (bead r6hk.4, P3)** — reconciled 112's R4-vs-R1 `{model:"",family}` ambiguity → resolve-to-family (a superseding comment + `TestLoad_EmptyStringModel`; the code already resolved to family). No behavioral change.

## The code diff vs main (ignore review-artifact JSONs)
- `cmd/mindspec/panel.go` (+251), `cmd/mindspec/panel_test.go` (+654) — R1 + R3.
- `internal/panel/create.go` (+13), `internal/panel/create_test.go` (+77), `internal/panel/panel_test.go` (+85) — R3.
- `internal/config/config.go` (+18), `internal/config/config_test.go` (+77) — R4.
- `plugins/mindspec/workflows/ms-panel.js` (+5) + `.claude/workflows/ms-panel.js` (mirror) + `plugins/mindspec/workflow_test.go` (+19) — R2.
- `.mindspec/domains/workflow/interfaces.md` — doc-sync. `spec.md` + `plan.md` — lifecycle docs.
- The rest (109 files total) are TRACKED review artifacts (panel JSONs) — the house policy (2026-07-08) tracks review artifacts; they are not code.

## What to verify (whole-spec — probe the aggregate + each fix on the MERGED branch)
1. **Build + full test (all, esp. G1/S1)** — `go build ./...`; `go test ./...` green (note `internal/instruct` `TestRun_IdleNoBeads` is a KNOWN pre-existing flake `z4ps`/`7y4d`, env-dependent, unrelated — do NOT fail the spec on it; confirm it fails identically on main).
2. **R1 on the merged branch (O1/G1 — LOAD-BEARING)** — the zero-diff-to-internal fence holds NET on the branch: `git diff 45ae7577 12c02de1 -- internal/panel internal/instruct internal/complete` shows ONLY test files + create.go's +13 (R3's `CreateInput.Gate` + `p.Gate` stamp) + panel_test.go's decision-inert test — NO change to `PanelGateDecision`/gate.go decision logic, `instruct`'s verdict(), or `complete`'s gate. Non-bead staleness genuinely blocks after target advance (real-binary scenario). `sanitizeNonBeadDecision` leaks no `bead/` empty-interpolation on any leg.
3. **R3 decision-inertness (O2/G2)** — `PanelGateDecision`/`ApproveThreshold` never read `Panel.Gate` (only advisory `PanelGateAdvisoryDefault`); `--gate` validated against the SINGLE enum (no second copy); byte-identical when omitted; `internal/panel` stays config-free leaf.
4. **R2 security (G2/S2)** — the bare-`$` fix is monotone + no bypass: every user value (slug/spec/target/bead_id) routes through `validateShellSafe` before `buildCommand`'s shell string; mirror byte-identical. (This is the workflow whose plan-round RCE-class injection was caught+fixed earlier — probe escaping hard.)
5. **R4 (O2/S2)** — resolve-to-family documented + pinned; no behavioral change; existing 112 config tests green.
6. **Aggregate coherence (F1/F2/F3)** — all 4 R's + their spec ACs (AC1–AC4 + AC-global) are met on the branch; the fixes compose without contradiction; no regression to the panel-gate decision model, the workflow layer, or config validation.
7. **ADR compliance (O3)** — ADR-0037 (panel-gate single decision home — preserved, gate.go decision logic unchanged), ADR-0035 (recovery lines), ADR-0040 (workflow L2/L3 layering), ADR-0030 (subprocess budget — R1's one extra rev-parse). No new ADR needed.
8. **Impl-approve readiness (G3/O3)** — the branch merges cleanly to origin/main (three-dot diff touches nothing already changed on main since the merge-base); no cross-spec contamination; the review artifacts are appropriately tracked.

## Per-slot lens defaults
- **F1** — spec-goal delivery (4 R's + ACs met). **F2** — grounding (shipped code matches spec/plan). **F3** — cross-requirement coherence / no contradiction / thinness.
- **O1** — R1 correctness on merged branch + zero-internal-diff net fence (LOAD-BEARING). **O2** — R3 decision-inertness + R4 resolve-to-family. **O3** — ADR compliance + impacted-domains + no regression to complete/instruct gates.
- **S1** — full test suite green + the diff-vs-main is exactly the 4 fixes. **S2** — R2 shell-injection security + aggregate test coverage adequacy. **S3** — doc-sync (interfaces.md) accuracy + review-artifact/scope appropriateness.
- **G1 codex** — empirical whole-branch (build + `go test ./...` + real-binary scenarios: non-bead staleness blocks after advance, `--gate` stamps/rejects, bare-`$` rejected, resolve-to-family). **G2 codex** — adversarial security (injection/escaping/control-bytes across panel verbs + ms-panel.js; try to leak `bead/` or slip a `$`/invalid gate). **G3 codex** — integration + impl-approve readiness (clean merge to main; no conflict with current origin/main; suite green).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
