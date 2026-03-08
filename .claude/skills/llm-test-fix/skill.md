---
name: llm-test-fix
description: Autonomous test fix loop — review, diagnose, fix, verify, commit for a list of failing LLM test scenarios
---

# LLM Test Fix Loop

You are entering an autonomous test-fix workflow. You will work through a list of test scenarios, fixing each one without user interaction. Do NOT stop or ask questions — work through the entire list autonomously.

## Input

The user provides a list of `TestLLM_*` scenario names (e.g., `ApprovePlanFromWorktree UnmergedBeadGuard`).

## Prerequisites

Read these files for context (if not already loaded):
- `internal/harness/TESTING.md` — operational guide, design principles, failure taxonomy
- `internal/harness/HISTORY.md` — improvement history (read the last 3 rows per relevant scenario)
- `internal/harness/scenario.go` — scenario definitions
- `internal/harness/scenario_test.go` — test runner

Always rebuild before testing:
```bash
make build
```

## Per-Scenario Loop

For **each** scenario in the provided list, execute this loop:

### Step 1: Review

Run `/llm-test-review <scenario>` to perform a deep-dive audit of the scenario's design quality. Record the verdict and recommended fixes.

### Step 2: Baseline Run

Run the test once with a 5-minute timeout:
```bash
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_<Name> -timeout 5m -count=1
```

Record: Result, Events, Turns, Time, Forward Ratio.

If the test PASSes, note it and move to the next scenario — no fix needed.

### Step 3: Implement Fixes

Based on the review and baseline run failure analysis, implement **all** recommended fixes. Follow the fix surface rule:
- **Agent behavior failures** → fix mindspec guidance (instruct templates, CLAUDE.md, CLI error messages)
- **Test design issues** → fix scenario setup, assertions, or turn budget (NOT the prompt's workflow instructions)
- **Assertion gaps** → tighten or relax assertions to match the test's spirit

After implementing fixes:
```bash
make build && go build ./internal/harness/
```

### Step 4: Verification Runs (3x)

Run the test 3 times to verify reliability:
```bash
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_<Name> -timeout 5m -count=3
```

Record: pass rate (e.g., 2/3 PASS), events range, turns range, time range.

### Step 5: Post-Fix Review

Run `/llm-test-review <scenario>` again on the modified scenario. Compare with Step 1 review.

### Step 6: Iterate or Commit

- If the review identifies **further changes needed** AND you have iterations remaining (max 3 per scenario), go back to Step 3.
- If all 3 runs PASS and the review is clean, commit the fixes:
  ```bash
  make build
  go test ./internal/harness/ -short -v   # Verify deterministic tests pass
  git add <changed files>
  git commit -m "fix(harness): <scenario> — <what changed>"
  ```
- If after 3 iterations the test still fails, document the remaining issue and move on.

### Step 7: Regression Check

After fixing each scenario, run `TestLLM_SingleBead` once to verify no regressions:
```bash
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 5m -count=1
```

If SingleBead regresses, revert the last change and try a different approach.

## After All Scenarios

### Update HISTORY.md

Add rows to each scenario's improvement history table with the results from this session.

### Final Commit & Push

```bash
git add internal/harness/HISTORY.md
git commit -m "docs: update HISTORY.md with test fix session results"
git push
```

### Final Report

Present a summary table:

| Scenario | Before | After | Iterations | Key Change |
|----------|--------|-------|------------|------------|
| ... | FAIL | 3/3 PASS | 2 | Relaxed branch assertion... |

Include:
- Total scenarios attempted
- Total scenarios fixed
- Any remaining failures with root cause analysis
- Regression check results

## Key Rules

- **Fix surface rule**: When an LLM test fails due to agent behavior, the fix MUST go into mindspec's guidance — NEVER into the test prompt
- **`env -u CLAUDECODE`** is REQUIRED for all test runs
- **`make build`** before testing — shims call `./bin/mindspec`
- **Do NOT stop** between scenarios — work through the full list autonomously
- **Max 3 iterations** per scenario to prevent infinite loops
- **Commit each scenario's fixes independently** for bisectability
