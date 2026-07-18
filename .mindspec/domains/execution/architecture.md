# Execution Domain — Architecture

## Key Patterns

### Executor Interface (Spec 077)

The `Executor` interface separates enforcement ("what") from execution ("how"):

```
Workflow Layer                    Execution Engine
┌─────────────────┐              ┌─────────────────────────────┐
│ approve/         │──Executor──▶│ executor/mindspec_executor.go│
│ complete/        │   interface │ (MindspecExecutor)           │
│ next/            │              │                             │
│ spec/            │              │ gitutil/                    │
│ cleanup/         │              │ (low-level ops)             │
└─────────────────┘              └─────────────────────────────┘
```

- **MindspecExecutor** — concrete implementation wrapping git+worktree operations (dispatches beads to worktrees, merges completed bead branches, finalizes specs)
- **MockExecutor** — test double for enforcement testing without git side effects
- **DI wiring** — `cmd/mindspec/root.go` has `newExecutor(root)` factory

The execution engine reads beads produced by the planning layer. Each bead is a self-contained work packet — requirements, context, dependencies, acceptance criteria — so a fresh agent can pick it up without session history. Beads are the substrate that makes the `Executor` interface pluggable: any orchestrator that can read a bead can dispatch work.

### withWorkingDir Safety

Worktree removal and branch deletion require CWD to be outside the target worktree. `MindspecExecutor` uses `withWorkingDir(root, func)` to temporarily chdir to the repo root before destructive operations, then restores the original CWD. This prevents "cannot remove worktree: in use" errors.

### Function Injection for Testability

`MindspecExecutor` exposes function variables (`WorktreeRemoveFn`, `DeleteBranchFn`, `MergeBranchFn`, etc.) that can be replaced in tests. This avoids requiring a real git repository for unit tests while keeping the production code straightforward.

### Branch Conventions

| Entity | Branch name | Worktree path |
|:-------|:-----------|:-------------|
| Spec | `spec/<specID>` | `.worktrees/worktree-spec-<specID>` |
| Bead | `bead/<beadID>` | `.worktrees/worktree-<beadID>` (nested under spec) |

Bead branches are created from the spec branch. On completion, bead branches merge back into the spec branch. On finalization, the spec branch merges into main.

### Directional Layout-Fingerprint Merge Guard (Spec 106)

`MindspecExecutor` installs a DIRECTIONAL layout-fingerprint guard in front of
its three REAL local merge seams — `CompleteBead`'s and `FinalizeEpic`'s
`gitutil.MergeInto` (bead→spec) and `FinalizeEpic`'s direct, no-remote
`gitutil.MergeBranch` (spec→main). `layoutAtRef` fingerprints each ref's tree
through the executor's own `TreeDirsAtRef(ref, ".mindspec")` read and the shared
`workspace.ClassifyLayout`/`LayoutMarkersFromMindspecChildren` helper (one source
of truth with `DetectLayout`, so the on-disk and ref-anchored signatures never
drift); legacy (a repo-root `docs/` tree) is probed only when neither flat nor
canonical markers are present.

The rule is precise: **block ⟺ source is canonical/legacy AND target is flat** —
the regression that would resurrect the pre-flatten `.mindspec/docs/...` paths on
top of an already-flattened tree. The flatten is forward-only (ADR-0023), so the
block carries a `git rebase <target> <source>` recovery line and mutates nothing
(the direct spec→main guard runs BEFORE any worktree cleanup). The MIGRATION
direction (flat source → canonical/legacy target) and same-layout merges are
explicitly ALLOWED, so the flatten itself can land. The block is EXEMPT inside a
recorded in-progress migration run (`workspace.MigrationRecoveryActive`, Bead-1's
in-flight-run-id scoping, not a stale record). A layout read failure fails open.
The REMOTE-PR path (`FinalizeEpic` pushes the branch for a PR when a remote
exists) does NOT local-merge, so this in-binary guard covers the local-merge
seams only; cross-layout protection on the PR path relies on the pre-flatten
precondition + PR review.

## Invariants

1. Workflow packages never import `internal/gitutil/` — all git operations go through `Executor`.
2. `withWorkingDir` wraps all worktree remove + branch delete operations.
3. `Executor` implementations are stateless — all state comes from the caller or the git repo.
4. `MockExecutor` records all calls for assertion in tests.

## Dead-code sweep — spec 107 wave 1 (2026-07-02)

Bead `mindspec-oexu.1` removed confirmed-dead execution-domain symbols
(zero live callers):

- `internal/gitutil/gitops.go`: `MainWorktreePath` + `IsMainWorktree`.
- `internal/harness`: `filterEnv` (`agent.go`; the live `filterEnvPrefix` is
  retained) and the unused assertion helpers `assertCommandUsedFlag` +
  `assertCleanWorktree` (`asserts.go`).

## Epic-scoped `FinalizeEpic` + the fault-injection stage hook (spec 119)

`MindspecExecutor.FinalizeEpic`'s two bead-branch enumerations — the
bead→spec auto-merge leg and the worktree/branch cleanup leg — are now
scoped to a caller-supplied `lifecycleAllowSet` (the finalizing spec's
plan-declared, lifecycle-classified bead IDs, computed by
`internal/approve.ApproveImpl` and passed in). A candidate `bead/<id>` is
admitted iff its ID is a member; `lifecycleAllowSet == nil` is the
"not computed" sentinel — encountering ANY `bead/<id>` candidate while it
is nil aborts the finalize fail-closed (a foreign-epic bead or a
same-epic non-lifecycle follow-up must survive both legs untouched,
R6/AC-14). This closes a class of bug where a `WorktreeOps.List()`
enumeration failure silently skipped the whole leg instead of aborting.

`FinalizeEpic` is a single COMMIT-phase mutation chain (ADR-0041 §1) with
no seam separating its internal steps, so spec 119 Bead 6 added
**`finalizeStepHookFn`** (`mindspec_executor.go`) — a test-only package-var
hook invoked at five labeled stages: `auto_merge` (after the bead-branch
auto-merge leg), `push` (after the unconditional spec-branch push),
`orphan_finalize` (after bug wu7t's `finalizeOrphanedSpecBranch` returns),
`pre_cleanup` (between the merge/push legs and the cleanup leg), and
`post_cleanup` (after the cleanup leg's mutations — worktree/branch
removals, the no-remote direct spec→main merge, spec-branch deletion —
complete). Nil in production (a pure no-op); `internal/executor/
finalize_fault_test.go` sets it per test to fault-inject each stage
individually against a real temp git repo, confirming the REAL mutation
up to that stage landed before the injected terminal error, then clears
the hook and re-invokes `FinalizeEpic` to confirm convergence. The LAST
stage (`post_cleanup`) is the one point where "convergence" means a
clean, named refusal (`FinalizeEpic`'s own "spec branch does not exist"
check) rather than completion — there is nothing left to finalize by the
time that stage's mutations have all landed.
