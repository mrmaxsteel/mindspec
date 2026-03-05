---
approved_at: "2026-03-05T07:31:44Z"
approved_by: user
status: Approved
---
# Spec 072-hook-cleanup: Simplify hooks — remove redundant guards, thin-shim everything

## Goal

Drastically simplify the hook system. Remove Claude Code guard hooks that duplicate what guidance already achieves. Convert remaining hooks to thin `mindspec hook <name>` shims. Keep only the pre-commit git hook as a hard safety net.

## Background

MindSpec has two enforcement layers:

1. **Git hooks** (`.git/hooks/`) — pre-commit blocks commits on protected branches
2. **Claude Code hooks** (`.claude/settings.json`) — PreToolUse guards that block file edits, bash commands, and mode transitions before they happen

The Claude Code guard hooks were added as belt-and-suspenders enforcement on top of the `instruct` guidance. But the LLM test harness runs with **empty hooks** (`{}`) and passes — proving that guidance alone drives correct agent behavior.

### Current Claude Code hooks inventory

| Hook | Matcher | Purpose | Candidate for removal? |
|------|---------|---------|----------------------|
| `worktree-file` | Write, Edit | Blocks file edits outside active worktree | Yes — guidance sufficient |
| `worktree-bash` | Bash | Blocks bash outside active worktree | Yes — guidance sufficient |
| `workflow-guard` | Write, Edit, Bash | Blocks edits in idle/spec/plan mode | Yes — very aggressive, guidance sufficient |
| `plan-gate-enter` | EnterPlanMode | Warns when entering plan mode | Yes — guidance sufficient |
| `plan-gate-exit` | ExitPlanMode | Blocks exiting plan mode without approval | Yes — guidance sufficient |
| `needs-clear` | Bash | Session freshness gate for `mindspec next` | Maybe — unique safety, but could move to CLI |
| SessionStart | (inline) | Session recording + instruct emission | Keep — but convert to shim |

### Current git hooks inventory

| Hook | Purpose | Action |
|------|---------|--------|
| `pre-commit` | Block commits on protected branches | Keep — convert to shim |
| `post-checkout` | No-op placeholder | Remove |
| `post-merge`, `pre-push`, `prepare-commit-msg` | Beads shims | No change (beads-managed) |

### Key insight

The hooks add friction (blocking legitimate operations, requiring escape hatches) while the guidance alone is sufficient for correct behavior. The pre-commit git hook is the only one that provides unique value — it catches mistakes from both humans and agents at the last possible moment.

The `needs-clear` session freshness check can move into the `mindspec next` CLI command itself (where it belongs — it's a precondition, not a hook concern).

## Impacted Domains

- `internal/hooks/` — git hook installation
- `internal/hook/` — hook dispatch and logic
- `internal/setup/claude.go` — Claude Code settings.json hook entries
- `internal/next/` — absorb session freshness check from hooks
- `cmd/mindspec/hook.go` — wire remaining hooks

## ADR Touchpoints

- [ADR-0019](../../adr/ADR-0019.md): Branch enforcement layers — this spec collapses Layer 2 (agent hooks) into guidance, keeping only Layer 1 (pre-commit)

## Requirements

1. **Remove all PreToolUse guard hooks** — Remove from `wantedHooks()` and `internal/hook/`: `worktree-file`, `worktree-bash`, `workflow-guard`, `plan-gate-enter`, `plan-gate-exit`, `needs-clear`. Move session freshness check into `mindspec next` as a CLI-level precondition (with `--force` bypass).
2. **Remove dead post-checkout git hook** — Delete `postCheckoutScript`, `InstallPostCheckout()`, and references from `InstallAll()`.
3. **Convert pre-commit git hook to thin shim** — Replace 78-line bash with `exec mindspec hook pre-commit "$@"`. Move branch-protection logic to `PreCommit()` in Go. Keep `MINDSPEC_ALLOW_MAIN=1` escape hatch.
4. **Convert SessionStart to thin shim** — Replace inline shell with `mindspec hook session-start`. Add `SessionStart()` to hook dispatch (reads source from stdin, writes session, emits instruct).
5. **Update setup code** — `wantedHooks()` emits only SessionStart. `install.go` uses thin pre-commit shim. Stale detection upgrades old hooks. `mindspec setup claude` removes stale PreToolUse entries from existing settings.json.
6. **Clean up dead code** — Remove `WorktreeFile()`, `WorktreeBash()`, `WorkflowGuard()`, `PlanGateEnter()`, `PlanGateExit()`, `SessionFreshnessGate()`, `EnforcementEnabled()`, associated helpers/constants/tests.
7. **Validate with LLM tests** — Run harness to confirm behavior unchanged (tests already run with empty hooks).

## Scope

### In Scope
- `internal/hooks/install.go` — remove post-checkout, thin-shim pre-commit
- `internal/hook/dispatch.go` — remove guard hooks, add PreCommit(), SessionStart()
- `internal/hook/hook.go` — update Names list, remove EnforcementEnabled()
- `internal/hook/helpers.go` — remove dead helpers
- `internal/setup/claude.go` — strip wantedHooks() down to SessionStart only, add stale-entry removal
- `cmd/mindspec/hook.go` — wire new hooks
- `internal/next/` — absorb session freshness check
- Tests for the above

### Out of Scope
- Beads git hooks — already thin shims
- Instruct templates / guidance content — not changing what guidance says
- Copilot hooks (`internal/setup/copilot.go`)

## Non-Goals

- Changing what the pre-commit hook enforces
- Modifying instruct templates or CLAUDE.md guidance
- Adding new hook types

## Acceptance Criteria

- [ ] `settings.json` has zero PreToolUse hook entries after `mindspec setup claude`
- [ ] `settings.json` SessionStart calls `mindspec hook session-start`
- [ ] `post-checkout` hook no longer installed
- [ ] `pre-commit` is a thin shim calling `mindspec hook pre-commit`
- [ ] `mindspec next` enforces session freshness directly (no hook needed)
- [ ] `make test` passes
- [ ] LLM harness SingleBead passes

## Validation Proofs

- `make test`: all tests pass
- `mindspec hook --list`: shows only `pre-commit` and `session-start`
- `echo '{}' | mindspec hook pre-commit`: exits 0 when not on protected branch
- `MINDSPEC_ALLOW_MAIN=1 echo '{}' | mindspec hook pre-commit`: exits 0 (escape hatch)
- LLM harness SingleBead: passes

## Open Questions

- (none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-05
- **Notes**: Approved via mindspec approve spec