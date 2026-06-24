---
approved_at: "2026-03-08T18:35:30Z"
approved_by: user
status: Approved
---
# Spec 077-execution-layer-interface: Separate Enforcement Layer from Execution Layer

## Goal

Separate mindspec into a **Workflow/Enforcement layer** (spec validation, approval gates, mode transitions, context packs) and an **Execution layer** (workspace creation, agent dispatch, merge operations) behind an interface, so the execution layer can be swapped for Gastown or any other orchestration system.

## Background

The enterprise knowledge vision (`~/enterprise-knowledge/vision-enterprise-knowledge.md`) describes mindspec as the enforcement layer and Gastown as the execution layer. Today these concerns are interleaved — `complete.Run()` validates bead state then merges branches, `ApproveImpl()` checks bead status then pushes and creates PRs.

The natural boundary is the **epic**. Enforcement creates the epic at spec approval and populates it with beads at plan approval. Then enforcement steps back. The entire implementation phase is execution's responsibility. Enforcement re-engages at `impl approve`.

```
Enforcement:  spec approve → create epic
Enforcement:  plan approve → populate epic with beads → HandoffEpic()
              ─── enforcement steps back ───
Execution:    dispatch beads, manage workspaces, merge code
              (GitExecutor: mindspec next/complete — manual)
              (GastownExecutor: mail mayor → autonomous)
Execution:    finalize when all beads closed
              (GitExecutor: requires human `impl approve` by default, configurable)
              (GastownExecutor: auto-finalizes — convoy resolves, refinery merges)
```

With `GastownExecutor`, the handoff is mail-based. `HandoffEpic()` sends a `task` message to the mayor with the epic ID, bead IDs, and suggestions (branch off `spec/<specID>`, adopt epic as convoy). The mayor dispatches on its own schedule. Mindspec doesn't call Gastown directly.

```
  HandoffEpic() → gt mail send mayor/ --type task
    epic: <epicID>, beads: [<beadID-1>, <beadID-2>, ...]
    base_branch: spec/<specID>
    suggestion: adopt epic as convoy

  mayor → convoy adopt → gt sling → polecats work → gt done → refinery merges
  human → mindspec impl approve → FinalizeEpic()
```

With `GitExecutor`, `HandoffEpic()` is a no-op. The user manually runs `mindspec next` and `mindspec complete` to drive execution one bead at a time.

## Impacted Domains

- gitops: `internal/gitops/` renamed to `internal/gitutil/` (the name "gitops" conflicts with the industry term for ArgoCD/Flux-style infrastructure-as-code)
- workflow: `internal/next/`, `internal/complete/`, `internal/approve/`, `internal/cleanup/`, `internal/specinit/` refactored to call the `Executor` interface

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Worktree-first flow becomes one implementation of the execution interface
- [ADR-0023](../../adr/ADR-0023.md): Enforcement queries beads for state, execution acts on the result

## Requirements

### Interface

1. A single `Executor` interface covering the epic/bead lifecycle:
   - `InitSpecWorkspace(specID string) (WorkspaceInfo, error)` — create workspace for spec authoring, including hooks and recording setup
   - `HandoffEpic(epicID string, specID string, beadIDs []string) error` — notify execution layer that beads are ready for dispatch
   - `DispatchBead(beadID string, specID string) (WorkspaceInfo, error)` — create workspace for a bead. `specID` provided so executor *can* branch from `spec/<specID>`, but branching strategy is the executor's concern
   - `CompleteBead(beadID string, specBranch string, msg string) error` — commit, merge bead work back, clean up workspace. Checks clean tree internally.
   - `FinalizeEpic(epicID string, specID string, specBranch string) (FinalizeResult, error)` — merge spec branch to main, push/PR, clean up
   - `Cleanup(specID string, force bool) error` — remove stale workspaces
   - `IsTreeClean(path string) error`, `DiffStat(base, head string) (string, error)`, `CommitCount(base, head string) (int, error)`, `CommitAll(path, msg string) error`
2. Interface uses domain terminology ("workspace" not "worktree") and lives in `internal/executor/` with no dependency on `internal/gitutil/`
3. `WorkspaceInfo`: `Path string`, `Branch string`. `FinalizeResult`: `MergeStrategy string`, `CommitCount int`, `DiffStat string`, `PRURL string`

### Enforcement boundaries

4. `plan approve` calls `HandoffEpic()` as its last step — enforcement's last action until finalization
5. `mindspec next` and `mindspec complete` call `DispatchBead`/`CompleteBead` directly as CLI commands, not through enforcement
6. Finalization is an execution concern. `GitExecutor` requires an explicit `mindspec impl approve` by default (configurable via `auto_finalize: true`). `GastownExecutor` auto-finalizes when all beads close — no human gate needed since Gastown's refinery already runs quality gates

### Implementation

7. `internal/gitops/` renamed to `internal/gitutil/`
8. `GitExecutor` wraps `internal/gitutil/`, preserving all current behavior (worktree-first creation, `--no-ff` merges, `.gitignore` management)
9. Consumers (`specinit`, `next`, `complete`, `cleanup`, `approve/impl`) refactored to call the `Executor` interface — no direct `gitutil` imports
10. `Executor` implementation injectable at CLI startup; defaults to `GitExecutor`

## Scope

### In Scope
- `internal/executor/executor.go` — interface definition
- `internal/executor/git.go` — `GitExecutor` wrapping `internal/gitutil/`
- Rename `internal/gitops/` → `internal/gitutil/`
- Refactor consumers to use `Executor`
- Dependency injection wiring in `cmd/mindspec/`

### Out of Scope
- `GastownExecutor` implementation (future spec)
- Changes to enforcement logic
- Headless/CI mode
- Changes to beads CLI or Dolt

## Non-Goals

- Supporting multiple executors simultaneously
- Abstracting beads — it's the shared substrate, not an execution concern
- Making the enforcement layer pluggable

## Acceptance Criteria

- [ ] `Executor` interface in `internal/executor/` with `InitSpecWorkspace`, `HandoffEpic`, `DispatchBead`, `CompleteBead`, `FinalizeEpic`, `Cleanup`, plus query methods
- [ ] `GitExecutor` implements `Executor` — all existing CLI commands produce identical behavior
- [ ] `internal/gitops/` renamed to `internal/gitutil/`, no `gitops` references remain
- [ ] No enforcement package imports `internal/gitutil` directly
- [ ] `plan approve` calls `HandoffEpic()`, enforcement does not call `DispatchBead`/`CompleteBead`
- [ ] `mindspec impl approve` calls `FinalizeEpic()` as a CLI command (execution), not an enforcement gate
- [ ] `GitExecutor` defaults to requiring `impl approve`; configurable via `auto_finalize`
- [ ] A mock `Executor` can run enforcement logic without git operations
- [ ] `go test ./...` passes with no behavioral changes

## Validation Proofs

- `go test ./...`: all existing tests pass
- `go test ./internal/executor/...`: interface + GitExecutor tests pass
- `grep -r "gitutil\." internal/specinit/ internal/next/ internal/complete/ internal/cleanup/ internal/approve/`: no direct imports
- `mindspec spec create test-dummy` + `mindspec complete "test"`: behavioral parity via GitExecutor

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-08
- **Notes**: Approved via mindspec approve spec