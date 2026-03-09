# Execution Domain — Overview

## What This Domain Owns

The **execution** domain owns all git, worktree, and filesystem operations — the "how" layer that performs operations delegated by the workflow layer.

- **Git operations** — branching, merging, diffstat, commit counting, push/PR creation
- **Worktree lifecycle** — creating, removing, and switching between isolated workspaces
- **Filesystem safety** — `withWorkingDir` CWD protection, clean-tree checks
- **Branch conventions** — `spec/<specID>`, `bead/<beadID>` naming and lifecycle

## Boundaries

Execution does **not** own:
- Lifecycle phase derivation or mode enforcement (workflow)
- Approval gates or validation logic (workflow)
- Beads integration or epic/bead queries (workflow)
- CLI infrastructure or project health checks (core)

Execution **receives** instructions from the workflow layer via the `Executor` interface. It never decides *what* should happen — only *how*.

## Key Packages

| Package | Purpose |
|:--------|:--------|
| `internal/executor/` | `Executor` interface + `GitExecutor` + `MockExecutor` |
| `internal/gitutil/` | Low-level git helpers (branch, merge, PR, diffstat) |

## Import Rule

Workflow packages (`internal/approve/`, `internal/complete/`, `internal/next/`, `internal/cleanup/`, `internal/specinit/`) MUST call `executor.Executor` methods. They MUST NOT import `internal/gitutil/` directly. This boundary is enforced by convention and checked by `mindspec doctor`.
