---
approved_at: "2026-03-10T21:08:26Z"
approved_by: user
status: Approved
---
# Spec 081-executor-docs-tests: Executor rename, architecture docs, and LLM test review

## Goal

Rename `GitExecutor` to `MindspecExecutor` (including file rename `git.go` → `mindspec_executor.go`), purge legacy `gitops` and `specinit` terminology from codebase and documentation, update architecture documentation to clearly articulate the two-layer design (Workflow layer + Execution engine), and audit all 18 LLM test scenarios for correctness after the "stop between beads" behavioral change. Test audit findings go in `HISTORY.md`.

## Background

Spec 077 introduced the Executor interface separating workflow enforcement from execution mechanics. The implementation is complete and working, but two issues remain:

1. **Naming**: The production executor is called `GitExecutor`, which implies it's merely a git wrapper. In reality it orchestrates the full MindSpec execution lifecycle — worktree creation, bead dispatch, branch merging, PR creation, and cleanup. The name `MindspecExecutor` better reflects its role as the standard execution engine for the MindSpec workflow.

2. **Documentation gaps**: The architecture documentation doesn't clearly articulate the conceptual model. The Workflow layer is responsible for feeding the Execution engine a plan that has been broken down into bitesize beads — reviewed thoroughly, validated against architecture, with high-quality tests and clear acceptance criteria. The Execution engine is responsible for implementing those beads. This maps to research on scaling agent systems (Kim et al., "Towards a Science of Scaling Agent Systems," arXiv:2512.08296) which demonstrates that task decomposition quality directly impacts agent execution success.

3. **Test staleness**: The recent change to stop between beads (rather than auto-continuing) may have invalidated some LLM test scenario assumptions. All 18 scenarios need a thorough review.

4. **Legacy terminology**: The codebase still contains references to `gitops` (the pre-077 name for `gitutil`) and `specinit` (the package name for spec creation). These legacy names should be cleaned up:
   - `gitops` appears in test fixtures, historical spec docs, and HISTORY.md
   - `specinit` is still a live Go package (`internal/specinit/`) — should be renamed to something that reflects the workflow layer's vocabulary (e.g., `internal/spec/` or folded into the `spec create` command path)
   - Old spec/plan documents (048, 050, 051, 058, 062) reference `gitops` — these are historical artifacts but comments/cross-references in live code should be updated

## Impacted Domains

- **execution**: Rename `GitExecutor` → `MindspecExecutor` across package, tests, docs, and DI wiring
- **workflow**: Update documentation references to executor naming
- **context-system**: Update domain docs, architecture descriptions, and AGENTS.md
- **core**: Update MEMORY.md references (auto-memory files)

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): Beads as single state authority — documentation should reference this as foundation for the decomposition model
- [ADR-0006](../../adr/ADR-0006.md): Protected main with PR-based merging — execution layer documentation should reference this

## Requirements

### R1: Rename GitExecutor → MindspecExecutor

1. Rename file `internal/executor/git.go` → `internal/executor/mindspec_executor.go`
2. Rename `GitExecutor` struct to `MindspecExecutor`
3. Rename `NewGitExecutor` constructor to `NewMindspecExecutor`
4. Update all references in:
   - `internal/executor/executor_test.go`
   - `cmd/mindspec/root.go` (DI factory)
   - Domain documentation (`execution/overview.md`, `execution/architecture.md`, `execution/interfaces.md`)
   - `AGENTS.md` section on Workflow/Execution Boundary
   - Auto-memory files referencing `GitExecutor`

### R1b: Purge legacy `gitops` terminology

