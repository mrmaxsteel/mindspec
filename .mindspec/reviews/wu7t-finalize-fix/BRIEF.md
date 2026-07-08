# wu7t-finalize-fix — Round 1 Review Panel

**Worktree**: /Users/Max/replit/mindspec/.claude/worktrees/agent-a9242fc424939a00f (branch checked out; you may run `go test` here)
**Repo (for diffs)**: /Users/Max/replit/mindspec
**Branch**: fix/wu7t-protected-main-finalize
**Commit under review**: 6f558bd66790e2c2bd1aded744fb3ff3fdf1463f — fix(approve): land finalize JSONL on a from-main branch when the spec branch is already merged (mindspec-wu7t)
**Panel type**: ad-hoc standalone-bug panel (bead mindspec-wu7t, **P1**) — NOT a spec-lifecycle gate. Bead-review model mix: 3 Opus + 3 Sonnet.

## What the work does

P1 bug: on protected-main repos, `mindspec impl approve` closes the epic/beads in Dolt, then `FinalizeEpic` auto-commits the refreshed `.beads/issues.jsonl` onto the SPEC branch and pushes it, expecting a PR to carry it to main. If the implementation PR was ALREADY merged (common), that finalize commit orphans on a dead branch; main's committed JSONL stays stale, and the bd post-merge hook re-imports it on every merge/FF, silently REVERTING the closes in Dolt (observed live on spec 106).

The fix, in `FinalizeEpic`'s remote/"pr" path only: capture `preFinalizeTip` before the finalize auto-commit; after it, `git fetch origin main` (new `gitutil.FetchRemoteBranch`) and check `IsAncestor(preFinalizeTip, "origin/main")`. Not-yet-merged → today's behavior exactly. Already-merged → `finalizeOrphanedSpecBranch`: create `chore/finalize-<specID>` from origin/main in a TEMPORARY worktree (plain-git `WorktreeAdd`/`WorktreeRemoveForce`), commit the refreshed export there via the existing `commitWithExport` (new `execBeadExportFn` seam for tests), push that branch; spec branch still pushed unconditionally. New `FinalizeResult.FinalizeBranch` → `ImplResult.FinalizeBranch` → `cmd/mindspec` `implApproveTail` prints an explicit NOTE (branch name, PR instruction, staleness warning). No `gh` dependency. No-remote "direct" path untouched (existing `TestFinalizeEpic_*` suite passes unmodified).

## Files in scope (final state at 6f558bd6)

- `internal/executor/mindspec_executor.go` (+139/-5) — preFinalizeTip capture, orphan detection, `finalizeOrphanedSpecBranch`, `execBeadExportFn` seam
- `internal/executor/executor.go` (+12) — `FinalizeResult.FinalizeBranch`
- `internal/executor/finalize_orphan_test.go` (new, 183) — `TestFinalizeEpic_OrphanedSpecBranch` (2 table cases, real temp repo + bare origin fixtures)
- `internal/gitutil/gitops.go` (+24) — `FetchRemoteBranch` (RejectOptionLike + noPrompt hygiene)
- `internal/workspace/worktree.go` (+27) — `FinalizeBranch`/`FinalizeWorktreeName`/`FinalizeWorktreePath` naming helpers
- `internal/approve/impl.go` (+8) — `ImplResult.FinalizeBranch` plumb-through
- `cmd/mindspec/impl.go` (+13) — `implApproveTail` NOTE

## Shared modules reused (unchanged)

- `commitWithExport` (export+commit primitive), `gitutil.IsAncestor`, `guardMergeLayout`, the whole direct-merge path, `WorktreeOps` for bead/spec worktrees

## Fix-author deviations (assess these explicitly)

A. `preFinalizeTip` computed unconditionally at the top of `FinalizeEpic` (even on the no-remote path) — author argues it must precede the first auto-commit, before remote-ness is known; one extra read-only subprocess.
B. Temp finalize worktree uses plain-git `gitutil.WorktreeAdd`/`WorktreeRemoveForce` rather than the `WorktreeOps` (bd worktree) seam — author argues `bd worktree` is Dolt-coupled and its doc scopes it to the bead-worktree CLI surface; this worktree is ephemeral and non-bd-tracked.
C. The local `chore/finalize-<specID>` branch is left behind after push (temp worktree is removed; branch is not deleted) — author calls it harmless and minimal-diff; possible follow-up.

## Your job

Evaluate the work cold (round 1). Scrutinize in particular:
1. **Ordering & invariants**: `TestApproveImplCallOrder` pins ApproveImpl's mutation ordering — is the FinalizeEpic-internal change invisible to it? Is Dolt state at export time guaranteed to contain the closes (ApproveImpl closes beads BEFORE calling FinalizeEpic — verify)? Is the orphan-branch push positioned BEFORE the cleanup block that removes worktrees/branches (so a cleanup failure can't strand the finalize)?
2. **Failure modes**: fetch fails (offline)? IsAncestor errors? The PR merges BETWEEN the ancestor check and the spec-branch push (race)? `chore/finalize-<specID>` already exists locally or on the remote from a prior failed/repeated run — is the flow idempotent/retryable? Temp worktree add fails — is cleanup correct and the error surfaced?
3. **The detection story**: if the operator ignores the NOTE and never merges the chore PR, is the failure at least LOUD now (vs the old silent revert)? Is the NOTE text accurate about the staleness consequence?
4. **Hygiene**: new exec sites use RejectOptionLike/noPrompt per SEC-5/spec-097 conventions? Naming helpers consistent with workspace conventions?
5. **Regression fidelity**: does the new test actually simulate the bug's conditions (already-merged PR), and would it catch a regression that re-orphans the finalize commit? Do the existing `TestFinalizeEpic_*` tests really pin the direct path unchanged?

Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/.mindspec/reviews/wu7t-finalize-fix/<your-slot>-round-1.json` with keys:
`reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
