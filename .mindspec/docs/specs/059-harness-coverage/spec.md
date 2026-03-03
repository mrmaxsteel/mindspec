---
approved_at: "2026-03-03T14:26:45Z"
approved_by: user
status: Approved
---
# Spec 059-harness-coverage: LLM Test Harness Coverage Gaps

## Goal

Close the identified gaps in the LLM test harness so that every state transition, artifact convention, and guard rail documented in WORKFLOW-STATE-MACHINE.md has at least one assertion exercising it. The harness should catch regressions in merge topology, beads lifecycle integration, focus state integrity, commit conventions, and invalid transition rejection — all areas currently untested.

## Background

A coverage audit (2026-03-03, documented in `internal/harness/TESTING.md § Coverage Analysis`) compared the 16 existing LLM test scenarios against WORKFLOW-STATE-MACHINE.md. Findings:

- **7/7 valid state transitions** are covered by at least one scenario, but the `implement → plan` (blocked-beads) path has zero coverage.
- **0/6 invalid transitions** are tested for rejection.
- **Beads lifecycle** (creation by `plan approve`, claiming by `next`, closing by `complete`) is never directly asserted — tests rely on `mindspec complete` succeeding as a proxy.
- **Merge topology** (bead→spec→main) is never verified — tests check branch existence but not merge commit structure.
- **Focus state** assertions check `mode` only, ignoring `activeSpec`, `activeBead`, `activeWorktree`, `specBranch`.
- **Commit message convention** (`impl(beadID): message`) is never checked.
- **Analyzer rules** `skip_next` and `skip_complete` are stubs (the two most common agent errors in practice).

## Impacted Domains

- `internal/harness/` — scenario definitions, analyzer rules, assertion helpers

## ADR Touchpoints

None — this work adds test coverage only, no architectural changes.

## Requirements

1. Implement `skip_next` analyzer rule: detect when agent writes code files without having run `mindspec next` first in the session.
2. Implement `skip_complete` analyzer rule: detect when agent finishes coding (session ends or next lifecycle command runs) without having run `mindspec complete`.
3. Add assertion helper `AssertBeadsState(sandbox, specEpicID)` that calls `bd list --json --parent <epicID>` and returns structured bead status, enabling scenarios to verify bead creation, claiming, and closure.
4. Add assertion helper `AssertMergeTopology(sandbox, specBranch)` that checks `git log --merges --oneline` on the spec branch after `complete` and verifies at least one merge commit from a `bead/` branch exists.
5. Add assertion helper `AssertFocusFields(sandbox, expected)` that reads `.mindspec/focus` and asserts all fields (mode, activeSpec, activeBead, activeWorktree, specBranch) — not just mode.
6. Add assertion `AssertCommitMessage(sandbox, pattern)` that greps `git log --oneline` for the expected `impl(<beadID>):` prefix after `complete`.
7. Add deterministic (non-LLM) test `TestInvalidTransitions` that verifies `spec approve` fails from idle, `plan approve` fails from spec, `impl approve` fails from implement, and backward transitions are rejected.
8. Add LLM scenario `BlockedBeadTransition` that sets up 2 beads where bead-2 depends on bead-1, has the agent complete bead-1, and asserts focus transitions to `plan` (not `implement`) because the only remaining bead is blocked.
9. Wire new assertion helpers into existing scenarios: add beads state checks to PlanApprove and SingleBead, merge topology checks to SingleBead and SpecToIdle, focus field depth to SpecInit/SpecApprove/PlanApprove/ImplApprove, commit message checks to SingleBead.
10. Update `internal/harness/TESTING.md` coverage analysis section to reflect closed gaps.

## Scope

### In Scope

- `internal/harness/analyzer.go` — implement `skip_next`, `skip_complete` rules
- `internal/harness/scenario.go` — new `ScenarioBlockedBeadTransition`, new assertion helpers
- `internal/harness/scenario_test.go` — new `TestLLM_BlockedBeadTransition`, new `TestInvalidTransitions`
- `internal/harness/assertions.go` (new) — reusable assertion helpers (beads state, merge topology, focus fields, commit message)
- `internal/harness/TESTING.md` — update coverage analysis

### Out of Scope

- Enforcement hook testing (requires `agent_hooks: true` sandbox mode — separate spec)
- New LLM scenarios beyond `BlockedBeadTransition` (existing scenarios get richer assertions instead)
- Changes to mindspec CLI commands (this spec tests existing behavior, not new behavior)

## Non-Goals

- 100% path coverage of every CLI error branch — focus is on state machine and convention coverage
- Performance benchmarking of test scenarios
- Replacing LLM tests with deterministic tests (LLM tests verify real agent behavior)

## Acceptance Criteria

- [ ] `go test ./internal/harness/ -short -v` passes with new deterministic tests (invalid transitions)
- [ ] `skip_next` rule fires when synthetic event stream has code edits before any `mindspec next`
- [ ] `skip_complete` rule fires when synthetic event stream has `mindspec next` and code edits but no `mindspec complete`
- [ ] `AssertBeadsState` correctly reads bead status from sandbox after `plan approve` (beads exist) and after `complete` (bead closed)
- [ ] `AssertMergeTopology` detects merge commits on spec branch after bead→spec merge
- [ ] `AssertFocusFields` catches focus field mismatches (not just mode)
- [ ] `TestLLM_BlockedBeadTransition` passes: focus is `plan` after completing bead-1 with bead-2 still blocked
- [ ] Existing LLM tests still pass with added assertions (no regressions)
- [ ] TESTING.md coverage analysis updated to show closed gaps

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-03
- **Notes**: Approved via mindspec approve spec