---
approved_at: "2026-02-28T23:29:01Z"
approved_by: user
bead_ids:
    - mindspec-3er9.1
    - mindspec-3er9.2
    - mindspec-3er9.3
last_updated: "2026-02-28"
spec_id: 056-test-prompt-hygiene
status: Approved
version: 1
---

# Plan: LLM Test Prompt Hygiene

## ADR Fitness

No ADRs are relevant to this change. The work is scoped entirely to the test harness (scenario prompts, assertions, and operational documentation). No architectural decisions are affected.

## Testing Strategy

- **Deterministic tests**: `go test ./internal/harness/ -short -v` validates scenario definitions compile and sandbox setup works
- **LLM regression**: `TestLLM_SingleBead` confirms no regression from shared infrastructure changes
- **LLM validation**: Run changed enforcement scenarios (HookBlocksStaleNext) to verify new assertions
- SpecToIdle is too expensive (75 turns) to validate in this session; defer to dedicated test run

## Bead 1: Add Test Design Principles to TESTING.md

**Steps**
1. Add a new "Test Design Principles" section to `internal/harness/TESTING.md` after "How to Run Tests" and before "Available Test Scenarios"
2. Document the deterministic setup rule: Setup() must create all required infrastructure before the agent runs
3. Document the minimal prompt rule: prompts describe tasks, not workflows
4. Document two prompt categories: workflow tests (task-only prompt) vs enforcement tests (instruct agent to attempt forbidden action)
5. Reinforce the fix surface rule with examples

**Verification**
- [ ] TESTING.md contains "Test Design Principles" section
- [ ] Section covers: deterministic setup, minimal prompts, two categories, fix surface rule
- [ ] `go test ./internal/harness/ -short -v` passes (no compilation errors from doc-only change)

**Depends on**
None

## Bead 2: Fix tautological scenario prompts

**Steps**
1. Rewrite `ScenarioSpecToIdle` prompt to contain only a task description (e.g., "add a greeting feature that prints Hello") — remove all 9 prescriptive steps
2. Rewrite `ScenarioAbandonSpec` prompt to contain only a task description (e.g., "evaluate whether a bad idea feature is worth pursuing, decide it's not, and exit") — remove prescriptive commands
3. Keep imperative Haiku framing ("Do NOT respond conversationally") since this is agent infrastructure, not workflow guidance

**Verification**
- [ ] `ScenarioSpecToIdle` prompt contains zero `mindspec` commands
- [ ] `ScenarioAbandonSpec` prompt contains zero `mindspec` commands
- [ ] `go test ./internal/harness/ -short -v` passes

**Depends on**
Bead 1

## Bead 3: Fix weak enforcement assertions

**Steps**
1. In `ScenarioHookBlocksMainCommit`: enable `agent_hooks: true` in Setup config override; add assertions checking that a hook (workflow-guard, worktree-bash, or pre-commit) fired with non-zero exit
2. In `ScenarioHookBlocksStaleNext`: add assertions checking that `needs-clear` hook fired with non-zero exit code
3. Both scenarios should also verify the negative outcome (files not committed, next not successfully executed)

**Verification**
- [ ] `go test ./internal/harness/ -short -v` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_HookBlocksStaleNext -timeout 10m -count=1` passes with new assertions
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m -count=1` passes (regression check)

**Depends on**
Bead 1

## Provenance

| Acceptance Criterion | Bead | Verification |
|---------------------|------|-------------|
| TESTING.md "Test Design Principles" section | Bead 1 | Manual: section exists |
| SpecToIdle prompt has zero mindspec commands | Bead 2 | Code review |
| AbandonSpec prompt has zero mindspec commands | Bead 2 | Code review |
| HookBlocksMainCommit assertions verify non-zero exit | Bead 3 | LLM test run |
| HookBlocksStaleNext assertions verify non-zero exit | Bead 3 | LLM test run |
| Deterministic tests pass | Bead 2, 3 | `go test -short` |
| SingleBead regression check | Bead 3 | LLM test run |
