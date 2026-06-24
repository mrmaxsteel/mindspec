---
adr_citations:
    - id: ADR-0006
      sections:
        - Bead 1
        - Bead 3
        - Bead 5
    - id: ADR-0019
      sections:
        - Bead 1
        - Bead 4
        - Bead 5
    - id: ADR-0020
      sections:
        - Bead 1
        - Bead 3
approved_at: "2026-02-28T17:26:47Z"
approved_by: user
bead_ids:
    - mindspec-eo0u.1
    - mindspec-eo0u.2
    - mindspec-eo0u.3
    - mindspec-eo0u.4
    - mindspec-eo0u.5
    - mindspec-eo0u.6
last_updated: "2026-02-28"
spec_id: 055-lifecycle-test-suite
status: Approved
version: "1"
---

# Plan: 055-lifecycle-test-suite

## ADR Fitness

**ADR-0006 (Protected main, worktree-first)**: Sound. The test suite validates exactly the invariants this ADR establishes — zero-on-main, bead→spec branch merging, single PR per lifecycle. No divergence needed.

**ADR-0019 (Three-layer enforcement)**: Sound. The hook matrix test (Bead 4) is a direct verification of this ADR's claims. One observation: ADR-0019 references `state.json` but the codebase uses `.mindspec/focus` — the ADR text is stale but the architecture is correct. Not worth superseding for a naming inconsistency.

**ADR-0020 (lifecycle.yaml as phase authority)**: Sound. The deterministic scenario tests (Bead 3) validate that lifecycle.yaml phase transitions are the source of truth, not focus. This ADR directly informs what state invariants the tests must assert.

No ADR divergence proposed.

## Testing Strategy

Three test tiers, each independently runnable:

1. **Unit tests** (`internal/harness/*_test.go`): Test event parsing, turn classification, wrong-action rule evaluation against static event logs. No git, no LLM, fast. Run via `make test`.

2. **Deterministic scenario tests** (`internal/lifecycle/*_test.go`): Create real git repos in `t.TempDir()`, run actual `mindspec` Go functions (not CLI), assert state after each transition. No LLM. Run via `make test`.

3. **LLM behavior tests** (`internal/harness/scenario_test.go`): Create sandbox repos, run a coding agent via the `Agent` interface, assert command history via recording shim. Expensive, slow. Run via `make bench-llm`. The coding agent is abstracted behind `internal/harness/agent.go` so Claude Code, Copilot, and Codex can be swapped without changing scenarios or assertions.

Shared infrastructure: `internal/harness/sandbox.go` (repo builder) and `internal/harness/agent.go` (coding agent abstraction) used by tiers 2 and 3.

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| Architecture doc with all state/transitions/hooks | Bead 1 |
| Happy path deterministic test | Bead 3 |
| Alternative flow deterministic tests (abandon, interrupt, resume) | Bead 3 |
| Hook enforcement matrix | Bead 4 |
| `internal/harness` package with event/recorder/sandbox/agent/session/analyzer/report | Beads 2, 5, 6 |
| Recording shim captures commands | Bead 2 |
| Turn classification (forward/correction/recovery/wrong_action/overhead) | Bead 5 |
| Wrong-action detection | Bead 5 |
| LLM scenario single_bead passes | Bead 6 |
| LLM scenario hook_blocks_code_in_spec passes | Bead 6 |
| Report with per-turn breakdown | Bead 5 |
| `make test` / `make bench-llm` targets | Bead 6 |
| Existing bench collector/report preserved | Bead 2 |
| All existing tests pass | Bead 6 |

---

## Bead 1: Architecture Document

Write the canonical lifecycle state machine reference document.

**Steps**

