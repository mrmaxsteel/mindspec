---
approved_at: "2026-03-10T21:18:25Z"
approved_by: user
bead_ids:
    - mindspec-4ya5.1
    - mindspec-4ya5.2
    - mindspec-4ya5.3
    - mindspec-4ya5.4
last_updated: "2026-03-10"
spec_id: 081-executor-docs-tests
status: Approved
version: 1
---
# Plan: 081-executor-docs-tests

## ADR Fitness

- **ADR-0023** (Beads as single state authority): Documentation updates reference beads as the foundation for decomposition and state tracking
- **ADR-0006** (Protected main with PR-based merging): Execution layer docs reference the branch/merge strategy

## Testing Strategy

- Beads 1-2 (renames): `make build` + `go test ./...` + `go vet ./...` catch all breakage
- Bead 3 (docs): Grep-based validation proofs confirm no stale terminology
- Bead 4 (test audit): Run `TestLLM_SingleBead` as smoke test; document all 18 scenario findings

## Bead 1: Rename GitExecutor → MindspecExecutor + purge gitops

Mechanical rename of the executor struct, constructor, and source file. Also purges the last `gitops` reference in live Go code.

**Steps**

1. `git mv internal/executor/git.go internal/executor/mindspec_executor.go`
2. In `mindspec_executor.go`: rename struct `GitExecutor` → `MindspecExecutor`, constructor `NewGitExecutor` → `NewMindspecExecutor`
3. In `executor_test.go`: update all `GitExecutor`/`NewGitExecutor` references
4. In `cmd/mindspec/root.go`: update `newExecutor()` to call `NewMindspecExecutor`
5. In `internal/adr/store_test.go`: replace `gitops` test fixture tag with `execution`
6. Update comment in `mindspec_executor.go` referencing `specinit` → `spec`

**Acceptance Criteria**

- [ ] Zero grep hits for `GitExecutor`, `NewGitExecutor`, and `gitops` in `internal/` and `cmd/` Go files
- [ ] `make build` succeeds and `go test ./internal/executor/... -v` passes
- [ ] `go vet ./...` clean

**Verification**

- [ ] `grep -r "GitExecutor" internal/ cmd/` → zero hits
- [ ] `grep -r "NewGitExecutor" internal/ cmd/` → zero hits
- [ ] `grep -rn "gitops" --include="*.go" internal/` → zero hits
- [ ] `make build` → exit 0
- [ ] `go test ./internal/executor/... -v` → all pass
- [ ] `go vet ./...` → clean

**Depends on**

None

## Bead 2: Rename `internal/specinit/` → `internal/spec/`

Package rename with import path updates. Also renames source files for clarity.

**Steps**

1. `git mv internal/specinit/ internal/spec/` and rename `specinit.go` → `create.go`, `specinit_test.go` → `create_test.go`
2. Update package declarations: `package specinit` → `package spec`
3. Update imports and call sites in `cmd/mindspec/spec.go` and `cmd/mindspec/spec_init.go` (`specinit.Run` → `spec.Run`)
4. Update all comments referencing `specinit` in `internal/executor/mindspec_executor.go`, `internal/lifecycle/scenario_test.go`, and `cmd/mindspec/root.go`
5. Verify no stale references remain

**Acceptance Criteria**

- [ ] Zero grep hits for `specinit` in Go source files under `internal/` and `cmd/`
- [ ] `internal/spec/create.go` exists with `package spec`
- [ ] `make build` succeeds and `go test ./internal/spec/... -v` passes

**Verification**

- [ ] `grep -rn "specinit" --include="*.go" internal/ cmd/` → zero hits
- [ ] `ls internal/spec/create.go` → exists
- [ ] `make build` → exit 0
- [ ] `go test ./internal/spec/... -v` → all pass
- [ ] `go test ./cmd/mindspec/... -v` → all pass
- [ ] `go vet ./...` → clean

**Depends on**

Bead 1 (file `mindspec_executor.go` must exist before updating its comments)

## Bead 3: Architecture documentation overhaul

Rewrite documentation to clearly articulate the two-layer architecture and reflect new naming.

**Steps**

