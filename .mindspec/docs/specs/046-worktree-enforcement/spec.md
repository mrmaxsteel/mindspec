---
approved_at: "2026-02-26T08:37:10Z"
approved_by: user
molecule_id: mindspec-mol-7d0
status: Approved
step_mapping:
    implement: mindspec-mol-wz9
    plan: mindspec-mol-dtw
    plan-approve: mindspec-mol-qmq
    review: mindspec-mol-li5
    spec: mindspec-mol-otu
    spec-approve: mindspec-mol-19e
    spec-lifecycle: mindspec-mol-7d0
---


# Spec 046-worktree-enforcement: Deterministic Worktree and Branch Enforcement

## Goal

Enforce zero-on-main invariant: all mindspec-managed changes happen on branches in worktrees, never on main. Enforcement is deterministic (git hooks, CLI guards, agent hooks) — not prompt-based. An agent running in an IDE workspace rooted at main must have its CLI calls and file writes redirected to the correct worktree context.

## Background

MindSpec currently relies on prompt guidance (`implement.md` template) to tell agents to work in worktrees. This fails regularly — agents write to main because:
- `mindspec next` prints `cd <path>` but the agent's CWD doesn't change
- No git hook blocks commits to protected branches
- No agent hook intercepts file writes outside the expected worktree
- CLI commands don't validate they're running from the correct worktree

ADR-0006 (Accepted) establishes the branching model: zero changes on main, one `spec/NNN-slug` branch per spec lifecycle, impl beads branching from the spec branch, one PR to main at the end. ADR-0019 (Accepted) defines the three-layer enforcement mechanism.

### Critical: CWD determines CLI context

When an agent opens a session in an IDE, the workspace root is typically the main worktree. All `mindspec` and `bd` CLI calls execute from this CWD. This means:
- `mindspec instruct` reads main's `.mindspec/state.json` (which is idle)
- `bd list` reads from main's context
- File writes go to main's working tree

**The enforcement must ensure that once a worktree is active, CLI calls happen from the worktree CWD, not main.** This is the most important enforcement point — without it, the agent operates in the wrong context entirely.

## Impacted Domains

- **workflow**: Spec/plan/implement lifecycle commands all gain worktree awareness and guards
- **git**: Pre-commit hook, branch naming, worktree creation/cleanup
- **agent-integration**: PreToolUse hooks for Claude Code and Copilot, SessionStart hook updates

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Defines the branching model (zero-on-main, spec branch as integration point, one PR per lifecycle)
- [ADR-0019](../../adr/ADR-0019.md): Defines the three-layer enforcement mechanism (git hook, CLI guards, agent hooks)
- [ADR-0015](../../adr/ADR-0015.md): Per-spec molecule-derived state — enforcement reads `state.json` as a fast hint, not the molecule
- [ADR-0002](../../adr/ADR-0002.md): MindSpec/Beads integration — enforcement policy is mindspec's, plumbing is beads'

## Requirements

### R1: Worktree lifecycle management

1. `mindspec spec-init <NNN-slug>` creates a `spec/NNN-slug` branch from main and a worktree, sets `activeWorktree` in state.json, and instructs the agent to `cd` into it.
2. Spec and plan phases share the same worktree and branch (`spec/NNN-slug`).
3. `mindspec next` creates `bead/<bead-id>` branches from `spec/NNN-slug` (not main), each in its own worktree, and updates `activeWorktree`.
4. `mindspec complete` merges the bead branch back to `spec/NNN-slug` and removes the bead worktree.
5. `/impl-approve` (all beads done) creates a PR or merges `spec/NNN-slug → main` based on `merge_strategy`, then removes the spec worktree.

### R2: Git pre-commit hook (Layer 1)

6. A mindspec pre-commit hook blocks commits to branches in the `protected_branches` list (default: `[main, master]`) whenever mindspec state is non-idle.
7. The hook is installed via beads' hook chaining mechanism (runs alongside beads' own pre-commit hook).
8. The hook reads `.mindspec/state.json` from the repo root to determine if mindspec is active.
9. An escape hatch (`MINDSPEC_ALLOW_MAIN=1`) allows bypassing the hook for emergencies.
10. The hook emits a clear error message including the worktree path to switch to.

