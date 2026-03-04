---
description: Enter the iterative LLM test harness loop (test -> observe -> fix -> retest)
---

# LLM Test Harness Mode

You are entering the iterative LLM testing mode. Read `internal/harness/TESTING.md` for the full operational guide — it contains improvement history, failure taxonomy, and architecture notes. You MUST update TESTING.md during this session.

## Step 1: Context

Read these files to understand current state:
- `internal/harness/TESTING.md` — operational guide and improvement history
- `internal/harness/scenario.go` — all scenario definitions
- `internal/harness/scenario_test.go` — test runner and observability logging

## Step 2: Ask the User

Present these options:

1. **Run a test** — pick a scenario to run (show the table from TESTING.md with MaxTurns and complexity)
2. **Run all tests** — run every TestLLM_* scenario sequentially
3. **Investigate a failure** — analyse a recent test run's events and agent output
4. **Improve a scenario** — modify prompts, turn budgets, assertions, or sandbox setup
5. **Add a new scenario** — create a new test for an untested workflow
6. **Review history** — show the improvement history tables and trends

Ask which scenario if they choose option 1 or 4.

## Step 3: Execute

### Before running any test:
```bash
make build                    # CRITICAL — tests use ./bin/mindspec via shims
```

### Running a test:
```bash
env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_<Name> -timeout <timeout>m -count=1
```
- SingleBead/AbandonSpec/ResumeAfterCrash/HookBlocks*: 10min timeout
- MultiBeadDeps/InterruptForBug: 10min timeout
- SpecToIdle: 15min timeout

### After each test run, record these metrics:
- Date
- Result (PASS/FAIL)
- Events count (from "Recorded events (N)")
- Estimated turns (from report "Turns: N (estimated)")
- Wall time (from report or test duration)
- What changed since last run (or "baseline" for first run)

## Step 4: Analyse

After each test run:
1. Read the recorded events — identify where the agent got stuck
2. Categorise the failure using the taxonomy in TESTING.md
3. Propose a targeted fix
4. If fixing, rebuild with `make build && go build ./internal/harness/`
5. Retest the same scenario
6. After fixing a complex scenario, also retest SingleBead to check for regressions

## Step 5: Update TESTING.md

**MANDATORY**: After every test session (even if no code changed), update `internal/harness/TESTING.md`:

### Improvement History Tables
Add a row to the relevant scenario's table with: Date, Result, Events, Turns, Time, Change.

Example:
```
| 2026-03-01 | PASS | 145 | 48 | 2m10s | Reduced retry friction in approve spec validation |
```

If a scenario doesn't have a history table yet, create one.

### Known Issues
If you discovered a new issue or fixed an existing one, update the Known Issues section.

### Metrics Trends
Note if metrics are trending in the right direction:
- Fewer turns for the same outcome = better
- Fewer retries = better CLI UX
- Higher forward ratio = less wasted work
- If a change regressed metrics, document it with "REGRESSION" tag

### New Scenarios
If you added a new scenario, add it to the Available Test Scenarios table.

## Step 6: Commit

After changes, commit and push:
```bash
make build
go test ./internal/harness/ -short -v   # Verify deterministic tests pass
git add <changed files>
git commit -m "descriptive message"
git push
```

## Key Reminders

- `env -u CLAUDECODE` is REQUIRED for all test runs
- `make build` before testing — the shims call ./bin/mindspec
- Haiku prompts must be imperative or the agent responds conversationally
- Always check SingleBead still passes after changing shared infrastructure
