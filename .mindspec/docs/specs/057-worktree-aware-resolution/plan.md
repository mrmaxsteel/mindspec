---
status: Draft
spec_id: "057-worktree-aware-resolution"
version: 1
last_updated: "2026-03-01"
approved_at: ""
approved_by: ""
bead_ids: []
adr_citations:
  - id: ADR-0006
    sections: ["Branch topology", "Worktree lifecycle"]
  - id: ADR-0019
    sections: ["Enforcement layers"]
  - id: ADR-0022
    sections: ["Spec artifact resolution", "ActiveSpecs worktree-aware"]
---

# Plan: 057-worktree-aware-resolution

## ADR Fitness

- **ADR-0006** (Protected Main, PR-based merging): Sound. This spec implements the resolution side of worktree isolation that ADR-0006 assumes. No divergence.
- **ADR-0019** (Deterministic enforcement): Sound. Enforcement layers remain unchanged; this spec fixes the path resolution that enforcement depends on. No divergence.
- **ADR-0022** (Worktree-aware spec resolution): This spec is the direct implementation of ADR-0022. The resolution algorithm (check worktree → main → legacy) matches exactly. No divergence.

## Testing Strategy

- **Unit tests**: `internal/workspace/workspace_test.go` — new tests for worktree-aware `SpecDir`, updated `EffectiveSpecRoot` tests become `SpecDir` tests
- **Package tests**: `go test ./internal/complete/ ./internal/resolve/ ./internal/next/ ./internal/approve/ ./cmd/mindspec/ -short` — all affected packages pass
- **Vet**: `go vet ./...` clean
- **LLM integration (existing)**: `TestLLM_CompleteFromSpecWorktree` continues to pass
- **LLM integration (new)**: Comprehensive suite of scenarios validating worktree-aware resolution end-to-end:
  - `ValidateFromWorktree` — `mindspec validate spec/plan` succeeds when spec artifacts only exist in worktree
  - `ApproveSpecFromWorktree` — `mindspec approve spec` succeeds from worktree with no spec files in main repo
  - `ApprovePlanFromWorktree` — `mindspec approve plan` succeeds from worktree with no plan in main repo
  - `ActiveSpecsWorktreeOnly` — agent discovers active spec when lifecycle.yaml exists only in the spec worktree (not main repo)
  - `NextFromSpecWorktree` — `mindspec next` resolves the bead correctly when lifecycle.yaml is only in the spec worktree

## Bead 1: Make SpecDir worktree-aware and remove EffectiveSpecRoot

**Steps**
1. Refactor `workspace.SpecDir(root, specID)` to check `root/.worktrees/worktree-spec-<specID>/.mindspec/docs/specs/<specID>/` first, then `root/.mindspec/docs/specs/<specID>/`, then `root/docs/specs/<specID>/` (legacy fallback)
2. Update `LifecyclePath(root, specID)` and `RecordingDir(root, specID)` — these already delegate to `SpecDir`, so they inherit worktree-awareness automatically (verify)
3. Remove `EffectiveSpecRoot` function (or mark deprecated with panic if called)
4. Update `workspace_test.go`: convert `TestEffectiveSpecRoot_*` tests to `TestSpecDir_WorktreeAware_*` tests that validate the 3-step resolution order

**Verification**
- [ ] `go test ./internal/workspace/ -v -run TestSpecDir` passes
- [ ] `go vet ./internal/workspace/` clean

**Depends on**
None

## Bead 2: Update all production callers to use plain SpecDir

**Steps**
1. `internal/approve/spec.go`: Remove `effectiveRoot := workspace.EffectiveSpecRoot(...)`, use `workspace.SpecDir(root, specID)` directly
2. `internal/approve/plan.go` (lines 42, 329): Same pattern — remove `effectiveRoot`, use `workspace.SpecDir(root, specID)` directly
3. `internal/complete/complete.go` (`advanceState`): Replace `effectiveRoot + manual filepath.Join` with `workspace.SpecDir(root, specID)`
4. `internal/next/beads.go` (`ResolveActiveBead`): Replace `effectiveRoot + manual filepath.Join` with `workspace.SpecDir(root, specID)`
5. `cmd/mindspec/validate.go`: Replace both `effectiveRoot` usages with direct `workspace.SpecDir(root, specID)` passed to validators
6. `internal/lifecycle/scenario_test.go`: Update test code callers (replace `EffectiveSpecRoot` with `SpecDir`)

**Verification**
- [ ] `go build ./...` compiles (no references to removed `EffectiveSpecRoot`)
- [ ] `go test ./internal/approve/ ./internal/complete/ ./internal/next/ ./cmd/mindspec/ -short` passes
- [ ] `go vet ./...` clean
- [ ] Zero `grep -r EffectiveSpecRoot` matches in `.go` files (excluding test history/docs)

**Depends on**
Bead 1

## Bead 3: Make ActiveSpecs worktree-aware

