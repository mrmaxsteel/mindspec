---
approved_at: "2026-03-08T18:46:21Z"
approved_by: user
bead_ids:
    - mindspec-yosv.6
    - mindspec-yosv.7
    - mindspec-yosv.8
    - mindspec-yosv.9
    - mindspec-yosv.10
last_updated: "2026-03-08"
spec_id: 077-execution-layer-interface
status: Approved
version: "1"
---
# Plan: 077 — Separate Enforcement Layer from Execution Layer

## ADR Fitness

- **ADR-0006** (Protected main with PR-based merging): The worktree-first flow becomes one implementation (`GitExecutor`) of the `Executor` interface. `InitSpecWorkspace` and `DispatchBead` encapsulate worktree creation; `FinalizeEpic` encapsulates the PR-or-merge decision. No ADR changes needed — the ADR describes the *what*, the interface abstracts the *how*.
- **ADR-0023** (Beads as single state authority): Enforcement queries beads for phase/status. Execution acts on the result. The `Executor` interface does NOT abstract beads — beads remain the shared substrate that both layers query directly. This preserves ADR-0023's intent.

## Testing Strategy

- **Unit tests** for `GitExecutor` methods using the existing function-variable injection pattern (same approach as `internal/gitops/gitops.go` today)
- **Mock executor** (`internal/executor/mock.go`) that records calls without git side effects — enables enforcement unit tests without git operations
- **Integration**: `go test ./...` must pass with zero behavioral changes. Existing LLM harness tests validate end-to-end behavior via the CLI.
- **Grep proof**: `grep -r "gitutil\." internal/specinit/ internal/next/ internal/complete/ internal/cleanup/ internal/approve/` returns no hits after refactoring

## Bead 1: Rename `internal/gitops/` → `internal/gitutil/`

Mechanical rename. Every consumer import updates from `gitops` to `gitutil`. No logic changes.

**Steps**
1. `git mv internal/gitops/ internal/gitutil/`
2. Update `package gitutil` declaration in all files under `internal/gitutil/`
3. Find-and-replace all `"github.com/mrmaxsteel/mindspec/internal/gitops"` → `"github.com/mrmaxsteel/mindspec/internal/gitutil"` across the codebase
4. Find-and-replace all `gitops.` call sites → `gitutil.` in consumer code
5. Update any references in MEMORY.md or CLAUDE.md

**Verification**
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `grep -r "gitops" internal/ cmd/` returns zero hits (excluding comments/docs)

**Depends on**
None

## Bead 2: Define `Executor` interface + implement `GitExecutor`

Create the interface and wrap existing `gitutil` functions into `GitExecutor` methods.

**Steps**
1. Create `internal/executor/executor.go` with:
   - `Executor` interface: `InitSpecWorkspace`, `HandoffEpic`, `DispatchBead`, `CompleteBead`, `FinalizeEpic`, `Cleanup`, `IsTreeClean`, `DiffStat`, `CommitCount`, `CommitAll`
   - `WorkspaceInfo` struct: `Path`, `Branch`
   - `FinalizeResult` struct: `MergeStrategy`, `CommitCount`, `DiffStat`, `PRURL`
2. Create `internal/executor/git.go` with `GitExecutor` struct implementing the full interface, delegating to `gitutil` + beads worktree CLI for workspace operations. Each method mirrors the current logic in its respective consumer (specinit, next, complete, cleanup, approve/impl) — extracted, not reimagined.
3. Create `internal/executor/mock.go` with `MockExecutor` that records method calls and returns configurable results
4. Create `internal/executor/executor_test.go` — unit tests for `GitExecutor` using function-variable injection
5. `internal/executor/` must NOT import any enforcement package (`validate`, `approve`, `bead/gate`, `state`)

**Verification**
- [ ] `go test ./internal/executor/...` passes
- [ ] `MockExecutor` satisfies the `Executor` interface (compile check)
- [ ] `go vet ./internal/executor/...` clean

**Depends on**
Bead 1

## Bead 3: Refactor workspace creation consumers (`specinit`, `next`)

Replace direct `gitutil` calls in the workspace-creation path with `Executor` methods.

