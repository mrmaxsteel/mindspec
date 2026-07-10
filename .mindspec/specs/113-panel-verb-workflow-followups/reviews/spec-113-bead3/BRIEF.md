# spec-113-bead3 — Round 1 Bead Panel (8 reviewers) — R3 (final bead)

**Bead**: `mindspec-r6hk.3` (spec 113, Bead 3 = R3). **Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-113-panel-verb-workflow-followups/.worktrees/worktree-mindspec-r6hk.3`
**Branch**: `bead/mindspec-r6hk.3` @ **d327cdce3dc68ec46aaed02ce0b2104852f5e8ee** — `feat(panel): panel create --gate stamps decision-inert gate field + gate-scoped creation defaults (R3)`
**Panel**: 8 slots — O1–O3 Opus, S1–S3 Sonnet, F1 Fable, **R8 sonnet-sub** (no codex on bead panels this session). **Pass = every finding adjudicated (fixed or evidenced-refuted) — a raised finding is NOT out-voted by the APPROVE count.**

**READ-ONLY RULE (MANDATORY)**: edit nothing but your verdict JSON; pin reads to `d327cdce`; scratch under ABSOLUTE /tmp only (or `t.TempDir()`); leave `git status` clean. Write your verdict ONLY to the exact absolute path at the bottom — do NOT create a `reviews/` dir inside the bead worktree.

## What the bead does
Adds `mindspec panel create --gate <name>` — the writer-side stamping spec 112 explicitly deferred. It stamps `panel.json`'s decision-inert `gate` field (112's field) and resolves creation-time defaults (`expected_reviewers`/`approve_threshold`) via 112 R3's gate-scoped resolvers, so a CLI/workflow-created `spec_approve`/`final_review` panel carries the gate identity its advisories key on.

## The design (verify each claim holds)
1. **`internal/panel/create.go`**: `CreateInput.Gate string` added; `Create` stamps `p.Gate = in.Gate`. The `gate` field keeps 112-R9's STABLE CONTRACT: name `gate`, type string, `omitempty`, parse-lenient. Omitted flag → `Gate==""` → NO `gate` key written (byte-identical panel.json to today).
2. **`cmd/mindspec/panel.go`**: `--gate` flag registered; validated against the SINGLE enum `config.PanelGateKeys` (internal/config/config.go:101 — `spec_approve`, `plan_approve`, `bead`, `final_review`, `adhoc`) via `isValidPanelGateKey` (iterates PanelGateKeys — NO second literal copy). Invalid value → `guard.NewFailure` naming ALL FIVE keys (ADR-0035), rejected BEFORE any write. `rejectControlBytes("--gate", gate)` applied like `--bead`/`--target`. When `--gate <g>` set, defaults resolved via `cfg.PanelGateExpectedReviewers(g)`/`cfg.PanelGateApproveThresholdExpr(g)`; when omitted, global resolvers (byte-identical to today).
3. **DECISION-INERT**: `PanelGateDecision`/`ApproveThreshold()` never read `Panel.Gate` (112 R6). `Panel.Gate` is read only by `PanelGateAdvisoryDefault` (pre-existing, spec 112) — advisory, not decision.

## Files in scope (final state at d327cdce)
- `internal/panel/create.go`, `internal/panel/create_test.go`, `cmd/mindspec/panel.go`, `cmd/mindspec/panel_test.go`, `.mindspec/domains/workflow/interfaces.md`

## What to verify (each concern → a disposition)
1. **Diff matches plan Bead 3 (O1)** — read `.mindspec/specs/113-panel-verb-workflow-followups/plan.md` Bead 3.
2. **DECISION-INERT (O2 — the core invariant)** — grep/read `PanelGateDecision`, `ApproveThreshold`, and every decision path: NONE reads `Panel.Gate`. The only reader is `PanelGateAdvisoryDefault` (advisory). If ANY decision logic reads the gate field, that's a REJECT-class finding. Also verify the gate-scoped resolvers (`PanelGateExpectedReviewers`/`PanelGateApproveThresholdExpr`) are used correctly at create-time (a `--gate final_review` panel gets that gate's reviewer count/threshold stamped).
3. **Byte-identical-when-absent (O3 — 112 R9 contract)** — omitting `--gate` writes NO `gate` key and produces a byte-identical panel.json to pre-113. The 112 pin `TestPanel_GateFieldDecisionInert` (and `TestPanelVerbs_DecisionIsPanelGateDecision`, `TestPanelTally_ExitCodeTracksDecision`) must be UNMODIFIED and green. Confirm `omitempty` on the marshalled field.
4. **Enum validation + recovery (S2)** — invalid `--gate` rejected BEFORE any write with a recovery line naming ALL FIVE keys; control bytes rejected (`rejectControlBytes`). CRITICAL: NO second copy of the gate-key enum — `isValidPanelGateKey` iterates `config.PanelGateKeys` (grep `"spec_approve"` in cmd/mindspec + internal/panel returns nothing). The leaf `internal/panel` stays config-free (`go list -deps ./internal/panel | grep internal/config` non-zero).
5. **Tests real (S1/S2)** — `TestCreate_StampsGate`, `TestPanelCreate_GateStampsPerGateDefaults`, `TestPanelCreate_GateInvalidRejectedBeforeWrite` (incl. control-byte row + writes-nothing snapshot check), `TestPanelCreate_GateOmittedByteIdentical`, `TestPanelCreate_GateAdvisoryReadThrough`. Are they real (drive the actual create/validate paths, snapshot before/after for the writes-nothing proof) or hollow?
6. **Scope + doc-sync (S3)** — exactly the 5 files, one commit; `interfaces.md` wording matches the plan + is accurate.
7. **Empirical (R8)** — real binary in a /tmp scratch repo: `panel create p113g --spec s --target main --gate final_review` → panel.json has `"gate":"final_review"` + the final_review gate's expected_reviewers/approve_threshold (cross-check `config show --gate final_review --json`); `--gate nonsense` → exit non-zero, 5-key recovery, NOTHING written (diff dir before/after); no `--gate` → no `gate` key, byte-identical. Run `go test ./cmd/mindspec ./internal/panel ./internal/config ./internal/complete`.
8. **Adversarial (F1)** — mutation-probe: (a) make some decision path read `Panel.Gate` in /tmp → does a decision-inert pin red? (if not, the inertness isn't pinned = finding); (b) revert the enum validation → does `TestPanelCreate_GateInvalidRejectedBeforeWrite` red? (c) can an invalid/control-byte gate slip past to a written panel.json? (d) does omitting --gate truly produce byte-identical output (compare bytes)?
Note: `internal/instruct`'s `TestRun_IdleNoBeads` is the KNOWN pre-existing z4ps flake (implementer confirmed it fails identically without this diff) — do NOT fail the bead on it.

## Per-slot lens defaults
- **O1 Opus** — author-of-record. **O2 Opus** — decision-inertness (the core invariant) + gate-scoped resolver correctness. **O3 Opus** — byte-identity-when-absent + 112 pins unmodified.
- **S1 Sonnet** — codebase-pin (symbols/tests exist + green). **S2 Sonnet** — enum validation + recovery line + no-second-enum + config-free-leaf. **S3 Sonnet** — scope fence + doc-sync.
- **F1 Fable** — adversarial (mutation-probe the decision-inertness + byte-identity + enum-validation pins; hunt a gate bypass). **R8 sonnet-sub** — empirical (real-binary AC3 + writes-nothing proof + full test suites).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to the EXACT absolute path `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id`, `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`.
