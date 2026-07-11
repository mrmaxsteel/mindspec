# spec-115-bead2 — Round 2 Bead Panel (8 reviewers)

**Bead worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate/.worktrees/worktree-mindspec-fgmg.2`
**Branch**: `bead/mindspec-fgmg.2` @ **196f07d2** (round-1 impl `8b664359` + a COMMENT-ONLY fix commit). **Pass = 8/8 UNANIMOUS.** Findings never out-voted.

**READ-ONLY**: verdict JSON only; ABSOLUTE `/tmp` scratch; NEVER edit; leave `git status --porcelain` clean.

## Round 1 → the round-2 fix
Round 1 was **7 APPROVE / 1 REQUEST_CHANGES** (R7 Fable). The gate itself (all four legs, fail-directions, the 11 tests, AC4, deviations A–E, import edges) was cleared by all 8 lenses incl. the adversarial codex slot (R8) and the fail-direction lens (R2). R7's sole RC was two comment-level issues:
1. The `TestApproveImpl_NoCommitsNoBeads` comment falsely claimed the "no commits beyond main" refusal is "reachable via a spec that DOES have a valid plan." It is NOT: since `readPlanBeadIDs` errors on empty `bead_ids`, `len(beadIDs)==0 ⟺ planErr != nil`, and Leg 3 (`runOrphanObligationGate`) refuses on `planErr != nil` BEFORE the preflight — so past the gate the disjunction `(planErr != nil || len(beadIDs) == 0)` is always false and the `impl.go` preflight refusal is unreachable (a valid-plan zero-commit spec is the cleanup path and PASSES). This was the misleading-comment anti-pattern (the AC8 class).
2. A stale line citation (`orphan_gate_test.go` cited `impl.go:249` for `MergeBase`; this commit moved it to `:294`).

**The fix (commit `196f07d2`, delta = `git diff 8b664359..196f07d2` — restricted to the 3 code files + the round-1 review artifacts; the CODE changes are COMMENT-ONLY):**
1. Rewrote the `TestApproveImpl_NoCommitsNoBeads` doc comment: truthfully states Leg 3 subsumes the preflight's degenerate-plan refusal (unreachable in normal flow); removed the false "reachable via a valid plan" clause; the test pins Leg 3 intercepting the missing-plan state before the CommitCount preflight.
2. Added a truthful code comment at the `impl.go` preflight explaining the disjunction is subsumed by Leg 3 (unreachable; retained as a defensive backstop / for the CONSENSUS-revision-9 call-order pin; valid-plan zero-commit is the cleanup path).
3. Fixed the stale citation → `impl.go:294`, preserving `:249` explicitly as the spec's pre-115 Fact-1 pin (distinguished, not conflated).
A follow-up `mindspec` issue was filed for removing the vestigial preflight entirely (touches the call-order pin — out of scope for a comment fix).

## What to verify at `196f07d2`
1. **R7's two findings are ADDRESSED** (R7's lens especially): the `NoCommitsNoBeads` comment is now truthful (no false reachability claim; correctly states Leg-3 subsumption); the preflight code comment is accurate; the `:294` citation is correct (verify `grep -n 'MergeBase' internal/approve/impl.go` → the true line) and `:249` is labeled as the pre-115 Fact-1 pin.
2. **The fix is genuinely COMMENT-ONLY** — `git diff 8b664359..196f07d2 -- internal/approve/*.go` changes only lines inside `//` comments; NO executable line, NO test assertion changed. `TestApproveImpl_NoCommitsNoBeads`'s assertions are byte-identical to round 1.
3. **No regression** — everything the round-1 panel confirmed (all four legs + fail-directions, the 11 named tests RED-on-revert, AC4 anchor, seams/imports, deviations A–E, scope/fences) is undisturbed by a comment-only change. `go build ./...` + `go test -count=1 ./internal/approve ./internal/executor ./internal/complete ./internal/lifecycle` green; gofmt/vet/golangci-lint clean; `BranchExistsE`/`show-ref` 0-hit; `git diff main -- internal/gitutil/` empty.

## Per-slot lens (as round 1; center on the delta)
- **R1 Opus** author/scope · **R2 Opus** fail-direction (undisturbed by comments) · **R3 Opus** RED-on-revert (the 11 tests still discriminate; the `NoCommitsNoBeads` assertion is unchanged) · **R4 Sonnet** empirical (build/tests/fences/gofmt/vet/lint) · **R5 Sonnet** seam/type (unchanged) · **R6 Sonnet** no-regression + deviations (the comment fix doesn't alter behavior) · **R7 Fable** — YOUR two findings: verify both are truthfully addressed and the fix is comment-only · **R8 codex** adversarial: confirm the comment fix changed no behavior and the gate's fail-directions are intact; the preflight is honestly documented as Leg-3-subsumed.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<slot>-round-2.json`: `reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
