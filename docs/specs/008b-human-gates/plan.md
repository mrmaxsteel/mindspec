---
status: Approved
spec_id: 008b-human-gates
version: "0.1"
last_updated: 2026-02-13
approved_at: 2026-02-13T12:00:00Z
approved_by: user
adr_citations:
  - id: ADR-0002
    sections: ["Beads as passive tracking substrate"]
  - id: ADR-0003
    sections: ["MindSpec owns orchestration"]
work_chunks:
  - id: 1
    title: "Gate creation/resolution helpers + bdcli wrappers"
    scope: "internal/bead/gate.go, internal/bead/bdcli.go"
    verify:
      - "`CreateGate()` creates a gate bead via `bd create --type=gate --parent=<id>`"
      - "`FindGate()` searches by title prefix and returns existing gate or nil"
      - "`ResolveGate()` calls `bd gate resolve <id> --reason=<reason>`"
      - "`IsGateResolved()` checks gate status (closed = resolved)"
      - "All functions are testable via `execCommand` override"
    depends_on: []
  - id: 2
    title: "Spec bead creates spec approval gate"
    scope: "internal/bead/spec.go"
    verify:
      - "`CreateSpecBead()` creates `[GATE spec-approve <id>]` gate as child of spec bead"
      - "Gate creation is idempotent — reuse existing gate on re-run"
      - "`CreateSpecBead()` returns gate ID alongside bead info"
      - "`make test` passes"
    depends_on: [1]
  - id: 3
    title: "Plan beads create plan approval gate + wire dependencies"
    scope: "internal/bead/plan.go"
    verify:
      - "`CreatePlanBeads()` creates `[GATE plan-approve <id>]` gate as child of molecule parent"
      - "Each impl chunk bead depends on the plan gate"
      - "Plan gate depends on spec gate (if one exists)"
      - "`CreatePlanBeads()` refuses if spec gate is open (exits with error)"
      - "Gate creation is idempotent"
      - "`make test` passes"
    depends_on: [1, 2]
  - id: 4
    title: "Shared instruct-tail helper"
    scope: "cmd/mindspec/instruct_tail.go"
    verify:
      - "`emitInstruct(root)` reads state, builds context, checks worktree, renders guidance"
      - "Function is callable from any command handler"
      - "Output matches `mindspec instruct` output for the current state"
    depends_on: []
  - id: 5
    title: "`mindspec approve spec` and `mindspec approve plan` commands"
    scope: "cmd/mindspec/approve.go, cmd/mindspec/root.go"
    verify:
      - "`mindspec approve spec <id>` validates, updates spec frontmatter, resolves gate, sets state, emits instruct"
      - "`mindspec approve plan <id>` validates, updates plan YAML frontmatter, resolves gate, sets state, emits instruct"
      - "Both fail with exit 1 if validation fails"
      - "Both warn but proceed if no gate exists"
      - "`make test` passes"
    depends_on: [1, 4]
  - id: 6
    title: "`mindspec complete` instruct-tail + `mindspec next` refactor"
    scope: "cmd/mindspec/complete.go, cmd/mindspec/next.go, internal/complete/complete.go"
    verify:
      - "`mindspec complete` emits instruct output for the new mode after advancing state"
      - "`mindspec next` uses shared `emitInstruct()` helper (existing behavior preserved)"
      - "Ad-hoc `FormatResult()` summary is supplemented by instruct output"
      - "`make test` passes"
    depends_on: [4]
  - id: 7
    title: "Simplify approval skills + doc-sync"
    scope: ".claude/commands/spec-approve.md, .claude/commands/plan-approve.md, CLAUDE.md, docs/core/CONVENTIONS.md"
    verify:
      - "`/spec-approve` skill just confirms spec ID and runs `mindspec approve spec <id>`"
      - "`/plan-approve` skill just confirms spec ID and runs `mindspec approve plan <id>`"
      - "CLAUDE.md lists `mindspec approve spec/plan` in command table"
      - "CONVENTIONS.md documents gate title conventions and instruct-tail convention"
      - "All procedural logic removed from skill files"
    depends_on: [5, 6]
generated:
  mol_parent_id: mindspec-iui
  bead_ids:
    "1": mindspec-cxj
    "2": mindspec-u4t
    "3": mindspec-s1u
    "4": mindspec-bau
    "5": mindspec-jn3
    "6": mindspec-yg5
    "7": mindspec-bay
---

