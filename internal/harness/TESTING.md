# LLM Test Harness — Operational Guide

## Overview

The harness in `internal/harness/` runs behavioral tests against real Claude Code sessions. Each test spawns a `claude -p` process in an isolated sandbox git repo, gives it a scenario prompt, and asserts the agent executed the expected mindspec workflow commands.

This is the most effective way to validate that the mindspec workflow actually works end-to-end with a real LLM agent. The iterative **test -> observe -> fix -> retest** loop is how we improve both the CLI and the agent experience.

## How to Run Tests

### Prerequisites
```bash
make build                    # Rebuild mindspec binary (CRITICAL -- tests use ./bin/mindspec)
bd dolt killall 2>/dev/null   # Kill orphan dolt servers from previous runs
```

### Running Individual LLM Tests
```bash
# ALWAYS use env -u CLAUDECODE to allow nested Claude Code sessions
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_<Name> -timeout 10m -count=1

# Examples:
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m -count=1
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SpecToIdle -timeout 15m -count=1
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_MultiBeadDeps -timeout 10m -count=1
```

### Running Deterministic Tests (no LLM)
```bash
go test ./internal/harness/ -short -v
```

### Running Multiple Iterations for Reliability
```bash
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 30m -count=3
```

### Critical Gotchas
1. **`env -u CLAUDECODE`** -- MUST unset this env var or nested claude sessions won't launch
2. **`make build`** -- MUST rebuild after changing any `cmd/mindspec/` or `internal/` code. The shims delegate to `./bin/mindspec`
3. **Dolt orphans** -- Previous test runs leak dolt sql-server processes. Run `bd dolt killall` before testing, or the sandbox `bd init` will fail with "too many dolt sql-server processes"
4. **Timeout** -- SpecToIdle needs 15min timeout; simpler tests need 10min
5. **Don't run multiple LLM tests in parallel** -- they share dolt server slots and can interfere

## Test Design Principles

Every LLM test must follow these rules. Violating them produces tautological tests that validate prompt adherence instead of product quality.

### 1. Deterministic Setup

`Setup()` must create **all** required infrastructure before the agent runs. The agent should never need to bootstrap its own environment. This includes:

- **Config**: `.mindspec/config.yaml` with appropriate `agent_hooks` setting
- **Hooks**: `setupClaudeForSandbox()` installs CLAUDE.md, `.claude/settings.json`, slash commands
- **State files**: `.mindspec/focus`, `.mindspec/session.json`, lifecycle.yaml
- **Beads**: Real beads via `sandbox.CreateBead()` / `sandbox.ClaimBead()`
- **Specs/plans**: Pre-written and pre-approved if the scenario starts mid-lifecycle
- **Worktrees**: Created if the scenario starts in a worktree context
- **Git state**: Clean working tree, committed setup files

The agent's first action should be executing the task, not setting up infrastructure.

### 2. Minimal Prompts

Prompts describe the **task**, not the **workflow**. The agent must discover how to use mindspec from its own guidance layer (CLAUDE.md, instruct templates, CLI error messages, hooks).

**Good prompt**: "Add a greeting feature that prints Hello."
**Bad prompt**: "Run `mindspec explore`, then `mindspec explore promote`, then write spec.md, then run `mindspec approve spec`..."

The only workflow information allowed in a prompt is the end-state: "Run `mindspec complete` when done" is acceptable because it names the finish line, not the path to get there.

Imperative Haiku framing ("Do NOT respond conversationally", "Execute NOW") is acceptable — it's agent infrastructure to prevent conversational mode, not workflow guidance.

### 3. Workflow Tests Only

All LLM scenarios test **workflow** behavior — the agent discovering and executing the mindspec lifecycle from guidance alone (CLAUDE.md, instruct templates, CLI error messages).

- Prompt = task description only
- Agent discovers workflow from mindspec guidance
- Assertions verify the agent reached the correct end state
- When a workflow test fails, fix mindspec's guidance — never the test prompt (Fix Surface Rule)

**Enforcement logic** (hook dispatch, worktree guards, session freshness gates) is tested deterministically in `internal/hook/dispatch_test.go` (37+ unit tests). These are pure functions that don't need an LLM agent — they're faster, cheaper, and more reliable as unit tests.

### 4. Fix Surface Rule

When an LLM test fails due to agent behavior, the fix MUST go into mindspec's own guidance (instruct templates, CLAUDE.md, CLI error messages) — NEVER into the test prompt. Putting workflow hints in the prompt makes the test a tautology instead of testing the product.

**Example**: If the agent doesn't know to commit before running `mindspec complete`, the fix goes into the implement.md instruct template ("commit your changes before completing"), not into the test prompt.

## Available Test Scenarios

| Test | MaxTurns | Complexity | What It Tests |
|------|----------|------------|---------------|
| `TestLLM_SingleBead` | 15 | Low | Pre-approved spec/plan, implement one bead, run complete |
| `TestLLM_MultiBeadDeps` | 30 | Medium | Three beads with dependency chain |
| `TestLLM_AbandonSpec` | 10 | Low | Enter explore, then dismiss |
| `TestLLM_InterruptForBug` | 25 | Medium | Mid-bead bug fix then resume |
| `TestLLM_ResumeAfterCrash` | 15 | Low | Pick up partial work |
| `TestLLM_SpecToIdle` | 75 | High | Full 9-step lifecycle (explore -> idle), verifies git state cleanup (branches, worktrees, CWD) |
| `TestLLM_SpecInit` | 15 | Low | Idle → spec-init → spec mode, worktree + branch created |
| `TestLLM_SpecApprove` | 15 | Low | Spec mode → approve spec → plan mode |
| `TestLLM_PlanApprove` | 20 | Medium | Plan mode → approve plan → mindspec next → implement mode |
| `TestLLM_ImplApprove` | 15 | Low | Review mode → approve impl → idle, merge + cleanup |
| `TestLLM_SpecStatus` | 10 | Low | Check current mode via state show / instruct (read-only) |
| `TestLLM_MultipleActiveSpecs` | 20 | Medium | Two active specs — agent must discover `--spec` flag from CLI errors |
| `TestLLM_StaleWorktree` | 20 | Medium | State references nonexistent worktree — agent must recover and complete |
| `TestLLM_BugfixBranch` | 25 | Medium | Fix a pre-existing bug on a branch via PR, never commit to main |

**Start with SingleBead** when validating changes -- it's the fastest and most reliable.

## Reading Test Output

The test logs three sections (in order, before assertions):

