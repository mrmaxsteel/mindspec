---
approved_at: "2026-02-28T23:28:01Z"
approved_by: user
status: Approved
---
# Spec 056-test-prompt-hygiene: LLM Test Prompt Hygiene

## Goal

Ensure LLM test scenarios validate that mindspec's own guidance (CLAUDE.md, instruct templates, CLI error messages, hooks) drives agent behavior — not that the agent can follow explicit step-by-step instructions baked into the test prompt.

## Background

Several LLM test scenarios prescribe exact mindspec commands in the prompt (e.g., "Run `mindspec explore`", "Run `mindspec approve spec`"), then assert those commands appeared in the event log. This is tautological — it tests prompt adherence, not whether mindspec successfully guides an agent through the workflow.

The HookBlocksCodeInSpec test was already fixed (enabled `agent_hooks: true`, prompt now asks agent to write code instead of telling it not to). The same pattern needs to be applied to other tautological and weak-assertion scenarios.

## Impacted Domains

- `internal/harness/`: scenario prompts, assertions, TESTING.md guidance

## ADR Touchpoints

None — no architectural decisions needed.

## Requirements

1. Add "Test Design Principles" section to TESTING.md codifying: deterministic setup, minimal prompts, two prompt categories (workflow vs enforcement), fix surface rule
2. Rewrite `ScenarioSpecToIdle` prompt to contain only a task description — no mindspec commands
3. Rewrite `ScenarioAbandonSpec` prompt to contain only a task description — no mindspec commands
4. Add assertions to `ScenarioHookBlocksMainCommit` verifying a hook/guard actually blocked
5. Add assertions to `ScenarioHookBlocksStaleNext` verifying the freshness gate fired

## Scope

### In Scope
- `internal/harness/scenario.go` — prompts and assertions for 4 scenarios
- `internal/harness/TESTING.md` — new design principles section

### Out of Scope
- SingleBead, MultiBeadDeps, InterruptForBug, ResumeAfterCrash (already task-focused)
- New scenarios
- Changes to mindspec CLI, hooks, or instruct templates
- Running SpecToIdle LLM test (expensive, defer to dedicated session)

## Non-Goals

- Not changing MaxTurns budgets
- Not adding new test scenarios
- Not modifying mindspec's guidance layer (this spec fixes tests, not the product)

## Acceptance Criteria

- [ ] TESTING.md "Test Design Principles" section exists with deterministic setup rule, minimal prompt rule, two prompt categories, fix surface rule
- [ ] `ScenarioSpecToIdle` prompt has zero mindspec commands — only a task description
- [ ] `ScenarioAbandonSpec` prompt has zero mindspec commands — only a task description
- [ ] `ScenarioHookBlocksMainCommit` assertions verify non-zero hook exit code
- [ ] `ScenarioHookBlocksStaleNext` assertions verify non-zero hook exit code
- [ ] `go test ./internal/harness/ -short -v` passes
- [ ] `TestLLM_SingleBead` passes (regression check)

## Validation Proofs

- `go test ./internal/harness/ -short -v`: all deterministic tests pass
- `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m -count=1`: regression check passes

## Open Questions

None — approach is clear from prior exploration.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-28
- **Notes**: Approved via mindspec approve spec