# Plan: Spec 008b — Human Gates + Instruct-Tail Convention

**Spec**: [spec.md](spec.md)

---

## Bead 008b-1: Gate creation/resolution helpers + bdcli wrappers

**Scope**: New `internal/bead/gate.go` with gate primitives; additions to `internal/bead/bdcli.go` if needed.

**Steps**:
1. Create `internal/bead/gate.go` with `CreateGate(title, parentID)`, `FindGate(titlePrefix)`, `ResolveGate(gateID, reason)`, `IsGateResolved(gateID)`
2. `CreateGate` calls `bd create <title> --type=gate --parent=<parentID> --json` and returns `BeadInfo`
3. `FindGate` calls `bd search <prefix> --json --status=open` (returns nil if not found) and also searches closed gates for `IsGateResolved` checks
4. `ResolveGate` calls `bd gate resolve <id> --reason=<reason>`
5. `IsGateResolved` searches for the gate by prefix — if found and status is closed, it's resolved; if open, not resolved; if not found, treat as "no gate" (backward compat)
6. Write unit tests using `execCommand` override pattern (consistent with existing bdcli tests)

**Verification**:
- [ ] `CreateGate()` produces correct `bd create` invocation
- [ ] `FindGate()` returns existing gate or nil
- [ ] `ResolveGate()` produces correct `bd gate resolve` invocation
- [ ] `IsGateResolved()` correctly distinguishes open/closed/missing

**Depends on**: nothing

---

## Bead 008b-2: Spec bead creates spec approval gate

**Scope**: Modify `internal/bead/spec.go` `CreateSpecBead()`.

**Steps**:
1. After creating the spec bead, call `FindGate("[GATE spec-approve <id>]")` for idempotency
2. If no gate exists, call `CreateGate("[GATE spec-approve <id>] Spec approval", specBeadID)` to create a gate as child of the spec bead
3. Return both the spec bead info and the gate ID (extend return value or add to a result struct)
4. Update `cmd/mindspec/bead.go` output to show the gate ID
5. Write tests

**Verification**:
- [ ] `CreateSpecBead()` creates gate alongside spec bead
- [ ] Re-running reuses existing gate
- [ ] Gate is a child of the spec bead

**Depends on**: 008b-1

---

## Bead 008b-3: Plan beads create plan approval gate + wire dependencies

**Scope**: Modify `internal/bead/plan.go` `CreatePlanBeads()`.

**Steps**:
1. At the start of `CreatePlanBeads()`, check if spec gate exists and is resolved via `IsGateResolved("[GATE spec-approve <id>]")`. If gate exists but is NOT resolved, return error: "spec not approved — resolve spec gate first"
2. After creating the molecule parent, call `FindGate("[GATE plan-approve <id>]")` for idempotency
3. If no gate exists, create plan gate as child of molecule parent: `CreateGate("[GATE plan-approve <id>] Plan approval", molParentID)`
4. If spec gate exists, wire plan gate to depend on spec gate: `DepAdd(planGateID, specGateID)`
5. When creating each impl chunk bead, add dependency on plan gate: `DepAdd(chunkBeadID, planGateID)`
6. Update `cmd/mindspec/bead.go` output to show the plan gate ID
7. Write tests

**Verification**:
- [ ] Plan gate created as child of molecule parent
- [ ] Each impl bead depends on plan gate
- [ ] Plan gate depends on spec gate
- [ ] Refuses if spec gate is open
- [ ] Idempotent on re-run

**Depends on**: 008b-1, 008b-2

---

## Bead 008b-4: Shared instruct-tail helper

**Scope**: New `cmd/mindspec/instruct_tail.go`.

**Steps**:
1. Create `emitInstruct(root string) error` function in the `main` package
2. Implementation: read state → `instruct.BuildContext()` → check worktree if implement mode → `instruct.Render()` → print to stdout
3. This is exactly the same logic currently in `nextCmd` lines 101-118 and in `instructCmd`, extracted to a shared function
4. Write a simple test confirming it renders without error for each mode

**Verification**:
- [ ] `emitInstruct()` renders guidance matching `mindspec instruct` for any mode
- [ ] Callable from any command handler in the `main` package

**Depends on**: nothing

---

## Bead 008b-5: `mindspec approve spec` and `mindspec approve plan` commands

**Scope**: New `cmd/mindspec/approve.go`, update `cmd/mindspec/root.go`. New `internal/approve/` package for approval logic (frontmatter update, validation, gate resolution, state transition).

