---
approved_at: "2026-03-04T00:17:54Z"
approved_by: user
bead_ids:
    - mindspec-7yzq.1
spec_id: 067-lifecycle-test-epic-leak
status: Approved
version: "1"
---
# Plan: 067-lifecycle-test-epic-leak

## ADR Fitness

No ADRs are relevant. This is a test isolation bug fix — the existing package-var stubbing pattern (used by `approve/plan_test.go`) is the established convention.

## Testing Strategy

Unit tests: run `go test ./internal/lifecycle/ -v` and verify no new beads are created. Also run `make test` for full suite.

## Bead 1: Export test setters and stub BD functions in lifecycle tests

**Steps**
1. In `internal/approve/spec.go`, add `SetSpecRunBDForTest(fn) func()` that swaps `specRunBDFn` and returns a restore function
2. In `internal/approve/plan.go`, add `SetPlanRunBDForTest(fn) func()` and `SetPlanRunBDCombinedForTest(fn) func()` with the same pattern
3. In `internal/lifecycle/scenario_test.go`, create a `stubApproveBeads(t)` helper that calls all three setters with no-op functions (returning empty JSON `{"id":"fake-xxx"}` for create calls)
4. Call `stubApproveBeads(t)` in every test function that invokes `ApproveSpec` or `ApprovePlan`
5. Run `make test` to verify all tests pass and no new beads are created

**Verification**
- [ ] `make test` passes
- [ ] `bd list --status=open` count does not increase after `go test ./internal/lifecycle/ -count=3`

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| `make test` passes without creating new issues | Bead 1: `make test` passes |
| `bd list` count stable after lifecycle tests | Bead 1: `bd list` count check |
| Exported setters follow `SetXxxForTest` convention | Bead 1: step 1-2 implementation |