1. Create `docs/architecture/lifecycle-state-machine.md` (or `.mindspec/docs/architecture/` if canonical path preferred)
2. Document every state artifact: focus (fields, readers, writers, when it changes), session.json, lifecycle.yaml, config.yaml, spec.md frontmatter, plan.md frontmatter
3. Document every lifecycle transition (explore→spec→plan→implement→review→idle) with pre-conditions, post-conditions, state changes, git operations, and commands
4. Document the hook enforcement matrix: for each (mode × tool × action-type), specify expected outcome (allow/block/warn) and which hook handles it
5. Document worktree topology: main repo → spec worktrees → bead worktrees, branch naming, merge direction
6. Write alternative flow narratives: abandon mid-spec, interrupt for bug fix, resume after crash, multi-bead dependency chains
7. Add state invariant rules: "lifecycle.yaml phase and focus mode must agree", "activeWorktree path must exist on disk", "activeBead must be in_progress in beads"

**Verification**

- [ ] `docs/architecture/lifecycle-state-machine.md` exists and has sections for: State Artifacts, Transitions, Hook Enforcement Matrix, Worktree Topology, Alternative Flows, Invariants
- [ ] Every state artifact from Spec 055 background section is documented with fields/readers/writers
- [ ] Hook matrix covers all 6 modes × 3 tool types (Write/Edit/Bash) = 18 cells minimum
- [ ] `make test` still passes (no code changes in this bead)

**Depends on**

None

## Bead 2: Harness Foundation — Event Schema, Recording Shim, Sandbox, Agent Interface

Build the core infrastructure for the behavioral test harness, including the coding agent abstraction.

**Steps**

1. Create `internal/harness/event.go` — `ActionEvent` struct with Timestamp, Turn, Phase, ActionType, ToolName, Command, Args, CWD, ExitCode, Blocked, BlockReason, Duration. Add `ParseEvents(path string) ([]ActionEvent, error)` to read JSONL logs. Add `EventLog` type for append/query operations.
2. Create `internal/harness/recorder.go` — functions to generate shim shell scripts for `mindspec`, `git`, `bd` that log to JSONL before delegating. `InstallShims(binDir, logPath string) error` writes the scripts; `ShimEnv(binDir string) []string` returns PATH-prepended env vars.
3. Create `internal/harness/sandbox.go` — `Sandbox` struct holding repo path, shim bin dir, log path. `NewSandbox(t *testing.T) *Sandbox` creates a fresh git repo with `.mindspec/`, config.yaml, initial commit, and recording shims installed. `Sandbox.Run(args ...string) (string, error)` executes mindspec commands in the sandbox. `Sandbox.ReadEvents() []ActionEvent` reads the log.
4. Create `internal/harness/agent.go` — `Agent` interface abstracting the coding agent:
   ```go
   type Agent interface {
       // Run executes an agent session in the sandbox with the given prompt.
       // Returns when the agent finishes or maxTurns is reached.
       Run(ctx context.Context, sandbox *Sandbox, prompt string, opts RunOpts) (*SessionResult, error)
       // Name returns the agent identifier (e.g. "claude-code", "copilot", "codex").
       Name() string
   }

   type RunOpts struct {
       MaxTurns int
       Model    string   // model override (e.g. "haiku", "sonnet")
       Env      []string // additional env vars
   }
   ```
   Implement `ClaudeCodeAgent` as the first concrete implementation. It shells out to `claude --print` (or `claude -p`) which uses the user's existing Claude Code auth — no separate `ANTHROPIC_API_KEY` needed. The agent runs in the sandbox directory with recording shims in PATH. Agent selection via `BENCH_AGENT` env var (default: `claude-code`).
5. Write unit tests: `event_test.go` (JSONL parsing round-trip), `recorder_test.go` (shim script generation and invocation), `sandbox_test.go` (repo creation with correct structure), `agent_test.go` (interface contract test with a mock agent).
6. Verify existing `internal/bench/collector.go` and `internal/bench/report.go` still compile and their tests pass (no modifications to these files).

**Verification**

- [ ] `go test ./internal/harness/ -run TestEvent` passes — JSONL round-trip works
- [ ] `go test ./internal/harness/ -run TestRecorder` passes — shim captures command, args, CWD, exit code
- [ ] `go test ./internal/harness/ -run TestSandbox` passes — sandbox has .mindspec/, .git/, shims in PATH
- [ ] `go test ./internal/harness/ -run TestAgent` passes — mock agent satisfies interface, ClaudeCodeAgent constructs correct CLI invocation
- [ ] `go test ./internal/bench/...` passes — existing bench tests unaffected
- [ ] `make test` passes