### 1. Recorded Events
```
[1] mindspec explore add greeting feature (exit=0)
[2] mindspec approve spec 001-greeting (exit=1)     <-- failed attempt
[3] mindspec approve spec 001-greeting (exit=0)     <-- retry succeeded
```
- These are shim-recorded commands (git, mindspec, bd)
- `exit=0` = success, `exit=1` = error, `exit=128` = git error
- `[BLOCKED: reason]` = hook blocked the command

### 2. Agent Output
```
--- Agent output (exit=0, dur=2m42s) ---
Error: Reached max turns (75)        <-- agent ran out of turns
```
- `exit=0` even with "Reached max turns" -- that's normal claude -p behavior
- The text is the final assistant message
- If it shows a conversational response ("What would you like to work on?"), the prompt failed

### 3. Report & Assertions
```
=== Session Report: spec_to_idle ===
Agent: claude-code
Turns: 52 (estimated)  Events: 170
Duration: 16.277s
Forward ratio: 85.3%
```
- **Turns**: estimated from event timestamp gaps (>2s gap = new turn). Shims don't know API turns.
- **Events**: total shim-recorded commands (multiple per turn since Claude calls tools in parallel)
- **Forward ratio**: % of turns doing productive work (vs corrections, recovery, overhead)

```
command "mindspec" with arg "complete" was not found in events   <-- FAIL
```

## The Iterative Improvement Loop

### Workflow
1. **Run the test**, observe failure
2. **Read the event log** -- identify WHERE the agent got stuck
3. **Categorize the failure** (see taxonomy below)
4. **Make a targeted fix** -- smallest change possible
5. **Rebuild**: `make build && go build ./internal/harness/`
6. **Retest** -- same scenario, check if the fix worked
7. **Check for regressions** -- rerun SingleBead to verify the baseline still passes
8. **Commit and push** each fix independently (for bisectability)

### Failure Taxonomy

| Category | Symptoms | Fix Location |
|----------|----------|-------------|
| **Conversational response** | Agent says "What would you like?" instead of executing | Prompt wording, hooks, CLAUDE.md |
| **Command exits non-zero** | Agent retries same command repeatedly | CLI code (`cmd/mindspec/`, `internal/`) |
| **CWD mismatch** | `mindspec complete` fails from wrong directory | Guard logic, auto-chdir, prompt |
| **Beads not initialized** | `bd init` fails, no bead IDs available | Dolt orphans, sandbox setup |
| **Hook blocks tool call** | PreToolUse hook rejects Write/Edit/Bash | settings.json hook config |
| **Max turns exhausted** | Agent runs out of turns before finishing | MaxTurns budget, prompt efficiency |
| **Worktree issues** | git worktree creation fails or wrong path | `internal/next/`, sandbox git setup |

### Common Fixes by Category

**Agent doesn't follow prompt:**
- Make prompt imperative: "Execute step 1 NOW", "Do NOT respond conversationally"
- Remove conflicting instructions from hooks (e.g., `mindspec instruct` idle template says "greet the user")

**CLI command fails in sandbox:**
- Check if the command needs `.mindspec/`, `.beads/`, or specific state files
- Check if CWD matters (guard.CheckCWD)
- Add auto-chdir or relax guards for agent use

**Infrastructure:**
- Always run `bd dolt killall` before/during sandbox setup
- Use `--server-port 0` for dolt (random port avoids collisions)
- Add `.beads/` and `.harness/` to `.gitignore` in sandbox

## Improvement History & Metrics

Track each test run with: scenario, date, pass/fail, recorded events count, turns used, wall-clock time, and what changed.

**Before adding a row**: re-read the LAST existing row in that scenario's table to know the actual baseline. Only claim a metric changed if it actually moved. Do not infer "before" values from the current session — check the table.

