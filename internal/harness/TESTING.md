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
| `TestLLM_SpecToIdle` | 75 | High | Full 9-step lifecycle (explore -> idle) |

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

### TestLLM_AbandonSpec

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | FAIL | 11 | 1 | 6.5s | Baseline: conversational response, agent asked "What would you like?" |
| 2026-02-28 | PASS | 18 | 2 | 10s | Imperative prompt pattern: "Execute these commands immediately" (50% fwd — infra noise) |
| 2026-02-28 | PASS | 18 | 2 | 8.8s | Filter infra git cmds from retry detection: **100% forward ratio** |
| 2026-02-28 | PASS | 31 | 1 | 11s | Full hooks enabled: `mindspec instruct` runs via SessionStart. Imperative prompt overrides idle template. 100% fwd, 1 turn (down from 2). |
| 2026-02-28 | PASS | 35 | 3 | 14s | Full suite run: stable, 100% fwd ratio |

### TestLLM_ResumeAfterCrash

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 45 | 3 | 29.4s | Baseline: 66.7% fwd ratio, 1 retry (complete before commit) |
| 2026-02-28 | PASS | 74 | 2 | 22s | Full hooks enabled: agent gets implement.md guidance via SessionStart. **100% fwd ratio** (up from 66.7%), 2 turns (down from 3). Still 1 retry on complete (session.json dirty). |
| 2026-02-28 | PASS | 86 | 3 | 33s | Full suite run: stable, 100% fwd ratio |

### TestLLM_InterruptForBug

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 81 | 3 | 26s | First recorded run: 100% fwd ratio, agent fixed bug + created feature + completed bead |

### TestLLM_MultiBeadDeps

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 230 | 6 | 66s | First recorded run: completed 2/3 beads within 30 turns, 66.7% fwd (2 retries on complete due to dirty tree), all 3 files created |

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

## Architecture Notes

### Sandbox Setup (`sandbox.go`)
- Creates temp dir with git repo, `.mindspec/`, `config.yaml`
- Runs `setup.RunClaude()` for CLAUDE.md, slash commands, **and full hooks** (SessionStart + PreToolUse)
- SessionStart hook runs `mindspec instruct` — agent gets mode-aware guidance
- PreToolUse enforcement hooks are installed but **no-op** because `config.yaml` has `agent_hooks: false` (non-enforcement scenarios work from main repo root, not a worktree)
- Runs `bd init --sandbox --skip-hooks --server-port 0`
- Installs recording shims in `.harness/bin/`
- Adds `.beads/` and `.harness/` to `.gitignore`

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

### mindspec complete CWD Guard
**Problem**: Agent runs from `sandbox.Root` (main repo) but `mindspec complete` requires CWD in the bead worktree.
**Fix applied**: `cmd/mindspec/complete.go` now auto-chdirs to `ActiveWorktree` from focus state when CWD is main.

### Nested Worktrees
**Status**: Git fully supports nested worktrees. `workspace.FindRoot()` correctly resolves them. The bead worktree is created inside the spec worktree: `.worktrees/worktree-spec-XXX/.worktrees/worktree-bead-YYY`. This is fine -- it reflects the merge hierarchy (bead -> spec -> main).

### mindspec instruct Idle Template
**Problem**: The idle template contains "Greet the user" / "Ask what they'd like to work on" which could override scenario prompts.
**Status**: SessionStart hook now runs in the sandbox (full hooks enabled). Scenarios starting in idle mode (SpecToIdle, AbandonSpec) use imperative prompts ("Execute these commands immediately. Do NOT respond conversationally.") which override the idle template greeting via `claude -p` mode. If a scenario fails due to idle template interference, fix the idle template itself (product improvement).
