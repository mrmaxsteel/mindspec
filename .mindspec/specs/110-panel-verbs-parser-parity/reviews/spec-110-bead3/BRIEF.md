# spec-110-bead3 — Round 1 Review Panel (8 reviewers)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.worktrees/worktree-mindspec-fbel.3`
**Branch**: `bead/mindspec-fbel.3`
**Commit under review**: `25ad5ca91c18376d99dd5d9e4a7ca0d48181c10a` — `refactor(110): ratchet instruct verdict() onto PanelGateDecision [mindspec-fbel.3]`
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, R8 codex. **Pass = ≥7 APPROVE, no REJECT.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; scratch under /tmp; pin all reads to SHA `25ad5ca9`; leave `git status` clean. Edit nothing except your own verdict file.

## What the work does (bead fbel.3 — delivers spec 110 R2, the instruct half)

The single decision matrix (abandoned / round-mismatch / stale-SHA / incomplete / REJECT / threshold) lives in `panel.PanelGateDecision` (spec 099, on main). Two copies existed: `internal/complete` (the real merge gate) and `internal/instruct`'s `PanelStateEntry.verdict()` (the read-only advisory that powers `mindspec instruct --panel-state`, showing a "gate would PASS/BLOCK" preview). This bead **deletes the instruct copy** and ratchets `verdict()` onto `panel.PanelGateDecision`, so there is one decision home.

Key design (verify each):
- `PanelStateEntry.verdict()` now builds `panel.GateFacts` and returns `mapGateAction(panel.PanelGateDecision(facts))`: `panel.Allow → GatePass`, `panel.Warn → GateWarn`, `panel.Block → GateBlock`. `Decision.Message` is **empty on both Allow branches** (gate.go:142 no-registered-panel, :258 threshold-met) and **non-empty on every Warn/Block branch** — so Warn/Block relay `Decision.Message` verbatim as the one-line reason; **Allow synthesizes the reason locally** from the tally fields, reusing today's exact wording.
- The two pre-existing `TestPanelStateEntry_Verdict` rows that pin an Allow reason (`at_threshold_fresh` → "meets threshold 5/6"; `above_threshold_fresh` → "6/6 APPROVE") must **still pass with no test-file edit**.
- **Architectural split** the author chose: `panel.ResolveGateFacts` always re-Tallies from disk (I/O), which would break the fabricated-`Result` test rows if called inside the pure `verdict()`. So the author kept `verdict()` **pure** (no I/O) and moved fact-resolution into `gatherPanelState` (which does the I/O). Judge whether this preserves instruct's read-only-snapshot behavior and whether the seam is clean.
- The `BranchSHAResolver` `(sha, exists)` pair is adapted into `panel.GateIO` via an `errBranchGone` sentinel for the `IsRefNotFound` seam; `Worktree` always returns `""` → `WorktreeAbsent` true → the dirty-tree leg is **inert** (instruct has never done dirty detection). Non-bead panels (final-review/PR; BeadID null) build `GateFacts` with **no** bead rev-parse (HeadSHA empty) so the staleness leg stays inert — intended to be byte-identical to the old `if p.IsBead()` guard.
- The enum (`GatePass`/`GateBlock`/`GateWarn`) and `gateLabel` stay (still used by `renderPanelState`).

## DELIBERATE behavior change — scrutinize this explicitly (R5/R7 especially)

Full delegation means a `Result` with `Panel == nil AND PanelErr == nil` (a truly-unregistered registration) now maps through `PanelGateDecision`'s **malformed-registration → Block** branch, whereas the old instruct copy locally computed a **fail-open Allow** for it. The author asserts this path is **dead code** per `panel.Scan()`'s guarantees (Scan never yields a nil Panel without setting PanelErr) and pins it with the new test row `malformed_registration_nil_panel`. Two questions for the panel:
1. Is that path genuinely unreachable given `Scan()`'s contract? (Grep `panel.Scan`/how `Result.Panel`/`PanelErr` are populated.)
2. If it were reachable, is fail-**closed** (Block) safe here? Note `verdict()` drives an **advisory display line** in `instruct --panel-state`, not an actual merge decision (the real gate is `internal/complete`), so the blast radius is the preview label, and Block is the conservative direction. Decide whether the change is acceptable/in-scope for "ratchet onto the single home."

## Files in scope (final state at 25ad5ca9)
- `internal/instruct/panelstate.go` (+218/−99 region) — the refactor
- `internal/instruct/panelstate_test.go` (+164) — new `TestPanelStateVerdict_DelegatesToPanelGateDecision` (14-row table) + a source-text check (panelstate.go contains `panel.PanelGateDecision(` and none of the 4 pre-refactor message literals)
- `.mindspec/domains/workflow/overview.md` (+1) — doc-sync

## Shared modules reused (unchanged — do NOT expect edits here)
- `internal/panel`: `PanelGateDecision`, `ResolveGateFacts`, `GateFacts`, `GateIO`, `Allow`/`Warn`/`Block` — spec-099 code on main. This bead must NOT change their semantics.

## Known pre-existing failure (NOT this bead)
`go test ./internal/instruct/...` fails **only** `TestRun_IdleNoBeads` — it reads real cwd/repo state and trips the "multiple active specs" branch because this worktree tree has 110/111/112 active. This is the documented **z4ps** test-isolation leak, failing identically on the parent commit `e32eeaed`. Not caused by this bead; do not weigh it against the diff. Every other instruct test — including the two pinned Allow-reason rows and the new 14-row delegation test — passes.

## Your job

Evaluate the work cold against the bead scope (spec 110 R2). Confirm: the delegation mapping is correct; the two pinned rows pass unchanged; the pure-verdict/IO-in-gatherPanelState split is sound and preserves read-only snapshot behavior; the GateIO adaptation keeps the dirty-tree + non-bead staleness legs inert exactly as before; the deliberate nil-Panel Block change is dead-code / safe / in-scope; the new test table actually spans the decision surface; doc-sync is accurate; scope fence honored (only instruct + doc; panel package untouched).

Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", e.g. "R1 opus" / "R7 fable" / "R8 gpt-5.5"), `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
