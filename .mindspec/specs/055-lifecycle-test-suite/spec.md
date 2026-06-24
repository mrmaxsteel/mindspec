---
approved_at: "2026-02-28T12:15:15Z"
approved_by: user
status: Approved
---
# Spec 055-lifecycle-test-suite: Lifecycle Documentation & LLM Behavior Test Suite

## Goal

Create a canonical reference for MindSpec's state machine and a new behavioral test harness that validates both the deterministic state transitions and — critically — whether a real LLM agent does the right thing at each phase. The harness must capture structured action events (not just tokens/cost), enabling turn-level analysis, wrong-action detection, and plan-outcome fidelity measurement.

## Background

MindSpec's lifecycle is distributed across focus, session.json, lifecycle.yaml, and spec/plan frontmatter, enforced by three hook layers across git worktrees. This complexity is documented only in code. There is no single reference, and no test validates the full lifecycle — especially not with a real LLM.

The existing `internal/bench` harness (Spec 018) measures tokens, cost, and cache hit rates via OTLP telemetry. It runs A/B/C comparisons (no-docs vs. baseline vs. full MindSpec) and generates qualitative analysis. **What it cannot do:**

- **Turn analysis**: How many turns did the agent take? Was turn 5 a self-correction or forward progress?
- **Wrong-action detection**: Did the agent try to write code during plan mode? Did a hook block it? Did it recover?
- **Command audit**: Which mindspec/git/bd commands were called, in what order, from what CWD, with what exit code?
- **Plan-outcome fidelity**: Did the agent execute the plan it wrote, or improvise?
- **Goal verification**: Did the implementation actually satisfy acceptance criteria, or just produce files?

The existing bench's token/cost reporting is useful but orthogonal. This spec replaces the behavioral testing architecture while preserving the OTLP collector for cost metrics.

## Impacted Domains

- lifecycle: canonical documentation of state machine, transitions, invariants
- testing: new `internal/harness` package replacing `internal/bench` for behavioral testing
- hooks: enforcement coverage matrix verified by tests

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Worktree-first branch strategy — tests validate worktree creation, bead merging, zero-on-main
- [ADR-0019](../../adr/ADR-0019.md): Three-layer enforcement — tests verify each layer independently and composed
- [ADR-0020](../../adr/ADR-0020.md): lifecycle.yaml as phase authority — tests validate phase transitions

## Requirements

1. A canonical architecture document describing every state artifact, transition, hook, and worktree topology
2. A deterministic scenario test suite exercising every lifecycle transition with state invariant assertions
3. A hook enforcement matrix test covering every (mode × tool × location) triple
4. A new behavioral test harness (`internal/harness`) with event-driven architecture capturing structured action events per turn
5. LLM behavior scenarios that assert command sequences, CWD correctness, and recovery from blocked actions
6. Turn-level analysis: count turns, classify each turn (forward progress / self-correction / error recovery / wrong action)
7. Wrong-action detection: identify when the LLM does something the lifecycle prohibits (write code in spec mode, commit to main, skip `mindspec next`)
8. Plan-outcome fidelity: compare the plan the agent wrote against the actions it actually took
9. Goal verification: run acceptance checks (tests pass, files exist, spec criteria met) after each scenario

## Scope

### In Scope

**Documentation:**
- `docs/architecture/lifecycle-state-machine.md` — canonical reference

**Deterministic tests (no LLM):**
- `internal/lifecycle/scenario_test.go` — state machine transitions
- `internal/lifecycle/hook_matrix_test.go` — enforcement coverage

