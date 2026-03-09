---
approved_at: "2026-03-08T22:17:46Z"
approved_by: user
status: Approved
---
# Spec 079-location-agnostic-commands: Location Agnostic Commands

## Goal

Make `mindspec next` and `mindspec complete` runnable from **any directory** — removing CWD enforcement that blocks multi-agent workflows. Fix the worktree-removal bug (mindspec-qh1w), simplify the execution engine, and establish a clean separation between the **workflow layer** (what to do) and the **execution layer** (how to do it).

## Background

Today's CWD restrictions arose when mindspec assumed a single agent working serially in nested worktrees. With multi-agent parallelism, agents may be spawned from main, from a spec worktree, or from an arbitrary directory. The current guards cause friction:

- **`mindspec next`** hard-errors if CWD is main ("must run from a spec worktree"). When exactly one spec is active it auto-resolves, but with multiple active specs it fails instead of prompting.
- **`mindspec complete`** hard-errors if CWD is main. When run from inside a bead worktree (the normal case), worktree removal fails because `git worktree remove` can't remove the worktree you're standing in (bug mindspec-qh1w).
- **`mindspec approve impl`** already works from anywhere — it auto-chdirs to the spec worktree. This is the target pattern for all commands.

### Related bugs

- **mindspec-qh1w**: `mindspec complete` fails to remove bead worktree when invoked from inside it. `CompleteBead()` doesn't `chdir` to repo root before removal.
- **mindspec-tzh8**: `mindspec complete` advances to a blocked bead instead of returning to plan mode. (Partially related — state advancement logic.)

## Architecture: Workflow vs Execution Layers

This spec formalizes the separation between two layers that already exist implicitly but are not clearly documented or consistently applied.

### Workflow Layer (the "what")

Determines *what action to take* based on lifecycle state, user intent, and beads metadata. Responsible for:

- **Target resolution**: Which spec? Which bead? (via `internal/resolve/`, `internal/phase/`)
- **Validation & guards**: Is this action allowed in the current phase? Are prerequisites met? (via `internal/approve/`, `internal/validate/`)
- **State advancement**: After completing a bead, what mode should we be in? (via `internal/complete/`, `internal/next/`)
- **User interaction**: When ambiguity exists, emit a prompt for the user (not a hard error).

Key packages: `internal/resolve/`, `internal/phase/`, `internal/approve/`, `internal/complete/`, `internal/next/`, `internal/validate/`

### Execution Layer (the "how")

Performs git/worktree/filesystem operations. Responsible for:

- **Worktree lifecycle**: Create, remove, chdir safety (via `internal/executor/`)
- **Branch operations**: Create, merge, delete, push (via `internal/executor/`, `internal/gitutil/`)
- **Commit operations**: Stage, commit, clean-tree checks (via `internal/executor/`)

Key packages: `internal/executor/`, `internal/gitutil/`

### Interface boundary

The `executor.Executor` interface (Spec 077) is the formal boundary. Workflow packages call `Executor` methods. They MUST NOT import `internal/gitutil/` directly. This boundary is already enforced — this spec documents it and ensures the new location-agnostic behavior respects it.

### What `approve impl` does (Execution Layer)

`FinalizeEpic()` in `GitExecutor` performs these steps in order:

1. **Auto-commit remaining artifacts** in spec worktree (`git add . && git commit`)
2. **Auto-merge unmerged bead branches** — iterates bead worktrees, commits remaining artifacts, merges each `bead/<id>` into the spec branch (handles beads closed via `bd close` without `mindspec complete`)
3. **Gather stats** — commit count and diffstat for the summary
4. **Push or merge** — if remote exists: push spec branch + print PR URL. If no remote: `git merge --no-ff` into main
5. **Cleanup** — remove all bead worktrees/branches, remove spec worktree, delete spec branch

All of this runs via `withWorkingDir(root, ...)` — which is why `approve impl` already works from anywhere.

## Impacted Domains

### Workflow Layer
- `cmd/mindspec/next.go`: Remove CWD enforcement, add multi-spec numbered prompt
- `cmd/mindspec/complete.go`: Remove CWD enforcement, require bead ID positional arg
- `internal/complete/complete.go`: Scope to impl beads only, auto-chdir to spec/main worktree
- `internal/next/select.go`: No change needed (already auto-selects first unclaimed bead)
- `internal/resolve/target.go`: Add `ResolveSpecPrefix()` for integer-prefix matching (e.g. `077` → `077-execution-layer-interface`)

### Execution Layer
- `internal/executor/git.go`: Fix `CompleteBead()` to use `withWorkingDir(root, ...)` before worktree removal

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): State derivation from beads queries — this spec extends that principle so CWD is never required for state resolution

## Requirements

### `mindspec next`

