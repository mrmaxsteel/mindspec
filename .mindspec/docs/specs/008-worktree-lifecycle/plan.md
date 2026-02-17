---
status: Approved
approved_at: 2026-02-13T08:00:00Z
approved_by: user
spec_id: 008-worktree-lifecycle
version: "2.0"
last_updated: 2026-02-13
adr_citations:
  - id: ADR-0002
    sections: ["Parallelism Compatibility"]
  - id: ADR-0003
    sections: ["MindSpec owns worktree conventions", "CLI Contract"]
  - id: ADR-0005
    sections: ["Completion and Reset", "Commit Ordering"]
work_chunks:
  - id: 1
    title: "bd worktree and bd mol wrappers in bdcli.go"
    scope: "internal/bead/bdcli.go, internal/bead/bdcli_test.go"
    verify:
      - "WorktreeCreate() calls bd worktree create with correct name and branch args"
      - "WorktreeList() parses bd worktree list output"
      - "WorktreeRemove() calls bd worktree remove"
      - "WorktreeInfo() calls bd worktree info --json"
      - "MolPour() calls bd mol pour with formula or parent args"
      - "MolReady() calls bd mol ready --json and parses output"
      - "MolShow() calls bd mol show --json"
      - "make test passes"
    depends_on: []
  - id: 2
    title: "Molecule-based plan decomposition"
    scope: "internal/bead/plan.go, internal/bead/plan_test.go"
    verify:
      - "CreatePlanBeads() creates a molecule parent with spec bead as parent"
      - "Work chunks become molecule children with correct dependencies"
      - "Idempotent: re-run does not create duplicates"
      - "WriteGeneratedBeadIDs() writes molecule child IDs to plan frontmatter"
      - "bd mol show displays the work DAG"
      - "make test passes"
    depends_on: [1]
  - id: 3
    title: "mindspec next — worktree creation + mol ready"
    scope: "cmd/mindspec/next.go, internal/next/beads.go, internal/next/beads_test.go"
    verify:
      - "QueryReady() uses bd mol ready when molecule beads exist, falls back to bd ready"
      - "After claiming, creates worktree via bd worktree create"
      - "Reuses existing worktree if one exists for the bead"
      - "Prints worktree path and cd instruction"
      - "make test passes"
    depends_on: [1]
  - id: 4
    title: "mindspec complete — close + clean + advance"
    scope: "cmd/mindspec/complete.go, internal/complete/complete.go, internal/complete/complete_test.go"
    verify:
      - "Refuses if worktree has uncommitted changes (exit 1)"
      - "Closes bead via bd close"
      - "Removes worktree via bd worktree remove"
      - "Advances state: next bead, plan, or idle"
      - "Defaults to activeBead from state when no arg provided"
      - "make test passes"
    depends_on: [1]
  - id: 5
    title: "Template update + deprecation + doc-sync"
    scope: "internal/instruct/templates/implement.md, internal/instruct/worktree.go, cmd/mindspec/bead.go, cmd/mindspec/root.go, CLAUDE.md, docs/core/CONVENTIONS.md"
    verify:
      - "implement.md completion checklist uses mindspec complete"
      - "CheckWorktree() uses bd worktree info"
      - "mindspec bead worktree prints deprecation notice"
      - "mindspec bead plan prints deprecation notice"
      - "complete command registered in root.go"
      - "CLAUDE.md and CONVENTIONS.md updated"
      - "make build && make test passes"
    depends_on: [2, 3, 4]
---

# Plan: Spec 008 — Workflow Lifecycle: Worktrees + Molecules

**Spec**: [spec.md](spec.md)

---

## Design Notes

### Beads Primitive Delegation

MindSpec calls Beads commands for all CRUD. New wrappers in `internal/bead/bdcli.go`:

| Wrapper | Beads command | Purpose |
|:--------|:-------------|:--------|
| `WorktreeCreate(name, branch)` | `bd worktree create <name> --branch <branch>` | Create worktree with redirect |
| `WorktreeList()` | `bd worktree list --json` | List worktrees |
| `WorktreeRemove(name)` | `bd worktree remove <name>` | Remove worktree |
| `WorktreeInfo()` | `bd worktree info --json` | Current worktree info |
| `MolPour(parent, children)` | `bd mol pour` or `bd create --parent` | Create molecule |
| `MolReady()` | `bd mol ready --json` | Ready molecule children |
| `MolShow(id)` | `bd mol show <id> --json` | Molecule structure |

These follow the existing `execCommand` pattern for testability.

### Molecule Creation Strategy

The spec leaves formula vs direct creation as a planning decision. **Direct creation** is the right choice for v1:

1. Create the molecule parent: `bd create "[PLAN <spec-id>] <title>" --type=epic --parent=<spec-bead-id>`
2. For each work chunk: `bd create "[IMPL <spec-id>.<chunk-id>] <title>" --type=task --parent=<mol-parent-id>`
3. Wire dependencies: `bd dep add <blocked> <blocker>`

