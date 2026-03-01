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
- **Deterministic integration tests** (new, in `internal/resolve/` and `internal/workspace/`): CLI-level tests that validate worktree-aware resolution without an LLM agent — faster, cheaper, deterministic:
  - `TestValidateFromWorktree` — `mindspec validate spec/plan` succeeds when spec artifacts only exist in worktree
  - `TestActiveSpecsWorktreeOnly` — `ActiveSpecs()` discovers specs when lifecycle.yaml exists only in the spec worktree
  - `TestNextFromSpecWorktree` — `mindspec next` resolves the bead correctly when lifecycle.yaml is only in the spec worktree
- **LLM integration (new)**: Scenarios that test agent navigation and decision-making under worktree ambiguity:
  - `ApproveSpecFromWorktree` — agent navigates to worktree, runs `mindspec approve spec`, handles mode transition
  - `ApprovePlanFromWorktree` — agent navigates to worktree, runs `mindspec approve plan`, confirms beads created

## Bead 1: Make SpecDir worktree-aware

**Steps**
1. Refactor `workspace.SpecDir(root, specID)` to check `root/.worktrees/worktree-spec-<specID>/.mindspec/docs/specs/<specID>/` first, then `root/.mindspec/docs/specs/<specID>/`, then `root/docs/specs/<specID>/` (legacy fallback)
2. Update `LifecyclePath(root, specID)` and `RecordingDir(root, specID)` — these already delegate to `SpecDir`, so they inherit worktree-awareness automatically (verify)
3. Add `// Deprecated: Use SpecDir directly — it is now worktree-aware. See ADR-0022.` comment to `EffectiveSpecRoot` (do NOT remove yet — callers still reference it until Bead 2)
4. Update `workspace_test.go`: convert `TestEffectiveSpecRoot_*` tests to `TestSpecDir_WorktreeAware_*` tests that validate the 3-step resolution order

**Verification**
- [ ] `go test ./internal/workspace/ -v -run TestSpecDir` passes
- [ ] `go build ./...` compiles (EffectiveSpecRoot still exists for callers)
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
7. Remove `EffectiveSpecRoot` function from `workspace.go` and its tests from `workspace_test.go` (all callers now gone)

**Verification**
- [ ] `go build ./...` compiles (no references to removed `EffectiveSpecRoot`)
- [ ] `go test ./internal/approve/ ./internal/complete/ ./internal/next/ ./cmd/mindspec/ -short` passes
- [ ] `go vet ./...` clean
- [ ] Zero `grep -r EffectiveSpecRoot` matches in `.go` files (excluding test history/docs)

**Depends on**
Bead 1

## Bead 3: Make ActiveSpecs worktree-aware

**Steps**
1. In `resolve.ActiveSpecs(root)`: the existing code manually constructs paths via `DocsDir(root)/specs/*/lifecycle.yaml`. Keep that main-repo scan, then add a second scan of `root/.worktrees/worktree-spec-*/` directories, reading lifecycle.yaml from each worktree's `.mindspec/docs/specs/<specID>/` path
2. Deduplicate by specID (worktree result wins over main repo if both exist)
3. Keep the sort-by-specID behavior
4. Update `ResolveMode(root, specID)` to use `workspace.SpecDir(root, specID)` instead of manual `DocsDir(root)/specs/<specID>` path construction

**Verification**
- [ ] `go test ./internal/resolve/ -v` passes (existing tests use main repo layout, still works via fallback)
- [ ] `go test ./internal/resolve/ -v -run TestActiveSpecs` — verify worktree scanning doesn't break existing behavior
- [ ] `go vet ./internal/resolve/` clean

**Depends on**
Bead 1

## Bead 4: Worktree-aware test suite (deterministic + LLM)

**Steps**
1. Add deterministic integration tests (no LLM, fast, run in `make test`):
   - `TestValidateFromWorktree` in `internal/lifecycle/scenario_test.go` — set up sandbox with spec worktree containing spec.md + plan.md (not in main repo), run `mindspec validate spec <id>` and `mindspec validate plan <id>` via `exec.Command`. Assert exit 0.
   - `TestActiveSpecsWorktreeOnly` in `internal/resolve/integration_test.go` — create temp dir with `.worktrees/worktree-spec-<id>/.mindspec/docs/specs/<id>/lifecycle.yaml` (no lifecycle in main repo), call `ActiveSpecs(root)`. Assert spec is found.
   - `TestNextFromSpecWorktree` in `internal/lifecycle/scenario_test.go` — set up sandbox with approved plan + epic + ready bead in spec worktree only, run `mindspec next` via `exec.Command`. Assert bead worktree created.
2. Add LLM integration scenarios (test agent navigation under worktree ambiguity):
   - `ScenarioApproveSpecFromWorktree()` — sandbox has spec in worktree only, spec mode active, agent CWD is main repo. Agent must navigate to worktree and run `mindspec approve spec <id>`. Assert: approve succeeds, mode transitions to plan.
   - `ScenarioApprovePlanFromWorktree()` — sandbox has spec+plan in worktree only, plan mode active, agent CWD is main repo. Agent must navigate to worktree and run `mindspec approve plan <id>`. Assert: approve succeeds, beads created.
3. Register LLM scenarios in `AllScenarios()`.
4. Add corresponding `TestLLM_*` test functions in `scenario_test.go`.

**Verification**
- [ ] `go test ./internal/resolve/ -v -run TestActiveSpecsWorktreeOnly` passes
- [ ] `go test ./internal/lifecycle/ -v -run "TestValidateFromWorktree|TestNextFromSpecWorktree"` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_ApproveSpecFromWorktree -timeout 10m` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_ApprovePlanFromWorktree -timeout 10m` passes

**Depends on**
Bead 1, Bead 2, Bead 3

## Bead 5: Final validation and cleanup

**Steps**
1. Run full test suite: `make test`
2. Run `go vet ./...`
3. Run existing LLM test: `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_CompleteFromSpecWorktree -timeout 10m`
4. Run new LLM tests: `env -u CLAUDECODE go test ./internal/harness/ -v -run "TestLLM_(ApproveSpecFromWorktree|ApprovePlanFromWorktree)" -timeout 20m`
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
| `ActiveSpecs` finds specs only in worktrees | Bead 3, 4 | `TestActiveSpecs` + `TestActiveSpecsWorktreeOnly` |
| Focus fallback in `ResolveTarget` kept as defense-in-depth | Bead 2 | Existing `ResolveTarget` tests (no code change, verify not removed) |
| Zero production callers of `EffectiveSpecRoot` | Bead 2, 5 | grep validation |
| Zero manual spec path construction outside workspace | Bead 2, 5 | grep validation |
| All existing unit tests pass | Bead 5 | `make test` |
| LLM test `CompleteFromSpecWorktree` passes | Bead 5 | `TestLLM_CompleteFromSpecWorktree` |
| `go vet ./...` clean | Bead 5 | `go vet` |
| Validate works from worktree | Bead 4 | `TestValidateFromWorktree` (deterministic) |
| Approve spec works from worktree | Bead 4 | `TestLLM_ApproveSpecFromWorktree` (LLM) |
| Approve plan works from worktree | Bead 4 | `TestLLM_ApprovePlanFromWorktree` (LLM) |
| Next works from spec worktree | Bead 4 | `TestNextFromSpecWorktree` (deterministic) |
