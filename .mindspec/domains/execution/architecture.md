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

## Landed-merge attestation substrate (spec 125, ADR-0041 §2(ii))

Spec 125 (beads `mindspec-xhd5.1`/`.2`) rebuilt the WRITE side of the
merge-time landed-binding (`mindspec_landed_merge_sha` +
`mindspec_landed_second_parent`, recorded on bd metadata by `complete` /
`FinalizeEpic` immediately after a bead→spec merge and BEFORE any
cleanup) on git-topology ground truth, and added the gitutil primitives
the workflow-domain read side (`internal/lifecycle.FindLandedMerge`,
`mindspec reattest`) shares. Root cause fixed (spec 125 Background):
the pre-125 locate required the merge subject to be exactly
`"Merge bead/<id>"`, so a conflict-recovery merge with git's default
subject (`Merge branch 'bead/<id>' into …`) was never matched and the
miss was silently swallowed — 755/757 fleet beads had no binding.

### `gitutil.ExactSecondParentMerges` — the one exact-match identity primitive

`ExactSecondParentMerges(workdir, branch, tip)` (`gitops.go`) filters
`FirstParentMerges` to plain two-parent merges whose SECOND parent
EQUALS `tip` exactly (newest-first). Landed-ness is git TOPOLOGY, never
subject text; octopus (>2-parent) candidates are excluded, and an
ancestor-consistent-but-not-equal second parent is excluded too —
ancestor tolerance is exactly the misattribution vector spec 125
removes. The executor WRITE path (`locateLandedMergeByIdentity`,
`beadTipLandedOnSpec`) consumes this primitive directly; the
workflow-domain READ path (`FindLandedMerge`/`ReattestLandedMerge` in
`internal/lifecycle`) instead scans the same `FirstParentMerges` stream
and applies the same two-parent + exact-second-parent equality filter
INLINE (it must evaluate every owned merge for subject-nominated
ownership, not just one tip's matches). The two sides are logically
equivalent — same scan, same exact-match semantics — but they are two
code paths, not one shared call site, and no cross-side anti-drift test
pins their equivalence; a change to either filter must be mirrored by
hand.

### Ground-truth binding persistence + loud fail-closed miss

`locateLandedMergeByIdentity` (`mindspec_executor.go`) now resolves the
bead branch's own tip and scans the spec branch via
`ExactSecondParentMerges` — no subject parsing at all. The write path
KNOWS the bead identity directly (it is completing that bead), which is
why the binding persists REGARDLESS of the merge's subject format
(spec 125 R1): the default conflict-recovery subject and MergeInto's
exact subject bind identically. It never rev-parses HEAD, so a no-op
re-run (already-ancestor branch, no new merge) finds the SAME merge.

`ensureLandedBinding` classifies a locate MISS structurally (R2)
instead of silently treating it as "nothing to bind": the discriminator
is first-parent MEMBERSHIP (`beadTipLandedOnSpec` — is the bead tip the
second parent of ANY first-parent merge on the spec branch?), computed
directly via gitutil and never through the injectable locate seam
(`locateLandedMergeFn`), so a test-forced locate lie cannot mask it. Not
a member ⟹ true nothing-to-bind (a trivially-ancestor branch that never
diverged — e.g. a bd_close orphan), quiet. A member whose locate missed,
or a failed binding write ⟹ LOUD fail-closed error: `CompleteBead` /
`FinalizeEpic` translate it into a cleanup-suppressing `guard` refusal,
so the surviving branch remains as the corroborating datum and a re-run
converges. An own-commit-count or merge-base classifier is proven
insufficient here — post-merge, a merged-then-ancestor bead is
byte-identical to a true orphan under both metrics.

The "already bound" idempotent skip is two-key (spec 125 G2-1): the
stored merge SHA AND stored second parent must BOTH match the located
merge; a mismatch on either overwrites with the located SHAs (a failed
overwrite fails closed, suppressing cleanup). And the bead→spec
conflict-recovery message (`beadToSpecConflictFailure`) now supplies
`-m "Merge <beadBranch>"` (R5/AC-1b) so an operator following it
verbatim produces an identifiable exact subject — belt-and-suspenders
now that identity is topology-corroborated, but it keeps subject-based
readers working.

### `gitutil.RevertShape` — reverse un-apply, rename-safe (spec 125 R3)

`RevertShape(workdir, mergeSHA, target)` (`neteffect.go`) is the
revert-vs-evolved discriminator the read side layers under
`ContentSubsumedOutcome`'s `SubsumptionCleanDivergence` arm: the REVERSE
"un-apply" three-way — `merge-tree(base = M, ours = target's tip,
theirs = M^1)` — with rename/copy detection DISABLED
(`-c merge.renames=false`, via the dedicated
`mergeTreeWriteTreeNoRenamesFn` seam). Revert-shape (true) iff the
un-apply is CLEAN and its result tree equals the tip's current tree: the
tip carries NONE of M's introduced content at its original paths —
exactly `git revert M`'s residue, and also the content-indistinguishable
clean-full-removal residual (a deliberate false-negative floor). Any
other outcome — the un-apply changes the tip (M's content wholly or
partially present) or conflicts (the tip built on M's region) — is NOT
revert-shape: evolved content identifies (the `mindspec-8nhe.2` fix).
Rename-off is load-bearing (G-BLOCK-1): merge-ort's default rename
detection lets a coincidentally-identical blob at an unrelated later
path count as M's "moved" content, turning a true revert into a
rename/delete conflict that would misread as evolved — an unsafe
false-positive attestation. This is why RevertShape does NOT route
through `ContentSubsumedOutcome` (whose forward leg keeps renames ON for
the spec-121 semantics). A <2-parent `mergeSHA` errors — never a
first-parent guess — and ANY infra failure propagates as
`(false, non-nil error)`: undetermined is never mapped to identify or
refuse; callers fail closed. `ContentSubsumedOutcome` itself is
byte-identical before/after 125.

## Merge safety layer (spec 127)

Spec 127 put two properties in front of every lifecycle merge: **no
merge executes without work-destruction evidence being consulted**, and
**no conflicted merge is ever aborted by the product**.

### The R4 work-destruction preflight — one chokepoint, three producers

`preflightMergeDestruction` (`internal/executor/merge_preflight.go`) is
the ONE function every lifecycle merge producer calls immediately before
its `gitutil.MergeInto`/`MergeBranch`: `CompleteBead`'s bead→spec merge,
`FinalizeEpic`'s bead→spec auto-merge loop, and `FinalizeEpic`'s direct
no-remote spec→main merge. It consults the shared predicate
`gitutil.EvaluateWorkDestruction` (bead 1; seamed via
`workDestructionFn`) and switches exhaustively over the closed
`guard.DestructionOutcome` enum: `ancestor` proceeds as a no-op (the
documented post-conflict recovery converges via ancestry); `clean`
proceeds; `superseded`, `stale-deletion`, and `evidence-error` refuse
fail-closed — absence of evidence is never treated as safety — with the
audited `--allow-net-deletion "<reason>"` override
(`AllowNetDeletionFlag`, byte-identical at every layer) as the only way
through, recorded by the CALLER (`internal/complete` /
`internal/approve`) on durable metadata and the friction registry.

Two ratchet tests compose to keep the producer set closed:
`merge_chokepoint_test.go` (package-scoped AST scan — exactly these
producers inside `internal/executor`, within its documented shape
vocabulary) and `merge_containment_test.go` (repo-wide, final-review
O1-1/O2-2 — no production file OUTSIDE `internal/executor` may call
`MergeInto`/`MergeBranch` at all).

**The zero-value trap** (`outcome.go`, `merge_preflight.go`):
`guard.DestructionAncestor` is the enum's zero value AND the permissive
disposition — the inversion of the house fail-closed-zero discipline
(the enum order is plan-pinned). A defaulted or stale
`DestructionOutcome` reads as "safe to merge"; the preflight closes this
structurally, and no consumer may treat mere possession of an outcome
value as proof the predicate ran.

### Never-abort conflict preservation + `--resolve-merge` resumption (R5(d))

`abortMergeState` and every product-side merge abort are DELETED (the
old spec-092 unwind design — see this domain's interfaces doc for the
inversion record). A conflicted lifecycle merge stops in place; the
refusal names the conflicted files, pinned resolution steps, and the
`--resolve-merge` re-entry invocation of the owning verb. Re-entry
(`resumeAwareMerge`, `merge_resumption.go`) re-runs the R4 preflight at
the CURRENT source tip, then:

- **No merge in progress** → fresh merge; on conflict, stop in place
  (never abort, never checkout — no product path checks out a target
  while `MERGE_HEAD` exists).
- **Preserved merge, conflicts still unresolved** → refuse again with
  the same resolution steps (`stillConflictedRefusal`).
- **Preserved merge, index fully resolved and staged** →
  `classifyPreservedMergeBinding` corroborates WHOSE merge it is
  (merge-subject naming + the `MergeSourceMarkerRef` marker ref):
  **exact** (preserved tip == expected source tip) completes the SAME
  merge via `gitutil.CommitNoEdit`; **drifted** (source advanced since
  the preserve) completes then incorporates the drift via the
  drift-collapse mechanics (`CommitTreeMerge` — signing-aware, S1-1 —
  plus `ResetSoft`/`UpdateRef`, each behind a fault-injectable seam,
  with `DanglingCollapsedMergeExists` distinguishing a genuinely
  stranded collapse on re-entry); **foreign** or **indeterminate**
  refuses fail-closed and preserves.
- **Unmerged/blocked worktree state that is NOT a preserved merge** →
  the distinct worktree-state refusal (blocking paths named, wording
  disjoint from the conflict diagnostic).

Without `--resolve-merge`, a plain re-run REFUSES over a resolved index
rather than silently completing someone else's staged resolution, and
the staged resolution survives byte-for-byte (AC-9(v)(δ)). The
committing surfaces that are not merge producers (`impl approve`'s
artifact commit, sibling-bead `complete`) check the preserved-merge
precondition BEFORE mutating, so a preserved merge can never be
swallowed into a two-parent `chore:` commit (AC-9(v)(ε); enforced at the
`commitWithExport` funnel, the only production caller of
`gitutil.CommitAll`).

## Prospective-snapshot / write-ahead materialization primitives (spec 128 Bead 6, OQ-1(a))

Spec 128's #186 fix (workflow domain's `complete` gate reorder — see that
domain's architecture doc) needed a way for `mindspec complete`'s content
gates (doc-sync, ADR-divergence) to measure a bead worktree's PROSPECTIVE
content BEFORE any complete-authored commit exists, so a gate refusal never
leaves the bead branch tip advanced past the reviewed SHA. This domain
supplies the git-I/O primitives that make that possible
(`internal/gitutil/materialize.go`), exposed to the workflow caller
exclusively through eight new `Executor` interface methods
(`internal/executor/executor.go`) — `internal/complete` gains no new
`internal/gitutil` import for the reorder (ADR-0030 boundary preserved):

- **`SnapshotWorktreeTree(wtPath, exclude)`** builds a candidate tree `T`
  from the worktree's prospective content (tracked + untracked + deletions,
  exactly what `git add -A` would stage) via a TEMPORARY index — the real
  index, refs, and worktree are never touched, only unreferenced loose
  objects are written. The temp index is seeded from the worktree's own
  HEAD tip FIRST, then `add -A` stages over that seed with `exclude`'s
  paths (today `.beads/issues.jsonl`) left out — seeding is load-bearing:
  an unseeded index plus the exclusion would make the excluded, currently
  tracked artifact ABSENT from `T`, silently deleting it from any commit
  built from `T`; seeded, the exclusion means "unchanged at tip".
- **`ChangedFilesTree(workdir, base, treeOID)`** diffs a committed ref
  against a bare tree OID (`-C workdir`, never a hardcoded root or the
  caller's cwd) — the tree-vs-tree leg the doc-sync/ADR-divergence gates
  use to measure `base..T` directly, so "committed tree ≡ measured tree"
  is mechanical once materialization runs.
- **`MaterializeStageCommit(wtPath, tree, parent, stage, subject)`** builds
  a commit object via plumbing (`git commit-tree`, no hooks) carrying a
  pinned `Mindspec-Stage` trailer (`MindspecStageTrailerKey`,
  `StageMaterialization`/`StageArtifactSync`) naming which of the two
  materialization stages it is — read back from the trailer on re-entry,
  never inferred from a tree diff (ambiguous when a stage's content is a
  no-op). An empty `subject` falls back to a stage-named default, never
  silently discarding an operator-supplied `--commit-msg`.
- **`PublishStageMarker`/`ReadStageMarker`/`ClearStageMarker`** implement
  the write-ahead marker protocol at `refs/mindspec/materialized/<bead-id>`:
  before either stage commit's branch advances, the intended tip is
  durably published via per-ref compare-and-swap (`PublishStageMarker`'s
  `expectedOld` — "" for CREATE semantics on the first publication of a
  run, the prior OID for the `C`→`D` transition). A failed publication
  refuses rather than overwriting another invocation's recovery evidence.
  `ClearStageMarker` deletes via compare-and-swap against the exact OID
  this invocation owns, so a stale clear can never delete evidence it
  does not own; clearing an already-absent marker is an idempotent no-op.
  These three stay PURE git primitives: the marker's provenance
  authentication (spec 128 final review — the `mindspec_stage_marker_oid`
  bd attestation written before each publish and required to match at
  resume, ADR-0041's provenance amendment) lives in the ENFORCEMENT
  layer (`internal/complete/materialize.go`), which owns the bd-metadata
  seam this boundary deliberately excludes.
- **`AdvanceBranchExpectedOld(branch, newSHA, expectedOld)`** moves
  `refs/heads/<branch>` via compare-and-swap — the tip advance a
  concurrent mover cannot silently clobber.
- **`ResetIndexToHead(wtPath)`** runs the index-only `reset --quiet --mixed HEAD`
  refreshing the real index to match the just-advanced HEAD's tree, leaving
  worktree files untouched — without it, a commit-tree-built commit's newly
  tracked paths read as phantom staged deletions in `git status`.

Together these implement the M(aterialize)→R(eset)→S(ync)→G(merge)→T(erminal
clear) staged sequence (ADR-0041 §3(i)): `M` builds candidate commit `C`
from `T` (`-p <tip>`); `R` refreshes the real index to `C`; `S` re-classifies
the exact delta about to be committed — any path outside the named-artifact
set REFUSES here, before the artifact-sync commit `D` is ever built — then
builds `D` (`-p C`); `G` is the pre-existing preserved-merge machinery,
untouched; `T` clears the marker on both the ordinary and landed-merge-
reconcile completion paths. `MockExecutor` (`internal/executor/mock.go`)
stubs all eight methods with configurable results/errors, defaulting to
zero values so a pre-existing test that never configures them exercises
the "nothing to materialize" (both stages elide) path unchanged.

`internal/bead/bdcli.go`'s `wrapBDError` (spec 128 R6.4/AC-11, motivated by
#146) decorates a failed bd invocation so its captured stderr
(`exec.ExitError.Stderr`) becomes part of the returned error's rendered
text — previously discarded by every `%w`-wrapping caller, leaving only
"exit status 1" for a diagnosable schema-drift or stale-binary failure. The
excerpt is capped at `bdStderrExcerptLineCap` (20) lines with a truncation
marker; non-exit errors and whitespace-only stderr pass through unchanged
rather than being decorated with visual noise. This is the CONTENT half of
AC-11 — the workflow domain's `internal/next` applies the per-line
`termsafe.Escape` RENDERING half before any such text reaches a terminal.

Two smaller `internal/gitutil` additions support workflow-domain honesty
requirements without themselves changing merge/materialization behavior:
`gitignore.go`'s `RuntimeIgnoreEntries` — the single canonical
never-track-these list `bootstrap`, `setup`, and `doctor` all share — gained
`.mindspec/phase.lock` (R3.3's lockfile); `gitops.go` gained
`FileExistsAtRef(workdir, ref, path)` (R1's cross-worktree draft-visibility
fix — `git cat-file -e` sees a freshly-committed spec draft on another
worktree's branch via the shared object store, without checking it out) and
`HasGitignoreEntry(root, entry)` (R5's claim-guard needs to know, in
advance, whether a coming `EnsureGitignoreEntry` call will actually WRITE,
so the two functions' notion of "present" must stay identical).

`internal/harness/sandbox.go` gained `.mindspec/phase.lock` to its list of
paths a sandbox fixture must gitignore/tolerate alongside `session.json`
(R3.3's lockfile is a local-runtime artifact, never version-controlled);
`scenario_bead_lifecycle.go` and `scenario_contract_hardening.go` exercise
the reordered materialization sequence end-to-end as fault-injection
scenarios (the KILL-TESTED/DOCUMENTED-FORWARD-SAFE classification, ADR-0041
§3, § M→R→S→G→T staged boundary, §3(i)).

## Finalization preflight and remote decision

Approval reads the remote configuration once through the error-returning
`RemoteConfigured` query. A read error refuses before lifecycle writes.
`FinalizeEpic` receives that value and does not probe again. Its `MergeStrategy`
is `pr` for true and `direct` for false, regardless of later remote configuration.

`PreflightFinalizeMerges` and `FinalizeEpic` use the same scoped candidate helper.
The early check covers pending bead-to-spec merges on both paths. It also covers
spec-to-main on the direct path. Both layout and work-destruction checks run
without writes. Live guards remain at each merge and read the tips after artifact
commits and auto-merges. A passed early check cannot replace a live check.

The push path runs `PushProbe` before the epic close. This probe skips hooks and
disables terminal prompts. It uses the real push's remote and refspec. A missing
credential helper alone does not refuse. Only a failed HTTP(S) authentication
probe gets a credential-helper hint. The real push still runs pre-push hooks.

## Final publication and stage resume

`FinalizeEpic` records `mindspec_finalize_tip` after export and bead auto-merges,
immediately before publication. It keeps `mindspec_pre_finalize_tip` as a separate
routing checkpoint. A retry at the recorded tip with a clean spec checkout skips
the export commit. Before it skips an already merged bead, it checks any remaining
worktree. Pending changes or an unreadable status stop the run before cleanup.
The branch and worktree remain. It does not create new artifacts on that bead.
It skips a push when the remote has the same tip and skips a completed direct
merge. The carrier branch is reused. A changed carrier head uses a lease on the
remote SHA read just before the push. No PR is opened by the executor.

`ResumeFinalizedEpic` handles a missing local spec branch. It checks the final tip
and all lifecycle child binding SHAs against the target. Remote evidence comes
from an exact head fetched into a temporary ref. A missing remote spec head
selects remote main. A failed fetch or ancestry check stops cleanup. Cleanup
repeats the destruction guard and requires branch-tip ancestry. Before removal,
it uses the same worktree check as `FinalizeEpic`. Pending changes, an unreadable
status, or a worktree inspection error preserve the worktree and branch and stop
cleanup. It can finish when a prior run removed a worktree but left its branch.

`FinalizeStageError` carries the failed stage without changing the inner error.
`SetFinalizeStepHookForTest` lets approval tests inject faults into real executor
runs. Tests use temporary git repos, fake tracker commands, and file remotes.
