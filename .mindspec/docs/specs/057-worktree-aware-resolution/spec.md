---
approved_at: "2026-03-01T09:59:27Z"
approved_by: user
status: Approved
---
# Spec 057-worktree-aware-resolution: Worktree-Aware Spec Resolution

## Goal

Make `workspace.SpecDir(root, specID)` worktree-aware so that every caller automatically finds spec artifacts (lifecycle.yaml, spec.md, plan.md) regardless of which worktree the process runs from. This eliminates per-callsite `EffectiveSpecRoot` workarounds that are fragile and easy to forget.

## Background

ADR-0006 establishes that spec artifacts live in spec worktrees during implementation. ADR-0019 enforces that agents work in worktrees. However, `workspace.FindRoot()` always resolves to the main repo root, and path functions like `SpecDir` only look there.

This caused P1 bug mindspec-ofp0: `mindspec complete` failed when CWD was the spec worktree because `ActiveSpecs()`, `ResolveActiveBead()`, and `advanceState()` couldn't find lifecycle.yaml in the main repo. The bugfix (merged) added per-callsite `EffectiveSpecRoot` calls and focus fallbacks, but the root issue is systemic.

ADR-0022 documents the architectural decision to make resolution worktree-aware by default.

## Impacted Domains

- workspace: `SpecDir`, `EffectiveSpecRoot`, `LifecyclePath`, `RecordingDir` path resolution
- resolve: `ActiveSpecs` lifecycle scanning, `ResolveTarget` focus fallback
- complete: `advanceState` lifecycle reading
- next: `ResolveActiveBead` lifecycle reading
- approve: `spec.go`, `plan.go` use `EffectiveSpecRoot` + `SpecDir` pattern
- state: `validate.go` calls `SpecDir(root, activeSpec)` for spec/plan existence checks (5 callsites, auto-fixed by worktree-aware `SpecDir`)

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Branch topology — spec artifacts live in worktrees
- [ADR-0019](../../adr/ADR-0019.md): Enforcement — agents work in worktrees, not main
- [ADR-0022](../../adr/ADR-0022.md): This spec implements the resolution decision

## Requirements

1. `workspace.SpecDir(root, specID)` checks spec worktree (`root/.worktrees/worktree-spec-<specID>`) before main repo paths
2. `resolve.ActiveSpecs(root)` scans worktree spec dirs in addition to main repo
3. All production callers of `EffectiveSpecRoot` are replaced with plain `SpecDir(root, specID)`
4. Manual spec path construction in `next/beads.go` and `complete/complete.go` replaced with `SpecDir`
5. `EffectiveSpecRoot` is removed (or deprecated with a doc comment pointing to `SpecDir`)
6. Focus fallback in `ResolveTarget` is kept as defense-in-depth
7. All existing unit tests pass without modification (they use main repo layout, covered by fallback)
8. LLM test `CompleteFromSpecWorktree` continues to pass

## Scope

### In Scope
- `internal/workspace/workspace.go` — `SpecDir` refactor, `EffectiveSpecRoot` removal
- `internal/resolve/resolve.go` — `ActiveSpecs` worktree scanning
- `internal/resolve/target.go` — keep focus fallback (already in place)
- `internal/complete/complete.go` — `advanceState` use `SpecDir` instead of manual paths
- `internal/next/beads.go` — `ResolveActiveBead` use `SpecDir` instead of manual paths
- `internal/approve/spec.go` — remove `EffectiveSpecRoot` + `SpecDir(effectiveRoot)` pattern
- `internal/approve/plan.go` — same
- `cmd/mindspec/validate.go` — same
- `internal/lifecycle/scenario_test.go` — same (test code)
- `internal/workspace/workspace_test.go` — update/add tests for worktree-aware `SpecDir`

### Out of Scope
- `mindspec switch` command (future work per ADR-0022)
- Changes to `FindRoot` behavior (stays resolving to main)
- Changes to `DocsDir` (not spec-specific, can't be worktree-aware)
- Write-path changes (callers that write spec files already have correct root)

## Non-Goals

- Changing how worktrees are created or managed
- Changing enforcement layers (ADR-0019)
- Making non-spec paths (ADR dir, domain dir, context map) worktree-aware

## Acceptance Criteria

- [ ] `SpecDir(mainRoot, specID)` returns worktree path when spec worktree exists with `.mindspec/docs/specs/<id>/`
- [ ] `SpecDir(mainRoot, specID)` returns main repo path when no worktree exists
- [ ] `SpecDir(mainRoot, specID)` returns canonical path when nothing exists yet (for new spec creation)
- [ ] `ActiveSpecs(root)` finds specs with lifecycle.yaml only in worktrees
- [ ] Zero production callers of `EffectiveSpecRoot` remain
- [ ] Zero manual spec path construction (`filepath.Join(root, ".mindspec", "docs", "specs", specID)`) in production code outside workspace package
- [ ] All existing unit tests pass
- [ ] LLM test `CompleteFromSpecWorktree` passes
- [ ] `go vet ./...` clean

## Validation Proofs

- `go test ./internal/workspace/ -v -run TestSpecDir`: worktree-aware resolution tests
- `go test ./internal/complete/ ./internal/resolve/ ./internal/next/ ./internal/approve/ -short`: affected packages pass
- `go vet ./...`: no issues
- `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_CompleteFromSpecWorktree -timeout 10m`: LLM test green

## Open Questions

None — ADR-0022 resolved the design questions.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-01
- **Notes**: Approved via mindspec approve spec