### R3: CLI guard rails (Layer 2)

11. `mindspec complete`, `mindspec approve spec`, `mindspec approve plan` refuse to execute if CWD is the main worktree (not the expected worktree).
12. `mindspec spec-init` and `mindspec next` create worktrees and emit a `cd <path>` instruction.
13. `mindspec instruct` emitted from the wrong CWD (main when a worktree is active) warns prominently and tells the agent to `cd`.

### R4: Agent PreToolUse hooks (Layer 3 — deterministic)

14. A `PreToolUse` hook on `Edit` and `Write` checks whether the target file path is within the active worktree. Blocks the tool call if outside.
15. A `PreToolUse` hook on `Bash` checks whether the agent's CWD is the active worktree (not main). Blocks ALL bash execution from the wrong CWD — the agent cannot execute any command from main while a worktree is active. This is the primary deterministic enforcement for CWD.
16. Blocked tool calls emit an error message: `"mindspec: blocked — your working directory is the main worktree. Run: cd <worktree-path>"`.
17. The hooks are installed/updated by `mindspec setup claude` and `mindspec setup copilot`.

### R5: CWD redirect via instruct (soft — complements R4)

18. When `mindspec instruct` detects it is running from main but a worktree is active (non-idle state with `activeWorktree` set), it **replaces** the normal mode template (spec/plan/implement guidance) with a CWD redirect instruction only: `"You are in the main worktree. Run: cd <path> — then run mindspec instruct for guidance."` No other guidance is emitted.
19. This is the first thing the agent sees on session start (via SessionStart hook). It is prompt-based (not deterministic) but is a strong soft signal — the agent receives NO operating instructions until it moves to the worktree and runs `mindspec instruct` again.
20. The `SessionStart` hook runs `mindspec instruct` which performs this check on every session start and after every context recovery.
21. `mindspec` commands that read state (`instruct`, `state show`) work from main (they need to detect the mismatch). Commands that write (`complete`, `approve`, `next`) refuse to act from main (Layer 2 — deterministic).

### R6: Configuration

22. `.mindspec/config.yaml` supports:
    - `protected_branches`: list of branches to protect (default: `[main, master]`)
    - `merge_strategy`: `pr` | `direct` | `auto` (default: `auto`)
    - `enforcement.pre_commit_hook`: bool (default: `true`)
    - `enforcement.cli_guards`: bool (default: `true`)
    - `enforcement.agent_hooks`: bool (default: `true`)

### R8: Documentation

25. A new `.mindspec/docs/core/GIT-WORKFLOW.md` document that covers:
    - The zero-on-main invariant and why it exists
    - Branch topology: spec branch as integration point, impl beads branching from spec
    - Worktree lifecycle: when worktrees are created, what lives in each, when they're removed
    - Merge strategies (`pr` / `direct` / `auto`) and how to configure them
    - The three enforcement layers and what each catches
    - The escape hatch (`MINDSPEC_ALLOW_MAIN=1`) and when to use it
    - Diagram of the full lifecycle (spec-init through impl-approve)
    - Troubleshooting: stale worktrees, CWD mismatch, merge conflicts on spec branch
26. Existing docs updated:
    - `USAGE.md` — reference GIT-WORKFLOW.md for branching/worktree details
    - `claude-code.md` quick start guide — mention worktree workflow and CWD requirement
    - Instruction templates (`spec.md`, `plan.md`, `implement.md`) — reference worktree context, not direct-to-main

### R7: State schema extension

23. `state.json` gains an `activeWorktree` field (absolute path to the current worktree directory).
24. `activeWorktree` is set by `mindspec spec-init` and `mindspec next`, cleared by worktree removal.

## Scope

### In Scope