1. **AGENTS.md** §138–147: Rewrite "Architecture: Workflow/Execution Boundary":
   - Workflow layer: spec creation, plan decomposition into bitesize beads, validation against architecture (ADRs, domain boundaries), quality gates (tests, acceptance criteria), phase enforcement
   - Execution engine (`MindspecExecutor`): dispatching beads to worktrees, implementing code changes, merging results, finalizing the spec
   - Reference arXiv:2512.08296 for decomposition quality rationale
   - Update package lists to use `internal/spec/` and `MindspecExecutor`

2. **`.mindspec/docs/domains/execution/overview.md`**: Update key packages table, refine "what this domain owns" with execution engine framing

3. **`.mindspec/docs/domains/execution/architecture.md`**: `GitExecutor` → `MindspecExecutor` throughout

4. **`.mindspec/docs/domains/execution/interfaces.md`**: Update implementation names

5. **`.mindspec/docs/domains/workflow/overview.md`**: `specinit` → `spec` in key packages table

6. **`.mindspec/docs/domains/workflow/architecture.md`**: Add plan quality responsibility section — workflow layer ensures beads are well-decomposed, reviewed, have clear acceptance criteria before handoff to execution engine

7. **Auto-memory** (`MEMORY.md`): Update `GitExecutor` → `MindspecExecutor`, `specinit` → `spec`

**Acceptance Criteria**

- [ ] Zero grep hits for `GitExecutor` and `specinit` in AGENTS.md and `.mindspec/docs/domains/`
- [ ] AGENTS.md architecture section describes workflow layer (decomposition, validation, quality gates) and execution engine (implementation, merging, finalization)
- [ ] `go test ./internal/executor/... -v` still passes (no code changes, but confirms docs didn't break anything)

**Verification**

- [ ] `grep -rn "GitExecutor" .mindspec/docs/ AGENTS.md` → zero hits (excluding historical spec 077)
- [ ] `grep -rn "specinit" AGENTS.md .mindspec/docs/domains/` → zero hits
- [ ] AGENTS.md architecture section clearly describes both layers
- [ ] Execution domain docs reference `MindspecExecutor`
- [ ] Workflow domain docs describe plan quality responsibility
- [ ] `go test ./internal/executor/... -v` → all pass (regression check)

**Depends on**

Beads 1-2 (docs must reference final names)

## Bead 4: LLM test scenario audit

Review all 18 scenarios for correctness. Document findings in HISTORY.md. Fix any broken expectations.

**Steps**

1. Read every scenario in `internal/harness/scenario.go` — prompts, assertions, setup, expected behavior
2. Cross-reference with `implement.md` template (lines 49, 94: STOP after complete)
3. For each scenario assess: prompt validity, assertion correctness, MaxTurns/timeout realism
4. Scrutinize specifically:
   - **SpecToIdle**: 100 turns for full manual lifecycle
   - **MultiBeadDeps**: Expects explicit `mindspec next`
   - **BlockedBeadTransition**: Mode→plan when only blocked beads remain
   - **UnmergedBeadGuard**: Recovery flow after close-without-complete
5. Write "Test Audit (Spec 081)" section in `internal/harness/HISTORY.md` with per-scenario findings
6. Fix any broken test expectations
7. Run `TestLLM_SingleBead` as smoke test

**Acceptance Criteria**

- [ ] HISTORY.md contains "Test Audit (Spec 081)" section with findings for all 18 scenarios
- [ ] Any outdated test expectations fixed
- [ ] `TestLLM_SingleBead` smoke test passes

**Verification**

- [ ] HISTORY.md contains "Test Audit (Spec 081)" section covering all 18 scenarios
- [ ] Any outdated expectations fixed (if found)
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m` → passes

**Depends on**

Beads 1-2 (code references in scenarios should use new names if applicable)

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| `GitExecutor` → `MindspecExecutor` (zero grep hits) | Bead 1 verification |
| `git.go` → `mindspec_executor.go` | Bead 1 verification |
| `specinit` → `spec` (zero grep hits) | Bead 2 verification |
| `gitops` purged from live code | Bead 1 verification |
| `make build` + `go test` + `go vet` pass | Beads 1, 2 verification |
| AGENTS.md two-layer architecture | Bead 3 verification |
| Domain docs updated | Bead 3 verification |
| 18 LLM test scenarios reviewed in HISTORY.md | Bead 4 verification |
| Outdated test expectations fixed | Bead 4 verification |
| SingleBead smoke test passes | Bead 4 verification |
