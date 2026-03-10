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

## Invariants

1. Workflow packages never import `internal/gitutil/` — all git operations go through `Executor`.
2. `withWorkingDir` wraps all worktree remove + branch delete operations.
3. `Executor` implementations are stateless — all state comes from the caller or the git repo.
4. `MockExecutor` records all calls for assertion in tests.
