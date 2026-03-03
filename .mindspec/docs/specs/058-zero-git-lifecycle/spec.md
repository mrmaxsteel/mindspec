---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 058-zero-git-lifecycle: Zero Raw Git Lifecycle

## Goal

Eliminate all raw git commands from the agent's workflow. The agent should only write files and call mindspec commands. All git operations (commit, merge, branch, worktree) are handled deterministically inside mindspec commands.

Secondary goal: simplify the lifecycle from 6 modes to 4 by merging idle and explore into a single mode, and unify spec creation under a `mindspec spec` namespace.

## Background

LLM test harness analysis (2026-03-03 full suite, SpecToIdle scenario) revealed that 8 of 15 agent retries were caused by the agent running `git merge main` during implementation — a reasonable-seeming action that mindspec's guidance didn't explicitly forbid, which then cascaded into 7 `approve impl` merge conflict failures.

The root cause: the agent has access to raw git and doesn't understand the merge topology that `approve impl` expects. Adding more guard hooks is a losing game — each new rule is another thing the LLM might ignore. The solution is to remove git from the agent's interface entirely.

Current state:
- `spec-init`, `approve spec`, `approve plan` already auto-commit
- `approve impl` already auto-merges (bead→spec→main) and auto-cleans (worktrees, branches)
- `mindspec next` already creates worktrees and branches
- **Only gap**: agent must run `git add` + `git commit` before `mindspec complete`

Additionally, the lifecycle has two redundant entry paths to spec mode (`spec-init` from idle, `explore promote` from explore), and explore mode adds state complexity with minimal value — the "exploration" is just a conversation that can happen in idle.

## Impacted Domains

- lifecycle: reduce modes from 6 (idle, explore, spec, plan, implement, review) to 4 (idle, spec, plan, implement+review)
- instruct: all 6 templates updated — lifecycle map added, raw git references removed
- CLI commands: `complete` gains auto-commit, `spec-init` renamed to `spec create`
- harness: test scenarios updated for new command names

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): worktree-first spec-init — `spec create` reuses `specinit.Run()`, no change to worktree creation
- [ADR-0022](../../adr/ADR-0022.md): worktree-aware resolution — complete's auto-commit uses same worktree-aware paths

## Requirements

1. `mindspec complete "message"` auto-commits all changes in the bead worktree before closing
2. All instruct templates include a lifecycle map showing the full command sequence with current phase highlighted
3. No instruct template references raw git commands (no `git add`, `git commit`, `git merge`, `git pull`, `git branch`, `git checkout`, `git rebase`)
4. `mindspec spec create <slug>` replaces `spec-init` as the primary spec creation command
5. `spec-init` remains as a hidden backward-compatible alias
6. Explore mode is removed as a distinct mode — `mindspec explore` becomes a guidance-only command that doesn't change state
7. `mindspec explore promote <slug>` becomes an alias for `mindspec spec create <slug>`
8. LLM test harness scenarios updated to reflect new command names

## Scope

### In Scope

- `internal/complete/complete.go` — add commitMsg parameter, auto-commit before clean-tree check
- `cmd/mindspec/complete.go` — wire positional commit message arg
- `cmd/mindspec/spec_init.go` → `cmd/mindspec/spec.go` — `spec` parent with `create` subcommand
- `cmd/mindspec/root.go` — register new command tree
- `cmd/mindspec/explore.go` — simplify explore (no mode change)
- `internal/explore/explore.go` — `Enter()` no longer writes state
- `internal/instruct/templates/*.md` — all 6 templates: lifecycle map, no raw git
- `internal/harness/scenario.go` — update scenario commands and assertions

### Out of Scope

- `mindspec bugfix` convenience command (future spec)
- Removing hook enforcement (stays as defense-in-depth)
- Changes to `approve spec/plan/impl` internals (already auto-commit)
- Changes to `mindspec next` (already handles worktree creation)
- Removing `ModeExplore` constant from state.go (keep for backward compat)

## Non-Goals

- Changing the merge topology (bead→spec→main hierarchy stays)
- Adding new modes or gates
- Changing the bead auto-creation logic in `approve plan`
- Redesigning the hook system

## Acceptance Criteria

- [ ] Agent can complete full lifecycle (spec→idle) using only: `mindspec spec create`, `mindspec approve spec`, `mindspec approve plan`, `mindspec next`, `mindspec complete "msg"`, `mindspec approve impl` — no raw git commands
- [ ] `mindspec complete "implemented greeting feature"` auto-commits dirty worktree, then closes bead, merges, cleans up
- [ ] `mindspec complete` with no message and dirty tree fails with hint: `mindspec complete "describe what you did"`
- [ ] `mindspec spec create <slug>` creates branch + worktree + spec template (same behavior as current `spec-init`)
- [ ] `mindspec spec-init` still works (hidden alias)
- [ ] `mindspec explore "idea"` does NOT change mode — emits explore guidance only
- [ ] `mindspec explore promote <slug>` delegates to `mindspec spec create <slug>`
- [ ] All 6 instruct templates contain a lifecycle map with phase-specific `>>>` marker
- [ ] No instruct template contains raw git command instructions
- [ ] `make test` passes
- [ ] `TestLLM_SingleBead` passes without agent running any raw git commands
- [ ] `TestLLM_SpecToIdle` passes — agent completes full lifecycle using only mindspec commands

## Validation Proofs

- `make build && make test`: all unit tests pass
- `go test ./internal/complete/ -v`: auto-commit unit tests pass
- `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m -count=1`: agent uses `mindspec complete "msg"` without raw git
- `env -u CLAUDECODE go test ./internal/harness/ -v -run '^TestLLM_' -timeout 180m -count=1`: full suite passes

## Open Questions

None — all design decisions resolved during exploration.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