**Behavioral harness (new package):**
- `internal/harness/event.go` — structured action event schema
- `internal/harness/recorder.go` — command recording shim (wraps mindspec/git/bd, logs invocations)
- `internal/harness/sandbox.go` — fresh repo builder with recording shim installed
- `internal/harness/agent.go` — `Agent` interface abstracting the coding agent (Claude Code, Copilot, Codex)
- `internal/harness/agent_claude.go` — Claude Code implementation using `claude` CLI (uses existing auth)
- `internal/harness/session.go` — session runner using Agent interface, with turn-level event capture
- `internal/harness/analyzer.go` — turn classification, wrong-action detection, plan fidelity scoring
- `internal/harness/scenario.go` — scenario definitions (happy path, interrupt, abandon, etc.)
- `internal/harness/scenario_test.go` — LLM behavior test cases
- `internal/harness/report.go` — structured report output (JSON + human-readable)

**Preserve from existing bench:**
- `internal/bench/collector.go` — OTLP telemetry collector (token/cost metrics)
- `internal/bench/report.go` — cost comparison reporting

**Deprecate from existing bench:**
- `internal/bench/runner.go` — replaced by `harness/session.go`
- `internal/bench/session.go` — replaced by `harness/session.go` + `harness/recorder.go`
- `internal/bench/qualitative.go` — replaced by `harness/analyzer.go`
- `internal/bench/worktree.go` — replaced by `harness/sandbox.go`

### Out of Scope

- Changing the state management architecture (separate spec, informed by this one)
- Multi-agent coordination testing
- Performance benchmarking (cost metrics preserved via collector, but not the focus)
- Prompt ablation framework (future iteration)

## Non-Goals

- This does not refactor state management — it documents and tests current design
- This does not add enforcement hooks — it verifies existing ones
- LLM tests are integration tests, not unit tests; they run separately and cost money

## Design

### Event Schema

Every observable action becomes a structured event:

```go
type ActionEvent struct {
    Timestamp   time.Time
    Turn        int              // which agent turn (0-indexed)
    Phase       string           // lifecycle phase at time of action
    ActionType  string           // "tool_invoke" | "tool_result" | "command" | "hook_block" | "state_change"
    ToolName    string           // "Read", "Write", "Bash", etc.
    Command     string           // for Bash: the command string
    Args        map[string]string // tool arguments
    CWD         string           // working directory
    ExitCode    int
    Blocked     bool             // was this action blocked by a hook?
    BlockReason string
    Duration    time.Duration
}
```

### Recording Shim

The sandbox installs wrapper scripts for `mindspec`, `git`, and `bd` that:
1. Log the invocation (command, args, CWD, timestamp) to a structured JSONL file
2. Execute the real command
3. Log the exit code and duration

This captures the full command history without modifying MindSpec internals or relying on OTLP.

### Turn Classification

The analyzer classifies each turn into one of:

| Classification | Meaning | Example |
|---------------|---------|---------|
| `forward` | Productive progress toward goal | Write implementation code |
| `correction` | Agent fixing its own mistake | Re-edit after syntax error |
| `recovery` | Agent recovering from hook block | cd to worktree after block |
| `wrong_action` | Action that violates lifecycle | Write code during plan mode |
| `overhead` | Necessary but non-productive | Reading files for context |

### Wrong-Action Rules

Configurable per-scenario, but defaults include:

| Rule | Condition | Severity |
|------|-----------|----------|
| `code_in_spec_mode` | Write/Edit to .go/.ts/.js file while phase=spec | error |
| `code_in_plan_mode` | Write/Edit to code file while phase=plan | error |
| `commit_to_main` | `git commit` while on main/master branch during active spec | error |
| `skip_next` | Write code without prior `mindspec next` call | error |
| `skip_complete` | Start new bead without `mindspec complete` on current | error |
| `wrong_cwd` | Command run from main repo when worktree is active | warning |
| `force_bypass` | Using `--force` or `MINDSPEC_ALLOW_MAIN=1` | warning |

### Scenario Structure

```go
type Scenario struct {
    Name        string
    Description string
    Setup       func(sandbox *Sandbox) error  // prepare repo state
    Prompt      string                        // what to tell the LLM
    Assertions  []Assertion                   // what to check after
    MaxTurns    int                           // budget
    Model       string                        // haiku / sonnet
}

type Assertion struct {
    Name    string
    Check   func(events []ActionEvent, sandbox *Sandbox) error
}
```

