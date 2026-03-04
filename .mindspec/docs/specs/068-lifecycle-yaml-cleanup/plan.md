---
approved_at: "2026-03-04T11:39:09Z"
approved_by: user
spec_id: 068-lifecycle-yaml-cleanup
status: Approved
version: "1"
---
# Plan: Remove Dead lifecycle.yaml References

## ADR Fitness

- ADR-0023: This cleanup completes the migration ADR-0023 started

## Testing Strategy

Existing tests verify no regressions. `go test ./... -short` must pass.

## Bead 1: Remove lifecycle.yaml writes and stale references

**Steps**
1. Remove 16 dead `WriteFile(...lifecycle.yaml...)` calls from `scenario.go`
2. Remove `WriteLifecycle` no-op from `sandbox.go`
3. Update stale comments/strings in instruct.go, next.go, bead.go, complete.go, derive.go, resolve.go, validate/spec.go, validate/plan.go, next/beads.go
4. Remove `workspace.LifecyclePath()` and its test if no runtime callers
5. Update TESTING.md stale references

**Verification**
- [ ] `make build` succeeds
- [ ] `go test ./... -short` passes
- [ ] No stale lifecycle.yaml references remain in non-doc, non-detection code

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| No lifecycle.yaml writes in scenario.go | Bead 1, grep verification |
| Build + tests pass | Bead 1 verification |