This is nearly identical to the current `CreatePlanBeads()` flow but uses an epic as the molecule parent. The key difference: Beads now knows the parent-child relationship natively, enabling `bd mol ready` and `bd mol show`.

Formula creation is a future enhancement once the pattern stabilizes.

### Molecule-Aware Ready Work

`mindspec next` currently calls `bd ready --json`. The change:

1. Try `bd mol ready --json` first — returns unblocked molecule children
2. If no results (no molecules exist), fall back to `bd ready --json`
3. Parse results through existing `ParseBeadsJSON()`

The `BeadInfo` struct doesn't change — molecule children are beads.

### `mindspec complete` Flow

```
mindspec complete [bead-id]
  │
  ├─ 1. Read state → get activeBead if no arg
  ├─ 2. Find worktree → bd worktree list, match by bead ID
  ├─ 3. Validate clean → git -C <wt-path> status --porcelain
  ├─ 4. Close bead → bd close <bead-id>
  ├─ 5. Remove worktree → bd worktree remove worktree-<bead-id>
  ├─ 6. Advance state:
  │     ├─ bd mol ready → more children ready? → set next bead
  │     ├─ bd mol ready → none ready but open? → set mode=plan
  │     └─ all closed? → set mode=idle
  └─ 7. Print summary
```

The state advancement logic from ADR-0005's "Completion and Reset" section is built into this command, so agents no longer need to remember to advance state manually.

### Worktree Naming

Passed to `bd worktree create` as arguments:
- Name: `worktree-<bead-id>` (e.g., `worktree-bd-a1b2`)
- Branch: `bead/<bead-id>` (e.g., `bead/bd-a1b2`)
- Location: Beads places worktrees as siblings to the project root by default

### Deprecation Strategy