**Steps**
1. **`internal/specinit/specinit.go`**: Replace `createBranchFn`, `worktreeCreateFn`, `ensureGitignore`, `gitops.CommitAll` with a single `Executor.InitSpecWorkspace()` call. `Run()` signature gains an `Executor` parameter. Spec-file writing (templates, recording) stays in `specinit` — that's enforcement content, not execution.
2. **`internal/next/beads.go`**: Replace `createBranchFn`, `worktreeCreate`, `ensureGitignore` in `EnsureWorktree()` with `Executor.DispatchBead()`. Bead querying/claiming stays in `next` — that's enforcement.
3. **`internal/next/git.go`**: Replace `CheckCleanTree()` with `Executor.IsTreeClean()`.
4. Update `cmd/mindspec/spec_init.go` and `cmd/mindspec/next.go` to construct and pass `GitExecutor`.
5. Remove now-unused function variables from both packages.

**Verification**
- [ ] `go test ./internal/specinit/...` passes
- [ ] `go test ./internal/next/...` passes
- [ ] `grep -r "gitutil\." internal/specinit/ internal/next/` returns zero hits

**Depends on**
Bead 2

## Bead 4: Refactor workspace teardown consumers (`complete`, `cleanup`)

Replace direct `gitutil` calls in the teardown path with `Executor` methods.

**Steps**
1. **`internal/complete/complete.go`**: Replace `commitAllFn`, `mergeIntoFn`, `worktreeRemoveFn`, `deleteBranchFn` with `Executor.CompleteBead()`. The `CompleteBead` method handles: commit pending changes → merge bead branch into spec branch → remove worktree → delete branch. Clean-tree checking uses `Executor.IsTreeClean()`. Bead closing (`bd close`) stays in `complete` — that's enforcement.
2. **`internal/cleanup/cleanup.go`**: Replace `worktreeRemoveFn`, `deleteBranchFn` with `Executor.Cleanup()`. Bead-context validation stays in `cleanup`.
3. Update `cmd/mindspec/complete.go` and `cmd/mindspec/cleanup.go` to pass `GitExecutor`.
4. Remove now-unused function variables.

**Verification**
- [ ] `go test ./internal/complete/...` passes
- [ ] `go test ./internal/cleanup/...` passes
- [ ] `grep -r "gitutil\." internal/complete/ internal/cleanup/` returns zero hits

**Depends on**
Bead 2

## Bead 5: Refactor `approve/impl`, add `HandoffEpic`, DI wiring, `auto_finalize`

Final consumer refactoring plus the enforcement boundary changes and dependency injection.

**Steps**
1. **`internal/approve/impl.go`**: Extract all git operations (commitAll, mergeBranch/pushBranch, worktreeRemove, deleteBranch, diffStat, commitCount, cleanupBeadBranchesAndWorktrees) into `Executor.FinalizeEpic()`. The enforcement shell in `ApproveImpl` retains: epic closure, phase validation, bead status verification. It calls `Executor.FinalizeEpic()` for the actual merge/push/cleanup.
2. **`internal/approve/plan.go`**: Add `Executor.HandoffEpic()` call as the last step of plan approval. For `GitExecutor`, `HandoffEpic` is a no-op (returns nil). For a future `GastownExecutor`, this would send `gt mail send mayor/ --type task` with epic/bead context.
3. **`auto_finalize` config**: Add `auto_finalize: bool` to mindspec config. `GitExecutor` reads this; when true, `FinalizeEpic` can be triggered automatically after the last bead closes (future: wired into `complete`). Default: false (require explicit `impl approve`).
4. **DI wiring in `cmd/mindspec/root.go`**: Construct `GitExecutor` once at startup, pass to all subcommand RunE functions. Use a `newExecutor(root string) executor.Executor` factory function.
5. **Final sweep**: Remove all direct `gitutil` imports from enforcement packages. Verify mock executor can drive `ApproveSpec` and `ApprovePlan` without git.

**Verification**
- [ ] `go test ./...` passes — zero behavioral changes
- [ ] `grep -r "gitutil\." internal/specinit/ internal/next/ internal/complete/ internal/cleanup/ internal/approve/` returns zero hits
- [ ] `go test ./internal/executor/...` passes with mock tests
- [ ] `mindspec spec create test-dummy` + `mindspec complete "test"` works end-to-end via GitExecutor

**Depends on**
Bead 3, Bead 4

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| Executor interface in internal/executor/ | Bead 2 compile check |
| GitExecutor implements Executor — identical behavior | Bead 5 end-to-end test |
| gitops renamed to gitutil, no references remain | Bead 1 grep proof |
| No enforcement package imports gitutil | Bead 5 grep proof |
| plan approve calls HandoffEpic | Bead 5 |
| impl approve calls FinalizeEpic | Bead 5 |
| GitExecutor defaults to requiring impl approve; auto_finalize configurable | Bead 5 |
| Mock Executor can run enforcement without git | Bead 5 mock test |
| go test ./... passes with no behavioral changes | Bead 5 final verification |
