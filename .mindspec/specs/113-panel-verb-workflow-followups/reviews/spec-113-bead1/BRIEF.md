# spec-113-bead1 — Round 1 Bead Panel (8 reviewers) — LOAD-BEARING (R1)

**Bead**: `mindspec-r6hk.1` (spec 113, Bead 1 = R1, P2). **Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-113-panel-verb-workflow-followups/.worktrees/worktree-mindspec-r6hk.1`
**Branch**: `bead/mindspec-r6hk.1` @ **b641a9fe4d7a855433db8c693d3da154ae6ec76a** — `feat(panel): truthful non-bead staleness in panel verify/tally via CLI-layer target rev-parse + sanitizeNonBeadDecision (R1); zero internal diff`
**Panel**: 8 slots — O1–O3 Opus, S1–S3 Sonnet, F1 Fable, **R8 sonnet-sub** (Claude standing in for codex; no codex on bead panels this session). **Pass = every finding adjudicated (fixed or evidenced-refuted) — a raised finding is NOT out-voted by the APPROVE count.**

**READ-ONLY RULE (MANDATORY)**: edit nothing but your verdict JSON; pin reads to `b641a9fe`; scratch under ABSOLUTE /tmp only (or `t.TempDir()`); leave `git status` clean. Write your verdict ONLY to the exact absolute path given at the bottom — do NOT create a `reviews/` dir inside the bead worktree.

## What the bead does
Fixes the non-bead panel staleness bug: today `resolvePanelGateFacts` (cmd/mindspec/panel.go ~302) guards on `IsBead()`, leaving `beadID=""` for a NON-BEAD panel → `internal/panel/gate.go` (~372) rev-parses the literal `bead/` → MissingRef → leg (5) Warn `references branch bead/` (gate.go:186-189), short-circuiting staleness/vote legs, so a non-bead panel keeps reporting PASS after its `--target` advances. The fix resolves staleness from the recorded `panel.json.target` through the SAME `PanelGateDecision` legs and fixes the malformed messages — **entirely in `cmd/mindspec`**.

## The design (verify each claim holds)
1. **ZERO-BYTE diff to `internal/panel`, `internal/instruct`, `internal/complete`** — the whole fix is in `cmd/mindspec` + a doc. `git show --name-only b641a9fe` = exactly `cmd/mindspec/panel.go`, `cmd/mindspec/panel_test.go`, `.mindspec/domains/workflow/interfaces.md`. No file under those three internal packages.
2. **Fact-gathering**: `resolvePanelGateFacts` builds a `nonBeadTargetRevParse` closure (for non-bead panels) that rev-parses `reg.Panel.Target` via the existing `revParseForPanelFn` seam instead of the doomed `"bead/"+""`. Bead path stays byte-identical (`exec.RevParseRef`). `GateIO.IsRefNotFound` still `exec.IsRefNotFound` (= `errors.Is(err, gitutil.ErrRefNotFound)`).
3. **Message hygiene in the CLI render layer only** — `sanitizeNonBeadDecision(d, slug, target)` rewrites the returned `Decision.Message` for non-bead panels: strips `panel.RawMergeFence("")`, replaces the `references branch bead/` fragment with a target-naming advisory, renames transient-error fragments; **never touches `Decision.Action`**. Applied by `renderPanelVerify`/`renderPanelTally` only when `res.Panel == nil || !res.Panel.IsBead()`.
4. **`tallyExitAction(d, slug)` UNCHANGED (2-arg, line ~620)** — its pinned test caller + `TestPanelTally_ExitCodeTracksDecision` stay UNMODIFIED. The non-bead Block recovery is a new `tallyExitActionNonBead(d, slug, target)` invoked in `panelTallyCmd`'s RunE handler (branch on `reg.Panel.IsBead()`), so no `mindspec complete <bead>` string reaches a non-bead panel.
5. **`panel verify` = read-only (always exit 0); `panel tally` = block-capable (non-zero on Block).**

## Files in scope (final state at b641a9fe)
- `cmd/mindspec/panel.go`, `cmd/mindspec/panel_test.go`, `.mindspec/domains/workflow/interfaces.md`

## What to verify (this is the LOAD-BEARING bead — probe hard; each concern → a disposition)
1. **Zero-diff fence (O1/O3)** — `git show --name-only b641a9fe` lists ONLY the 3 files; NO `internal/panel|instruct|complete`. The consistency fence holds BY not editing those packages.
2. **`sanitizeNonBeadDecision` correctness (O2/F1) — the sharpest check.** Read it. For EVERY leg a non-bead panel can hit (2 unreadable, 4 round-mismatch, 5/5b missing-ref, 6 staleness, 8 incomplete, 9 REJECT, 10 threshold): does the sanitized Message contain NO `bead/` empty interpolation (`references branch bead/,`, `git merge bead/ `), NO `mindspec complete <bead>` recovery, and name the recorded target instead? Does it NEVER mutate `Decision.Action`? Is `TestSanitizeNonBeadDecision` a REAL table test built from the real `panel.PanelGateDecision` (not hardcoded)? Any leg it misses = a finding.
3. **Non-bead staleness actually BLOCKS now (S2/R8)** — the spec's falsification: after `panel create <slug> --spec <id> --target <ref>` (no `--bead`) then advancing `<ref>` by one commit, `panel verify` output contains NO `PASS` and NO `references branch bead/`, and `panel tally` exits non-zero. Also: a non-bead panel with a REJECT verdict (or < expected_reviewers) at an un-advanced target still does NOT PASS. The incomplete/REJECT/threshold legs (8)-(10) are now reachable.
4. **Missing-ref uses the REAL classification (O2/R8)** — the missing-target-ref test stubs an error WRAPPING `gitutil.ErrRefNotFound` (`fmt.Errorf("...%w", gitutil.ErrRefNotFound)`) so `IsRefNotFound` is genuinely exercised, not a fake constant.
5. **Bead-panel behavior byte-identical (O3/S1)** — the pinned tests `TestPanelTally_ExitCodeTracksDecision` + `TestPanelVerbs_DecisionIsPanelGateDecision` are UNMODIFIED and green; `go test ./internal/panel ./internal/complete` green with zero test files touched. (Note: `internal/instruct`'s `TestRun_IdleNoBeads` is a KNOWN pre-existing flake `z4ps` — env-dependent, unrelated to this diff which has zero instruct diff; the panel-related instruct tests incl. `TestPanelStateVerdict_DelegatesToPanelGateDecision`'s `non_bead_panel` subtest pass. Do not fail the bead on z4ps.)
6. **Scope/doc-sync (S3)** — exactly the 3 files, one commit; `interfaces.md` wording updated to match the plan (verify=read-only / tally=block-capable + non-bead-target staleness).
7. **Empirical (R8 sonnet-sub)** — build the binary + a scratch repo; actually run the create→advance→verify/tally scenario and paste the output; run the full `go test ./cmd/mindspec` + `./internal/panel ./internal/complete`. Try to BREAK it: can you get a non-bead panel to PASS after its target advances, or make `sanitizeNonBeadDecision` leak a `bead/` empty interpolation on some leg?

## Per-slot lens defaults
- **O1 Opus** — author-of-record (diff ↔ plan Bead 1) + zero-diff fence + tallyExitAction 2-arg. **O2 Opus** — `sanitizeNonBeadDecision` correctness across ALL legs + the missing-ref real-classification test. **O3 Opus** — consistency fence (bead-panel byte-identical, pinned tests unmodified, complete/instruct untouched).
- **S1 Sonnet** — codebase-pin (symbols/tests exist + green). **S2 Sonnet** — non-bead staleness behavior (falsification clause: advanced target ⇒ Block; REJECT/incomplete legs reachable). **S3 Sonnet** — scope fence + doc-sync wording.
- **F1 Fable** — adversarial/hollow-test (mutation-probe: can a non-bead panel still PASS after target advance? is any test hollow? does sanitize really strip on real message strings?).
- **R8 sonnet-sub** — empirical prober (run the scratch-repo scenario end-to-end + full test suites; try to break it).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id`, `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`.