`beadWorktreeCmd` and `beadPlanCmd` in `cmd/mindspec/bead.go` get a deprecation wrapper:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    fmt.Fprintln(os.Stderr, "DEPRECATED: use 'bd worktree list' or 'mindspec complete' instead")
    // ... existing logic for backward compat during transition ...
}
```

The underlying `internal/bead/worktree.go` functions are replaced with `bd worktree` calls. The custom `git worktree list --porcelain` parsing is removed.

### Exit Codes

Consistent with existing conventions:
- 0: success
- 1: validation failure (dirty tree, bead not in expected state)
- 2: Beads CLI error (`bd` command failed)

---

## Bead 008-1: bd worktree and bd mol wrappers

**Scope**: `internal/bead/bdcli.go`, `internal/bead/bdcli_test.go`

**Steps**:
1. Define `WorktreeInfo` struct (Path, Branch, BeadID, IsMain). Add worktree wrappers: `WorktreeCreate(name, branch)`, `WorktreeList()`, `WorktreeRemove(name)`, `WorktreeInfo()` — each shells out to the corresponding `bd worktree` subcommand
2. Add molecule wrappers: `MolReady()` (returns `[]BeadInfo`), `MolShow(id)` (returns raw output) — shells out to `bd mol ready --json` and `bd mol show`
3. Add `Close(id string) error` — runs `bd close <id>` (currently missing from bdcli.go)
4. Write tests: argument construction via execCommand mock, JSON parsing for new structs, WorktreeList output format

**Verification**:
- [ ] All new wrappers construct correct `bd` arguments
- [ ] JSON parsing handles `bd worktree list` output format
- [ ] `MolReady()` returns `[]BeadInfo` compatible with existing `SelectWork()`
- [ ] `make test` passes

**Depends on**: nothing

---

## Bead 008-2: Molecule-based plan decomposition

**Scope**: `internal/bead/plan.go`, `internal/bead/plan_test.go`

**Steps**:
1. Modify `CreatePlanBeads()` — instead of creating flat beads, create a molecule parent first: `Create("[PLAN <spec-id>] <title>", desc, "epic", 2, specBeadID)`
2. Create children under the molecule: `Create("[IMPL <spec-id>.<id>] <chunk-title>", desc, "task", 2, molParentID)` — same as current but `parent` is the molecule, not the spec bead
3. Wire dependencies with `DepAdd()` — same as current loop
4. Update idempotency: search for `[PLAN <spec-id>]` to find existing molecule parent; search for `[IMPL <spec-id>.<id>]` for existing children
5. `WriteGeneratedBeadIDs()` — add molecule parent ID alongside child IDs in `generated` block
6. Update tests: verify molecule parent creation, child-parent relationship, dep wiring, idempotency

**Verification**:
- [ ] Creates epic bead as molecule parent with spec bead as its parent
- [ ] Work chunks become children of the molecule parent
- [ ] Dependencies wired correctly between children
- [ ] Idempotent on re-run
- [ ] `WriteGeneratedBeadIDs()` includes molecule parent ID
- [ ] `make test` passes

**Depends on**: 008-1 (needs `Close()` wrapper; molecule parent uses `Create()` with type=epic)

---

## Bead 008-3: mindspec next — worktree creation + mol ready

**Scope**: `cmd/mindspec/next.go`, `internal/next/beads.go`, `internal/next/beads_test.go`

**Steps**:
1. Modify `QueryReady()` — try `bead.MolReady()` first; if empty result, fall back to `bead.ListOpen()` filtered to ready (current `bd ready` call)
2. After `ClaimBead()` in `next.go`, add worktree creation:
   - Check for existing worktree: `bead.WorktreeList()` → match by bead ID
   - If not found: `bead.WorktreeCreate("worktree-"+beadID, "bead/"+beadID)`
   - Print worktree path and `cd` instruction
3. If worktree already exists, print existing path
4. Update `ClaimBead()` to use the `bead` package's `Update()` instead of its own exec call (reduce duplication)
5. Write tests: mol ready preference, fallback to bd ready, worktree creation after claim, existing worktree reuse

**Verification**:
- [ ] Uses `bd mol ready` when molecule beads exist
- [ ] Falls back to `bd ready` for standalone beads
- [ ] Creates worktree via `bd worktree create` after claiming
- [ ] Reuses existing worktree, prints path
- [ ] `make test` passes

**Depends on**: 008-1 (needs `WorktreeCreate`, `WorktreeList`, `MolReady`)

---

## Bead 008-4: mindspec complete

**Scope**: `cmd/mindspec/complete.go`, `internal/complete/complete.go`, `internal/complete/complete_test.go`

**Steps**:
1. Create `internal/complete/complete.go` with `Run(root, beadID string) error`:
   - Read state → use `activeBead` if beadID is empty
   - Find worktree via `bead.WorktreeList()` matching bead ID
   - Check clean tree: `git -C <wt-path> status --porcelain`
   - Close bead: `bead.Close(beadID)`
   - Remove worktree: `bead.WorktreeRemove("worktree-" + beadID)`
   - Advance state: check `bead.MolReady()` for next work; set mode accordingly
   - Return summary of actions
2. Create `cmd/mindspec/complete.go` — cobra command, 0 or 1 args, calls `complete.Run()`
3. Handle edge cases: no worktree found (bead worked on main), worktree already removed, bead already closed
4. Write tests: clean flow, dirty tree refusal, state advancement logic, no-worktree case

**Verification**:
- [ ] Refuses on dirty worktree (exit 1)
- [ ] Closes bead via `bd close`
- [ ] Removes worktree via `bd worktree remove`
- [ ] Advances state correctly (next bead / plan / idle)
- [ ] Defaults to `activeBead` from state
- [ ] Handles edge cases gracefully
- [ ] `make test` passes

**Depends on**: 008-1 (needs `Close`, `WorktreeList`, `WorktreeRemove`, `MolReady`)

---

## Bead 008-5: Template update + deprecation + doc-sync

**Scope**: `internal/instruct/templates/implement.md`, `internal/instruct/worktree.go`, `cmd/mindspec/bead.go`, `cmd/mindspec/root.go`, `CLAUDE.md`, `docs/core/CONVENTIONS.md`

**Steps**:
1. Update `implement.md` completion checklist — replace multi-step checklist with `mindspec complete`
2. Simplify `internal/instruct/worktree.go` `CheckWorktree()` — use `bead.WorktreeInfo()` instead of custom `git worktree list` parsing
3. Add deprecation notices to `beadWorktreeCmd` and `beadPlanCmd` in `cmd/mindspec/bead.go`
4. Register `completeCmd` in `root.go` init()
5. Update `CLAUDE.md` — add `mindspec complete` to command table and Build & Run section
6. Update `docs/core/CONVENTIONS.md` — document molecule conventions, worktree lifecycle via `bd worktree`
7. Remove `internal/bead/worktree.go` custom git parsing (replaced by `bd worktree` wrappers)

**Verification**:
- [ ] `implement.md` uses `mindspec complete` as single close-out step
- [ ] `CheckWorktree()` uses `bd worktree info`
- [ ] Deprecation notices printed for `bead worktree` and `bead plan`
- [ ] `complete` command registered and accessible
- [ ] `CLAUDE.md` and `CONVENTIONS.md` updated
- [ ] `make build && make test` passes

**Depends on**: 008-2, 008-3, 008-4

---

## Dependency Graph

```
008-1 (bd worktree + bd mol wrappers)
  ├── 008-2 (molecule-based plan decomposition)
  ├── 008-3 (mindspec next — worktree + mol ready)
  └── 008-4 (mindspec complete)
        ↓ all three
      008-5 (template + deprecation + doc-sync)
```

008-2, 008-3, and 008-4 are parallelizable after 008-1.

---

## End-to-End Verification

```bash
make build && make test

# Create plan beads as molecule
./bin/mindspec bead plan 008-worktree-lifecycle
bd mol show <molecule-id>

# Claim and start work
./bin/mindspec next
# → creates worktree, prints path

# Complete work
./bin/mindspec complete
# → closes bead, removes worktree, advances state

# Inspect
bd worktree list
./bin/mindspec state show
```