- `cmd/mindspec/spec_init.go` — extend to create branch + worktree
- `cmd/mindspec/next.go` — branch from spec branch, update `activeWorktree`
- `cmd/mindspec/complete.go` — merge bead → spec branch, remove bead worktree
- `cmd/mindspec/approve.go` — add CWD guards; `/impl-approve` creates PR or merges spec → main
- `cmd/mindspec/instruct.go` — CWD mismatch detection and blocking redirect
- `cmd/mindspec/instruct_tail.go` — same CWD check
- `internal/state/state.go` — `activeWorktree` field
- `internal/instruct/worktree.go` — rewrite `CheckWorktree()` to use `activeWorktree` binding
- `internal/instruct/templates/*.md` — update all templates for worktree-based workflow
- `internal/setup/claude.go` — install PreToolUse enforcement hook
- `internal/setup/copilot.go` — install PreToolUse enforcement hook
- `.mindspec/hooks/pre-commit` — new mindspec pre-commit hook script
- `.mindspec/config.yaml` — new configuration file with schema
- `internal/config/config.go` — new package to read `.mindspec/config.yaml`
- `.mindspec/docs/core/GIT-WORKFLOW.md` — new doc: branching model, worktree lifecycle, enforcement, troubleshooting
- `.mindspec/docs/core/USAGE.md` — update to reference GIT-WORKFLOW.md
- `.mindspec/docs/guides/claude-code.md` — update quick start for worktree workflow

### Out of Scope

- Changes to beads itself (beads provides plumbing, mindspec adds policy)
- Server-side branch protection (GitHub branch rules) — complements but is separate
- Parallel spec work (multiple spec branches active simultaneously) — future work
- Worktree creation for non-mindspec work (manual developer branches)

## Non-Goals

- Enforcing worktree isolation for non-mindspec git workflows (mindspec only enforces when its state is non-idle)
- Preventing all commits to main (only prevented when mindspec is active — `MINDSPEC_ALLOW_MAIN=1` and idle state allow normal git usage)
- Replacing beads' worktree plumbing (`bd worktree create/info`) — mindspec calls into beads for worktree operations

## Acceptance Criteria

- [ ] `mindspec spec-init 999-test` creates a `spec/999-test` branch and worktree; `state.json` has `activeWorktree` set
- [ ] Running `git commit` on main while mindspec is active fails with a clear error mentioning the worktree path
- [ ] `MINDSPEC_ALLOW_MAIN=1 git commit` on main succeeds (escape hatch works)
- [ ] `mindspec complete` from main CWD fails with an error; from the bead worktree CWD succeeds
- [ ] `mindspec instruct` from main CWD (when worktree is active) emits ONLY a CWD redirect instruction — no normal spec/plan/implement guidance
- [ ] `mindspec instruct` from the worktree CWD emits normal mode guidance
- [ ] `mindspec next` creates a bead branch from `spec/NNN-slug` (not from main)
- [ ] `mindspec complete` merges the bead branch back to `spec/NNN-slug` (not to main)
- [ ] `/impl-approve` with `merge_strategy: pr` pushes `spec/NNN-slug` and creates a PR to main
- [ ] `/impl-approve` with `merge_strategy: direct` merges `spec/NNN-slug` to main locally
- [ ] Claude Code `PreToolUse` hook on Edit/Write blocks file writes outside the active worktree with a clear error
- [ ] Claude Code `PreToolUse` hook on Bash blocks ALL bash execution when CWD is main and a worktree is active
- [ ] `.mindspec/config.yaml` with `protected_branches: [main, develop]` protects both branches
- [ ] `.mindspec/docs/core/GIT-WORKFLOW.md` exists and covers branching model, worktree lifecycle, enforcement layers, merge strategies, and troubleshooting
- [ ] Full lifecycle test: spec-init → spec work → approve spec → plan → approve plan → next → implement → complete → impl-approve — all in worktrees, zero commits on main

## Validation Proofs

- `git log main --oneline -5`: shows no new commits after a full spec lifecycle (all changes arrive via merge)
- `mindspec doctor`: reports healthy worktree state, no stale worktrees
- `git worktree list`: shows expected worktrees during lifecycle, clean after completion
- `cat .mindspec/state.json`: shows `activeWorktree` pointing to correct worktree during work, cleared after completion

## Open Questions

All resolved — decisions captured in ADR-0006 and ADR-0019.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-26
- **Notes**: Approved via mindspec approve spec