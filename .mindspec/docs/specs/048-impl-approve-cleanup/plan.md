---
status: Draft
spec_id: 048-impl-approve-cleanup
version: 1
last_updated: "2026-02-26"
approved_at: ""
approved_by: ""
bead_ids: []
adr_citations:
  - ADR-0006
  - ADR-0019
---

# Plan: 048-impl-approve-cleanup

## ADR Fitness

- **ADR-0006** (Protected Main Branch with PR-Based Merging): Sound. The bugs we're fixing are implementation gaps — `spec-init` violates zero-on-main by writing to main before the worktree exists, and `impl-approve` doesn't complete the PR lifecycle. The ADR's design is correct; the code doesn't match it yet.
- **ADR-0019** (Deterministic Worktree and Branch Enforcement): Sound architecture, broken implementation. The three-layer enforcement model is right, but Layer 3 (PreToolUse hooks) uses nonexistent env vars (`$CLAUDE_TOOL_ARG_FILE_PATH`, `$CLAUDE_TOOL_ARG_COMMAND`) instead of reading stdin JSON. The fix aligns the implementation with the ADR's intent. No divergence needed.

## Testing Strategy

- **Unit tests**: Each changed Go function gets table-driven tests with function-variable injection (existing pattern: `xxxFn` vars for testability)
- **Integration**: `make test` runs all tests; no new test infrastructure needed
- **Manual verification**: Hook scripts tested by inspecting JSON output from `mindspec setup claude --check`

## Bead 1: Fix PreToolUse enforcement hooks (Claude Code + Copilot)

**Steps**
1. In `internal/setup/claude.go`, rewrite `worktreeBashGuardScript()` to read `$CLAUDE_TOOL_ARG_COMMAND` from stdin JSON (`tool_input.command`) instead of using `pwd`. Allow commands starting with `cd <worktree>`, `mindspec`, and `bd` (they have Layer 2 guards). Block other commands when a worktree is active.
2. In `internal/setup/claude.go`, rewrite `worktreeFileGuardScript()` to read `tool_input.file_path` from stdin JSON instead of `$CLAUDE_TOOL_ARG_FILE_PATH`.
3. In `internal/setup/copilot.go`, update `mindspec-worktree-guard.sh` template: the bash/shell case uses `pwd` — replace with stdin JSON parsing for the command, same allowlist logic as Claude Code.
4. Add `hookEntryNeedsUpdate()` to `internal/setup/claude.go` that compares existing hook commands against wanted commands, so `mindspec setup claude` can detect and replace stale hooks (currently `hookEntryExists` only checks matcher, not command content).
5. Update `ensureSettings()` to replace stale hooks when detected.
6. Write tests for the new hook scripts (verify they produce correct shell scripts, verify stale detection).

**Verification**
- [ ] `go test ./internal/setup/...` passes
- [ ] `mindspec setup claude --check` shows hooks would be updated (stale detection)
- [ ] Manual: create a test hook script, pipe it mock stdin JSON, verify allow/deny behavior

**Depends on**
None

## Bead 2: Fix spec-init zero-on-main violation

**Steps**
1. In `internal/specinit/specinit.go`, reorder `Run()`: move worktree creation (lines 119-153) to before spec file creation (lines 52-72).
2. After worktree is created, use the worktree path as the target for `os.MkdirAll` and `os.WriteFile` instead of `workspace.SpecDir(root, specID)`.
3. Make worktree creation failure a hard error (not a warning) — if the worktree can't be created, `spec-init` fails.
4. Update molecule binding write (`specmeta.Write`) to target the worktree spec dir.
5. Update recording setup to target the worktree.
6. Update tests in `internal/specinit/specinit_test.go`.

**Verification**
- [ ] `go test ./internal/specinit/...` passes
- [ ] Manual: `mindspec spec-init 999-test` creates files only in worktree; `git status` on main shows no untracked spec files

**Depends on**
None

## Bead 3: Improve impl-approve summary and direct merge flow

**Steps**
1. Add `DiffStat(workdir, base, head string) (string, error)` and `CommitCount(workdir, base, head string) (int, error)` to `internal/gitops/gitops.go`.
2. In `internal/approve/impl.go`, after successful direct merge: call `DiffStat` and `CommitCount`, print structured summary (strategy, branches, stats, cleanup status).
3. After direct merge + cleanup, if `HasRemote()`: print suggestion to push main (informational, not interactive — CLI shouldn't block on stdin for agent compatibility).
4. Write tests for new gitops functions and updated impl approval flow.

**Verification**
- [ ] `go test ./internal/gitops/...` passes
- [ ] `go test ./internal/approve/...` passes
- [ ] `make test` passes

**Depends on**
None

## Bead 4: Improve impl-approve PR flow with --wait flag

**Steps**
1. Add `MergePR(url string) error`, `PRChecksWatch(url string) error`, and `PRStatus(url string) (string, error)` to `internal/gitops/gitops.go`.
2. In `cmd/mindspec/approve.go`, add `--wait` and `--no-wait` flags to the impl subcommand. Default: `--no-wait` (non-blocking, agent-safe).
3. In `internal/approve/impl.go`, extend `mergeSpecToMain()` for the PR path:
   - Always: push branch, create PR, print URL
   - With `--wait`: call `PRChecksWatch`, then `MergePR` on success, then cleanup worktree/branch
   - Without `--wait`: print PR URL and instruction for deferred cleanup
4. On successful PR merge (--wait path): clean up worktree and branch, print summary.
5. Write tests with mocked gitops functions.

**Verification**
- [ ] `go test ./internal/approve/...` passes
- [ ] `go test ./internal/gitops/...` passes
- [ ] `make test` passes

**Depends on**
Bead 3 (uses DiffStat/CommitCount for summary)

## Bead 5: Add mindspec cleanup command

**Steps**
1. Create `internal/cleanup/cleanup.go` with `Run(root, specID string) error`:
   - Read state to find spec branch
   - Call `PRStatus` to check if PR is merged/open/closed
   - If merged: remove worktree, delete local branch, set state to idle
   - If open: print PR URL and status, suggest `--wait` to poll
   - If closed without merge: warn, suggest re-opening or `--force` cleanup
2. Create `cmd/mindspec/cleanup.go` wiring the cobra command.
3. Register in `cmd/mindspec/root.go`.
4. Write tests.

**Verification**
- [ ] `go test ./internal/cleanup/...` passes
- [ ] `make build` succeeds
- [ ] `./bin/mindspec cleanup --help` shows usage

**Depends on**
Bead 4 (uses PRStatus from gitops)

## Provenance

| Acceptance Criterion | Satisfied By |
|---|---|
| spec-init creates spec.md only in worktree | Bead 2 verification |
| spec-init fails if worktree creation fails | Bead 2 step 3 |
| impl-approve direct prints diffstat summary | Bead 3 verification |
| impl-approve pr pushes branch, creates PR | Bead 4 verification |
| impl-approve --wait polls CI, merges | Bead 4 verification |
| impl-approve --no-wait returns without polling | Bead 4 step 3 |
| cleanup detects merged PR, cleans up | Bead 5 verification |
| cleanup on open PR offers to wait | Bead 5 step 1 |
| Fixed Bash hook allows cd/mindspec/bd from main | Bead 1 verification |
| Fixed Edit/Write hooks read file path correctly | Bead 1 verification |
| setup claude detects stale hooks | Bead 1 steps 4-5 |
| Agent can run approve from main CWD | Bead 1 verification |
| All new gitops functions have tests | Beads 3, 4 verification |
