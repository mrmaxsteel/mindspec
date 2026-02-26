---
approved_at: "2026-02-26T17:52:06Z"
approved_by: user
molecule_id: mindspec-mol-8orbb
status: Approved
step_mapping:
    implement: mindspec-mol-spakr
    plan: mindspec-mol-bpf8j
    plan-approve: mindspec-mol-ye7z2
    review: mindspec-mol-if40v
    spec: mindspec-mol-gjtgt
    spec-approve: mindspec-mol-lx1bf
    spec-lifecycle: mindspec-mol-8orbb
---







# Spec 048-impl-approve-cleanup: Implementation Approval Cleanup

## Goal

Fix three related gaps in the spec lifecycle endgame: (1) `spec-init` writes spec files to main before creating the worktree, violating zero-on-main, (2) `impl-approve` silently performs merge/cleanup without informing the user what happened, offering no interactive PR flow, and never waiting for CI, and (3) the `PreToolUse` enforcement hooks (spec 046) block agents completely — the Bash hook uses `pwd` which always returns the main CWD, and the Edit/Write hooks receive an empty `$CLAUDE_TOOL_ARG_FILE_PATH`, making it impossible for an agent to comply.

## Background

### Bug: spec-init writes to main

`specinit.Run()` creates the spec directory and writes `spec.md` at lines 52-72 using `workspace.SpecDir(root, specID)` where `root` is the main worktree. The worktree isn't created until lines 119-153. This means the spec files land on main first, violating ADR-0006's zero-on-main invariant. The files should be created *only* in the worktree after it exists.

### UX gap: impl-approve cleanup

When `impl-approve` completes, the current behavior is:

- **Direct merge**: silently merges `spec/<id>` → main, deletes worktree and branch, prints one line: `"Implementation for <id> approved. Mode: idle."`
- **PR merge**: pushes the branch, creates a PR, prints the URL, then *returns without merging*. The worktree and branch are only cleaned up if merge succeeds — but for the PR path, merge never happens in the CLI, so the worktree persists.

Problems:
1. User is never told their work was on a worktree branch and what happened to it
2. PR path creates a PR but doesn't wait for CI or offer to merge
3. No interactive confirmation before creating PR
4. Worktree and branch leak when using PR strategy (cleanup is gated on merge success, but PR merge happens out-of-band)
5. No summary of what was merged (commit count, files changed)

### Bug: PreToolUse enforcement hooks block all agent activity

The spec 046 enforcement hooks in `.claude/settings.json` have two bugs that make them completely block agents instead of guiding them:

1. **Bash hook** (`worktreeBashGuardScript()` in `internal/setup/claude.go:267-275`): Uses `cwd=$(pwd)` to check if the agent is in the worktree. But Claude Code `PreToolUse` hooks run in the *process* CWD (always the main worktree), not the command's target CWD. So `pwd` always returns main, the check always fails, and *every* bash command is blocked — including `cd /path/to/worktree`. The agent is told to `cd` to the worktree but cannot execute the `cd`.

2. **Edit/Write hooks** (`worktreeFileGuardScript()` in `internal/setup/claude.go:255-263`): Use `$CLAUDE_TOOL_ARG_FILE_PATH` to check if the target file is inside the worktree. But this env var is empty at hook execution time, so the path comparison `case "$fp" in "$wt"*) exit 0;; esac` always falls through to the block. Every file edit is blocked regardless of whether it targets the worktree.

The net effect: once a worktree is active, an agent cannot run any bash command or edit any file. The enforcement hooks must be manually disabled to proceed, defeating their purpose.

## Impacted Domains

- **workflow**: `spec-init` file creation order; `impl-approve` merge/cleanup flow
- **git**: Branch push, PR creation, CI polling, merge+cleanup sequencing
- **agent-integration**: PreToolUse enforcement hooks need redesign to actually work

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Zero-on-main, one PR per spec lifecycle — `spec-init` currently violates this; `impl-approve` PR path doesn't complete the lifecycle
- [ADR-0019](../../adr/ADR-0019.md): Three-layer enforcement — `spec-init` bypasses Layer 1 by writing to main before the worktree exists

