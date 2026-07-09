# wu7t-finalize-fix — Round 2 Review Panel

**Worktree**: /Users/Max/replit/mindspec/.claude/worktrees/agent-a9242fc424939a00f (branch checked out; run `go test` here)
**Repo (for diffs)**: /Users/Max/replit/mindspec
**Branch**: fix/wu7t-protected-main-finalize
**Commit under review**: 6cf718c9 (round-2 fix commit, on top of round-1 6f558bd6; review the CUMULATIVE diff `git diff main...fix/wu7t-protected-main-finalize` with attention to the round-2 delta `git diff 6f558bd6..6cf718c9`)
**Prior round verdict**: 3 APPROVE (R1/R2/R3) / 3 REQUEST_CHANGES (R4/R5/R6). Consolidated asks: this dir's consolidated-round-1.md. Round-1 BRIEF: BRIEF.md (context; supplement, not replacement).

## Round-1 concrete_changes_required (consolidated) and how the fix answered

1. **Retry idempotency (R4/R5/R1-nit)** → `finalizeOrphanedSpecBranch` now probes the remote chore branch via new `gitutil.RemoteHeadSHA` (`git ls-remote --heads`, remote-authoritative); absent → plain push, present → new `gitutil.PushBranchForceWithLease` with the lease pinned to the OBSERVED remote SHA (`refs/heads/<branch>:<sha>`) — machine-owned tip overwritten, third-party tip movement fails the lease loudly. Leftover temp worktree self-heals BEFORE branch ops (stat → WorktreeRemoveForce → os.RemoveAll fallback → WorktreePrune). Wrong "spec branch survives" comment fixed.
2. **implApproveTail composition (R6/R4)** → old spec-branch PR block gated on `result.Pushed && result.FinalizeBranch == ""`; composition test `cmd/mindspec/impl_finalize_note_test.go` asserts orphan case prints ONLY the NOTE (and never offers `gh pr create --head spec/...`).
3. **Baseline push first (R5/R3-low)** → `PushBranch(specBranch)` now first and unconditional in the remote block; orphan detect + chore flow after; ordering comments updated.
4. **Tests** → new: `TestFinalizeEpic_OrphanedSpecBranch_RetryConverges`, `_LeftoverTempWorktreeSelfHeals`, `TestFinalizeEpic_OrphanFetchFailureFallsBackToBaseline`, `TestFinalizeEpic_OrphanChoreFailureStillPushesSpecBranch` (extra, pins #3), `TestFetchRemoteBranch_RunsNarrowFetch`, `TestRemoteHeadSHA_ParsesLsRemote`, `TestPushBranchForceWithLease_ArgvAndLease`, `TestFinalizeBranch`/`TestFinalizeWorktreeName`/`TestFinalizeWorktreePath`(+custom-root), `TestImplApproveTail_FinalizeBranchComposition`. Full suites green (executor/approve/gitutil/workspace/cmd uncached).
5. **Squash blind-spot comment (R1)** → documented at the IsAncestor detection site, naming the follow-up. (Follow-up bead mindspec-3xqm already filed by the orchestrator.)

## Fix-author deviations (assess explicitly)

A. Retry test drives run-1's failure by injecting `fake.removeErr` into the spec-worktree cleanup AFTER both pushes (a real mid-run failure shape) rather than replaying a fully-successful run — a literal replay hits the pre-existing "spec branch does not exist" gate (cleanup deleted it), out of scope.
B. One extra test beyond the list (`TestFinalizeEpic_OrphanChoreFailureStillPushesSpecBranch`) to pin Group 3's ordering contract.
C. In passing, fixed its own round-1 composition test's cwd discipline (save/restore via t.Cleanup) after it cascaded failures into unrelated cmd/mindspec tests during full-package runs.

## New surface added in round 2 (fresh scrutiny warranted)

`gitutil.RemoteHeadSHA` and `gitutil.PushBranchForceWithLease` are NEW exec sites — check argv hygiene (RejectOptionLike on branch/sha operands, noPrompt) and the lease semantics (could a stale observed SHA ever force-overwrite a HUMAN's work on a chore/finalize-* branch?).

## Your job

Round ≥2 protocol: each slot keeps its round-1 lens. **R4/R5/R6 (round-1 RC voters): evaluate each of YOUR OWN round-1 concrete_changes_required items as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE**, plus assess deviations A–C. **R1/R2/R3 (round-1 approvers): confirm the round-2 delta introduces no regressions in your lens** (R2: re-run the suites; R3: re-verify ordering contracts survived the Group-3 reorder — especially that closes-before-export and push-before-cleanup still hold, and TestApproveImplCallOrder still passes).

Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/.mindspec/reviews/wu7t-finalize-fix/<your-slot>-round-2.json` with keys:
`reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings` (per your round-1 item: ADDRESSED/PARTIAL/MISSED/NEW_ISSUE).
