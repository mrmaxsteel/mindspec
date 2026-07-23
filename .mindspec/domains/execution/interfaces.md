# Execution Domain — Interfaces

## Provided Interfaces

### Executor Interface (`internal/executor/executor.go`)

```go
type Executor interface {
    // Workspace lifecycle
    InitSpecWorkspace(specID string) (WorkspaceInfo, error)
    DispatchBead(beadID, specID string) (WorkspaceInfo, error)
    CompleteBead(beadID, specBranch, msg string) error
    FinalizeEpic(epicID, specID, specBranch string) (FinalizeResult, error)
    Cleanup(specID string, force bool) error

    // Epic handoff (notification hook — no-op for MindspecExecutor)
    HandoffEpic(epicID, specID string, beadIDs []string) error

    // Query methods
    IsTreeClean(path string) error
    DiffStat(base, head string) (string, error)
    CommitCount(base, head string) (int, error)
    CommitAll(path, msg string) error
}
```

### GitUtil Helpers (`internal/gitutil/gitutil.go`)

Low-level git operations used only by `MindspecExecutor`:

| Function | Purpose |
|:---------|:--------|
| `BranchExists(name)` | Check if a branch exists |
| `CreateBranch(name, from)` | Create a branch from a ref |
| `DeleteBranch(name)` | Delete a local branch |
| `MergeBranch(source, target)` | Merge source into target |
| `DiffStat(base, head)` | Short diffstat summary |
| `CommitCount(base, head)` | Count commits between refs |
| `PRStatus(branch)` | Check PR merge status via gh |
| `PRChecksWatch(branch)` | Watch CI checks via gh |
| `MergePR(branch)` | Merge PR via gh |

### Gitignore Ensure (`internal/gitutil/gitignore.go`, spec 123 R4)

Unlike the executor-only helpers above, this surface is consumed by the
workflow domain's scaffolding verbs (`internal/bootstrap`,
`internal/setup`) and by `internal/doctor`'s not-gitignored `--fix`:

| Symbol | Purpose |
|:-------|:--------|
| `RuntimeIgnoreEntries` | The single canonical list of MindSpec local runtime files that must never be tracked (`.mindspec/session.json`, `.mindspec/focus` — ADR-0015). Bootstrap, setup, and doctor all consume THIS var, so the writer sides and the doctor detection side cannot drift. |
| `EnsureGitignoreEntries(root, entries...)` | Entry-granular, byte-idempotent `.gitignore` append: existing bytes are never reordered or rewritten; only genuinely missing entries are appended once, under a shared header comment; creates the file if absent; a call with every entry present is a true no-op (the file is not opened for writing). Presence detection is exact-line (delimiter-stripped only — a leading-whitespace line is a DIFFERENT pattern git does not honor, so it never satisfies presence). Deliberately separate from the pre-existing directory-specialized `EnsureGitignoreEntry` (singular), which appends a trailing `/`. |

## Consumed Interfaces

- **core**: `workspace.FindRoot()` for locating the repository root
- **beads**: `bead.WorktreeList()`, `bead.WorktreeRemove()` for worktree operations via bd CLI

## Implementations

| Type | Package | Purpose |
|:-----|:--------|:--------|
| `MindspecExecutor` | `internal/executor/mindspec_executor.go` | Production: real git+worktree operations |
| `MockExecutor` | `internal/executor/mock.go` | Testing: records calls, returns configured errors |

## Merge-conflict hardening (spec 092 Reqs 13–15, 18)

- `internal/gitutil` merge-state helpers: `MergeInProgress(workdir)`,
  `ConflictedFiles(workdir)`, `AbortMerge(workdir)` — detect and
  unwind an in-progress merge before reporting a guard failure.
- `MindspecExecutor` conflict paths (`CompleteBead` bead→spec merge and
  the direct spec-merge site) abort the conflicted merge
  (`abortMergeState`) and emit structured failures
  (`beadToSpecConflictFailure`, `directMergeConflictFailure`) that name
  the conflicted files and end with a copy-pastable recovery command.
- `internal/bead.MergeMetadata` error text no longer quotes a raw
  `bd update --metadata` command line (Req 19 / HC-5: `--metadata`
  REPLACES the whole metadata map; agents must never be handed one to
  paste).