### TestLLM_SingleBead

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | FAIL | 1 | 15 | ~30s | Baseline: no CLAUDE.md, no beads, --no-input flag |
| 2026-02-28 | FAIL | 1 | 15 | ~30s | Added setup.RunClaude + PreToolUse hooks: hooks blocked all tools |
| 2026-02-28 | FAIL | ~5 | 15 | ~60s | SessionStart only (no PreToolUse): agent ran but PATH wrong |
| 2026-02-28 | FAIL | ~10 | 15 | ~90s | Fixed PATH dedup: mindspec ran but no .beads/ |
| 2026-02-28 | FAIL | ~15 | 15 | ~120s | Added bd init: dolt runtime files made tree dirty |
| 2026-02-28 | FAIL | ~15 | 15 | ~120s | Added .gitignore: fake bead IDs don't exist |
| 2026-02-28 | PASS | ~20 | 15 | ~90s | Real beads (CreateBead/ClaimBead): **first pass** |
| 2026-02-28 | 3/3 PASS | ~20 | 15 | ~90s | Reliability confirmed with -count=3 |
| 2026-02-28 | PASS | 45 | 2 | 19.6s | Re-baseline: 2 turns, 100% forward ratio, 1 retry on complete (no commit yet) |
| 2026-02-28 | PASS | 34 | 2 | 15.5s | Added "commit before completing" to prompt — eliminated retry, -24% events |
| 2026-02-28 | 5/5 PASS | 34 | 12-16s | Reliability: 34 events, 2 turns, 100% fwd ratio, 0 retries across all 5 runs |
| 2026-02-28 | PASS | 34 | 2 | 14.2s | Infra filter: no change (already 100% fwd), regression check only |
| 2026-02-28 | PASS | 45 | 3 | 23s | Removed prompt workaround "MUST commit before completing" — fix moved to instruct template. Agent now retries once (complete→error→commit→complete). 1 retry is expected: sandbox has no hooks so instruct template doesn't run, agent learns from CLI error. |
| 2026-02-28 | PASS | 74 | 3 | 23s | Full hooks enabled: SessionStart runs `mindspec instruct`, PreToolUse hooks installed (no-op via agent_hooks:false). Agent gets implement.md guidance. 100% fwd ratio. More events due to hook invocations. |
| 2026-02-28 | PASS | 80 | 2 | 25s | Full suite run: stable, 100% fwd ratio |
| 2026-03-01 | FAIL | - | - | 3.25s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` now hits main-branch guard in implement mode. |
| 2026-03-01 | PASS | 141 | 5 | 45.10s | Fix: harness setup commits now use `MINDSPEC_ALLOW_MAIN=1` escape hatch. Setup regression resolved for this scenario. |
| 2026-03-01 | FAIL | 75 | 8 | 43.89s | Full-suite rerun: agent stayed in diagnostics/retry flow, never created `greeting.go` and never ran successful `mindspec complete`. |
| 2026-03-02 | PASS | 107 | 4 | 29.16s | Full-suite rerun after guard tightening: scenario passes again with one retry in commit/complete flow. |
| 2026-03-02 | PASS | 94 | 3 | 26.54s | Regression check after `approve impl` focus-write fix: still green, one expected commit/complete retry remains. |
| 2026-03-02 | PASS | 102 | 3 | 27.61s | Regression check after `mindspec-ce5b` worktree-anchor fix: remains green with one expected retry before final `mindspec complete`. |
| 2026-03-02 | PASS | 91 | 3 | 24.69s | Regression check for `mindspec-n9j7`: implement guidance + pre-commit messaging changes still keep SingleBead green. |
| 2026-03-02 | FAIL | 142 | 5 | 42.58s | Full-suite rerun: agent created and staged `greeting.go` but never reached successful `mindspec complete` before max turns. |
| 2026-03-02 | PASS | 129 | 7 | 67.16s | Hardened setup to start with active bead worktree + imperative prompt; targeted rerun passes. |
| 2026-03-02 | PASS | 141 | 4 | 54.12s | Final full-suite verification after setup hardening remains green. |
| 2026-03-02 | FAIL | 32 | 2 | 15.27s | De-tautologized prompt (no explicit complete command) was too open: agent used `bd close` directly instead of lifecycle completion. |
| 2026-03-02 | PASS | 173 | 3 | 44.90s | Prompt revised to require lifecycle end-state (review mode) without naming commands; agent discovered completion path and passed. |
| 2026-03-03 | PASS | 167 | 5 | 52.10s | Spec 058 fixes (DetectWorktreeContext last-match, focus propagation, plan scaffold). 100% fwd ratio. |
| 2026-03-03 | PASS | 120 | 11 | 56.92s | After sandbox .gitignore fix (added .mindspec/focus + session.json). 100% fwd ratio, clean bead→spec merge at [92]. |

### TestLLM_SpecToIdle

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | FAIL (assertions pass) | 125 | 50 | 2m16s | Baseline with hooks: agent completed lifecycle but `complete` failed (CWD guard) |
| 2026-02-28 | FAIL | 17 | 50 | 7s | Removed SessionStart hook: agent greeted instead of executing (instruct idle template) |
| 2026-02-28 | FAIL | 11 | 50 | 6s | Empty hooks{}: still conversational (CLAUDE.md influence) |
| 2026-02-28 | FAIL | 107 | 50 | 2m3s | Imperative prompt: agent executed but dolt orphans blocked bd init |
| 2026-02-28 | FAIL (1 assertion) | 108 | 50 | 1m52s | bd dolt killall in initBeads: agent reached `next` but ran out of turns before `complete` |
| 2026-02-28 | **PASS** | 170 | 75 | 2m42s | MaxTurns 50->75: **agent completed full lifecycle** |
| 2026-02-28 | FAIL | 327 | 30 | 3m16s | Full suite run: agent skipped `explore` (went to spec-init), then stuck retrying `complete` in worktree (17 retries, 43% fwd ratio) |
| 2026-03-01 | **PASS** | 358 | 28 | 4m10s | Fix: auto-commit `.mindspec/` state files in `complete.Run()`, remove dead `--spec` flag, accept explore+promote as valid path. 71.4% fwd ratio (20 fwd / 8 retry). Remaining retries: `approve plan` (needs bead creation) and `approve impl` (merge conflicts). |
| 2026-03-01 | FAIL (new assertions) | 377 | 22 | 3m8s | Added git state assertions (branch cleanup, worktree removal, CWD contains .worktrees/). Existing assertions pass (explore+promote, next, complete all ran). New assertions caught: spec/ and bead/ branches not deleted, worktree not removed. Agent stuck retrying `complete` from spec worktree CWD (not bead worktree). 59.1% fwd ratio (13 fwd / 9 retry). Root cause: guidance gap — agent doesn't know to cd into bead worktree. |
| 2026-03-01 | FAIL | 452 | 53 | 4m20s | **REGRESSION**: lifecycle advanced but cleanup assertions failed again (spec/* and bead/* branches + worktree left behind). |
| 2026-03-01 | FAIL | 485 | 25 | 199.62s | Full-suite rerun: still no successful `spec-init`/`explore promote` milestone and cleanup assertions fail (`spec/*`, `bead/*`, and spec worktree remain). |
| 2026-03-02 | FAIL | 430 | 22 | 2m48.19s | Full-suite rerun: lifecycle progressed, but cleanup assertions still fail (`spec/*`, `bead/*`, and spec worktree remain). |
| 2026-03-02 | FAIL | 545 | 35 | 3m54.61s | Baseline for `mindspec-ce5b`: recursive bead worktree nesting from CWD-sensitive `mindspec next`, plus `approve impl` retries, left `spec/*`, `bead/*`, and worktrees behind. |
| 2026-03-02 | PASS | 416 | 20 | 2m25.28s | Fix: `next.EnsureWorktree` now anchors worktree creation to spec worktree/main root (not caller CWD). Recursive nesting stopped and cleanup assertions passed. |
| 2026-03-02 | FAIL | 506 | 26 | 3m21.03s | Full-suite rerun after SingleBead hardening: lifecycle advanced but cleanup assertions failed (`spec/*`, `bead/*`, worktrees remained) after max turns. |
| 2026-03-02 | PASS | 675 | 28 | 4m10.23s | Increased MaxTurns 75->100 and clarified prompt end-state; targeted rerun completed cleanup and returned to idle. |
| 2026-03-02 | PASS | 530 | 33 | 4m02.18s | Final full-suite verification remains green under the higher turn budget. |
| 2026-03-02 | FAIL | 437 | 34 | 3m28.77s | De-tautologized full-suite validation: lifecycle progressed but strict cleanup assertions failed again (`spec/*`, `bead/*`, worktrees remained). |
| 2026-03-03 | PASS | 550 | 39 | 254.50s | Spec 058 fixes (DetectWorktreeContext last-match, focus propagation, plan scaffold). 41.0% fwd ratio (16 fwd / 23 retry). Retries from manual merge conflicts on `.mindspec/focus` (committed to both branches). |
| 2026-03-03 | PASS | 423 | 28 | 189.29s | After sandbox .gitignore fix (added .mindspec/focus + session.json). **71.4% fwd ratio** (20 fwd / 8 retry). Zero merge conflicts — bead→spec merge at [312] clean. `approve impl` succeeded after auto-merge of unmerged bead branch at [311-312]. |

### TestLLM_AbandonSpec

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | FAIL | 11 | 1 | 6.5s | Baseline: conversational response, agent asked "What would you like?" |
| 2026-02-28 | PASS | 18 | 2 | 10s | Imperative prompt pattern: "Execute these commands immediately" (50% fwd — infra noise) |
| 2026-02-28 | PASS | 18 | 2 | 8.8s | Filter infra git cmds from retry detection: **100% forward ratio** |
| 2026-02-28 | PASS | 31 | 1 | 11s | Full hooks enabled: `mindspec instruct` runs via SessionStart. Imperative prompt overrides idle template. 100% fwd, 1 turn (down from 2). |
| 2026-02-28 | PASS | 35 | 3 | 14s | Full suite run: stable, 100% fwd ratio |
| 2026-03-01 | PASS | 51 | 3 | 26.39s | Pass in full-suite run. More infra events than previous baseline, behavior still correct (`explore` + `dismiss`). |
| 2026-03-01 | FAIL | 47 | 4 | 27.55s | Full-suite rerun: dismiss commands were attempted but only with non-zero exits, so no successful `dismiss` event matched stricter assertions. |
| 2026-03-02 | FAIL | 25 | 2 | 10.48s | **REGRESSION**: `mindspec explore dismiss` exits 2 (panic), so no successful `explore`/`dismiss` events are recorded. |
| 2026-03-02 | PASS | 35 | 3 | 13.18s | Fixed nil-focus handling in `explore.Dismiss`/`explore.Promote`; `mindspec explore dismiss` now exits 0 and the scenario passes. |

### TestLLM_ResumeAfterCrash

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 45 | 3 | 29.4s | Baseline: 66.7% fwd ratio, 1 retry (complete before commit) |
| 2026-02-28 | PASS | 74 | 2 | 22s | Full hooks enabled: agent gets implement.md guidance via SessionStart. **100% fwd ratio** (up from 66.7%), 2 turns (down from 3). Still 1 retry on complete (session.json dirty). |
| 2026-02-28 | PASS | 86 | 3 | 33s | Full suite run: stable, 100% fwd ratio |
| 2026-03-01 | FAIL | - | - | 2.15s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | PASS | 138 | 6 | 43.81s | Full-suite rerun pass: resume-after-crash flow completed under current setup and assertions. |
| 2026-03-02 | PASS | 111 | 7 | 45.59s | Full-suite rerun pass; scenario still completes after one retry in the complete/commit flow. |

### TestLLM_InterruptForBug

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 81 | 3 | 26s | First recorded run: 100% fwd ratio, agent fixed bug + created feature + completed bead |
| 2026-03-01 | FAIL | - | - | 2.12s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | PASS | 180 | 7 | 61.76s | Full-suite rerun pass: interrupt-for-bug scenario still completes with current assertions. |
| 2026-03-02 | FAIL | 148 | 12 | 1m13.62s | **REGRESSION**: run reached `mindspec complete`, but `feature.go` was never created so artifact assertion failed. |
| 2026-03-02 | PASS | 156 | 8 | 57.97s | `mindspec-n9j7` validation: guidance/hook updates plus artifact assertion hardened to accept root or worktree output; scenario completes successfully. |
| 2026-03-02 | FAIL | 140 | 8 | 1m02.81s | De-tautologized full-suite validation: agent handled interrupts but never produced `feature.go`; artifact assertion failed. |

### TestLLM_MultiBeadDeps

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 230 | 6 | 66s | First recorded run: completed 2/3 beads within 30 turns, 66.7% fwd (2 retries on complete due to dirty tree), all 3 files created |
| 2026-03-01 | FAIL | - | - | 2.46s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 228 | 7 | 69.37s | Full-suite rerun: scenario advanced, but artifact assertions failed (`formatter.go` and `formatter_test.go` not found at expected location). |
| 2026-03-02 | FAIL | 131 | 12 | 1m15.11s | Full-suite rerun: max turns reached without successful `mindspec next`; no `.worktrees/` CWD observed. |
| 2026-03-02 | PASS | 187 | 12 | 1m19.56s | `mindspec-n9j7` fix: implement template + pre-commit guardrails now steer retries toward `mindspec next` and managed worktree handoff, restoring pass. |

### TestLLM_SpecInit

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 57 | 6 | 37s | Baseline: agent ran spec-init, created worktree + branch. 100% fwd ratio. Hit max turns (15) while writing spec content. |
| 2026-03-01 | FAIL | 49 | 3 | 20.39s | **REGRESSION**: assertions failed after run (`.mindspec/focus` missing in repo root after spec-init). |
| 2026-03-01 | FAIL | 56 | 5 | 29.78s | Full-suite rerun: `mindspec spec-init` never succeeded (exit non-zero) and root `.mindspec/focus` assertion remains red. |
| 2026-03-02 | PASS | 54 | 3 | 28.96s | Full-suite rerun pass: `mindspec spec-init` succeeded and focus/worktree assertions held. |

### TestLLM_SpecApprove

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 47 | 4 | 39s | Baseline: agent ran `mindspec approve spec` (3 attempts, exit=1 each — spec validation failures). 50% fwd ratio. Hit max turns (15). Validation errors are a product gap, not test issue. |
| 2026-03-01 | PASS | 68 | 5 | 35s | Fixed setup: realistic worktree structure. Removed misleading `assertBranchIs(main)`. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.22s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in spec mode. |
| 2026-03-01 | FAIL | 74 | 6 | 37.52s | Full-suite rerun: scenario remained in `spec` mode; expected transition to `plan` did not occur. |
| 2026-03-02 | PASS | 59 | 4 | 40.16s | Full-suite rerun pass: `approve spec` succeeded and scenario met transition assertions. |

### TestLLM_PlanApprove

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 117 | 9 | 56s | Baseline: agent ran `approve plan` (succeeded on 3rd try) then `mindspec next` (claimed bead, created nested worktree). 77.8% fwd ratio (7 fwd / 2 retry). |
| 2026-03-01 | PASS | 130 | 8 | 54s | Fixed plan.md to pass ValidatePlan (added version, ADR Fitness, Testing Strategy, proper bead Steps/Verification). Added git state assertions. 100% fwd ratio. |
| 2026-03-01 | PASS | 90 | 2 | 23s | Fixed assertions: removed misleading `assertBranchIs(main)`, added `assertHasWorktrees`. Agent CWD enters bead worktree. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.07s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in plan mode. |
| 2026-03-01 | FAIL | 148 | 3 | 46.93s | Full-suite rerun: `approve plan`/`next` activity occurred, but focus mode stayed `plan` instead of expected `implement`. |
| 2026-03-02 | PASS | 155 | 6 | 43.84s | Full-suite rerun pass with higher retries/events; `approve plan` plus `next` completed successfully. |

### TestLLM_ImplApprove

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 60 | 3 | 24s | Baseline: agent ran `state show`, then `approve impl` (direct merge + cleanup), then session close (bd sync, git commit, git push). 100% fwd ratio. |
| 2026-03-01 | PASS | 58 | 3 | 22s | Fixed setup: realistic spec worktree (not just branch), focus.activeWorktree set. Added `assertFileExists(done.go)` to verify merge. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.67s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in review mode. |
| 2026-03-01 | PASS | 69 | 3 | 26.33s | Full-suite rerun pass: review-to-idle transition and merge assertions still hold. |
| 2026-03-02 | FAIL | 101 | 5 | 31.75s | **REGRESSION**: `approve impl` command succeeded, but focus remained `review` instead of transitioning to `idle`. |
| 2026-03-02 | PASS | 84 | 4 | 29.54s | Fixed `ApproveImpl` focus persistence: fallback to root focus when local missing and write idle focus to both local+root targets. |

### TestLLM_SpecStatus

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 40 | 2 | 14s | Baseline: agent ran `state show` and `instruct`, reported implement mode with active bead. 100% fwd ratio. |
| 2026-03-01 | PASS | 33 | 2 | 17s | Fixed setup: realistic spec + bead worktrees, branches, focus.activeWorktree. Added branch/worktree preservation assertions. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.39s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | PASS | 33 | 2 | 19.25s | Full-suite rerun pass: read-only status checks and branch/worktree preservation assertions still pass. |
| 2026-03-02 | PASS | 23 | 3 | 13.51s | Full-suite rerun pass with lower events/time while preserving status assertions. |

### TestLLM_MultipleActiveSpecs

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 87 | 5 | 47s | Baseline: agent discovered `--spec` flag after initial `complete` and `state show` failures. Tried `complete`, `complete --spec=001-alpha`, `bd close`, `state set`, then `complete` again. 80% fwd ratio (4 fwd / 1 retry). Agent reached max turns (20) but all assertions pass. |
| 2026-03-01 | FAIL | - | - | 2.26s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 108 | 3 | 27.85s | Added explicit `--spec` assertion: agent completed bead successfully but never used `--spec` on `mindspec complete`. This indicates current product path can disambiguate without the flag, so scenario intent/assertion may no longer match runtime behavior. |
| 2026-03-01 | FAIL | 169 | 6 | 51.79s | Full-suite rerun: bead closed successfully, but no successful `mindspec complete --spec...` invocation (new assertion still failing). |
| 2026-03-02 | PASS | 179 | 8 | 51.33s | Full-suite rerun pass; scenario succeeds but retry overhead remains high (37.5% forward ratio). |
| 2026-03-02 | FAIL | 151 | 5 | 57.41s | Full-suite rerun: no successful `mindspec complete --spec ...`; artifact/complete assertions failed. |
| 2026-03-02 | PASS | 62 | 4 | 31.16s | Hardened setup with active bead worktree (while keeping activeSpec unset) + imperative prompt; targeted rerun passes and uses `--spec`. |
| 2026-03-02 | PASS | 62 | 2 | 25.62s | Final full-suite verification remains green with successful `mindspec complete --spec ...`. |
| 2026-03-02 | FAIL | 145 | 13 | 81.71s | De-tautologized prompt v1 (too open) regressed disambiguation completion: no successful `mindspec complete --spec ...` observed. |
| 2026-03-02 | PASS | 203 | 7 | 72.26s | Prompt revised to lifecycle end-state (001-alpha to review, 002-beta unchanged, no `bd close` shortcut) restored `--spec` completion path without command-level prescription. |

### TestLLM_StaleWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 70 | 7 | 42s | Baseline: agent recovered from missing worktree by manually closing the bead via `bd close` and resetting state with `mindspec state set --mode idle`. 71.4% fwd ratio (5 fwd / 2 retry). `mindspec complete` failed (stale worktree), agent worked around it. |
| 2026-03-01 | FAIL | - | - | 2.13s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 126 | 8 | 53.00s | Full-suite rerun: stale-worktree recovery attempts happened, but no successful `mindspec complete` event was recorded. |
| 2026-03-02 | PASS | 101 | 5 | 47.28s | Full-suite rerun pass via documented fallback (`state set --mode idle` + `bd close`) after `complete` failure. |

### TestLLM_CompleteFromSpecWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | FAIL | - | - | 2.48s | Baseline in full-suite run: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 152 | 2 | 35.91s | Full-suite rerun: scenario progressed further, but never produced a successful `mindspec complete`. |
| 2026-03-02 | PASS | 132 | 9 | 49.42s | Full-suite rerun pass: successful `mindspec complete` observed from spec-worktree context. |
| 2026-03-02 | PASS | 127 | 6 | 36.92s | Regression check after worktree-anchor fix: still green; `mindspec complete` succeeds from spec-worktree context. |

### TestLLM_ApproveSpecFromWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | FAIL | - | - | 2.18s | Baseline in full-suite run: setup failed before agent run. `sandbox.Commit()` blocked on main in spec mode. |
| 2026-03-01 | FAIL | 40 | 4 | 29.10s | Full-suite rerun: repeated `mindspec approve spec 001-greeting` attempts all exited non-zero; no successful approval event. |
| 2026-03-02 | PASS | 55 | 6 | 44.53s | Full-suite rerun pass: successful `approve spec` recorded in worktree-only artifact context. |

### TestLLM_ApprovePlanFromWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | FAIL | - | - | 2.22s | Baseline in full-suite run: setup failed before agent run. `sandbox.Commit()` blocked on main in plan mode. |
| 2026-03-01 | PASS | 104 | 4 | 27.41s | Full-suite rerun pass: worktree-context `approve plan` assertion now succeeds end-to-end. |
| 2026-03-02 | PASS | 61 | 4 | 24.56s | Full-suite rerun pass; stable behavior with fewer events than prior pass. |

### TestLLM_BugfixBranch

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 45 | 2 | 25s | TAUTOLOGICAL — prompt said "create a branch, create PR, don't commit to main". Agent followed instructions. Not a valid workflow test. |
| 2026-03-01 | PASS | 51 | 3 | 23s | Still tautological, added real GitHub remote (mrmaxsteel/test-mindspec). |
| 2026-03-01 | 3/3 PASS | 74-82 | 3-4 | 32-42s | Reliability of tautological prompt confirmed. |
| 2026-03-01 | FAIL | 70 | 3 | 25s | Removed workflow hints from prompt (task-only). Agent committed directly to main — no branch, no PR. **Confirmed guidance gap.** |
| 2026-03-01 | FAIL | 25 | 2 | 12s | Added "Branch Policy — MANDATORY" section to idle.md. Agent still edited directly on main. Policy section too passive for Haiku. |
| 2026-03-01 | PASS | 50 | 4 | 23s | Restructured idle.md: "How to Make Changes" with numbered steps, "you cannot edit files until you are on a branch". First non-tautological pass. |
| 2026-03-01 | 2/3 PASS | 43-46 | 2-4 | 22-27s | Reliability (3 runs). 2 pass, 1 fail (Haiku skipped guidance, edited directly). ~67% reliability with guidance-only approach on Haiku. |
| 2026-03-01 | FAIL | 23 | 2 | 32.85s | Regression check: agent fixed code but never created/pushed a branch or opened PR (`git push`/`gh pr` missing). |
| 2026-03-01 | PASS | 47 | 4 | 42.11s | Full-suite rerun pass for current prompt contract; branch/PR workflow assertions succeeded. |
| 2026-03-02 | PASS | 44 | 4 | 27.27s | Full-suite rerun pass: agent created branch/worktree, pushed, and opened PR successfully. |
| 2026-03-02 | FAIL | 23 | 2 | 13.47s | De-tautologized full-suite validation: agent fixed on main and exited without branch/push/PR workflow (`git push`/`gh pr` missing). |

### Session Summary — 2026-03-01 Full Suite

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 6 PASS (`TestLLM_ApprovePlanFromWorktree`, `TestLLM_BugfixBranch`, `TestLLM_ImplApprove`, `TestLLM_InterruptForBug`, `TestLLM_ResumeAfterCrash`, `TestLLM_SpecStatus`), 11 FAIL.
- Setup-on-main regression is resolved; remaining failures are runtime behavior/assertion mismatches rather than sandbox bootstrap failures.
- Highest-impact remaining failures: completion/approval success assertions (`SingleBead`, `AbandonSpec`, `StaleWorktree`, `CompleteFromSpecWorktree`, `ApproveSpecFromWorktree`, `MultipleActiveSpecs`), mode transition assertions (`SpecApprove`, `PlanApprove`), artifact/focus assertions (`MultiBeadDeps`, `SpecInit`), and cleanup assertions (`SpecToIdle`).

### Session Summary — 2026-03-02 Full Suite

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 12 PASS (`TestLLM_SingleBead`, `TestLLM_ResumeAfterCrash`, `TestLLM_SpecInit`, `TestLLM_SpecApprove`, `TestLLM_PlanApprove`, `TestLLM_SpecStatus`, `TestLLM_MultipleActiveSpecs`, `TestLLM_StaleWorktree`, `TestLLM_CompleteFromSpecWorktree`, `TestLLM_ApproveSpecFromWorktree`, `TestLLM_ApprovePlanFromWorktree`, `TestLLM_BugfixBranch`), 5 FAIL (`TestLLM_SpecToIdle`, `TestLLM_MultiBeadDeps`, `TestLLM_AbandonSpec`, `TestLLM_InterruptForBug`, `TestLLM_ImplApprove`).
- Main-branch setup regression remains resolved; failures are now concentrated in runtime behavior and cleanup/state-transition correctness.
- Highest-impact remaining failures after targeted rerun: cleanup leakage in `SpecToIdle`, workflow adherence in `MultiBeadDeps`, missing artifact completion in `InterruptForBug`, and review→idle focus transition mismatch in `ImplApprove`.

### Session Summary — 2026-03-02 Final Full Suite (mindspec-kt01)

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 17 PASS, 0 FAIL.
- Stabilization changes in this session:
  - `ScenarioSingleBead`: setup now starts with active bead worktree; prompt made imperative.
  - `ScenarioMultipleActiveSpecs`: setup now includes active bead worktree while preserving `--spec` disambiguation requirement; prompt made imperative; artifact assertion accepts worktree evidence.
  - `ScenarioSpecToIdle`: MaxTurns increased from 75 to 100 with explicit idle/cleanup end-state wording.
- Final full-suite command: `env -u CLAUDECODE go test ./internal/harness/ -v -run '^TestLLM_' -timeout 180m -count=1` (log: `/tmp/mindspec-kt01-fullsuite-final.log`).

### Session Summary — 2026-03-02 De-tautologized Full Suite Validation

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 14 PASS, 3 FAIL.
- Failing scenarios:
  - `TestLLM_SpecToIdle`: cleanup assertions failed (`spec/*`, `bead/*`, worktrees remained).
  - `TestLLM_InterruptForBug`: no observable `feature.go` artifact.
  - `TestLLM_BugfixBranch`: no non-main branch workflow (`git push`/`gh pr` absent).
- Command/log: `env -u CLAUDECODE go test ./internal/harness/ -v -run '^TestLLM_' -timeout 180m -count=1` (`/tmp/mindspec-kt01-fullsuite-detautologized.log`).

### Key Metrics to Track Per Run
- **Events**: total shim-recorded commands (multiple per turn -- measures total agent activity)
- **Turns (estimated)**: API round-trips, estimated from event timestamp gaps >2s. The `--max-turns` flag sets the budget; "Reached max turns" means all were consumed
- **Wall time**: total test duration (includes LLM thinking time between turns)
- **Retry count**: how many times the agent retried failing commands (measures CLI friction)
- **Events/turn ratio**: commands per turn (higher = agent is batching tool calls efficiently)
- **Forward ratio**: % of turns classified as productive work (from analyzer report)
- **Key milestone events**: which step in the lifecycle was reached before failure

### What Makes a Good Improvement
- **Reduces turns used** for the same outcome (agent is more efficient)
- **Reduces retry count** (fewer CLI errors = smoother workflow)
- **Increases first-time success rate** across multiple runs
- **Doesn't regress other scenarios** (always recheck SingleBead)

### What Can Regress
- Changing hooks/settings.json or instruct templates can make agent conversational
- Changing CWD guards can break scenarios that depend on worktree enforcement
- Changing mindspec instruct templates can override scenario prompts
- Changing beads integration can break bead creation/claiming

## Coverage Analysis (2026-03-03)

### State Transition Coverage

| Transition | Trigger | LLM Scenario(s) | Status |
|:-----------|:--------|:-----------------|:-------|
| idle → spec | `spec create` | SpecToIdle, SpecInit | Covered |
| spec → plan | `spec approve` | SpecToIdle, SpecApprove, ApproveSpecFromWorktree | Covered |
| plan → implement | `plan approve` + `next` | SpecToIdle, PlanApprove, ApprovePlanFromWorktree | Covered |
| implement → implement | `complete` + `next` (more beads) | MultiBeadDeps | Covered |
| implement → plan | `complete` (only blocked beads) | — | **Gap** |
| implement → review | `complete` (all done) | SingleBead, SpecToIdle, MultiBeadDeps, InterruptForBug, ResumeAfterCrash, CompleteFromSpecWorktree, MultipleActiveSpecs, StaleWorktree | Covered |
| review → idle | `impl approve` | SpecToIdle, ImplApprove | Covered |

**Unsupported transitions (none tested for rejection)**:
idle→plan, idle→implement, spec→implement, plan→review, review→implement, review→plan.

### Assertion Depth

**Well-covered areas:**
- Git branch/worktree cleanup after `impl approve` (SpecToIdle, ImplApprove)
- Focus mode transitions at each phase boundary (SpecInit, SpecApprove, PlanApprove, ImplApprove)
- Wrong-action detection: code edits in spec/plan mode, commits to main, wrong CWD, force bypass
- Edge cases: stale worktree recovery, crash recovery, interrupt-for-bug, multi-spec disambiguation, complete-from-spec-worktree auto-redirect
- No-mutation on read-only commands (SpecStatus)
- Pre-approve merge/PR prohibition (ImplApprove, SpecToIdle)

**Gaps:**

| Category | Gap | Impact |
|:---------|:----|:-------|
| Merge topology | No test verifies bead→spec→main merge chain (only checks branches exist/don't) | Could miss direct commits masquerading as merges |
| Beads state | No test checks beads were created by `plan approve`, claimed by `next`, or closed by `complete` via `bd list` | Beads integration regressions would go undetected |
| Invalid transitions | No negative tests for skipping phases or going backward | State machine enforcement untested |
| implement→plan | "Only blocked beads" path after `complete` never exercised | Entire branch of next-state logic untested |
| Commit message format | `impl(beadID): message` convention from `complete` not verified | Convention drift undetectable |
| Focus field depth | Tests check `mode` but not `activeSpec`, `activeBead`, `activeWorktree`, `specBranch` | State corruption in non-mode fields would go unnoticed |
| Analyzer placeholders | `skip_next` and `skip_complete` rules are stubs | Most common agent errors not auto-detected |
| Auto-commit verification | No test checks that `spec approve`, `plan approve` produce commits on the spec branch | Auto-commit regressions could break downstream merges |

### Recommendations (Priority Order)

1. **Implement `skip_next` / `skip_complete` analyzer rules** — these are the most common agent errors
2. **Add beads state assertions** — `bd list --json` checks post-`plan approve` and post-`complete`
3. **Add merge topology assertion** — verify merge commits exist with `git log --merges` on spec branch after `complete`
4. **Add focus field depth helper** — read `.mindspec/focus` and assert all fields, not just mode
5. **Add invalid transition rejection tests** — deterministic (no LLM needed), just verify CLI exits non-zero
6. **Add implement→plan scenario** — 2-bead setup where bead-2 depends on bead-1, complete bead-1, verify mode=plan
7. **Add commit message format assertion** — grep `git log` for `impl(<beadID>):` after `complete`
8. **Add auto-commit verification** — check spec branch has commits from `spec approve` / `plan approve`

See Spec 059 for planned implementation.

## Architecture Notes

### Sandbox Setup (`sandbox.go`)
- Creates temp dir with git repo, `.mindspec/`, `config.yaml`
- Runs `setup.RunClaude()` for CLAUDE.md, slash commands, **and full hooks** (SessionStart + PreToolUse)
- SessionStart hook runs `mindspec instruct` — agent gets mode-aware guidance
- PreToolUse enforcement hooks are installed but **no-op** because `config.yaml` has `agent_hooks: false` (non-enforcement scenarios work from main repo root, not a worktree)
- Runs `bd init --sandbox --skip-hooks --server-port 0`
- Installs recording shims in `.harness/bin/`
- Adds `.beads/`, `.harness/`, `.mindspec/session.json`, `.mindspec/focus`, `.mindspec/current-spec.json` to `.gitignore`

### Recording Shims (`recorder.go`)
- Shell scripts in `.harness/bin/` that log to `events.jsonl` then delegate to real binary
- Shims for: git, mindspec, bd
- Each event has: command, args_list, exit_code, cwd, timestamp
- Events are the primary diagnostic -- always check them first

### Agent Invocation (`agent.go`)
- `claude -p <prompt> --permission-mode bypassPermissions --max-turns N --model haiku`
- `cmd.Dir = sandbox.Root` (agent starts in main repo)
- `filterEnv(sandbox.Env(), "CLAUDECODE")` strips CLAUDECODE for nesting
- `cmd.CombinedOutput()` captures all agent text output

### Scenario Structure (`scenario.go`)
- `Setup func(sandbox *Sandbox) error` -- prepare sandbox state
- `Prompt string` -- the agent's task (MUST be imperative for Haiku)
- `Assertions func(t, sandbox, events)` -- post-run checks
- `MaxTurns int` -- turn budget (too low = incomplete, too high = slow)
- `Model string` -- always "haiku" for cost/speed

### Why Haiku?
- Cost: ~$0.01-0.05 per test run vs $0.50+ for Sonnet/Opus
- Speed: 1-3 minutes vs 5-10 minutes
- If Haiku can follow the workflow, Sonnet/Opus definitely can
- Tradeoff: Haiku needs more imperative prompts and retries more

## Sandbox Helpers for Scenario Setup

```go
sandbox.CreateBead(title, issueType, parentID) string  // Create real beads issue
sandbox.ClaimBead(beadID)                                // Set to in_progress
sandbox.WriteFile(relPath, content)                      // Write file in sandbox
sandbox.WriteFocus(content)                              // Write .mindspec/focus
sandbox.WriteLifecycle(specID, content)                  // Write lifecycle.yaml
sandbox.Commit(msg)                                      // git add -A && commit
sandbox.FileExists(relPath) bool                         // Check file exists
sandbox.ReadFile(relPath) string                         // Read file content
sandbox.GitBranch() string                               // Current branch
sandbox.BranchExists(branch) bool                        // Check branch exists
sandbox.ListBranches(prefix) []string                    // List branches matching prefix
sandbox.ListWorktrees() []string                         // List .worktrees/ entries
```

## Prompt Engineering for Haiku

Haiku in `claude -p` mode tends to be conversational unless strongly directed. Rules:

1. **Say "Do NOT respond conversationally"** -- prevents Haiku from greeting instead of executing
2. **Describe the task, not the workflow** -- "add a greeting feature", not "run mindspec explore"
3. **Be specific about what to build** -- "create greeting.go with a Greet(name) function"
4. **End-state instructions are OK** -- "run `mindspec complete` when done" names the finish line
5. **Do NOT prescribe intermediate commands** -- the agent must discover `mindspec explore`, `mindspec approve`, `mindspec next` from CLAUDE.md and instruct templates

## Known Issues & Workarounds

### Dolt Server Orphans
**Problem**: Each sandbox `bd init` starts a dolt sql-server. If the test crashes or the process isn't cleaned up, orphan servers accumulate and block new ones (max 3).
**Workaround**: `initBeads()` calls `bd dolt killall` before `bd init`. Also run `bd dolt killall` manually before test sessions.
**Permanent fix needed**: Per-sandbox dolt cleanup in `t.Cleanup()`.

### Setup Commits Blocked on Main (REGRESSION — 2026-03-01)
**Problem**: Many scenarios call `sandbox.Commit()` after setting non-idle mode state. Current guard rules reject commits on `main` in spec/plan/implement/review, so setup fails before agent execution.
**Workaround**: In setup helpers, either commit before moving mode state out of idle, create/use the expected worktree branch first, or allow setup commits via `MINDSPEC_ALLOW_MAIN=1`.
**Status (2026-03-01)**: Harness setup now applies the explicit escape hatch in `Sandbox.Commit()` (`MINDSPEC_ALLOW_MAIN=1`), restoring deterministic scenario bootstrap without changing runtime guard behavior.

### Explore Dismiss Panic (RESOLVED — 2026-03-02)
**Problem**: `mindspec explore dismiss` exited with code 2 and a nil-pointer panic in `TestLLM_AbandonSpec` when focus state was absent.
**Workaround**: N/A (fixed in CLI).
**Status (2026-03-02)**: Fixed by treating missing focus as implicit idle in `internal/explore` mode checks. Targeted rerun passes (35 events, 3 turns, 13.18s).

### ImplApprove Focus Transition Mismatch (RESOLVED — 2026-03-02)
**Problem**: `mindspec approve impl` could succeed while root `.mindspec/focus` stayed `review` if the command executed in a worktree.
**Workaround**: N/A (fixed in workflow logic).
**Status (2026-03-02)**: Fixed in `internal/approve/impl.go` by falling back to root focus when local focus is missing and writing idle focus to both local and root targets. Added deterministic coverage in `internal/approve/impl_test.go`. Targeted rerun passes (84 events, 4 turns, 29.54s).

### mindspec complete CWD Guard
**Problem**: Agent runs from `sandbox.Root` (main repo) but `mindspec complete` requires CWD in the bead worktree.
**Fix applied**: `cmd/mindspec/complete.go` now auto-chdirs to `ActiveWorktree` from focus state when CWD is main.

### Implement Mode Manual Worktree Bypass (RESOLVED — 2026-03-02)
**Problem**: In implement mode with no recorded `activeWorktree`, agents could bypass lifecycle commands by creating spec/bead branches or worktrees manually, then get stuck in `complete`/`next` retries.
**Workaround**: N/A (fixed in guidance + hook messaging).
**Status (2026-03-02)**: Fixed by strengthening implement template handoff rules and pre-commit guardrail messaging for implement mode (including no-active-worktree branch commits). Added deterministic coverage in `internal/hooks/install_test.go` and `internal/complete/complete_test.go`. Targeted reruns now pass (`MultiBeadDeps`, `InterruptForBug`, and `SingleBead` regression check).

### Sandbox .gitignore Missing Focus Entry (RESOLVED — 2026-03-03)
**Problem**: `sandbox.go` overwrote `.gitignore` with only `.beads/` and `.harness/`, dropping `.mindspec/focus` and `.mindspec/session.json` entries. This caused `gitops.CommitAll()` (`git add -A`) to commit focus files to both spec and bead branches with different content, creating merge conflicts on every bead→spec merge in `mindspec complete`.
**Status (2026-03-03)**: Fixed by adding `.mindspec/session.json`, `.mindspec/focus`, and `.mindspec/current-spec.json` to the sandbox `.gitignore`. SpecToIdle forward ratio improved from 41% to 71.4%, retries dropped from 23 to 8, and all merge conflicts eliminated.

### DetectWorktreeContext First-Match Bug (RESOLVED — 2026-03-03)
**Problem**: `workspace.DetectWorktreeContext()` returned on the FIRST `.worktrees` match in the path. For nested bead worktrees (`repo/.worktrees/worktree-spec-XXX/.worktrees/worktree-beadID/`), it matched the outer spec worktree and returned `WorktreeSpec` instead of `WorktreeBead`. This caused `mindspec complete` to hit the "you're in a spec worktree" hard error.
**Status (2026-03-03)**: Fixed by changing to last-match semantics — the innermost worktree type wins. SingleBead and SpecToIdle both pass.

### Nested Worktrees
**Status**: Git fully supports nested worktrees. `workspace.FindRoot()` correctly resolves them. The bead worktree is created inside the spec worktree: `.worktrees/worktree-spec-XXX/.worktrees/worktree-bead-YYY`. This is fine -- it reflects the merge hierarchy (bead -> spec -> main).

### mindspec instruct Idle Template
**Problem**: The idle template contains "Greet the user" / "Ask what they'd like to work on" which could override scenario prompts.
**Status**: SessionStart hook now runs in the sandbox (full hooks enabled). Scenarios starting in idle mode (SpecToIdle, AbandonSpec) use imperative prompts ("Execute these commands immediately. Do NOT respond conversationally.") which override the idle template greeting via `claude -p` mode. If a scenario fails due to idle template interference, fix the idle template itself (product improvement).

### Focus File Deadlock (mindspec-wpqg)
**Problem**: When multiple specs are active and one is in plan/spec mode, the agent hits a deadlock: (1) Edit on `.mindspec/focus` is blocked by workflow guard (plan mode blocks "code" edits), (2) `mindspec state set` via Bash is blocked by worktree-bash hook (CWD is main, not the active worktree). The agent cannot change spec context without fragile workarounds (writing focus from a different allowed worktree).
**Status**: Product bug filed as mindspec-wpqg (P1). Not currently testable in LLM harness because sandbox has `agent_hooks: false` (enforcement hooks are no-op). Fix should go into hook logic: whitelist `mindspec state` in worktree-bash allowlist, and/or whitelist `.mindspec/focus` in workflow guard.
**Related scenarios**: `TestLLM_MultipleActiveSpecs` tests the CLI `--spec` flag disambiguation (non-enforcement). Full enforcement testing requires `agent_hooks: true` scenarios.

### Worktree CWD Sensitivity (RESOLVED — 2026-03-02)
**Problem**: Running `git worktree add` from inside an existing worktree can create the new worktree relative to CWD, causing recursive `.worktrees/.../.worktrees/...` nesting and cleanup leakage.
**Status (2026-03-02)**: Fixed in `internal/next/beads.go`: `mindspec next` now anchors worktree creation to the spec worktree (when active) or main root, independent of caller CWD. Added deterministic unit coverage in `internal/next/next_test.go` and validated with LLM reruns (`SpecToIdle` pass, `CompleteFromSpecWorktree` regression check pass).
