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
    CheckIgnore(workdir, file string) error
    IsTreeClean(path string) error
    LsTree(workdir, rev string) (string, error)
    DiffStat(base, head string) (string, error)
    CommitCount(base, head string) (int, error)
    CommitAll(path, msg string) error
}
```

The optional `LandedReviewSetter` interface exposes
`SetLandedReview(sha, panel string)` without changing `CompleteBead`.
`MindspecExecutor` and `MockExecutor` implement it. `complete` supplies the
first Allow panel's `reviewed_head_sha` and `<slug>@r<N>` before
`CompleteBead`. That call consumes the identity and writes
`mindspec_landed_reviewed_sha` and `mindspec_landed_review_panel` in the same
metadata merge as the landed binding. The already-bound check compares these
two keys only when supplied. Without a setter call, neither key is written or
removed. `FinalizeEpic` leaves both keys unchanged (Spec 130 R12).

`LsTree` returns a recursive, repository-relative listing at a ref, including
when the workdir is a spec subdirectory. It rejects option-like refs. Panel
consumers select roots and exclusions in Go and hash the retained listing.

`CheckIgnore` delegates to `gitutil.CheckIgnore`: nil means ignored, exit 1
means not ignored; all other errors fail the ledger preflight closed.

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

Low-level git operations used by `MindspecExecutor`; `internal/next` also consumes `FileAtRefTyped` for committed plan mapping:

| Function | Purpose |
|:---------|:--------|
| `FileAtRefTyped(workdir, ref, path)` | Probes a local branch then its tree path before reading the blob; returns `ErrRefAbsent` or `ErrPathNotAtRef` only for proven absence. Other git failures propagate. Also consumed by `internal/next` for committed-plan selection. |
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
| `ReachableMerges(workdir, ref)` | Every merge reachable from `ref`, newest-first in topological order (`git log --merges --topo-order`), with SHA, full parents and subject. Includes merges off the first-parent chain. Rejects option-like refs. Lifecycle uses it only after zero owned primary candidates; the primitive itself grants no landing or close decision (spec 129). |
| `RevertShape(workdir, mergeSHA, target)` | Reverse un-apply no-op test — `merge-tree(base=M, ours=target tip, theirs=M^1)` with rename/copy detection DISABLED (`-c merge.renames=false`). True iff the un-apply is a clean no-op whose tree equals the tip's (the tip carries none of M's content at its original paths — a true `git revert M`, or its content-indistinguishable clean-full-removal residual). Requires a >=2-parent merge; any infra failure propagates as `(false, err)` — undetermined is never a classification. Consulted only under `ContentSubsumedOutcome`'s `SubsumptionCleanDivergence` arm; the forward (rename-detecting) legs are untouched. |

### Gitignore Ensure (`internal/gitutil/gitignore.go`, spec 123 R4)

Unlike the executor-only helpers above, this surface is consumed by the
workflow domain's scaffolding verbs (`internal/bootstrap`,
`internal/setup`) and by `internal/doctor`'s not-gitignored `--fix`:

| Symbol | Purpose |
|:-------|:--------|
| `RuntimeIgnoreEntries` | The single canonical list of MindSpec local runtime files that must never be tracked (`.mindspec/session.json`, `.mindspec/focus` — ADR-0015). Bootstrap, setup, and doctor all consume THIS var, so the writer sides and the doctor detection side cannot drift. |
| `ReviewIgnoreEntries` | Optional local panel artifacts: `.mindspec/specs/*/reviews/**` and `.mindspec/reviews/**` (spec 129 R8). Separate from runtime entries so already-tracked reviews do not become doctor runtime-file findings. The following negations retain the flat spec ledger: `!.mindspec/specs/*/reviews/*/` and `!.mindspec/specs/*/reviews/*/dispositions.jsonl` (spec 130 R6). Bootstrap writes the ledger negations by exact-line presence only, preserving bytes on a second init; adhoc ledgers remain ignored. |
| `EnsureGitignoreEntriesWithHeader(root, header, entries...)` | The same append and ignore-check semantics as `EnsureGitignoreEntries`, with a caller-supplied header. Runtime callers use this ignore-check helper; bootstrap uses this for the plain review patterns and writes their ledger negations separately by exact-line presence because negations cannot be checked as ignored paths. `EnsureGitignoreEntries` delegates with the runtime header. |
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

### Prospective-snapshot / write-ahead materialization primitives (spec 128 Bead 6, `internal/gitutil/materialize.go`)

The eight `Executor` interface methods `internal/complete` calls to
implement the #186 gate-before-mutate reorder (workflow domain, ADR-0041
§1 amendment) — no new `internal/gitutil` import crosses the ADR-0030
boundary:

| Symbol | Purpose |
|:-------|:--------|
| `SnapshotWorktreeTree(wtPath, exclude)` | Builds candidate tree `T` from wtPath's prospective content (tracked+untracked+deletions) via a TEMPORARY index seeded from HEAD first, so an excluded path (`.beads/issues.jsonl`) survives unchanged in `T` rather than being dropped. Exclusions use `:(exclude,glob)` pathspecs so a single `*` never matches `/`, agreeing with the tool-artifact matcher. Touches no real index/ref/worktree state. |
| `ChangedFilesTree(workdir, base, treeOID)` | Diffs a committed ref against a bare tree OID, `-C workdir` — the tree-vs-tree leg the doc-sync/ADR-divergence gates measure `base..T` with. |
| `MaterializeStageCommit(wtPath, tree, parent, stage, subject)` | Builds a commit via plumbing (`git commit-tree`, no hooks) carrying the pinned `Mindspec-Stage` trailer (`MindspecStageTrailerKey`; `StageMaterialization`/`StageArtifactSync`) — read back on re-entry instead of inferred from a tree diff. |
| `PublishStageMarker(beadID, oid, expectedOld)` / `ReadStageMarker(beadID)` / `ClearStageMarker(beadID, expectedOID)` | The write-ahead marker protocol at `refs/mindspec/materialized/<bead-id>`, all via per-ref compare-and-swap: publish before either stage's branch advances (CREATE semantics when `expectedOld == ""`), read to discriminate a resumed run, clear on the terminal step only against the OID this invocation owns. |
| `AdvanceBranchExpectedOld(branch, newSHA, expectedOld)` | CAS branch-ref advance a concurrent mover cannot silently clobber. |
| `ResetIndexToHead(wtPath)` | Index-only `reset --mixed HEAD` refreshing the real index to the just-advanced tip without touching worktree files. |

Together: `M` builds candidate commit `C` from `T`; `R` resets the index to
`C`; `S` re-classifies the exact delta about to be committed (any
non-artifact path outside the named set REFUSES here, before commit `D` is
built) then builds `D`; `G` is the pre-existing preserved-merge machinery
(unchanged); `T` clears the marker on both completion paths. `MockExecutor`
stubs all eight with configurable results defaulting to zero values, so a
pre-existing test that never configures them exercises the
nothing-to-materialize (both stages elide) path unchanged.

### bd stderr passthrough content (spec 128 R6.4/AC-11, `internal/bead/bdcli.go`)

`ReadyJSON` also captures stderr on success. A `Showing N of M` notice refuses the read with `bd truncated the ready set (<escaped notice>) although mindspec requested no cap`, followed by `recovery: bd ready -n 0 --json`. Benign stderr leaves the JSON rows intact (spec 129 R2).

| Symbol | Purpose |
|:-------|:--------|
| `wrapBDError(err)` | Decorates a failed bd invocation so its captured `exec.ExitError.Stderr` becomes part of the returned error's rendered text (previously discarded by every `%w`-wrapping caller — a stale-binary schema error surfaced as a bare "exit status 1"). |
| `bdStderrExcerptLineCap` (20) | Bounds the surfaced excerpt for a pathological/hostile bd; legitimate bd/Dolt diagnostics are one to a handful of lines. |

This is the CONTENT half of AC-11 only — non-exit errors and whitespace-only
stderr pass through unchanged. The workflow domain's `internal/next`
applies the per-line `termsafe.Escape` RENDERING half (ADR-0042) before any
of this text reaches a terminal, since raw bd stderr is agent-writable/
hostile-adjacent.

## Consumed Interfaces

- **core**: `workspace.FindRoot()` for locating the repository root
- **beads**: `bead.WorktreeList()`, `bead.WorktreeRemove()` for worktree operations via bd CLI

## Implementations

| Type | Package | Purpose |
|:-----|:--------|:--------|
| `MindspecExecutor` | `internal/executor/mindspec_executor.go` | Production: real git+worktree operations |
| `MockExecutor` | `internal/executor/mock.go` | Testing: records calls, returns configured errors |

## Readiness advisory metadata (`internal/bead`, spec 124)

`bead.PlanBeadIndex(epicID, id string) (n int, ok bool)` (`planbead.go`,
spec 129) is the shared form predicate: exactly `<epicID>.<N>`, one
positive decimal segment with no leading zero. It checks form; workflow
callers separately verify lineage and positional plan mapping.

`internal/bead` names the two dedicated bd metadata keys spec 124's
readiness gate writes — advisory audit annotations only (ADR-0023: bd/
Dolt stays the single lifecycle-state authority; these are never
lifecycle state, and no mechanical readiness signal ever reads them —
the spec 124 R8e/AC-12 layer boundary holds by construction):

| Symbol | Purpose |
|:-------|:--------|
| `MetaKeyReadinessOverride` (`"mindspec_readiness_override"`) | Advisory schema-v2 marker: `{schema: 2, signals, fingerprints, overridden, evidence: {path, sha256, bytes}}`. Signals name the deliberately bypassed MF failures; fingerprints are sorted, unique SHA-256 digests of their canonical failure records. Evidence stores its path, hash and byte count, never its text; prefer a repository-relative evidence path. |
| `MetaKeyReadinessAttempt` (`"mindspec_readiness_attempt"`) | The append-only readiness-attempt record written by `mindspec bead clarify`: the original ordinal-keyed NOT-READY report plus span-grounded clarification entries. |
| `WriteAttemptRecord(beadID, AttemptRecord)` (`clarify.go`) | The ONLY writer of the attempt key — exactly one `MergeMetadata` write per bead, ever. Refuses fail-closed (zero write) on: a malformed bead ID; an EXISTING attempt record (the categorical, restart-proof R8d cap); an empty report or non-positive/duplicate ordinals; a clarification citing an ordinal absent from the report; an empty `span` (presence check only — whether the span SUPPORTS the answer is the re-dispatched Phase-0 subagent's judgment, never verified here). No update/finalize API exists (R8e derive-don't-write): the terminal READY/escalated disposition is derived from the re-dispatch outcome. |

`ParseOverrideMarker(raw any) (OverrideMarker, MarkerValidity)` validates
the closed schema: non-empty unique MF-1..MF-4 signals, exactly one non-empty
sorted unique 64-hex fingerprint list per signal, RFC-3339 `overridden`,
non-empty evidence path, 64-hex evidence hash, and 1..65536 evidence bytes.
Unknown keys or inconsistent fields are MALFORMED. The prior
`{signals, overridden}` shape without a schema is LEGACY; neither grants
coverage. `ReadOverrideMarker` uses `GetMetadata`, distinguishing a missing
key from a metadata read error. `NewOverrideMarkerV2` builds and validates
writer input with this same contract. Marker integrity is shape and
self-consistency only; metadata is agent-writable and unauthenticated.

`readiness.ApplyOverride` consumes marker data supplied by the command;
`EvaluateReadiness` never reads it. Current failure digests must be a subset
of recorded fingerprints, and indeterminate records never cover. Growth
uncovers and shrinkage stays covered. This read-back authorizes dispatch
ingress only; a claim still requires fresh scoped flags and evidence.

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

### Deferral reads and rename paths

- `internal/bead.CheckDeferral` reads status, description, parent chains and `discovered-from` dependencies through `bd show`. It checks that the follow-up exists, stays open and remains outside the owning epic. The defer verb, completion and implementation approval use the same checks. Each compares path-redacted finding text with the path-redacted description.
- `ChangedFiles` accepts optional `ChangedFilesOptions{NoRenames: true}`. This option lists both sides of a rename. Rebind checks and records use this option.


### Landing-tree ownership inputs

| Symbol | Purpose |
|:-------|:--------|
| `gitutil.MergeTreeLanding(workdir, base, ours, theirs)` | Runs `git merge-tree --write-tree --name-only -z --merge-base=<base>`. Returns the tree and raw conflicted paths on exit 0 or 1. Other failures return an error. It changes no ref, index, or worktree. |
| `Executor.LandingTree(workdir, base, specTip, beadRef)` | Previews the bead content into the spec tip. A bare candidate tree is wrapped in an unreferenced commit for the merge. During a preserved merge in `workdir`, returns the resolved index tree instead. Unmerged entries refuse. `MockExecutor` records the call and supports a result, conflict list, error, or function. |
| `executor.IsPreservedMergeError(err)` | Identifies a preserved-merge refusal, including a wrapped refusal. |
| `executor.PreservedMergeReentryRefusal(err, expectedSource, reentryHint)` | Uses the refusal's worktree to render the unresolved `resumeAwareMerge` refusal. Checks source binding first. Returns the foreign or indeterminate refusal when binding fails, or the edit, stage, and re-run steps for the bound conflict. It changes no merge, ref, index entry, or worktree file. |
| `gitutil.LandingIndexTree(workdir)` | Runs `git write-tree` on the real index. Returns an unreferenced tree. Unmerged entries refuse. It can update the index cache-tree extension. Staged entries do not change. |

The workflow layer passes the spec worktree as `workdir` when it exists.
Source conflicts retain a preview tree with clean ownership manifests.
Ownership conflicts return their path list for the caller's path decision.
