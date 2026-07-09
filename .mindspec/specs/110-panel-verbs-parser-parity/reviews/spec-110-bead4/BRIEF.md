# spec-110-bead4 — Round 1 Review Panel (8 reviewers)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.worktrees/worktree-mindspec-fbel.4`
**Branch**: `bead/mindspec-fbel.4`
**Commit under review**: `75ae9c6a86fcbc332f66e605437ac38bb73c3966` — `feat(110): cmd/mindspec panel create|verify|tally verb tree [mindspec-fbel.4]`
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, R8 codex. **Pass = ≥7 APPROVE, no REJECT.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `75ae9c6a`; leave `git status` clean. **Any scratch repo/config MUST use ABSOLUTE `/tmp` paths (or `t.TempDir()`) — NEVER a relative `.mindspec/...` write** (a relative-path scratch write + this harness's cwd-reset-between-Bash-calls corrupted a sibling worktree earlier this session).

## What the work does (bead fbel.4 — delivers spec 110 R1/R2/R3, pins R7a)
Adds the `cmd/mindspec` `panel create | verify | tally` verb tree — **thin adapters** over the single decision home (`panel.Create`, `panel.ResolveGateFacts`, `panel.PanelGateDecision`), never a second decision implementation. This is the CLI half of the ADR-0040 portability contract that lets the ms-panel workflow (spec 111) and skills drive panels via the binary instead of hand-authoring `panel.json`.

Verify each:
- **`panel create <slug> --spec <id> --target <ref> [--bead <id>] [--round N]`**: stamps `expected_reviewers`/`approve_threshold` from the 109 config resolvers (`cfg.PanelExpectedReviewers()`/`cfg.PanelApproveThresholdExpr()`, read-only) and `reviewed_head_sha` from the live `--target` ref via a swappable seam (`revParseForPanelFn`), resolves the panel directory **layout-aware** (same `workspace.DetectLayout`+`SpecDir` logic `internal/complete.panelGateRoots` uses), then calls `panel.Create` — the single co-bumping write of `panel.json` + BRIEF header.
- **`panel verify <slug>`**: read-only completeness/staleness report, **decision-identical to the gate**; writes nothing; exits 0.
- **`panel tally <slug>`**: per-slot verdicts + aggregate + decision + aggregated `concrete_changes_required`; **exit code derived from `Decision.Action` alone (`tallyExitAction`)**, never from raw verdict counts.
- **R7a single-home (the load-bearing invariant)**: every subcommand routes its decision through `panel.PanelGateDecision`, NOT a re-implemented matrix — pinned by `TestPanelVerbs_DecisionIsPanelGateDecision`. Confirm no second decision path exists in `panel.go`.

## The security-critical validator (R7 Fable + R8 codex — probe HARDEST)
`validatePanelSlug` + `rejectControlBytes` run **before ANY `filepath.Join`** in all three subcommands. They close two classes:
1. **path-traversal**: a slug/`--bead`/`--target` like `../../etc` must never escape the panel-directory root.
2. **terminal-injection** (the spec-109-final-review G2 class): a slug/`--bead`/`--target` bearing a control byte (`\n`/`\r`/NUL/ESC) must never reach a rendered message, a `guard.NewFailure` recovery line (where an embedded newline could forge a fake `recovery:` line, ADR-0035), or `panel.json`.
Try to defeat these empirically: hostile slugs (`../`, absolute paths, `\n`/`\r`/NUL/ESC embedded), hostile `--bead`/`--target` values. Any value that reaches a `filepath.Join`, a raw rendered message, or `panel.json` un-rejected is a REJECT-worthy defect. `TestPanelCreate_RejectsUnsafeSlugAndControlBytes` covers empty/`.`/`..`/traversal/newline-in-slug/`--bead`/`--target` — confirm it's exhaustive and genuinely falsifying.

## Files in scope (final state at 75ae9c6a — 5 files, +1175)
- `cmd/mindspec/panel.go` (+531) — the verb tree + validators
- `cmd/mindspec/panel_test.go` (+555) — tests incl. the 6 named acceptance tests
- `cmd/mindspec/root.go` (+1) — register the `panel` parent command
- `.mindspec/domains/workflow/interfaces.md` (+85) — doc-sync (Panel CLI Verb Tree section, appends on Bead 1's schema doc)
- `internal/redact/redact.go` (+3) — **necessary companion**: registers `panel`/`tally`/`verify` in the `CommandTokens`/`SubcommandTokens` closed-set enums that `redact_enum_drift_test.go` requires for ANY new cobra command (documented in commit). Confirm this is the drift-guard requirement and not scope creep.

## Do NOT expect changes to (leaf invariant)
`internal/panel` and `internal/config` are called only, never modified. `go list -deps ./internal/panel | grep internal/config` must be empty.

## Known pre-existing failures (NOT this bead)
`go test ./...` has 2 unrelated failures — `internal/harness` (timeout) and `internal/instruct` `TestRun_IdleNoBeads` (z4ps). Both reproduce on the parent `eece0bcc`; not this diff. The bead's own package (`cmd/mindspec`) is green.

## Per-slot lens defaults
- **R1 Opus** — author-of-record: diff matches plan Bead 4 (R1/R2/R3/R7a)? exactly the intended scope?
- **R2 Opus** — codebase-pin: the 6 named acceptance tests exist + pass (`go test ./cmd/mindspec/...`); named symbols/seams real?
- **R3 Opus** — R7a single-home + scope-fence: no second decision matrix (grep `panel.go` for any re-implemented tally/threshold logic); leaf invariant; redact enum necessary not creep.
- **R4 Sonnet** — empirical: build the binary (absolute /tmp), run `panel create`/`verify`/`tally` against a /tmp scratch repo; confirm stamping (resolvers → panel.json), verify is read-only+exit0, tally exit code from Decision.Action.
- **R5 Sonnet** — schema/type: `revParseForPanelFn` seam correctness, layout-aware dir resolution parity with `internal/complete.panelGateRoots`, `tallyExitAction` mapping, nil guards, error/recovery-line correctness.
- **R6 Sonnet** — next-bead integration: does this CLI surface match what fbel.5 (skills) + spec 111's ms-panel workflow + 112's resolvers expect? Is `panel create`'s output (the reported layout) consumable by the workflow?
- **R7 Fable** — sharpest adversarial on the SECURITY validators (see above) + any decision drift from the gate.
- **R8 codex** — empirical injection/traversal: hostile slugs/targets/beads through the built binary; confirm no traversal escape, no control byte in output/panel.json/recovery lines.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