**Steps**
1. In `resolve.ActiveSpecs(root)`: after scanning `DocsDir(root)/specs/*/lifecycle.yaml`, also scan `root/.worktrees/worktree-spec-*/` directories for lifecycle.yaml
2. Deduplicate by specID (worktree result wins over main repo if both exist)
3. Keep the sort-by-specID behavior
4. Update `ResolveMode(root, specID)` to use `workspace.SpecDir(root, specID)` instead of manual `DocsDir(root)/specs/<specID>` path construction

**Verification**
- [ ] `go test ./internal/resolve/ -v` passes (existing tests use main repo layout, still works via fallback)
- [ ] `go test ./internal/resolve/ -v -run TestActiveSpecs` — verify worktree scanning doesn't break existing behavior
- [ ] `go vet ./internal/resolve/` clean

**Depends on**
Bead 1

## Bead 4: Comprehensive LLM test suite for worktree-aware resolution

**Steps**
1. Add `ScenarioValidateFromWorktree()` — sandbox sets up spec worktree with spec.md only in worktree (not main), agent runs `mindspec validate spec <id>`. Assert: validate succeeds (exit 0). Repeat for plan validation.
2. Add `ScenarioApproveSpecFromWorktree()` — sandbox has spec in worktree only, spec mode active. Agent runs `mindspec approve spec <id>`. Assert: approve succeeds, mode transitions to plan.
3. Add `ScenarioApprovePlanFromWorktree()` — sandbox has spec+plan in worktree only, plan mode active. Agent runs `mindspec approve plan <id>`. Assert: approve succeeds, beads created.
4. Add `ScenarioActiveSpecsWorktreeOnly()` — sandbox has lifecycle.yaml only in spec worktree (no lifecycle.yaml in main repo). Agent runs `mindspec state show` or equivalent. Assert: agent finds the active spec without errors.
5. Add `ScenarioNextFromSpecWorktree()` — sandbox has approved plan in spec worktree with epic+ready bead. Agent runs `mindspec next`. Assert: next succeeds, bead worktree created.
6. Register all new scenarios in `AllScenarios()`.
7. Add corresponding `TestLLM_*` test functions in `scenario_test.go`.

**Verification**
- [ ] `go build ./internal/harness/...` compiles
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_ValidateFromWorktree -timeout 10m` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_ApproveSpecFromWorktree -timeout 10m` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_ApprovePlanFromWorktree -timeout 10m` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_ActiveSpecsWorktreeOnly -timeout 10m` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_NextFromSpecWorktree -timeout 10m` passes

**Depends on**
Bead 1, Bead 2, Bead 3

## Bead 5: Final validation and cleanup

**Steps**
1. Run full test suite: `make test`
2. Run `go vet ./...`
3. Run existing LLM test: `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_CompleteFromSpecWorktree -timeout 10m`
4. Run new LLM test suite: `env -u CLAUDECODE go test ./internal/harness/ -v -run "TestLLM_(ValidateFromWorktree|ApproveSpecFromWorktree|ApprovePlanFromWorktree|ActiveSpecsWorktreeOnly|NextFromSpecWorktree)" -timeout 30m`
5. Verify zero manual spec path construction in production code outside workspace package
6. Remove any remaining dead code or comments referencing `EffectiveSpecRoot`

**Verification**
- [ ] `make test` passes
- [ ] `go vet ./...` clean
- [ ] All LLM tests pass (existing + new)
- [ ] `grep -rn 'EffectiveSpecRoot' --include='*.go'` returns only test files and docs

**Depends on**
Bead 2, Bead 3, Bead 4

## Provenance

| Acceptance Criterion | Bead | Verification |
|:-|:-|:-|
| `SpecDir` returns worktree path when spec worktree exists | Bead 1 | `TestSpecDir_WorktreeAware` |
| `SpecDir` returns main repo path when no worktree | Bead 1 | `TestSpecDir_WorktreeAware` |
| `SpecDir` returns canonical path for new spec creation | Bead 1 | `TestSpecDir_WorktreeAware` |
| `ActiveSpecs` finds specs only in worktrees | Bead 3 | `TestActiveSpecs` + `TestLLM_ActiveSpecsWorktreeOnly` |
| Zero production callers of `EffectiveSpecRoot` | Bead 2, 5 | grep validation |
| Zero manual spec path construction outside workspace | Bead 2, 5 | grep validation |
| All existing unit tests pass | Bead 5 | `make test` |
| LLM test `CompleteFromSpecWorktree` passes | Bead 5 | `TestLLM_CompleteFromSpecWorktree` |
| `go vet ./...` clean | Bead 5 | `go vet` |
| Validate works from worktree | Bead 4 | `TestLLM_ValidateFromWorktree` |
| Approve spec works from worktree | Bead 4 | `TestLLM_ApproveSpecFromWorktree` |
| Approve plan works from worktree | Bead 4 | `TestLLM_ApprovePlanFromWorktree` |
| Next works from spec worktree | Bead 4 | `TestLLM_NextFromSpecWorktree` |
