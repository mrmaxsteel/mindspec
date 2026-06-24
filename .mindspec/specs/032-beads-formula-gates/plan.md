---
adr_citations:
    - id: ADR-0012
      sections:
        - Decision
        - Applied to Beads
        - Consequences
    - id: ADR-0013
      sections:
        - Decision
        - How mindspec uses it
        - Consequences
approved_at: "2026-02-16T16:58:54Z"
approved_by: user
last_updated: "2026-02-16"
spec_id: 032-beads-formula-gates
status: Approved
version: 1
work_chunks:
    - depends_on: []
      id: foundation
      title: Formula + RunBD helper + State fields
    - depends_on:
        - foundation
      id: spec-init
      title: spec-init pours formula
    - depends_on:
        - foundation
      id: approval-flow
      title: Approval flow migration
    - depends_on:
        - foundation
      id: complete-next
      title: Complete + Next migration
    - depends_on:
        - spec-init
        - approval-flow
        - complete-next
      id: delete-cleanup
      title: Delete wrappers + cleanup
---

# Plan: 032 — Native Beads Integration

## Context

Mindspec's `internal/bead/` package has grown into a 39-function Go wrapper around the `bd` CLI. This wrapping layer is fragile (gate.go has been broken since inception), reimplements features beads handles natively (propagate.go, molecule construction in plan.go), and creates multi-layer indirection that obscures intent. ADR-0012 established "compose, don't wrap" as the principle; ADR-0013 declared the spec lifecycle should be a beads formula. This plan implements both decisions.

**Goal**: Replace the deep wrapper layer with a thin composition layer. Delete 4 files (~700 lines), reduce `internal/bead/` to <10 exports, and use a beads formula for lifecycle orchestration.

## Bead 1: Formula + RunBD helper + State fields

**Scope**: Create the foundation that all other beads depend on.

**Steps**:
1. Create `.beads/formulas/spec-lifecycle.formula.toml` with 6 steps (spec, spec-approve, plan, plan-approve, implement, review) and correct `needs` chain
2. Add `RunBD(args ...string) ([]byte, error)` to `internal/bead/bdcli.go` — generic exec helper using `cmd.Output()` (stdout only), wraps errors with stderr, includes trace instrumentation
3. Add `RunBDCombined(args ...string) ([]byte, error)` for commands that don't return JSON
4. Add `ActiveMolecule` (string) and `StepMapping` (map[string]string) fields to `State` struct in `internal/state/state.go`
5. Write tests for `RunBD` and state serialization with new fields

**Verification**:
- [ ] `bd mol pour spec-lifecycle --var spec_id=test --dry-run` succeeds
- [ ] `make test` passes with new RunBD and state tests

**Depends on**: None

## Bead 2: spec-init pours formula

**Scope**: Wire `mindspec spec-init` to create a molecule via `bd mol pour`.

**Steps**:
1. In `internal/specinit/specinit.go`, after creating spec dir and setting state, call `bd mol pour spec-lifecycle --var spec_id=<id> --json`
2. Parse JSON response to extract `new_epic_id` and `id_mapping`
3. Store `ActiveMolecule` and `StepMapping` in state via updated `State` struct
4. Mark the `spec` step as in_progress: `bd update <spec-step-id> --status in_progress`
5. Add `bead.Preflight()` check at the start (best-effort — don't fail if beads not initialized)
6. Update `internal/specinit/specinit_test.go` with mock bd output

**Verification**:
- [ ] `mindspec spec-init 999-test` creates molecule
- [ ] `bd mol show <id>` shows 6 steps
- [ ] state.json contains `activeMolecule` and `stepMapping`

**Depends on**: Bead 1

## Bead 3: Approval flow migration

**Scope**: Simplify approve/spec.go, approve/plan.go, approve/impl.go to use `bd close` on molecule steps.

**Steps**:
1. Rework `approve/spec.go`: read state for StepMapping, replace CreateSpecBead+FindGateAnyStatus+ResolveGate with `bd close <stepMapping["spec-approve"]>`, add backward compat (skip bead ops if no molecule)
2. Rework `approve/plan.go`: read state for StepMapping, replace CreatePlanBeads+WriteGeneratedBeadIDs+FindGateAnyStatus+ResolveGate with `bd close <stepMapping["plan-approve"]>`, add backward compat
3. Rework `approve/impl.go`: read state for StepMapping, replace SearchAny+Close with `bd close <stepMapping["review"]>`, transition to idle
4. Update `approve/spec_test.go`, `approve/plan_test.go`, `approve/impl_test.go`

**Verification**:
- [ ] `mindspec approve spec <id>` closes the spec-approve step
- [ ] `bd ready --parent <mol-id>` shows plan step next after spec approval
- [ ] Pre-032 specs without molecules still work through approve flow

**Depends on**: Bead 1

## Bead 4: Complete + Next migration

**Scope**: Simplify complete.go and next/beads.go to use direct `bd` calls and state-based molecule lookup.

**Steps**:
1. In `complete/complete.go`: remove `propagateCloseFn`/`bead.PropagateClose` call (molecules handle natively), read `ActiveMolecule` from state instead of parsing plan frontmatter, use `bead.RunBD()` for ready/search calls in `advanceState()`
2. In `next/beads.go`: read state for `ActiveMolecule`, use `bd ready --parent <mol-id> --json` if set, remove `queryMolChildren()` and `convertBeadInfos()`, use `bead.BeadInfo` directly instead of duplicate struct
3. Update `complete/complete_test.go` and `next/beads_test.go`

**Verification**:
- [ ] `mindspec complete` closes bead and advances state without propagate calls
- [ ] `mindspec next` discovers work from active molecule via state lookup

**Depends on**: Bead 1

## Bead 5: Delete wrappers + cleanup

**Scope**: Delete unused wrapper files, reduce bdcli.go to minimal exports.

**Steps**:
1. Delete `internal/bead/gate.go` + `gate_test.go`
2. Delete `internal/bead/spec.go` + `spec_test.go`
3. Delete `internal/bead/plan.go` + `plan_test.go`
4. Delete `internal/bead/propagate.go` + `propagate_test.go`
5. Reduce `internal/bead/bdcli.go`: delete `Create()`, `Search()`, `SearchAny()`, `Show()`, `ListOpen()`, `DepAdd()`, `Update()`, `Close()`; keep `Preflight()`, `RunBD()`, `RunBDCombined()`, `BeadInfo`, worktree functions, structs
6. Update `hygiene.go` to use `RunBD()` instead of `ListOpen()`/`Update()`; inline bd call in `instruct/worktree.go`
7. Fix remaining compilation errors, verify `internal/bead/` exports <10 functions

**Verification**:
- [ ] `go build ./...` compiles cleanly
- [ ] `wc -l internal/bead/*.go` shows <200 lines (excluding tests)
- [ ] `make test` passes with no new failures

**Depends on**: Beads 2, 3, 4

## ADR Fitness

**ADR-0012 (Compose, don't wrap)**: Sound and directly applicable. This plan is the primary implementation. No divergence.

**ADR-0013 (Use beads formulas)**: Sound. The 6-step formula with `type = "human"` gates and `needs` dependencies matches beads' native design. No divergence.

## Backward Compatibility

Pre-032 specs (those created before formula integration) won't have `ActiveMolecule` or `StepMapping` in state. All bead operations in approve/complete/next check for molecule presence first and fall back to direct state transitions if absent. This is tested explicitly.
