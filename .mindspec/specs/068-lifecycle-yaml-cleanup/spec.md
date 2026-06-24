---
approved_at: "2026-03-04T11:38:35Z"
approved_by: user
status: Approved
---
# Remove Dead lifecycle.yaml References

## Goal

Remove all dead `lifecycle.yaml` writes and stale comments from the codebase. ADR-0023 eliminated lifecycle.yaml — state is derived from beads. But 16 `WriteFile` calls in `scenario.go` still write this dead file, and comments across the codebase reference it as if active.

## Background

ADR-0023 replaced file-based lifecycle state with beads-derived state. Runtime code already queries beads, but test scaffolding and comments were never cleaned up.

## Impacted Domains

- harness: Remove dead lifecycle.yaml writes from scenario setup
- workspace: Remove `LifecyclePath()` helper if unused at runtime

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): Eliminated lifecycle.yaml in favor of beads-derived state

## Requirements

1. Remove all `sandbox.WriteFile(...lifecycle.yaml...)` calls in scenario.go
2. Update stale comments referencing lifecycle.yaml across code files
3. Keep doctor/validate detection code (useful for brownfield repos)
4. Keep ADR/spec docs unchanged (historical)

## Scope

### In Scope
- `internal/harness/scenario.go` — 16 dead WriteFile calls
- `internal/harness/sandbox.go` — dead WriteLifecycle no-op comment
- `internal/harness/TESTING.md` — stale references
- `cmd/mindspec/instruct.go` — stale Long description
- `cmd/mindspec/next.go` — stale comment
- `cmd/mindspec/bead.go` — stale deprecation messages
- `internal/complete/complete.go` — stale comments
- `internal/phase/derive.go` — stale package comment
- `internal/resolve/resolve.go` — stale comments
- `internal/validate/spec.go`, `plan.go` — stale comments
- `internal/next/beads.go` — stale comment
- `internal/workspace/workspace.go` — `LifecyclePath()` if unused

### Out of Scope
- ADR and spec documentation (historical)
- Doctor/validate stale-detection code (still useful)

## Non-Goals

- Changing any runtime behavior
- Modifying test assertions that verify lifecycle.yaml is NOT created

## Acceptance Criteria

- [ ] `grep -r 'lifecycle\.yaml' internal/harness/scenario.go` returns no results
- [ ] `make build` succeeds
- [ ] `go test ./internal/harness/ -short -v` passes
- [ ] `go test ./... -short` passes

## Validation Proofs

- `grep -rn 'lifecycle.yaml' --include='*.go' | grep -v '_test.go' | grep -v docs/ | grep -v doctor/ | grep -v validate/'`: Should show no remaining stale references
- `make build && go test ./... -short`: All pass

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-04
- **Notes**: Approved via mindspec approve spec