1. Update `internal/adr/store_test.go` test fixture — replace `gitops` tag with `gitutil` or `execution`
2. Update comment in `internal/executor/git.go` (line 74) referencing `specinit`
3. Update any live code comments referencing `gitops` (historical spec docs are left as-is — they're closed artifacts)

### R1c: Rename `specinit` package

1. Rename `internal/specinit/` → `internal/spec/` (or `internal/speccreate/`)
2. Update all import paths:
   - `cmd/mindspec/spec.go`
   - `cmd/mindspec/spec_init.go`
   - `internal/executor/git.go` (comment reference)
   - `internal/lifecycle/scenario_test.go` (comments)
3. Update domain documentation referencing `specinit`
4. Update AGENTS.md and MEMORY.md references
5. Update `spec_init.go` backward-compat alias registration in `root.go`

### R2: Architecture documentation overhaul

1. Update `AGENTS.md` §138–147 to clearly describe the two-layer model:
   - **Workflow layer**: Responsible for spec creation, plan decomposition into bitesize beads, validation against architecture (ADRs, domain boundaries), quality gates (tests, acceptance criteria), and phase enforcement
   - **Execution engine**: Responsible for implementing the plan — dispatching beads to worktrees, executing code changes, merging results, and finalizing the spec
2. Update `.mindspec/docs/domains/execution/overview.md` with the refined conceptual model
3. Update `.mindspec/docs/domains/execution/architecture.md` to reference `MindspecExecutor`
4. Update `.mindspec/docs/domains/workflow/` docs to describe the workflow layer's responsibility for plan quality
5. Reference the decomposition research (arXiv:2512.08296) where appropriate

### R4: Harden phase-transition stop behavior

Observed failure mode: after `mindspec approve plan`, the agent auto-proceeded to implement bead 1 on the spec branch instead of stopping, running `/clear`, and using `mindspec next` to create a proper bead worktree. Two root causes:

1. **`plan.md` instruct template is outdated** — still says "This will approve the plan AND automatically claim the first bead" (false since Spec 080)
2. **Plan approve output is not emphatic enough** — agent ignored the "Run /clear" guidance
3. **`mindspec complete` output says "Next bead ready: X"** which implicitly invites continuation

Fixes:

1. **Fix `plan.md` template** — remove the "auto-claim" lie. Clearly state: after plan approval, STOP. Run `/clear` or start a fresh agent, then `mindspec next`.
2. **Strengthen plan approve CLI output** — make the STOP instruction unmissable. Use a clear separator/banner.
3. **Strengthen `mindspec complete` CLI output** — after reporting "Next bead ready", add explicit STOP instruction: "Run `/clear`, then `mindspec next` to claim it."
4. **Remove dead `--no-next` flag** from `approve.go` — it's unused and misleading.
5. **Classify `mindspec next` and `mindspec complete` as execution layer commands** in documentation — they create/destroy worktrees and manage branch topology, which is execution, not workflow.

Note: `mindspec next` already correctly branches from the spec branch via `exec.DispatchBead(beadID, specID)`. No new `--base-branch` parameter needed — the specID already determines the base. The problem was the agent skipping `mindspec next` entirely.

### R3: LLM test scenario audit

Review all 18 scenarios in `internal/harness/scenario.go` and `scenario_test.go`:

1. **Verify each scenario's assumptions** match current behavior (stop between beads, manual `mindspec next`)
2. **Flag scenarios that are outdated** or test behavior that no longer exists
3. **Document findings** in `internal/harness/HISTORY.md` as a test audit section
4. **Fix any broken test expectations** — update assertions, prompts, or setup to match current behavior
5. Specific scenarios to scrutinize:
   - `TestLLM_SpecToIdle` — does the 100-turn full lifecycle still work with manual bead transitions?
   - `TestLLM_MultiBeadDeps` — already expects manual `mindspec next`, should be OK
   - `TestLLM_BlockedBeadTransition` — mode should be `plan` when only blocked beads remain
   - `TestLLM_UnmergedBeadGuard` — tests recovery flow, verify assumptions
   - Any test that previously assumed auto-continuation between beads

## Scope

### In Scope

- `internal/executor/git.go` → `internal/executor/mindspec_executor.go` (file rename + struct/constructor rename)
- `internal/executor/executor_test.go` — update references
- `internal/executor/mock.go` — no changes expected (already `MockExecutor`)
- `cmd/mindspec/root.go` — update DI factory
- `internal/specinit/` → `internal/spec/` (package rename)
- `cmd/mindspec/spec.go`, `cmd/mindspec/spec_init.go` — update imports
- `internal/adr/store_test.go` — update `gitops` test fixture
- `.mindspec/docs/domains/execution/` — all three docs
- `.mindspec/docs/domains/workflow/` — architecture and overview updates
- `AGENTS.md` — architecture section + legacy terminology
- `internal/harness/scenario.go` — test scenario review and fixes
- `internal/harness/scenario_test.go` — test function review and fixes
- `internal/harness/HISTORY.md` — test audit findings
- `internal/instruct/templates/plan.md` — fix outdated auto-claim guidance
- `internal/instruct/templates/implement.md` — strengthen STOP after complete
- `cmd/mindspec/plan_cmd.go` — strengthen plan approve output
- `cmd/mindspec/complete.go` or `internal/complete/complete.go` — strengthen complete output
- `cmd/mindspec/approve.go` — remove dead `--no-next` flag
- Auto-memory files referencing old naming

### Out of Scope

- New Executor implementations (e.g., GastownExecutor)
- Changes to the `Executor` interface itself
- New LLM test scenarios (only reviewing/fixing existing ones)
- Changes to `internal/gitutil/` package
- Historical spec/plan documents (048, 050, 051, 058, 062, 077 — closed artifacts, left as-is)

## Non-Goals

- Changing the Executor interface methods or signatures
- Adding new executor capabilities
- Refactoring the workflow layer's internal logic
- Writing new LLM test scenarios beyond fixing existing ones
- Updating historical spec/plan documents that reference `gitops` — these are closed artifacts
- Renaming `internal/gitutil/` (already correct per Spec 077)

## Acceptance Criteria

- [ ] `GitExecutor` renamed to `MindspecExecutor` everywhere (zero grep hits for `GitExecutor` in live code)
- [ ] `NewGitExecutor` renamed to `NewMindspecExecutor` everywhere
- [ ] `git.go` renamed to `mindspec_executor.go`
- [ ] `internal/specinit/` renamed (zero grep hits for `specinit` in Go imports)
- [ ] Zero grep hits for `gitops` in live Go code (test fixtures, comments)
- [ ] `make build` succeeds
- [ ] `go test ./internal/executor/... -v` passes
- [ ] `go test ./internal/... -v` passes (catch import path breakage)
- [ ] `go vet ./...` clean
- [ ] AGENTS.md clearly describes the two-layer architecture with workflow/execution responsibilities
- [ ] Domain docs updated with `MindspecExecutor` naming and refined conceptual model
- [ ] All 18 LLM test scenarios reviewed — findings documented in HISTORY.md
- [ ] Any outdated test expectations fixed
- [ ] `go test ./internal/harness/ -run TestLLM_SingleBead -timeout 10m` passes (smoke test)
- [ ] `plan.md` template no longer claims auto-claim behavior
- [ ] Plan approve output includes emphatic STOP + `/clear` + `mindspec next` instructions
- [ ] `mindspec complete` output includes STOP + `/clear` instruction when next bead is ready
- [ ] Dead `--no-next` flag removed from `approve.go`
- [ ] Documentation classifies `mindspec next` and `mindspec complete` as execution layer commands

## Validation Proofs

- `grep -r "GitExecutor" internal/ cmd/` → zero results
- `grep -r "NewGitExecutor" internal/ cmd/` → zero results
- `grep -rn "specinit" --include="*.go" internal/ cmd/` → zero import hits
- `grep -rn "gitops" --include="*.go" internal/ cmd/` → zero hits
- `ls internal/executor/mindspec_executor.go` → exists
- `ls internal/spec/` → exists (replaces `internal/specinit/`)
- `make build` → exit 0
- `go test ./internal/executor/... -v` → all pass
- `go test ./internal/... -v` → all pass
- `go vet ./...` → clean

## Open Questions

- [x] Should `internal/specinit/` become `internal/spec/` or `internal/speccreate/`? → **`internal/spec/`** — shorter, natural, no collision risk since it's the only package dealing with spec creation.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-10
- **Notes**: Approved via mindspec approve spec