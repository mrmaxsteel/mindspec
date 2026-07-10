# spec-114-bead1 — Round 2 Bead Panel (8 reviewers) — fix-up re-verify

**Bead**: `mindspec-mvp8.1`. **Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-114-panel-gate-block-on-request-changes/.worktrees/worktree-mindspec-mvp8.1`
**Branch**: `bead/mindspec-mvp8.1` @ **a2f2451afe50f0c756100d4a8c597631c90ce507** (round-1 tip was `4dfebbcb`).
**Panel**: same 8 slots — O1–O3 Opus, S1–S3 Sonnet, F1 Fable, R8 sonnet-sub. **Pass = every finding adjudicated; never out-voted.**

**READ the round-1 BRIEF too** (`BRIEF.md`, same dir) for full context on the bead. This round is a FOCUSED re-verify of the round-1 fix-up — but you still own your round-1 lens for regression.

**READ-ONLY RULE (MANDATORY, HARDENED — this session already had a /tmp-fence violation that corrupted the shared worktree)**: Do ALL build/test/mutation work in an ABSOLUTE `/tmp` copy (`git archive` or `rsync`), NEVER in the live worktree. Do NOT edit `internal/panel/gate.go` (or anything) in the shared worktree — even transiently. Pin reads to `a2f2451`. Write your verdict ONLY to the absolute path at the bottom. Leave `git status` clean.

## What changed since round 1 (the entire round-2 delta — 3 files, comment/test only)
Round 1 was 7 APPROVE / 1 REQUEST_CHANGES. The single RC (S2) proved one test row was hollow. The fix commit `a2f2451` (diff `git diff 4dfebbcb a2f2451`):
1. `internal/panel/panel_decision_test.go` — the "two unresolved RC slots → Block naming both" row renamed its RC slots `x`/`y` → `revA`/`revB`. Rationale: `x`/`y` are substrings of fixed Block-message text (`/ms-bead-fix` has `x`; `bypass`/`only` have `y`), so the `mustHave: ["x","y",...]` passed even with leg 9.5 disabled. `revA`/`revB` appear ONLY via the `%s` slot-list substitution → the row now genuinely discriminates leg 9.5's multi-slot naming. (4/6 approve fact unchanged — a 2-RC row is inherently sub-threshold; that's correct, the rename is the fix.)
2. `internal/complete/panel_gate_e2e_test.go` `TestPanelGate_RequestChangesBlocksComplete` — added `assert msg contains "unresolved REQUEST_CHANGES"` so RED-on-revert pins the block to leg 9.5, not an incidental doc-sync gate overlap.
3. `internal/panel/gate.go` — comment reword only: "byte-superset" → "substring-set superset" (accuracy; no logic change). Same reword in a panel_decision_test.go doc comment.

## What to verify (round 2)
1. **The S2 fix is real (S2/F1/R8 — the crux)** — MUTATION-VERIFY in a /tmp copy: disable leg 9.5 (`if unresolved := f.Res.UnresolvedVerdicts(); false && len(unresolved) > 0 {`), run `go test ./internal/panel -run UnresolvedRequestChanges -v`. ALL THREE rows (incl. "two unresolved RC slots") must now FAIL — the target row must report `message missing "revA"`/`"revB"`. Confirm `revA`/`revB` are NOT fixed substrings of the gate.go message format (they may appear only via `%s`). Restore/delete the /tmp copy. If the row still passes under the mutation, the fix is inadequate = REQUEST_CHANGES.
2. **e2e hardening sound (O1/S2)** — the new `"unresolved REQUEST_CHANGES"` assertion in `TestPanelGate_RequestChangesBlocksComplete` matches the actual leg-9.5 message text (so the test still PASSES on the real code), and genuinely pins the block to leg 9.5.
3. **No production logic changed (O1/O2/O3/S3)** — confirm `git diff 4dfebbcb a2f2451 -- internal/panel/gate.go` is COMMENT-ONLY (one word) and `internal/panel/tally.go` is untouched. Leg 9.5 / UnresolvedVerdicts / VoteDecision are byte-identical to the round-1-approved code.
4. **No regression / scope intact (all)** — the bead is still exactly the R1 change (now 9 files touched vs plan base + this delta; NO internal/bead, NO refutation code). `go build ./...`, `gofmt -l ./cmd ./internal` (empty), `go vet ./internal/panel ./internal/complete`, `go test -count=1 ./internal/panel ./internal/complete ./cmd/mindspec` all green (z4ps `TestRun_IdleNoBeads` flake in internal/instruct pre-existing — ignore). All round-1-verified properties (leg-9.5 placement/layering, VoteDecision lockstep, byte/substring-superset message, 4 intended fixture flips + no unclassified flip) still hold.

## Per-slot lens (same as round 1, applied to the delta + regression)
- O1 leg-9.5 correctness (unchanged) + e2e assertion soundness. O2 VoteDecision lockstep + message superset (unchanged). O3 no-regression / contract. S1 codebase-pin (symbols/tests green at a2f2451). S2 tests-real — re-run YOUR round-1 mutation, confirm the row is now discriminating. S3 scope + AC5 fixture classification (the rename is outcome-preserving: the row still Blocks). F1 adversarial re-probe (mutation-kill the fixed row; confirm nothing else regressed). R8 empirical (build/gofmt/vet/test in /tmp clone; mutation-verify).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-2.json` — keys `reviewer_id`, `verdict`, `confidence`, `rationale` (≤170 words), `concrete_changes_required`, `findings`.