1. **Runnable from anywhere**: MUST work from main repo root, spec worktree, bead worktree, or any directory within the repo.
2. **Multi-spec disambiguation via numbered prompt**: When multiple specs have unblocked beads and no `--spec` flag is given, the command MUST emit a numbered list and exit non-zero with an instruction for the agent to present the list to the user for selection. Format must follow Claude Code's `server_tool_use` / selection dialog conventions so the agent can relay the choice.
3. **`--spec` overrides CWD**: Even if CWD is inside spec worktree A, `--spec=B` targets spec B.
4. **Auto-select within a spec**: When a spec is resolved (via `--spec`, CWD, or single-active-spec), any unblocked unclaimed bead is selected automatically — no human prompt needed.

### `mindspec complete`

5. **Impl beads only**: `mindspec complete` is for implementation beads. Spec-phase and plan-phase beads use their own approval flows. If the bead is not an impl bead (i.e., the epic phase is not `implement`), error with guidance.
6. **Bead ID required as positional arg**: `mindspec complete <bead-id> ["commit message"]`. Remove auto-resolution from CWD/state. The bead ID is always explicit.
7. **Runnable from anywhere**: MUST work from main, spec worktree, or bead worktree. After resolving the bead, auto-chdir to the spec worktree (or main repo root) before worktree cleanup operations.
8. **Worktree removal fix**: `CompleteBead()` in `GitExecutor` MUST use `withWorkingDir(root, ...)` before calling worktree remove and branch delete. (Fixes mindspec-qh1w.)
9. **State advancement respects blockers**: When advancing state after completion, if only blocked beads remain, advance to `plan` mode, not `implement`. (Fixes mindspec-tzh8.)

### `--spec` prefix resolution (shared)

10. **Integer prefix matching**: `--spec=077` MUST resolve to the full spec ID (e.g., `077-execution-layer-interface`) by matching the numeric prefix against active specs. If ambiguous (shouldn't happen — spec numbers are unique), error with candidates.

## Scope

### In Scope

- `cmd/mindspec/next.go` — remove worktree scoping guards, add multi-spec numbered prompt, `--spec` overrides CWD
- `cmd/mindspec/complete.go` — remove worktree scoping guards, require bead ID positional arg, impl-only guard
- `internal/complete/complete.go` — accept explicit bead ID, auto-chdir before cleanup, blocker-aware state advancement
- `internal/executor/git.go` — fix `CompleteBead()` to use `withWorkingDir()`
- `internal/resolve/target.go` — add `ResolveSpecPrefix()` for integer prefix matching
- `internal/resolve/resolve.go` — expose prefix resolution in `ActiveSpecs` path

### Out of Scope

- Changing the worktree nesting architecture (bead WTs nested under spec WTs)
- Changing branch naming conventions (`spec/`, `bead/`)
- Modifying the Executor interface signature (methods stay the same, implementations change)
- Auto-finalize after last bead (already in config, orthogonal)
- Session freshness gate changes
- Changes to `approve impl` behavior (already location-agnostic; documented here for clarity)

## Non-Goals

- This spec does not change what `approve impl` *does* — only documents it clearly and ensures the other commands match its location-agnostic pattern.
- This spec does not add remote/distributed agent coordination — it enables local multi-agent parallelism within a single repo.
- This spec does not change the `approve spec` or `approve plan` flows.

## Acceptance Criteria

- [ ] `mindspec next` succeeds when CWD is main repo root (with exactly one active spec)
- [ ] `mindspec next` with multiple active specs and no `--spec` flag emits a numbered list and exits non-zero
- [ ] `mindspec next --spec=077` resolves to `077-execution-layer-interface` and selects beads from that spec
- [ ] `mindspec next --spec=077` overrides CWD even when running inside a different spec's worktree
- [ ] `mindspec complete <bead-id> "msg"` succeeds when CWD is main repo root
- [ ] `mindspec complete <bead-id>` succeeds when CWD is inside the bead worktree being completed (worktree removal works)
- [ ] `mindspec complete <bead-id>` errors with guidance when the bead is not an impl bead
- [ ] `mindspec complete` with no bead ID argument errors with usage guidance
- [ ] After completing a bead when only blocked beads remain, mode advances to `plan` (not `implement`)
- [ ] All existing LLM harness tests pass (no regression in SingleBead, MultiBeadLinear, etc.)

## Validation Proofs

- `mindspec next` from main with one active spec: should claim bead and create worktree
- `mindspec next` from main with two active specs (no `--spec`): should emit numbered list, exit 1
- `mindspec next --spec=079` from inside a different spec's worktree: should target 079
- `mindspec complete <bead-id> "done"` from main: should close bead, merge, remove worktree
- `mindspec complete <bead-id>` from inside that bead's worktree: should succeed (chdir before cleanup)
- `mindspec complete` with no args: should error with "bead ID required"
- LLM harness: `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM -timeout 10m`

## Open Questions

*All resolved:*
- ~~`--spec` overrides CWD~~ → Yes, `--spec` always wins.
- ~~`--bead` flag vs positional arg~~ → Positional arg. `mindspec complete <bead-id> ["msg"]`.
- ~~Numbered list vs instruction~~ → Numbered list, following Claude Code selection dialog conventions.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-08
- **Notes**: Approved via mindspec approve spec