**Depends on**

None

## Bead 3: Deterministic Lifecycle Scenario Tests

Test every lifecycle transition without an LLM by calling Go functions directly.

**Steps**

1. Create `internal/lifecycle/scenario_test.go` with `TestScenario_HappyPath` — calls explore.Enter, specinit.Run, approve.ApproveSpec, approve.ApprovePlan, next.Run (mocked beads), complete.Run, approve.ApproveImpl in sequence. After each call, assert: focus mode, lifecycle.yaml phase, worktree existence, branch existence.
2. Add `TestScenario_Abandon` — explore.Enter → explore.Dismiss. Assert: mode=idle, no worktree, optional ADR created.
3. Add `TestScenario_InterruptForBug` — implement bead 1, create bug bead mid-flight, complete bug bead, resume bead 1. Assert: both beads closed, spec branch has both sets of changes.
4. Add `TestScenario_ResumeAfterCrash` — set up state as if session died (focus has activeBead, bead is in_progress, worktree exists with partial work). Call next.Run with appropriate state. Assert: agent picks up existing bead, doesn't create duplicate worktree.
5. Add helper `assertState(t, root, expectedMode, expectedPhase, expectedSpec)` that reads both focus and lifecycle.yaml and fails if they disagree.
6. Each test creates a real git repo via `t.TempDir()` with `.mindspec/`, config, and initial commit. Beads operations are mocked via the existing `planRunBDFn` / `planRunBDCombinedFn` test seams.

**Verification**

- [ ] `go test ./internal/lifecycle/ -run TestScenario_HappyPath` passes with state assertions at every transition
- [ ] `go test ./internal/lifecycle/ -run TestScenario_Abandon` passes
- [ ] `go test ./internal/lifecycle/ -run TestScenario_Interrupt` passes
- [ ] `go test ./internal/lifecycle/ -run TestScenario_Resume` passes
- [ ] `make test` passes

**Depends on**

None

## Bead 4: Hook Enforcement Matrix Test

Systematically test every (mode × tool × location) enforcement combination.

**Steps**

