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