### Scenarios

**Happy path:**
1. `spec_to_idle` — full lifecycle: explore → spec → plan → implement → review → idle
2. `single_bead` — claim one bead, implement, complete
3. `multi_bead_deps` — implement beads respecting dependency order

**Alternative flows:**
4. `abandon_spec` — start spec, decide it's not worth it, dismiss
5. `interrupt_for_bug` — mid-implementation, surface a bug, create new bead, fix it, resume
6. `resume_after_crash` — session dies mid-bead, new session picks up

**Enforcement:**
7. `hook_blocks_code_in_spec` — agent tries to write code during spec mode, gets blocked, recovers
8. `hook_blocks_main_commit` — agent tries to commit to main, gets blocked, switches to worktree
9. `hook_blocks_stale_next` — agent tries `mindspec next` without `/clear`, gets blocked

## Acceptance Criteria

- [ ] `docs/architecture/lifecycle-state-machine.md` documents all state artifacts (fields, readers, writers), all transitions (pre/post conditions), hook enforcement matrix, worktree topology, and at least 3 alternative flow narratives
- [ ] Deterministic scenario tests cover happy path idle→explore→spec→plan→implement→review→idle with state assertions after each transition (`go test ./internal/lifecycle/ -run TestScenario_HappyPath`)
- [ ] Deterministic tests cover abandon, interrupt, and resume alternative flows
- [ ] Hook matrix test covers all (mode × tool) combinations, asserting allow/block/warn per ADR-0019
- [ ] `internal/harness` package exists with event schema, recorder, sandbox builder, session runner, analyzer, and report generator
- [ ] Recording shim captures command, args, CWD, exit code, and timestamp for every mindspec/git/bd invocation in the sandbox
- [ ] Analyzer classifies turns as forward/correction/recovery/wrong_action/overhead
- [ ] Wrong-action detection flags at least: code_in_spec_mode, commit_to_main, skip_next, skip_complete
- [ ] LLM scenario `single_bead` passes: agent calls `mindspec next`, works in correct worktree, calls `mindspec complete`, does not commit to main (`go test ./internal/harness/ -run TestLLM_SingleBead`)
- [ ] LLM scenario `hook_blocks_code_in_spec` passes: agent gets blocked, recovers, produces spec not code
- [ ] Report output includes per-turn breakdown with classification, total wrong-actions, plan-fidelity score
- [ ] `make test` runs deterministic tests; `make bench-llm` runs LLM tests (skipped when `claude` CLI not available)
- [ ] Existing `internal/bench/collector.go` and `internal/bench/report.go` preserved and functional
- [ ] All existing tests pass (`make test` green)

## Validation Proofs

- `make test`: deterministic lifecycle and hook matrix tests pass
- `make bench-llm`: LLM behavior scenarios pass against Claude Haiku
- `go test ./internal/lifecycle/ -v`: all scenario and hook matrix tests green
- `go test ./internal/harness/ -v -run TestLLM`: behavioral tests green with turn analysis output

## Open Questions

- [x] Scrap or evolve existing bench? **Decision: Preserve collector.go and report.go for cost metrics. Replace runner/session/qualitative/worktree with new harness package.**
- [x] Should LLM tests use Haiku or Sonnet? **Decision: Haiku default (CI cost). `BENCH_MODEL=sonnet` env var for realistic runs.**
- [x] How to capture command history? **Decision: Recording shim scripts installed in sandbox PATH that log to JSONL before delegating to real binaries.**
- [x] How to measure plan-outcome fidelity? **Decision: Compare plan.md bead sections against recorder JSONL — did the agent touch the files/commands listed in each bead's steps?**

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-28
- **Notes**: Approved via mindspec approve spec