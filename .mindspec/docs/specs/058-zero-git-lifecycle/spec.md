---
approved_at: "2026-03-03T09:20:31Z"
approved_by: user
status: Approved
---
# Spec 058-zero-git-lifecycle: Zero Raw Git Lifecycle

## Goal

Eliminate all raw git commands from the agent's normal workflow. The agent should only write files and call mindspec commands. All git operations (commit, merge, branch, worktree) are handled deterministically inside mindspec commands.

The happy path never requires raw git. Raw git remains available for repair and recovery — it is not hard-blocked.

Secondary goals:
- Remove explore as a distinct mode and command (idle is sufficient)
- Unify spec creation under `mindspec spec create` namespace
- Adopt phase-first CLI namespacing (`mindspec spec approve` instead of `mindspec approve spec`)

## Background

LLM test harness analysis (2026-03-03 full suite, SpecToIdle scenario) revealed that 8 of 15 agent retries were caused by the agent running `git merge main` during implementation — a reasonable-seeming action that mindspec's guidance didn't explicitly forbid, which then cascaded into 7 `approve impl` merge conflict failures.

The root cause: the agent has access to raw git and doesn't understand the merge topology that `approve impl` expects. Adding more guard hooks is a losing game — each new rule is another thing the LLM might ignore. The solution is to make raw git unnecessary by handling all git operations inside mindspec commands.

Current state before this spec:
- `spec-init`, `approve spec`, `approve plan` already auto-commit
- `approve impl` already auto-merges (bead→spec→main) and auto-cleans (worktrees, branches)
- `mindspec next` already creates worktrees and branches
- **Only gap**: agent must run `git add` + `git commit` before `mindspec complete`

Additionally:
- Explore mode adds state complexity with minimal value — the "exploration" is just a conversation that can happen in idle
- Two redundant entry paths to spec mode exist (`spec-init` from idle, `explore promote` from explore)
- Command namespacing is inconsistent (`mindspec approve spec` vs. `mindspec spec-init`)

## Impacted Domains

- lifecycle: remove explore mode, reduce from 6 modes (idle, explore, spec, plan, implement, review) to 5 (idle, spec, plan, implement, review)
- instruct: all templates updated — lifecycle map added, raw git references removed, explore template deleted
- CLI commands: `complete` gains auto-commit; `spec create`, `spec approve`, `plan approve`, `impl approve` added as phase-first commands; `explore` command and package removed entirely
- state: `ModeExplore` constant removed from `state.go`
- hooks: explore case removed from WorkflowGuard; idle block message updated
- harness: AbandonSpec scenario removed; remaining scenarios updated for new command names

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): worktree-first spec-init — `spec create` reuses `specinit.Run()`, no change to worktree creation
- [ADR-0022](../../adr/ADR-0022.md): worktree-aware resolution — complete's auto-commit uses same worktree-aware paths

## Requirements

1. `mindspec complete "message"` auto-commits all changes in the bead worktree before closing
2. `mindspec complete` with no message and dirty tree fails with hint suggesting `mindspec complete "describe what you did"`
3. All instruct templates include a lifecycle map showing the full command sequence with current phase highlighted
4. No instruct template references raw git commands as part of the normal workflow (raw git mentioned only as available for repair/recovery)
5. `mindspec spec create <slug>` replaces `spec-init` as the primary spec creation command
6. `mindspec spec approve`, `mindspec plan approve`, `mindspec impl approve` adopt phase-first namespacing
7. Old commands (`spec-init`, `approve spec`, `approve plan`, `approve impl`) remain as hidden backward-compatible aliases
8. Explore mode removed entirely — `mindspec explore` command deleted, `ModeExplore` constant deleted, explore instruct template deleted, explore package deleted
9. LLM test harness scenarios updated to reflect new command names
10. WORKFLOW-STATE-MACHINE.md updated to reflect new lifecycle (no explore, phase-first commands, git convention)

## Scope

### In Scope

