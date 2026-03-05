---
approved_at: "2026-03-05T07:33:47Z"
approved_by: user
bead_ids: []
last_updated: 2026-03-05T00:00:00Z
spec_id: 072-hook-cleanup
status: Approved
version: "1"
---
# Plan: 072-hook-cleanup

## ADR Fitness

- ADR-0019: Branch enforcement layers — Layer 2 (agent hooks) collapses into guidance only. Layer 1 (pre-commit) remains but moves to Go via thin shim.

## Testing Strategy

- Unit tests for new `PreCommit()` and `SessionStart()` functions
- Updated unit tests after removing dead hook code
- `make test` at each bead
- LLM harness SingleBead at bead 3

## Bead 1: Remove guard hooks + dead post-checkout

Remove all 6 PreToolUse Claude Code hooks and the dead post-checkout git hook.

**Steps**
1. `internal/hook/dispatch.go` — delete `PlanGateExit`, `PlanGateEnter`, `WorktreeFile`, `WorktreeBash`, `SessionFreshnessGate`, `WorkflowGuard`, and constants `blockIdle`, `warnOutsideWorktreeCode`, `warnReview`. Strip `Run()` cases.
2. `internal/hook/hook.go` — remove 6 names from `Names` slice. Remove `EnforcementEnabled()`.
3. `internal/hook/helpers.go` — delete `outsideActiveWorktree`, `isCodeFile`, `isAllowedCommand`, `allowedPrefixes`, `containsWord`, `getGitBranch`, `protectedGitWrite`, `parseGitCommand`. Keep `dirExists`, `hasPathPrefix`, `stripEnvPrefixes`, `parseEnvPrefixes`, `isEnvVarName`, `getCwd`.
4. `internal/hook/dispatch_test.go` — delete all tests for removed functions. Keep `TestRun_UnknownHook`.
5. `internal/hook/hook_test.go` — keep ParseInput/Emit/Names tests only.
6. `internal/lifecycle/hook_matrix_test.go` — delete entire file.
7. `internal/hooks/install.go` — delete `postCheckoutScript`, `InstallPostCheckout()`. Update `InstallAll()` to call only `InstallPreCommit()`.
8. `internal/hooks/install_test.go` — delete `TestInstallPostCheckout_*`, `TestPostCheckout_*` tests. Update `TestInstallAll`.
9. `internal/setup/claude.go` — remove entire `"PreToolUse"` section from `wantedHooks()`.
10. `internal/setup/claude_test.go` — update/remove tests: no PreToolUse assertions, delete stale-hook tests that reference removed hooks.

**Verification**
- [ ] `make test` passes
- [ ] `mindspec hook --list` shows no guard hooks

**Depends on**
None

## Bead 2: Thin-shim pre-commit + session-start

Replace complex bash pre-commit with Go-backed shim. Replace inline SessionStart with shim.

**Steps**
1. `internal/hooks/install.go` — replace `preCommitScript` with thin shim: `#!/usr/bin/env bash\n# MindSpec pre-commit hook v5 (thin shim)\nexec mindspec hook pre-commit "$@"`
2. `internal/hook/dispatch.go` — add `PreCommit()` function: check `MINDSPEC_ALLOW_MAIN` env, read `.mindspec/focus` for mode, read `config.yaml` for opt-out, get git branch, check protected list, return Block/Pass with helpful messages.
3. `internal/hook/hook.go` — add `"pre-commit"` and `"session-start"` to `Names`.
4. `internal/hook/dispatch.go` — add `"pre-commit"` case to `Run()`.
5. `cmd/mindspec/hook.go` — add `"session-start"` special case: read source from stdin, call `state.WriteSession()`, then exec instruct emission, print output to stdout.
6. `internal/setup/claude.go` — update `wantedHooks()` SessionStart command to `mindspec hook session-start`.
7. `internal/hooks/install_test.go` — update `TestInstallPreCommit_NewHook` to check for v5 marker. Update integration tests for thin shim.
8. `internal/hook/dispatch_test.go` — add tests for `PreCommit()` (nil state, allow-main bypass, protected branch block, non-protected pass, config opt-out).
9. `internal/setup/claude_test.go` — update `TestWantedHooks_SessionStartIncludesClearFlag` for new command format.

**Verification**
- [ ] `make test` passes
- [ ] `echo '{}' | mindspec hook pre-commit` exits 0 on non-protected branch
- [ ] `mindspec hook --list` shows `pre-commit` and `session-start`

**Depends on**
Bead 1

## Bead 3: Stale-entry cleanup + validation

Make `mindspec setup claude` clean up old PreToolUse entries from existing installs.

**Steps**
1. `internal/setup/claude.go` — in `ensureSettings()`, after merging wanted hooks, scan PreToolUse entries for mindspec hook commands and remove them. Delete PreToolUse key if empty.
2. `internal/setup/claude_test.go` — add test: settings.json with old PreToolUse mindspec hooks → after `RunClaude()`, PreToolUse is removed.
3. Run `make test`.
4. Run LLM harness SingleBead.

**Verification**
- [ ] `make test` passes
- [ ] LLM harness SingleBead passes
- [ ] `mindspec setup claude` on a repo with old hooks removes PreToolUse entries

**Depends on**
Bead 2

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| Zero PreToolUse entries after setup | Bead 3: stale cleanup test |
| SessionStart calls `mindspec hook session-start` | Bead 2: wantedHooks test |
| post-checkout no longer installed | Bead 1: InstallAll test |
| pre-commit is thin shim | Bead 2: install test |
| `mindspec next` enforces session freshness | Already exists (next.go Step 0b) |
| `make test` passes | All beads |
| LLM harness SingleBead passes | Bead 3 |
