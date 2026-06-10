# LLM Test Harness — Operational Guide

## Overview

The harness in `internal/harness/` runs behavioral tests against real Claude Code sessions. Each test spawns a `claude -p` process in an isolated sandbox git repo, gives it a scenario prompt, and asserts the agent executed the expected mindspec workflow commands.

This is the most effective way to validate that the mindspec workflow actually works end-to-end with a real LLM agent. The iterative **test -> observe -> fix -> retest** loop is how we improve both the CLI and the agent experience.

## How to Run Tests

### Prerequisites
```bash
make build                    # Rebuild mindspec binary (CRITICAL -- tests use ./bin/mindspec)
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
3. **Dolt isolation** -- Each sandbox gets its own dolt server on a random port (Spec 070). No need to kill orphans before testing
4. **Timeout** -- SpecToIdle needs 15min timeout; simpler tests need 10min
5. **Parallel tests** -- Each sandbox has its own dolt server, but parallel LLM tests still compete for LLM API quota

## Test Design Principles

Every LLM test must follow these rules. Violating them produces tautological tests that validate prompt adherence instead of product quality.

### 1. Deterministic Setup

`Setup()` must create **all** required infrastructure before the agent runs. The agent should never need to bootstrap its own environment. This includes:

- **Config**: `.mindspec/config.yaml` with appropriate `agent_hooks` setting
- **Hooks**: `setupClaudeForSandbox()` installs CLAUDE.md, `.claude/settings.json`, slash commands
- **State files**: `.mindspec/session.json` (lifecycle state derived from beads per ADR-0023)
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
| `TestLLM_MultipleActiveSpecs` | 30 | Medium | Two active specs — agent completes one without disrupting the other |
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

- Add `.beads/` and `.harness/` to `.gitignore` in sandbox

---

**History & reference material** (improvement history tables, session summaries, coverage analysis, architecture notes, known issues) is in [HISTORY.md](HISTORY.md).