1. Create `internal/lifecycle/hook_matrix_test.go` with table-driven tests.
2. Define the matrix: 6 modes (idle, explore, spec, plan, implement, review) × 3 tool types (Write code file, Write doc file, Bash command) × 2 locations (main CWD, worktree CWD) = 36 test cases.
3. For each case, construct a `hook.HookState` with the mode, activeWorktree (if applicable), and call `hook.Run(hookName, input, state, enforce=true)`. Assert the result is `Pass`, `Warn`, or `Block` per ADR-0019.
4. Add edge cases: enforcement disabled (config flag), no focus file (nil state), stale worktree path (directory doesn't exist).
5. Build the expected-outcome matrix as a Go map for readability and maintainability.

**Verification**

- [ ] `go test ./internal/lifecycle/ -run TestHookMatrix` passes — all 36+ cases assert correct outcome
- [ ] Edge cases (enforcement disabled, nil state, stale path) covered
- [ ] Matrix matches ADR-0019 specification
- [ ] `make test` passes

**Depends on**

None

## Bead 5: Analyzer — Turn Classification, Wrong-Action Detection, Plan Fidelity

Build the behavioral analysis layer that interprets recorded events.

**Steps**

1. Create `internal/harness/analyzer.go` — `Analyzer` struct. `Classify(events []ActionEvent) []TurnSummary` groups events by turn and classifies each as forward/correction/recovery/wrong_action/overhead.
2. Implement classification heuristics: `wrong_action` = event matches a WrongActionRule and was NOT blocked by a hook (blocked actions are `recovery`); `correction` = Write/Edit to same file within 2 turns of a failed tool result; `overhead` = Read/Glob/Grep without subsequent Write; `forward` = everything else productive.
3. Create `WrongActionRule` type and default rule set: code_in_spec_mode, code_in_plan_mode, commit_to_main, skip_next, skip_complete, wrong_cwd, force_bypass. Each rule is a `func(ActionEvent) (bool, string)` returning (violated, reason).
4. Implement `PlanFidelity(planPath string, events []ActionEvent) float64` — parse plan.md bead sections, extract file paths and commands mentioned in steps, check what fraction the agent actually touched. Score 0.0–1.0.
5. Create `internal/harness/report.go` — `Report` struct with TurnSummaries, WrongActions, PlanFidelityScore, TotalTurns, TurnsByClassification. `FormatText()` for human-readable output, `FormatJSON()` for machine consumption.
6. Write unit tests against static event logs: `analyzer_test.go` with known-good and known-bad event sequences.

**Verification**

- [ ] `go test ./internal/harness/ -run TestAnalyzer_Classify` passes — correct classification for forward, correction, recovery, wrong_action, overhead
- [ ] `go test ./internal/harness/ -run TestAnalyzer_WrongAction` passes — each default rule fires on matching events, doesn't fire on non-matching
- [ ] `go test ./internal/harness/ -run TestAnalyzer_PlanFidelity` passes — returns ~1.0 for agent that followed plan, <0.5 for agent that improvised
- [ ] `go test ./internal/harness/ -run TestReport` passes — JSON and text output include all fields
- [ ] `make test` passes

**Depends on**

Bead 2

## Bead 6: Session Runner and Behavior Scenarios

Wire the Agent interface into scenario execution and implement all 9 behavior scenarios.

**Steps**

1. Create `internal/harness/session.go` — `RunSession(ctx context.Context, agent Agent, scenario Scenario, sandbox *Sandbox) (*SessionResult, error)`. Calls `agent.Run()` with the scenario prompt, collects events from the recording shim log, respects MaxTurns via RunOpts. `SessionResult` includes raw events, agent output, and timing.
2. Create `internal/harness/scenario.go` — `Scenario` struct with Name, Description, Setup, Prompt, Assertions, MaxTurns, Model. Implement all 9 scenarios with their Setup functions (prepare sandbox state) and Assertion functions (check events + sandbox).
3. Implement happy path scenarios: `spec_to_idle` (full lifecycle, 50+ turn budget), `single_bead` (pre-approved plan, 15 turn budget), `multi_bead_deps` (3 beads with deps, 30 turn budget).
4. Implement alternative flow scenarios: `abandon_spec` (explore + dismiss, 10 turn budget), `interrupt_for_bug` (mid-bead interrupt, 25 turn budget), `resume_after_crash` (pick up existing state, 15 turn budget).
5. Implement enforcement scenarios: `hook_blocks_code_in_spec` (write code attempt gets blocked, 10 turns), `hook_blocks_main_commit` (CWD enforcement, 10 turns), `hook_blocks_stale_next` (freshness gate, 5 turns).
6. Create `internal/harness/scenario_test.go` — `TestLLM_*` functions. Skip gate: test checks `claude --version` succeeds (i.e. Claude Code is installed and authed) rather than checking for an API key env var. Each test creates a sandbox, resolves the agent via `BENCH_AGENT` env var (default: `claude-code`), runs the scenario, runs the analyzer, asserts zero wrong-actions (or expected wrong-actions for enforcement tests) and all scenario-specific assertions pass.
7. Add `bench-llm` target to Makefile: `go test ./internal/harness/ -v -run TestLLM -timeout 600s`.

**Verification**

- [ ] `go test ./internal/harness/ -run TestLLM_SingleBead` passes with Claude Code (Haiku model) — agent calls mindspec next, works in worktree, calls mindspec complete
- [ ] `go test ./internal/harness/ -run TestLLM_HookBlocksCodeInSpec` passes — agent gets blocked, recovers, no code files created
- [ ] `go test ./internal/harness/ -run TestLLM_AbandonSpec` passes — agent dismisses cleanly, mode=idle
- [ ] `make bench-llm` runs all LLM scenarios and produces per-turn report
- [ ] `make test` passes (LLM tests skipped when `claude` CLI not available)
- [ ] All existing tests unaffected
- [ ] Swapping `BENCH_AGENT=copilot` or `BENCH_AGENT=codex` compiles (implementations can be stubs initially, but the interface is proven by ClaudeCodeAgent)

**Depends on**

Bead 2, Bead 5
