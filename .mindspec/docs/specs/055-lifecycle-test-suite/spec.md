---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 055-lifecycle-test-suite: Lifecycle Documentation & LLM Behavior Test Suite

## Goal

Create a canonical reference document for MindSpec's state machine (every file, every transition, every hook) and a comprehensive test suite that validates both the deterministic state machine and LLM agent behavior against it. The test suite must catch regressions in state management, hook enforcement, and — critically — whether a real LLM actually calls the right commands, from the right CWD, at the right lifecycle phase.

## Background

MindSpec's lifecycle is distributed across multiple state artifacts (focus, session.json, lifecycle.yaml, spec/plan frontmatter), enforced by three hook layers (git pre-commit, CLI guards, agent PreToolUse), and orchestrated across git worktrees. This complexity is currently documented only implicitly in code. There is no single reference, and no test that validates the full lifecycle end-to-end — especially not with a real LLM in the loop.

The existing `internal/bench` harness (Spec 018) provides runner/session/worktree infrastructure but does not test lifecycle state transitions or LLM behavioral compliance.

Key risks without this:
- State artifacts drift out of sync (focus says "plan" but lifecycle says "implement")
- Hook enforcement has gaps (a new tool goes unguarded)
- LLM ignores guidance and writes code during spec mode, commits to main, skips `mindspec next`
- Refactoring state management (e.g. eliminating focus in favor of lifecycle-only) has no safety net

## Impacted Domains

- lifecycle: canonical documentation of state machine, transitions, invariants
- testing: new test packages for scenario and LLM behavior testing
- bench: extension of existing harness for LLM-in-the-loop scenarios
- hooks: enforcement coverage matrix verified by tests

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Worktree-first branch strategy — tests must validate worktree creation, bead branch merging, and zero-on-main invariant
- [ADR-0019](../../adr/ADR-0019.md): Three-layer enforcement — tests must verify each layer independently and composed
- [ADR-0020](../../adr/ADR-0020.md): lifecycle.yaml as phase authority — tests must validate phase transitions match lifecycle, not just focus

## Requirements

1. A canonical architecture document describing every state artifact, every lifecycle transition, every hook, and every worktree topology — with diagrams
2. A deterministic scenario test suite (no LLM) that exercises every lifecycle transition and validates state invariants after each step
3. An LLM behavior test suite that runs a real Claude session in a sandbox repo and asserts the agent calls correct commands at correct times
4. Coverage of alternative flows: interrupt implementation for bug fix, abandon spec, resume after crash, multi-bead dependency ordering
5. Hook enforcement coverage matrix: for each (mode × tool × location) triple, assert the expected allow/block outcome
6. All tests runnable via `make test` (deterministic) and `make bench-llm` (LLM, requires API key)

## Scope

### In Scope

- `docs/architecture/lifecycle-state-machine.md` — canonical reference document
- `internal/lifecycle/scenario_test.go` — deterministic state machine tests
- `internal/lifecycle/hook_matrix_test.go` — enforcement coverage matrix
- `internal/bench/llm_scenario_test.go` — LLM behavior tests using Claude API
- `internal/bench/sandbox.go` — sandbox repo builder for integration tests
- Extensions to existing `internal/bench/` harness as needed

### Out of Scope

- Changing the state management architecture (e.g. eliminating focus) — that's a separate spec informed by this one
- Performance benchmarking (Spec 018/019 scope)
- Multi-agent coordination testing (Spec 025 scope)
- UI/visualization testing

## Non-Goals

- This spec does not refactor state management — it documents and tests the current design
- This spec does not add new enforcement hooks — it verifies existing ones
- LLM tests are not expected to be fast; they are integration tests that run separately from unit tests

## Acceptance Criteria

- [ ] `docs/architecture/lifecycle-state-machine.md` exists and documents: (a) every state artifact with fields, readers, writers; (b) every transition with pre/post conditions; (c) hook enforcement matrix; (d) worktree topology diagrams; (e) alternative flow narratives
- [ ] Deterministic scenario tests cover the happy path: idle → explore → spec → plan → implement (multi-bead) → review → idle, asserting state after each transition
- [ ] Deterministic scenario tests cover at least 3 alternative flows: (1) abandon spec mid-implementation, (2) interrupt for bug fix, (3) resume after session crash
- [ ] Hook enforcement matrix test covers all (mode × tool) combinations from ADR-0019, asserting correct allow/block/warn outcome
- [ ] LLM behavior tests run a real Claude Haiku session against a sandbox repo and assert: (a) agent calls `mindspec next` before writing code, (b) agent works in correct worktree CWD, (c) agent calls `mindspec complete` after finishing bead, (d) agent does not commit to main
- [ ] LLM behavior tests use structured assertions on command history (not just "did it succeed"), verifying command sequence, CWD, and arguments
- [ ] `make test` runs deterministic tests; `make bench-llm` runs LLM tests (skipped without API key)
- [ ] All existing tests continue to pass (`make test` green)

## Validation Proofs

- `make test`: all deterministic scenario and hook matrix tests pass
- `make bench-llm`: LLM behavior tests pass against Claude Haiku in sandbox repo
- `go test ./internal/lifecycle/ -v -run TestScenario_HappyPath`: happy path scenario completes with correct state at each step
- `go test ./internal/bench/ -v -run TestLLM_BeadWorkflow`: LLM correctly navigates bead claim → implement → complete cycle

## Open Questions

- [x] Should LLM tests use Claude Haiku (cheap, fast) or Sonnet (more realistic)? **Decision: Haiku for CI cost, Sonnet as optional flag**
- [x] Should the sandbox repo be created fresh per test or reused? **Decision: Fresh per test for isolation**
- [x] How to capture LLM command history — intercept bash calls or parse git log? **Decision: Wrap mindspec/git/bd commands with a recording shim that logs invocations to a file, then assert against the log**

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
