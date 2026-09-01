# Execution Domain — Interfaces

## Provided Interfaces

### Executor Interface (`internal/executor/executor.go`)

```go
type Executor interface {
    // Workspace lifecycle
    InitSpecWorkspace(specID string) (WorkspaceInfo, error)
    DispatchBead(beadID, specID string) (WorkspaceInfo, error)
    CompleteBead(beadID, specBranch, msg, overrideReason string, resolveMerge bool) error
    FinalizeEpic(epicID, specID, specBranch string, lifecycleAllowSet []string, overrideReason string, resolveMerge bool) (FinalizeResult, error)
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

The two merge-producing methods gained paired parameters in spec 127
(R4(b)/R5(d)): `overrideReason` is the operator's audited
`--allow-net-deletion "<reason>"` text ("" = no override), consulted by
the R4 work-destruction preflight immediately before each
`MergeInto`/`MergeBranch`; `resolveMerge` is the `--resolve-merge`
re-entry flag, consulted only when a merge is ALREADY in progress with a
fully resolved, staged index — see § Merge safety layer below.
(`lifecycleAllowSet` is spec 119's finalize scoping — see the
architecture doc.)

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

### Merge-safety primitives (spec 127, `internal/gitutil`)

Added by spec 127's work-destruction preflight (R4) and never-abort
merge resumption (R5(d)). NOTE: the spec's Impacted Domains originally
declared `internal/gitutil` read-only; that declaration was violated and
amended at final review (O3-1) — the five mutating primitives below are
the record of why.

**Read-only evidence/preview helpers:**

| Symbol | Purpose |
|:-------|:--------|
| `EvaluateWorkDestruction(workdir, branch, target)` (`workdestruction.go`) | The shared work-destruction predicate: returns the closed `guard.DestructionOutcome` enum (ancestor / superseded / stale-deletion / clean / evidence-error, count-sentinel `DestructionOutcomeCount`) plus `WorkDestructionEvidence`. Evidence-unavailable is its own outcome, never folded into safety. CAUTION: `DestructionAncestor` is the zero value — possession of a value is not proof the predicate ran (see `outcome.go`'s doc comment and `merge_preflight.go`'s zero-value trap note). |
| `PreviewDeletedPaths(workdir, target, branch)` (`neteffect.go`) | Read-only merge preview of target paths the merge would net-delete — the R4(b) refusal's named-paths evidence. |
| `BranchExistsIn(workdir, name)` / `RemoteExistsIn(workdir, name)` | Explicit-root variants of the cwd-scoped probes; `BranchExistsIn` is the CompleteBead guard probe as of the final-review code round (mindspec-6f5p — the cwd-scoped `BranchExists` silently skipped both guarded legs when the process cwd was outside the repo). Both return `(bool, error)`: indeterminate refuses, never folds into "absent". |
| `FetchRemoteBranchIn(workdir, remote, branch)` | Explicit-root fetch; returns the fetched tip's SHA resolved from `FETCH_HEAD` immediately after the fetch (poisoned-tracking-ref hygiene). |
| `MergeMsgSubject(workdir)` / `TreeSHA` / `CommitParents` / `CommitMessageBody` / `CommitSigningEnabled` | Small read probes the resumption classifier and signing-aware drift-collapse consult. |
| `DanglingCollapsedMergeExists(workdir, tree, p1, p2, msg)` | `git fsck --unreachable` probe distinguishing a genuinely stranded drift-collapse from a topology that merely looks like one (necessary-but-not-sufficient; stated limits in its doc comment). |

**Mutating primitives (all consumed only by `internal/executor`'s
resumption/drift-collapse mechanics behind fault-injectable seams):**

| Symbol | Purpose |
|:-------|:--------|
| `CommitNoEdit(workdir)` | Completes an in-progress, fully staged merge (`git commit --no-edit`) — the `--resolve-merge` exact-binding completion. |
| `CommitTreeMerge(workdir, tree, p1, p2, msg)` | Creates an unreferenced merge commit object for drift-collapse. Signing-aware (final-review S1-1): `git commit-tree` does not honor `commit.gpgsign`, so this consults the effective signing config, passes `-S`, and fails CLOSED when signing is configured but unavailable. |
| `UpdateRef(workdir, ref, sha)` / `ResetSoft(workdir, target)` | Move a ref / branch tip onto the collapse result. |
| `recordMergeSourceMarker` (unexported; `MergeSourceMarkerRef(source)` names the ref) | Writes the merge-start marker ref `MergeInto`/`MergeBranch` record so resumption can corroborate WHICH source a preserved merge belongs to. |

## Consumed Interfaces

- **core**: `workspace.FindRoot()` for locating the repository root
- **beads**: `bead.WorktreeList()`, `bead.WorktreeRemove()` for worktree operations via bd CLI

## Implementations

| Type | Package | Purpose |
|:-----|:--------|:--------|
| `MindspecExecutor` | `internal/executor/mindspec_executor.go` | Production: real git+worktree operations |
| `MockExecutor` | `internal/executor/mock.go` | Testing: records calls, returns configured errors |

## Readiness advisory metadata (`internal/bead`, spec 124)

`internal/bead` names the two dedicated bd metadata keys spec 124's
readiness gate writes — advisory audit annotations only (ADR-0023: bd/
Dolt stays the single lifecycle-state authority; these are never
lifecycle state, and no mechanical readiness signal ever reads them —
the spec 124 R8e/AC-12 layer boundary holds by construction):

| Symbol | Purpose |
|:-------|:--------|
| `MetaKeyReadinessOverride` (`"mindspec_readiness_override"`) | Durable `--allow-not-ready` override marker written by `mindspec next` (workflow domain) via `MergeMetadata`: the bypassed MF signal IDs + a UTC timestamp. |
| `MetaKeyReadinessAttempt` (`"mindspec_readiness_attempt"`) | The append-only readiness-attempt record written by `mindspec bead clarify`: the original ordinal-keyed NOT-READY report plus span-grounded clarification entries. |
| `WriteAttemptRecord(beadID, AttemptRecord)` (`clarify.go`) | The ONLY writer of the attempt key — exactly one `MergeMetadata` write per bead, ever. Refuses fail-closed (zero write) on: a malformed bead ID; an EXISTING attempt record (the categorical, restart-proof R8d cap); an empty report or non-positive/duplicate ordinals; a clarification citing an ordinal absent from the report; an empty `span` (presence check only — whether the span SUPPORTS the answer is the re-dispatched Phase-0 subagent's judgment, never verified here). No update/finalize API exists (R8e derive-don't-write): the terminal READY/escalated disposition is derived from the re-dispatch outcome. |

`AttemptRecord` is `{report: [{ordinal, signal, reason}], clarifications:
[{ordinal, reason, answer, span}]}` — the record carries the FULL
original report so a later reader (`bd show`, the dispatch ingress, an
auditor) never needs the long-gone subagent transcript. `internal/bead`
itself never interprets the values under either key; it only names them
so every writer/reader shares one literal definition.

## Merge-conflict handling — never-abort (spec 127 R5(d); supersedes spec 092 Reqs 13–15, 18 here)

**Spec 127 inverted this section's design.** Spec 092 had the executor
detect and unwind an in-progress merge (`abortMergeState` →
`gitutil.AbortMerge`) before reporting a guard failure. That unconditional
abort is exactly the destruction mechanism spec 127's E-r5-1 lab evidence
convicted — it discards an operator's staged hand-resolution — and the
shipped design is the opposite:

- **`abortMergeState` no longer exists.** No product path aborts a fresh
  conflict, and no product path performs a target checkout while
  `MERGE_HEAD` exists in that worktree. `gitutil.AbortMerge`
  (`gitops.go`) remains exported with ZERO production callers — the
  never-abort property is currently enforced only by that absence, not
  by an assertion that it persists (recorded final-review residual). Do
  not reintroduce a caller; the executor is NOT supposed to use it.
- **Conflicts stop in place.** A conflicted lifecycle merge is preserved
  exactly as git left it. The structured failures
  (`beadToSpecConflictFailure`, `directMergeConflictFailure`) name the
  conflicted files and end with the `--resolve-merge` re-entry
  invocation of the owning verb (`mindspec complete --resolve-merge` /
  `mindspec impl approve --resolve-merge`) plus pinned per-outcome
  resolution steps whose `git add` operands equal the emitter-computed
  conflicted set — never a raw `git merge` line (the R5(d) constructor
  conversion; the `internal/lint` scan REDs a raw merge line here).
- **Re-entry resumes the SAME merge** via `resumeAwareMerge`
  (`internal/executor/merge_resumption.go`):
  `classifyPreservedMergeBinding` gates resumption as exact / drifted /
  foreign against the expected source (merge-subject naming plus the
  `MergeSourceMarkerRef` merge-start marker); exact completes the
  preserved merge (`gitutil.CommitNoEdit` — no checkout, no abort, no
  new merge); drifted incorporates source drift via the drift-collapse
  mechanics (`CommitTreeMerge` + `ResetSoft`/`UpdateRef`, signing-aware);
  foreign or indeterminate refuses fail-closed. A plain no-flag re-run
  over a resolved index refuses WITHOUT aborting and the staged
  resolution survives byte-for-byte; `--resolve-merge` is required to
  complete someone's staged resolution deliberately.
- `internal/bead.MergeMetadata` error text still never quotes a raw
  `bd update --metadata` line (spec 092 Req 19 / HC-5 — unchanged, and
  guarded by AC-9(iii)).

The R4 work-destruction preflight in front of every merge producer, and
the full resumption state machine, are described in this domain's
architecture doc, § Merge safety layer (spec 127).
