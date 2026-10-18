# Execution Domain — Overview

## What This Domain Owns

The **execution engine** owns all git, worktree, and filesystem operations — the "how" layer that implements operations delegated by the workflow layer. It dispatches beads to worktrees, executes code changes, merges results, and finalizes specs.

- **Git operations** — branching, merging, diffstat, commit counting, push/PR creation
- **Worktree lifecycle** — creating, removing, and switching between isolated workspaces
- **Filesystem safety** — `withWorkingDir` CWD protection, clean-tree checks
- **Branch conventions** — `spec/<specID>`, `bead/<beadID>` naming and lifecycle

## Boundaries

Execution does **not** own:
- Lifecycle phase derivation or mode enforcement (workflow)
- Approval gates or validation logic (workflow)
- Plan decomposition or quality assessment (workflow)
- Beads integration or epic/bead queries (workflow)
- CLI infrastructure or project health checks (core)

Execution **receives** instructions from the workflow layer via the `Executor` interface. It never decides *what* should happen — only *how*.

## Key Packages

| Package | Purpose |
|:--------|:--------|
| `internal/executor/` | `Executor` interface + `MindspecExecutor` (production) + `MockExecutor` (testing) |
| `internal/gitutil/` | Low-level git helpers (branch, merge, PR, diffstat) — used only by `MindspecExecutor` |

## Import Rule

Workflow packages (`internal/approve/`, `internal/complete/`, `internal/next/`, `internal/cleanup/`, `internal/spec/`) MUST call `executor.Executor` methods. They MUST NOT import `internal/gitutil/` directly. This boundary is enforced by convention and checked by `mindspec doctor`.

## Maintenance Notes

- **2026-09-03 (spec 128 Bead 6, #186 gate-before-mutate reorder):**
  `internal/gitutil/materialize.go` (new) plus eight paired `Executor`
  methods implement a prospective-snapshot / write-ahead materialization
  protocol so `mindspec complete`'s content gates can measure a bead
  worktree's prospective content before any complete-authored commit
  exists — closing the #186 false-panel-staleness wedge by construction.
  `internal/bead/bdcli.go`'s `wrapBDError` also now folds a failed bd
  invocation's captured stderr into the returned error (R6.4/AC-11's
  content half). See `architecture.md` and `interfaces.md` for the full
  surface; the calling reorder itself lives in the workflow domain.
