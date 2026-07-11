# spec-115-bead1 — Round 3 Bead Panel (8 reviewers)

**Bead worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate/.worktrees/worktree-mindspec-fgmg.1`
**Branch**: `bead/mindspec-fgmg.1` @ **e71a7991** (impl `75e82b52` + r1-fix `19bc8114` + r2-fix `e71a7991`). **Pass = 8/8 UNANIMOUS.** Findings never out-voted.

**READ-ONLY**: verdict JSON only; ABSOLUTE `/tmp` scratch; NEVER edit; leave `git status --porcelain` clean.

## Rounds 1-2 history → the round-3 structural fix
- Round 1 (7/1): R8 found `refutation_pending_entries: null` passed the fail-closed gate. Fixed (comma-ok present-null → error).
- Round 2 (7/1): R8 found the SIBLING — `panel_refuted_entries` corrupt/null passed when `pending` was empty, because its validation sat AFTER the `len(pending)==0` early return (a patch, not a structural close).
- **Round 3 fix (commit `e71a7991`, delta = `git diff 19bc8114..e71a7991`, only `internal/complete/panel_advisory.go` + test):** STRUCTURAL — `uncoveredPendingObligations` now validates BOTH metadata keys (present-null via comma-ok + decode) **up front, before ANY early return**. Order: (1) validate `refutation_pending_entries`; (2) validate `panel_refuted_entries` UNCONDITIONALLY (moved before the early return); (3) `len(pending)==0` no-op return; (4) coverage loop. No code path can skip a shape-validation. `TestPendingObligationPredicate` now 16 subtests (added: empty-pending+present-null-refuted→error; absent-pending+corrupt-refuted→error; both-absent→nil; valid-empty-pending+valid-empty-refuted→nil). The fixer empirically RED-on-revert-checked (moved validation back → the 2 new subtests failed → restored).

## What to verify at `e71a7991` (focus on the delta; re-confirm your lens)
1. **The corrupt-shape class is now CLOSED (R8's lens especially).** Every realizable metadata shape with a present-but-corrupt value in EITHER `refutation_pending_entries` OR `panel_refuted_entries` — regardless of whether pending is empty/absent — now returns a non-nil error. Try to find ANY remaining early-return or skip path that bypasses a shape-validation. If none remains, the class is closed. (R8: is there a round-4 sibling, or is the structural reorder exhaustive?)
2. **NO false-refuse (R2's lens especially).** Genuinely ABSENT keys still no-op: `{}` → nil (pristine bead passes). Valid present EMPTY arrays still pass: `{pending:[], refuted:[]}` → nil. A valid non-empty pending with valid covering refuted → unchanged coverage result. The restructure must NOT make any VALID input newly error (over-correction).
3. **NO regression (R6's lens especially).** `uncoveredPendingObligations` is still the single shared core; `reconcilePendingRefutations` calls it and its behavior on VALID inputs is byte-identical (the reconcile/`TestPanelRefuted_*` suite passes unchanged). No existing test expectation edited (delta is the reorder in .go + additive subtests).
4. **Scope + fences + green.** Delta touches ONLY `internal/complete/panel_advisory.go` + test. No approve/gitutil/docs; `git diff main -- internal/gitutil/` empty; `BranchExistsE`/`show-ref` 0-hit; `go build ./...` + `go test -count=1 ./internal/lifecycle ./internal/complete ./internal/executor` green; gofmt/vet/golangci-lint clean.
5. **Everything from rounds 1-2 still holds** — your prior APPROVE lens (author faithfulness, seam signatures, the four named tests RED-on-revert, AC12(a) deviation, byte-identity for consumers, scope/grounding) is undisturbed by a fix confined to the obligation core's validation ordering.

## Per-slot lens (as before; center on the delta)
- **R1 Opus** author/scope · **R2 Opus** fail-closed completeness + NO false-refuse over-correction · **R3 Opus** RED-on-revert of the new subtests · **R4 Sonnet** empirical (build/tests/16-subtests/fences/gofmt/vet/lint) · **R5 Sonnet** seam/type (signatures + shared-core intact) · **R6 Sonnet** no-regression (reconcile + consumers unchanged) · **R7 Fable** scope/grounding · **R8 codex** adversarial — verify the corrupt-shape class is definitively closed (no round-4 sibling) and no false-refuse introduced.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<slot>-round-3.json`: `reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
