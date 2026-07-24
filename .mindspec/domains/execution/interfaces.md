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

### Landed-merge identity primitives (spec 125, ADR-0041 §2(ii))

Unlike the executor-only helpers above, these two are ALSO consumed by
the workflow domain's landed-merge read side
(`internal/lifecycle.FindLandedMerge` / `ReattestLandedMerge`) — they
are the shared root-of-trust primitives, so the write and read sides
cannot drift:

| Function | Purpose |
|:---------|:--------|
| `ExactSecondParentMerges(workdir, branch, tip)` | `branch`'s two-parent first-parent merges whose second parent EQUALS `tip` exactly, newest-first. The ONE exact-match landed-ness primitive: octopus merges and ancestor-consistent-but-not-equal candidates are excluded, never guessed at. `tip` is option-reject gated before reaching any git argv. |
| `RevertShape(workdir, mergeSHA, target)` | Reverse un-apply no-op test — `merge-tree(base=M, ours=target tip, theirs=M^1)` with rename/copy detection DISABLED (`-c merge.renames=false`). True iff the un-apply is a clean no-op whose tree equals the tip's (the tip carries none of M's content at its original paths — a true `git revert M`, or its content-indistinguishable clean-full-removal residual). Requires a >=2-parent merge; any infra failure propagates as `(false, err)` — undetermined is never a classification. Consulted only under `ContentSubsumedOutcome`'s `SubsumptionCleanDivergence` arm; the forward (rename-detecting) legs are untouched. |

### Gitignore Ensure (`internal/gitutil/gitignore.go`, spec 123 R4)

Unlike the executor-only helpers above, this surface is consumed by the
workflow domain's scaffolding verbs (`internal/bootstrap`,
`internal/setup`) and by `internal/doctor`'s not-gitignored `--fix`:

| Symbol | Purpose |
|:-------|:--------|
| `RuntimeIgnoreEntries` | The single canonical list of MindSpec local runtime files that must never be tracked (`.mindspec/session.json`, `.mindspec/focus` — ADR-0015). Bootstrap, setup, and doctor all consume THIS var, so the writer sides and the doctor detection side cannot drift. |
| `EnsureGitignoreEntries(root, entries...)` | Entry-granular, negation-aware `.gitignore` ensure (final review G1): guarantees each entry is ACTUALLY ignored by git, not merely present as a line. Existing bytes are never reordered or rewritten; entries needing a fresh line are appended under a shared header comment; creates the file if absent. An entry needs a fresh line when its exact line is absent (delimiter-stripped comparison only — a leading-whitespace line is a DIFFERENT pattern git does not honor, so it never satisfies presence), OR when the line IS present but `git check-ignore` reports the path un-ignored anyway — a LATER negation rule (`!entry`) defeats it under git's last-match-wins ordering, so the plain entry is RE-APPENDED (a harmless duplicate line) to make the last match the ignore rule again. So the same entry can legitimately appear more than once, and a converged call still shells out to `git check-ignore` per line-present entry before concluding nothing needs writing (no write happens in that case). On an indeterminate git verdict (no repository / exit status outside {0,1}, e.g. 128) it falls back to line-presence alone rather than force a spurious re-append. Deliberately separate from the pre-existing directory-specialized `EnsureGitignoreEntry` (singular), which appends a trailing `/`. |

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
