---
approved_at: "2026-03-03T15:36:45Z"
approved_by: user
bead_ids:
    - mindspec-9dx0.1
    - mindspec-9dx0.2
    - mindspec-9dx0.3
    - mindspec-9dx0.4
    - mindspec-9dx0.5
    - mindspec-9dx0.6
last_updated: "2026-03-03"
spec_id: 059-harness-coverage
status: Approved
version: 1
---

# Plan: LLM Test Harness Coverage Gaps

## ADR Fitness

No ADRs are directly relevant to this work. The harness was introduced in Spec 055 without accompanying ADRs — it's purely internal test infrastructure. No architectural decisions need revisiting.

## Testing Strategy

- **Unit/deterministic tests**: All new assertion helpers get deterministic tests in `scenario_test.go` (synthetic event streams, mock sandbox state). Invalid transition tests are purely deterministic (no LLM).
- **LLM tests**: One new LLM scenario (`BlockedBeadTransition`). Existing LLM scenarios get enriched assertions but their prompts don't change.
- **Validation**: `go test ./internal/harness/ -short -v` for deterministic; full LLM suite for regression.

## Bead 1: Assertion Helpers

Add reusable assertion helpers to `internal/harness/scenario.go` (alongside existing assertion helpers — no new file needed since the existing pattern puts helpers in the same file).

**Steps**
1. Add `assertFocusFields(t, sandbox, expected map[string]string)` — reads `.mindspec/focus`, unmarshals JSON, asserts each key/value pair in `expected` matches. Existing `assertFocusMode` becomes a thin wrapper.
2. Add `assertBeadsState(t, sandbox, epicID string, expectedStatuses map[string]string)` — runs `bd list --json --parent <epicID>` via sandbox helper, parses JSON, asserts each bead's status matches.
3. Add `assertMergeTopology(t, sandbox, specBranch string)` — runs `git log --merges --oneline <specBranch>` via sandbox, asserts at least one merge commit from a `bead/` branch exists.
4. Add `assertCommitMessage(t, sandbox, pattern string)` — runs `git log --oneline` on current branch, asserts at least one commit matches the regex pattern.
5. Add deterministic tests for each helper in `scenario_test.go` using synthetic data.

**Verification**
- [ ] `go test ./internal/harness/ -short -v -run TestAssertFocusFields` passes
- [ ] `go test ./internal/harness/ -short -v -run TestAssertBeadsState` passes
- [ ] `go test ./internal/harness/ -short -v -run TestAssertMergeTopology` passes
- [ ] `go test ./internal/harness/ -short -v -run TestAssertCommitMessage` passes

**Depends on**
None

## Bead 2: Analyzer Rules (skip_next, skip_complete)

Implement the two stub analyzer rules that detect the most common agent errors.

**Steps**
1. Refactor `skip_next` and `skip_complete` from per-event stubs to session-level checks. Add a post-classification pass in `Classify()` that scans the full event stream for these patterns.
2. `skip_next`: scan events for any code-modifying action (Write/Edit tool on a code path, or `git add`/`git commit`) that occurs before any `mindspec next` event. Flag as wrong action.
3. `skip_complete`: scan events for `mindspec next` followed by code-modifying actions but no `mindspec complete` before the session ends or the next lifecycle command (`mindspec approve`, `mindspec next`).
4. Add deterministic tests with synthetic event streams: (a) stream with next→code→complete = no violation, (b) stream with code before next = skip_next fires, (c) stream with next→code but no complete = skip_complete fires.

**Verification**
- [ ] `go test ./internal/harness/ -short -v -run TestSkipNext` passes
- [ ] `go test ./internal/harness/ -short -v -run TestSkipComplete` passes

**Depends on**
None

## Bead 3: Invalid Transition Tests

Add deterministic tests verifying the state machine rejects invalid transitions.

**Steps**
1. Create `TestInvalidTransitions` in `scenario_test.go`.
2. For each invalid transition (idle→plan, idle→implement, spec→implement, plan→review, review→implement, review→plan): set up sandbox with focus in the source mode, run the command that would attempt the transition, assert non-zero exit code and appropriate error message.
3. These are deterministic — use `exec.Command` to run `mindspec` binary directly in the sandbox, no LLM needed.

**Verification**
- [ ] `go test ./internal/harness/ -short -v -run TestInvalidTransitions` passes
- [ ] Each invalid transition produces a non-zero exit and error mentioning expected mode

**Depends on**
None

## Bead 4: Wire Assertions into Existing Scenarios

Enrich existing LLM scenarios with the new assertion helpers (no prompt changes).

**Steps**
1. `ScenarioImplApprove`: add `assertFocusFields` (verify activeSpec="" after idle transition).
2. `ScenarioPlanApprove`: add `assertBeadsState` (verify beads created after plan approve).
3. `ScenarioSingleBead`: add `assertCommitMessage` for `impl(` prefix, add `assertMergeTopology` for bead→spec merge.
4. `ScenarioSpecInit`, `ScenarioSpecApprove`: add `assertFocusFields` for specBranch, activeSpec.
5. `ScenarioSpecToIdle`: add `assertMergeTopology` if spec branch has merge commits before cleanup.
6. Run deterministic tests to verify no compilation errors.

**Verification**
- [ ] `go test ./internal/harness/ -short -v` passes (all deterministic tests)
- [ ] `make test` passes (full suite)

**Depends on**
Bead 1

## Bead 5: BlockedBeadTransition Scenario

Add LLM scenario testing the implement→plan transition when only blocked beads remain.

**Steps**
1. Define `ScenarioBlockedBeadTransition()` in `scenario.go`: setup with 2 beads where bead-2 depends on bead-1, agent starts in implement mode with bead-1 claimed.
2. Prompt: implement bead-1 (simple file creation), run `mindspec complete`.
3. Assertions: focus mode is `plan` (not implement, since bead-2 is blocked), bead-1 is closed, bead-2 is still open.
4. Register in `AllScenarios()` and add `TestLLM_BlockedBeadTransition` in `scenario_test.go`.

**Verification**
- [ ] `go test ./internal/harness/ -short -v` passes (deterministic portion)
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_BlockedBeadTransition -timeout 10m -count=1` passes

**Depends on**
Bead 1 (uses assertFocusFields, assertBeadsState)

## Bead 6: Update TESTING.md Coverage Analysis

Update the coverage analysis section to reflect all gaps closed by this spec.

**Steps**
1. Update the State Transition Coverage table: mark implement→plan as covered.
2. Update the Assertion Depth gaps table: mark each closed gap.
3. Add a note about invalid transition deterministic coverage.
4. Update the Recommendations section to mark completed items.

**Verification**
- [ ] `go test ./internal/harness/ -short -v` still passes
- [ ] Coverage table reflects actual test state

**Depends on**
Bead 1, Bead 2, Bead 3, Bead 4, Bead 5

## Provenance

| Acceptance Criterion | Bead(s) |
|:---------------------|:--------|
| `skip_next` rule fires on synthetic events | Bead 2 |
| `skip_complete` rule fires on synthetic events | Bead 2 |
| `AssertBeadsState` reads bead status correctly | Bead 1, Bead 4 |
| `AssertMergeTopology` detects merge commits | Bead 1, Bead 4 |
| `AssertFocusFields` catches field mismatches | Bead 1, Bead 4 |
| `TestLLM_BlockedBeadTransition` passes | Bead 5 |
| Existing LLM tests still pass | Bead 4 |
| TESTING.md updated | Bead 6 |
| `TestInvalidTransitions` passes | Bead 3 |