- `internal/complete/complete.go` — add commitMsg parameter, auto-commit before clean-tree check
- `cmd/mindspec/complete.go` — wire positional commit message arg
- `cmd/mindspec/spec.go` (new) — `spec` parent with `create` and `approve` subcommands
- `cmd/mindspec/plan_cmd.go` (new) — `plan` parent with `approve` subcommand
- `cmd/mindspec/impl.go` (new) — `impl` parent with `approve` subcommand
- `cmd/mindspec/approve.go` — rewritten as hidden backward-compat aliases
- `cmd/mindspec/spec_init.go` — marked as hidden alias
- `cmd/mindspec/root.go` — register new command tree, remove explore
- `cmd/mindspec/explore.go` — deleted
- `internal/explore/` — deleted (entire package)
- `internal/instruct/templates/explore.md` — deleted
- `internal/instruct/templates/*.md` — remaining 5 templates: lifecycle map, softened git rules
- `internal/instruct/instruct.go` — remove explore from BuildContext and gatesForMode
- `internal/state/state.go` — remove `ModeExplore` and from `ValidModes`
- `internal/hook/dispatch.go` — remove explore case from WorkflowGuard, update blockIdle message
- `internal/setup/claude.go` — remove ms-explore skill definition
- `internal/bootstrap/bootstrap.go` — remove explore from modes list and skills table
- `CLAUDE.md` — remove explore references, update command surface
- `.mindspec/docs/core/WORKFLOW-STATE-MACHINE.md` — full rewrite for new lifecycle
- `internal/harness/scenario.go` — remove AbandonSpec, update remaining scenarios
- All corresponding test files updated

### Out of Scope

- `mindspec bugfix` convenience command (future spec)
- Removing hook enforcement (stays as defense-in-depth)
- Changes to `approve spec/plan/impl` internals (already auto-commit)
- ~~Enforcing worktree scoping~~ (implemented: `next` requires spec worktree, `complete` requires bead worktree)
- Multi-agent parallel bead execution (the primitives support it but orchestration is future work)

## Non-Goals

- Changing the merge topology (bead→spec→main hierarchy stays)
- Adding new modes or gates
- Changing the bead auto-creation logic in `approve plan`
- Redesigning the hook system
- Hard-blocking raw git commands — the happy path simply doesn't require them

## Design Decisions

### Git commands: convention, not enforcement

Raw git commands are not blocked by hooks or guards. The approach is to make them unnecessary:
- `mindspec complete "msg"` handles commit + merge + cleanup
- `mindspec spec create` handles branch + worktree creation
- All approve commands handle their own commits

The instruct templates say "you should not need raw git" rather than "do NOT run raw git." This is intentional — agents sometimes need to repair git state, and blocking that creates more problems than it solves.

### Explore removed entirely, not just simplified

Originally the plan was to keep explore as a guidance-only command. During implementation, we determined explore provided no value that idle mode doesn't already cover:
- `explore "idea"` just printed guidance → the idle template already covers this
- `explore promote <slug>` was just an alias for `spec create` → unnecessary indirection
- `explore dismiss` was a no-op → pointless

Removing it entirely (command, package, template, state constant) is simpler than maintaining dead code.

### `mindspec next` / `mindspec complete` worktree scoping

The execution model:
- `mindspec next` runs from a **spec worktree** → creates a bead worktree off the spec branch
- `mindspec complete` runs from a **bead worktree** → auto-commits, merges bead→spec, cleans up bead worktree + branch

This scoping is enforced via `workspace.DetectWorktreeContext()`. Running from the wrong context produces a hard error — there is no escape hatch. `complete` will auto-redirect to the active bead worktree from focus before checking the guard.

Parallel bead execution (multiple agents running `next`/`complete` in separate bead worktrees) works correctly by design: each bead worktree has its own focus file, and the DAG dependency graph ensures dependent beads cannot be claimed until prerequisites are closed.

## Acceptance Criteria

- [x] Agent can complete full lifecycle (spec→idle) using only mindspec commands — no raw git commands required
- [x] `mindspec complete "implemented greeting feature"` auto-commits dirty worktree, then closes bead, merges, cleans up
- [x] `mindspec complete` with no message and dirty tree fails with hint
- [x] `mindspec spec create <slug>` creates branch + worktree + spec template
- [x] `mindspec spec-init` still works (hidden alias)
- [x] `mindspec spec approve`, `mindspec plan approve`, `mindspec impl approve` work as phase-first commands
- [x] Old command forms (`approve spec`, etc.) still work as hidden aliases
- [x] Explore mode fully removed (command, package, template, state constant)
- [x] All 5 instruct templates contain a lifecycle map with phase-specific `>>>` marker
- [x] No instruct template references raw git as part of normal workflow
- [x] `make test` passes
- [x] `TestLLM_SingleBead` passes — agent completes bead using mindspec workflow
- [x] `TestLLM_SpecToIdle` passes — agent completes full lifecycle (idle→spec→plan→implement→review→idle)

## Validation Proofs

- `make build && make test`: all unit tests pass
- `go test ./internal/complete/ -v`: auto-commit unit tests pass
- `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m -count=1`: **PASS** (5 turns, 141 events)
- `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SpecToIdle -timeout 15m -count=1`: **PASS** (19 turns, 485 events)

## Open Questions

None — all design decisions resolved during implementation.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-03
- **Notes**: Approved via mindspec approve spec