## Requirements

### R1: spec-init must not write to main

1. `specinit.Run()` must create the worktree *before* writing any spec files.
2. The spec directory (`docs/specs/<id>/`) and `spec.md` must be created inside the worktree, not in the main worktree.
3. If worktree creation fails, `spec-init` must fail — not fall back to writing on main.
4. The molecule binding write (`specmeta.Write`) and recording setup must also target the worktree path.

### R2: impl-approve user-facing summary

5. After merge (direct or PR), `impl-approve` must print a structured summary:
   - Merge strategy used (direct / PR)
   - Source branch and target branch
   - Commit count and diffstat (files changed, insertions, deletions)
   - Worktree removed: yes/no
   - Branch deleted: yes/no
6. Warnings (if any) are printed after the summary.

### R3: impl-approve interactive PR flow

7. When merge strategy resolves to `pr`:
   a. Print what will happen: "Will push `spec/<id>` and create a PR to `main`"
   b. Push the branch
   c. Create the PR
   d. Print the PR URL
   e. If `--wait` flag is passed (or interactive TTY detected): poll CI status via `gh pr checks <url> --watch` and report result
   f. If CI passes and PR is mergeable: prompt user "Merge and clean up? [Y/n]"
   g. On confirmation: merge PR via `gh pr merge <url> --merge --delete-branch`, remove worktree
   h. On decline: print "PR is open at <url>. Run `mindspec cleanup <spec-id>` to finish later."

8. A new `--no-wait` flag skips CI polling and auto-merge (current behavior, for non-interactive/CI contexts).

### R4: impl-approve direct merge improvements

9. When merge strategy resolves to `direct`:
   a. Print what will happen: "Will merge `spec/<id>` into `main` locally"
   b. Perform the merge
   c. Print the diffstat summary
   d. Clean up worktree and branch
   e. If a remote exists, prompt: "Push main to origin? [Y/n]"

### R5: cleanup command for deferred PR merges

10. A new `mindspec cleanup <spec-id>` command handles deferred cleanup when a PR was created but not merged during `impl-approve`:
    - Checks PR status via `gh pr view`
    - If merged: remove worktree, delete local branch, transition state to idle
    - If open: offer to wait for CI and merge
    - If closed without merge: warn and offer to re-open or clean up anyway

### R7: Fix PreToolUse enforcement hooks

12. **Bash hook**: Replace `pwd`-based check with inspection of `$CLAUDE_TOOL_ARG_COMMAND`. The hook should:
    a. Allow commands that begin with `cd <worktree-path>` (the agent is trying to comply)
    b. Allow `mindspec` and `bd` CLI commands (they have their own Layer 2 CWD guards)
    c. Block other commands when CWD is main and a worktree is active
    d. The hook cannot change the process CWD, so it must be permissive for commands that target the worktree

13. **Edit/Write hooks**: Replace `$CLAUDE_TOOL_ARG_FILE_PATH` with stdin JSON parsing. Claude Code passes tool arguments as JSON on stdin (field `tool_input.file_path`). The hook must read stdin and use `jq` to extract the path: `fp=$(cat | jq -r '.tool_input.file_path')`.

14. **`mindspec setup claude`** must regenerate the hooks with the fixed scripts. Running `mindspec setup claude` should detect stale hooks (by comparing command strings) and update them.

15. **Copilot hooks**: The worktree guard script (`.github/hooks/mindspec-worktree-guard.sh`) has the same `pwd` bug for bash/shell tool checks (line 210-217 of `copilot.go`). Fix it to read stdin JSON for the command and use the same allowlist logic as the Claude Code hook.

16. All hooks (Claude Code and Copilot) must preserve the escape hatch: `config.yaml` with `enforcement.agent_hooks: false` disables them.

### R6: gitops package additions

