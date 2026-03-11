---
approved_at: "2026-03-10T21:18:25Z"
approved_by: user
bead_ids:
    - mindspec-4ya5.1
    - mindspec-4ya5.2
    - mindspec-4ya5.3
    - mindspec-4ya5.4
    - mindspec-qszb
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
- Bead 5 (stop behavior): Fix instruct templates + CLI output to prevent agent auto-proceeding

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

7. **Classify `mindspec next` and `mindspec complete` as execution layer commands** in AGENTS.md and domain docs — they create/destroy worktrees and manage branch topology, which is execution concern. The workflow layer (approve commands) decides *when* transitions happen; the execution layer (next/complete) performs them.

8. **Auto-memory** (`MEMORY.md`): Update `GitExecutor` → `MindspecExecutor`, `specinit` → `spec`

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
8. **Create stop-behavior LLM test scenarios** — new tests that verify:
   - After `mindspec approve plan`, agent STOPS (does not auto-implement or write code)
   - After `mindspec complete`, agent STOPS (does not auto-claim or run `mindspec next`)
   - Agent uses `mindspec next` to create bead worktree (not working on spec branch directly)
   These tests are expected to **fail before Bead 5** (which fixes the guidance). Run them to establish the baseline, document failures in HISTORY.md. Bead 5 re-runs them as verification.

**Acceptance Criteria**

- [ ] HISTORY.md contains "Test Audit (Spec 081)" section with findings for all 18 scenarios
- [ ] Any outdated test expectations fixed
- [ ] `TestLLM_SingleBead` smoke test passes
- [ ] Stop-behavior test scenarios created (`TestLLM_StopAfterPlanApprove`, `TestLLM_StopAfterComplete`)
- [ ] Baseline results documented in HISTORY.md (expected failures before Bead 5)

**Verification**

- [ ] HISTORY.md contains "Test Audit (Spec 081)" section covering all 18 scenarios
- [ ] Any outdated expectations fixed (if found)
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_SingleBead -timeout 10m` → passes
- [ ] Stop-behavior tests exist and run (failures expected — baseline captured)

**Depends on**

Beads 1-2 (code references in scenarios should use new names if applicable)

## Bead 5: Harden phase-transition stop behavior

Fix observed failure: agent auto-proceeded after plan approval and worked on the spec branch instead of using `mindspec next` to create a bead worktree. Root causes: outdated instruct template, insufficiently emphatic CLI output.

**Steps**

1. **Fix `internal/instruct/templates/plan.md`**: Remove false claim "This will approve the plan AND automatically claim the first bead." Replace with clear guidance: after plan approval, STOP, run `/clear`, then `mindspec next`.
2. **Strengthen plan approve output in `cmd/mindspec/plan_cmd.go`**: Make STOP instruction unmissable with clear separator (e.g., `⛔ STOP` or `---` banner). Emphasize: do NOT proceed, run `/clear` first, then `mindspec next`.
3. **Strengthen `mindspec complete` output in `internal/complete/complete.go` `FormatResult()`**: When reporting "Next bead ready: X", append explicit instruction: "STOP. Run `/clear`, then `mindspec next` to claim it."
4. **Remove dead `--no-next` flag** from `cmd/mindspec/approve.go`.
5. **Update `implement.md` template** if needed — verify STOP guidance is clear and consistent with the CLI output changes.

**Acceptance Criteria**

- [ ] `plan.md` template no longer says "automatically claim the first bead"
- [ ] Plan approve output includes emphatic STOP + `/clear` + `mindspec next` instructions
- [ ] `mindspec complete` output includes STOP instruction when next bead is ready
- [ ] `--no-next` flag removed from `approve.go`
- [ ] `make build` succeeds and `go test ./cmd/mindspec/... -v` passes
- [ ] Stop-behavior LLM tests pass (created in Bead 4, expected to fail before this bead)

**Verification**

- [ ] `grep "auto.*claim" internal/instruct/templates/plan.md` → zero hits
- [ ] `grep "no-next" cmd/mindspec/approve.go` → zero hits
- [ ] `make build` → exit 0
- [ ] `go test ./cmd/mindspec/... -v` → all pass
- [ ] `go vet ./...` → clean
- [ ] Manual review: `./bin/mindspec approve plan --help` no longer shows `--no-next`
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_StopAfterPlanApprove -timeout 10m` → passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_StopAfterComplete -timeout 10m` → passes

**Depends on**

Bead 4 (stop-behavior tests must exist before this bead fixes the guidance)

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
| `plan.md` template no longer claims auto-claim | Bead 5 verification |
| Plan approve output has emphatic STOP | Bead 5 verification |
| `mindspec complete` output has STOP for next bead | Bead 5 verification |
| Dead `--no-next` flag removed | Bead 5 verification |
| Stop-behavior tests created (baseline) | Bead 4 verification |
| Stop-behavior tests pass (after fix) | Bead 5 verification |
| `next`/`complete` classified as execution layer | Bead 3 verification |