**Steps**:
1. Create `internal/approve/spec.go` with `ApproveSpec(root, specID)`:
   - Call `validate.ValidateSpec()` — fail if errors
   - Read spec file, find `## Approval` section, replace Status/ApprovedBy/Date lines
   - Call `FindGate("[GATE spec-approve <id>]")` + `ResolveGate()` — warn if not found
   - Call `state.SetMode(root, "plan", specID, "")`
   - Return structured result
2. Create `internal/approve/plan.go` with `ApprovePlan(root, specID)`:
   - Call `validate.ValidatePlan()` — fail if errors
   - Read plan file, parse YAML frontmatter, update `status`, `approved_at`, `approved_by`
   - Call `FindGate("[GATE plan-approve <id>]")` + `ResolveGate()` — warn if not found
   - Call `state.SetMode(root, "implement", specID, "")`
   - Return structured result
3. Create `cmd/mindspec/approve.go` with `approveCmd` (parent) + `approveSpecCmd` and `approvePlanCmd` (sub-commands)
4. Each sub-command: call approve function → print result summary → call `emitInstruct(root)`
5. Register in `root.go`
6. Write tests for approve logic (mock bd calls and file I/O)

**Verification**:
- [ ] `mindspec approve spec <id>` does full approval flow + emits plan mode guidance
- [ ] `mindspec approve plan <id>` does full approval flow + emits implement mode guidance
- [ ] Validation failure → exit 1 with error
- [ ] No gate → warning, proceeds anyway
- [ ] `make test` passes

**Depends on**: 008b-1, 008b-4

---

## Bead 008b-6: `mindspec complete` instruct-tail + `mindspec next` refactor

**Scope**: `cmd/mindspec/complete.go`, `cmd/mindspec/next.go`, `internal/complete/complete.go`.

**Steps**:
1. In `cmd/mindspec/complete.go`: after `complete.FormatResult(result)`, call `emitInstruct(root)` to emit guidance for the new mode
2. Keep `FormatResult()` as a brief transition summary (bead closed, worktree removed) before the full instruct output — the instruct output provides the operating context for what to do next
3. In `cmd/mindspec/next.go`: replace the inline instruct logic (lines 101-118) with a call to `emitInstruct(root)`
4. Verify behavior is preserved — `next` should still show the same output
5. Test both commands

**Verification**:
- [ ] `mindspec complete` emits instruct for new mode
- [ ] `mindspec next` uses shared helper, same output as before
- [ ] `make test` passes

**Depends on**: 008b-4

---

## Bead 008b-7: Simplify approval skills + doc-sync

**Scope**: `.claude/commands/spec-approve.md`, `.claude/commands/plan-approve.md`, `CLAUDE.md`, `docs/core/CONVENTIONS.md`.

**Steps**:
1. Rewrite `.claude/commands/spec-approve.md`: identify active spec → run `mindspec approve spec <id>` → report result. Remove all frontmatter editing, validation, state management, and multi-step procedures.
2. Rewrite `.claude/commands/plan-approve.md`: identify active spec → run `mindspec approve plan <id>` → report result. Same simplification.
3. Update `CLAUDE.md` command table: add `mindspec approve spec/plan <id>`, note skills are thin wrappers
4. Update `docs/core/CONVENTIONS.md`:
   - Add gate title conventions: `[GATE spec-approve <id>]`, `[GATE plan-approve <id>]`
   - Add instruct-tail convention: state-changing commands emit `mindspec instruct` as tail
   - Update `mindspec approve` in tooling interface section

**Verification**:
- [ ] Skills are minimal (confirm ID + run command)
- [ ] CLAUDE.md documents approve commands
- [ ] CONVENTIONS.md documents gate and instruct-tail conventions
- [ ] No procedural logic in skill files

**Depends on**: 008b-5, 008b-6

---

## Dependency Graph

```
008b-1 (gate helpers)        008b-4 (instruct-tail helper)
  ├── 008b-2 (spec gate)       ├── 008b-5 (approve commands)
  │     └── 008b-3 (plan gate) │     └── 008b-7 (skills + docs)
  ├── 008b-3 (plan gate)       └── 008b-6 (complete/next refactor)
  └── 008b-5 (approve commands)      └── 008b-7 (skills + docs)
```

Two independent starting points: **008b-1** (gate primitives) and **008b-4** (instruct-tail helper). These can be implemented in parallel. Everything converges at **008b-7** (skills + docs).