16. New functions in `internal/gitops`:
    - `MergePR(url string) error` — calls `gh pr merge <url> --merge --delete-branch`
    - `PRChecksWatch(url string) error` — calls `gh pr checks <url> --watch`, returns nil on pass
    - `PRStatus(url string) (string, error)` — returns "open", "merged", or "closed"
    - `DiffStat(base, head string) (string, error)` — returns `git diff --stat` output
    - `CommitCount(base, head string) (int, error)` — returns number of commits between base and head

## Scope

### In Scope

- `internal/specinit/specinit.go` — reorder to create worktree before writing spec files
- `internal/approve/impl.go` — structured summary, interactive PR flow, `--wait`/`--no-wait`
- `cmd/mindspec/approve.go` — wire `--wait`/`--no-wait` flags
- `internal/gitops/gitops.go` — new PR merge, CI check, diffstat functions
- `cmd/mindspec/cleanup.go` — new cleanup command (deferred PR merge)
- `internal/setup/claude.go` — fix `worktreeBashGuardScript()` and `worktreeFileGuardScript()`, add stale hook detection
- `internal/setup/copilot.go` — fix `mindspec-worktree-guard.sh` template (same `pwd` bug)
- Tests for all changed functions

### Out of Scope

- Server-side branch protection rules (GitHub settings) — complementary but separate
- Changes to beads worktree plumbing
- Reworking the `mindspec complete` flow (bead-level, not spec-level)
- Parallel spec lifecycle support

## Non-Goals

- Full CI/CD integration beyond `gh pr checks` — we rely on GitHub CLI, not custom API calls
- Auto-merge without user confirmation — always prompt
- Supporting non-GitHub remotes for PR flow (GitLab, Bitbucket) — future work

## Acceptance Criteria

- [ ] `mindspec spec-init 999-test` creates spec.md only in the worktree, not on main; `git status` on main shows no untracked spec files
- [ ] `mindspec spec-init 999-test` fails if worktree creation fails (does not fall back to writing on main)
- [ ] `impl-approve` with `merge_strategy: direct` prints diffstat summary (commit count, files changed) before transitioning to idle
- [ ] `impl-approve` with `merge_strategy: pr` pushes branch, creates PR, and prints URL
- [ ] `impl-approve --wait` with `merge_strategy: pr` polls CI, prompts for merge on pass, cleans up worktree/branch on merge
- [ ] `impl-approve --no-wait` with `merge_strategy: pr` creates PR and returns without polling (current behavior, plus summary)
- [ ] `mindspec cleanup <spec-id>` detects merged PR and cleans up worktree + branch + state
- [ ] `mindspec cleanup <spec-id>` on an open PR offers to wait and merge
- [ ] Fixed Bash `PreToolUse` hook allows `cd <worktree>` and `mindspec`/`bd` commands from main CWD
- [ ] Fixed Edit/Write `PreToolUse` hooks correctly read the target file path and allow writes inside the worktree
- [ ] `mindspec setup claude` detects stale hooks and replaces them with fixed versions
- [ ] An agent can run `mindspec approve spec <id>` from main CWD without being blocked by enforcement hooks
- [ ] All new gitops functions have unit tests

## Validation Proofs

- `git status` on main after `spec-init`: no untracked files in `.mindspec/docs/specs/`
- `impl-approve` output includes merge summary with commit count and diffstat
- `gh pr list` shows PR created by impl-approve
- `git worktree list` after full cleanup shows no spec worktrees
- `mindspec state show` shows idle mode after cleanup

## Open Questions

- [x] **Resolved**: Claude Code does NOT expose tool arguments as env vars. `$CLAUDE_TOOL_ARG_FILE_PATH` and `$CLAUDE_TOOL_ARG_COMMAND` do not exist — that's why they're always empty. Tool arguments are passed as **JSON on stdin** in a `tool_input` field. Hooks must read stdin and parse with `jq`. For Bash: `.tool_input.command`, for Edit/Write: `.tool_input.file_path`. Hooks can also return `permissionDecision` (allow/deny/ask) and `updatedInput` to modify tool args.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-26
- **Notes**: Approved via mindspec approve spec