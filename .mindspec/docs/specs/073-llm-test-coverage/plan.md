---
status: Draft
spec_id: 073-llm-test-coverage
version: "1"
last_updated: "2026-03-05"
adr_citations:
  - id: ADR-0023
    sections: [Bead 1, Bead 2]
---
# Plan: 073-llm-test-coverage — Improve LLM test coverage and iteration

## ADR Fitness

ADR-0023 (beads-based phase derivation) is the primary touchpoint. The `skip_next` analyzer fix and scenario setup fixes both stem from the ADR-0023 migration — scenarios still reference retired focus files, and the analyzer doesn't account for beads-derived phase being empty in non-implement sessions.

## Testing Strategy

- Deterministic: `go test ./internal/harness/ -short -v` — validates analyzer fixes, assertion helpers, sandbox changes
- LLM: `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_<Name> -timeout 10m -count=1` — per-scenario validation
- Full suite: `env -u CLAUDECODE go test ./internal/harness/ -v -run '^TestLLM_' -timeout 180m -count=1` — regression check

## Bead 1: Fix skip_next analyzer false positives

**Steps**
1. In `detectSkipNext()`, add early bail-out: if no event has `Phase == "implement"` AND no `mindspec next` command appears in the event stream, return nil (skip_next is irrelevant)
2. Add deterministic tests: session with only spec-phase events and git commits should not trigger skip_next; session with implement-phase events but no `next` should still trigger
3. Run existing analyzer tests to confirm no regressions

**Verification**
- [ ] `go test ./internal/harness/ -short -v -run TestSkipNext` passes
- [ ] New test `TestSkipNext_NonImplementSessionNoViolation` passes
- [ ] Existing `TestSkipNext_Violation` still catches real violations

**Depends on**: None

## Bead 2: Fix scenario setup and assertions for simple tests

**Steps**
1. ApproveSpecFromWorktree: add `sandbox.CreateSpecEpic(specID)` to setup; increase MaxTurns from 10 to 15; add assertions for branch/worktree persistence
2. ApprovePlanFromWorktree: add assertions for bead creation (`assertBeadsMinCount`) and branch state (`assertHasBranches(t, sandbox, "bead/")`)
3. SpecApprove: add mode transition assertion — run `mindspec state show` in sandbox post-test and verify mode is `"plan"`
4. De-tautologize FromWorktree prompts: replace prescriptive prompts with outcome-oriented ones (e.g. "The spec is finished. Advance to the next phase.")
5. Clean up stale focus comments in scenario.go — update comments and commit messages that reference focus files to reflect beads-based phase derivation

**Verification**
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run 'TestLLM_(ApproveSpecFromWorktree|ApprovePlanFromWorktree|SpecApprove)' -timeout 30m -count=1` — all 3 pass
- [ ] No regressions in other simple scenarios (SpecInit, PlanApprove, ImplApprove, SpecStatus)

**Depends on**: Bead 1 (skip_next fix needed so SpecInit and PlanApprove don't false-positive)

## Bead 3: Stale git hook cleanup in mindspec setup claude

**Steps**
1. In `internal/hooks/install.go` (or `internal/setup/claude.go`), add a `CleanStaleGitHooks()` function that removes: files matching `*.backup`, `*.pre-mindspec` in `.git/hooks/`, and known-removed hooks (e.g. `post-checkout`)
2. Call `CleanStaleGitHooks()` from `mindspec setup claude`
3. Add deterministic test that creates stale hook files and verifies they're removed

**Verification**
- [ ] `go test ./internal/hooks/ -v -run TestCleanStaleGitHooks` passes
- [ ] `mindspec setup claude` in a repo with stale hooks removes them
- [ ] `make test` passes

**Depends on**: None

## Bead 4: Fix UnmergedBeadGuard setup failure

**Steps**
1. Investigate why `bd create spec epic` exits 1 in the UnmergedBeadGuard sandbox
2. Fix the setup — likely a beads initialization or epic creation ordering issue
3. Verify the scenario runs (may still fail on agent behavior, but setup must succeed)

**Verification**
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_UnmergedBeadGuard -timeout 10m -count=1` — setup completes without error
- [ ] Agent reaches at least turn 1 (not blocked by setup)

**Depends on**: None

## Bead 5: Validate and record improvements

**Steps**
1. Run all 7 simple scenarios in parallel, confirm 100% pass with 100% forward ratio
2. Run full suite, record results in TESTING.md history tables
3. Compare pass count vs baseline (11/18 from 2026-03-04)

**Verification**
- [ ] TESTING.md updated with new history rows for each changed scenario
- [ ] At least 14/18 pass (3+ improvement over baseline)
- [ ] No regressions in previously-passing scenarios

**Depends on**: Bead 1, Bead 2, Bead 3, Bead 4

## Provenance

| Acceptance Criterion | Bead | Verification |
|:---------------------|:-----|:-------------|
| 3+ failing scenarios pass reliably | Bead 5 | Full suite run |
| UnmergedBeadGuard setup fixed | Bead 4 | Setup completes |
| No regressions in 11 passing scenarios | Bead 5 | Full suite comparison |
| TESTING.md updated | Bead 5 | History rows added |
| Fixes in guidance/CLI, not prompts | Bead 1, 2 | Code review |
| Stale git hooks removed by setup | Bead 3 | Deterministic test |
| skip_next false positives fixed | Bead 1 | Deterministic + LLM tests |
| ApproveSpecFromWorktree MaxTurns increased | Bead 2 | Scenario definition |
