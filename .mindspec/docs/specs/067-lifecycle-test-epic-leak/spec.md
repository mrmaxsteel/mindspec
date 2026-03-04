---
approved_at: "2026-03-04T00:17:18Z"
approved_by: user
status: Approved
---
# Spec 067: Fix lifecycle tests leaking epics into shared beads DB

## Goal

Prevent `internal/lifecycle/scenario_test.go` from creating real beads epics/tasks in the shared `.beads` database on every test run.

## Background

`scenario_test.go` calls `approve.ApproveSpec()` and `approve.ApprovePlan()` directly. These functions invoke real `bd create` commands via `specRunBDFn` and `planRunBDFn`, creating lifecycle epics and implementation beads in the shared `.beads` database.

The existing `stubNoEpics()` only stubs `phase.runBDFn` (the collision check), but doesn't stub the `approve` package's own BD functions — because they're unexported package-level vars in a different package.

Each test run creates ~5 stale epics (001-test-feature, 002-main-feature, 003-hotfix-bug, 005-artifact-check, 006-plan-artifact). Over time this accumulated 49 orphaned epics that polluted `mindspec next` output.

## Impacted Domains

- approve: needs exported test setters for `specRunBDFn` and `planRunBDFn`
- lifecycle: `scenario_test.go` needs to call the new setters

## ADR Touchpoints

None — this is a test isolation bug fix.

## Requirements

1. `internal/approve` MUST export setter functions for `specRunBDFn` and `planRunBDFn` (and `planRunBDCombinedFn` if used by tests)
2. `internal/lifecycle/scenario_test.go` MUST stub all BD functions before calling `ApproveSpec`/`ApprovePlan`
3. Running `make test` MUST NOT create any issues in the shared `.beads` database

## Scope

### In Scope
- `internal/approve/spec.go` — add `SetSpecRunBDForTest()`
- `internal/approve/plan.go` — add `SetPlanRunBDForTest()`
- `internal/lifecycle/scenario_test.go` — call the new setters

### Out of Scope
- Other test files that already stub BD functions internally (e.g., `approve/plan_test.go`)
- LLM harness tests (they use sandboxed beads)

## Non-Goals

- Replacing the package-var stubbing pattern with a different DI approach

## Acceptance Criteria

- [ ] `make test` passes without creating any new issues in `.beads`
- [ ] `bd list --status=open` count does not increase after `go test ./internal/lifecycle/`
- [ ] Exported setter functions follow the `SetXxxForTest` convention with cleanup restore

## Validation Proofs

- `bd list --status=open --json | python3 -c "import json,sys; print(len(json.load(sys.stdin)))"` before and after `go test ./internal/lifecycle/ -count=3`: count stays the same
- `make test`: all tests pass

## Open Questions

None.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-04
- **Notes**: Approved via mindspec approve spec