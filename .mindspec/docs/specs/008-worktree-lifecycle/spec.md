# Spec 008: Workflow Lifecycle — Worktrees + Molecules

## Goal

Make the implementation happy path seamless: `mindspec next` claims work, creates a worktree, and emits guidance. `mindspec complete` closes the bead, cleans the worktree, and advances state. Plan decomposition uses Beads molecules instead of custom DAG code. All worktree and bead CRUD is delegated to Beads — MindSpec provides orchestration only.

## Background

Spec 007 introduced bead lifecycle tooling (`mindspec bead spec/plan/worktree/hygiene`). While functional, it has two structural problems:

### 1. Worktrees are manual, not implicit

`mindspec bead worktree <bead-id> --create` exists but agents must remember to call it. There's no completion command — bead close-out is a multi-step checklist. The worktree code (`internal/bead/worktree.go`) reimplements `git worktree` parsing and creation, bypassing Beads' `bd worktree` infrastructure (which handles redirects, daemon-awareness, and safety checks).

### 2. Plan decomposition reimplements molecules

`mindspec bead plan` (`internal/bead/plan.go`) parses work_chunks YAML, creates beads one-by-one with `bd create`, and wires dependencies with `bd dep add`. This is exactly what Beads molecules do natively. Beads provides formulas (DAG templates), `bd mol pour` (instantiation), `bd mol ready` (ready-work discovery), and `bd mol show --parallel` (visualization). MindSpec's custom code duplicates all of this.

### Design principle

Beads owns primitives: worktree CRUD, molecule/DAG management, bead lifecycle, ready-work discovery. MindSpec owns orchestration: when to create worktrees, when to pour molecules, naming conventions, state management, mode transitions. This aligns with ADR-0002 (Beads as passive substrate) and ADR-0003 (MindSpec owns orchestration).

### Existing code to replace

- `internal/bead/worktree.go` — `ParseWorktreeList()`, `FindWorktree()`, `CreateWorktree()`, `checkCleanTree()`. Replace with `bd worktree` calls.
- `internal/bead/plan.go` — `CreatePlanBeads()`, `WriteGeneratedBeadIDs()`. Replace with molecule creation via `bd mol pour` or equivalent.
- `internal/next/beads.go` — `QueryReady()`. Replace with `bd mol ready` for molecule-based work, fall back to `bd ready` for standalone beads.
- `internal/instruct/worktree.go` — `CheckWorktree()`. Simplify to use `bd worktree info`.
- `cmd/mindspec/bead.go` — `beadWorktreeCmd`, `beadPlanCmd`. Deprecate.

### Parallel workstreams

ADR-0007 (Proposed) explores per-worktree state for parallel spec/plan work. This spec extends naturally to other phases once ADR-0007 is accepted, but v1 scope is implementation beads only.

## Impacted Domains

- workflow: worktree lifecycle and plan decomposition become implicit in workflow commands
- tracking: Beads owns worktree CRUD and molecule DAG; MindSpec coordinates with bead/molecule status

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Beads as passive tracking substrate. Worktree CRUD and molecule management are Beads primitives; MindSpec orchestrates when they're called.
- [ADR-0003](../../adr/ADR-0003.md): MindSpec owns worktree conventions and orchestration rules.
- [ADR-0005](../../adr/ADR-0005.md): State file tracks active bead; completion resets state. `mindspec complete` coordinates worktree cleanup with state advancement.
- [ADR-0007](../../adr/ADR-0007.md) (Proposed): Per-worktree state for parallel workstreams.

## Requirements

### Workflow entry: `mindspec next`

1. **Creates worktree after claiming bead** — After claiming a bead and setting state to implement, `mindspec next` calls `bd worktree create worktree-<bead-id> --branch bead/<bead-id>`. Prints the worktree path and instructs the agent to `cd` into it. If a worktree already exists (checked via `bd worktree list`), prints the existing path.
2. **Uses molecule-aware ready-work discovery** — When beads are molecule children (created by `mindspec bead plan`), `mindspec next` uses `bd mol ready` to find unblocked work. Falls back to `bd ready` for standalone beads. This replaces the custom `QueryReady()` in `internal/next/beads.go`.

### Workflow exit: `mindspec complete`

3. **New `mindspec complete [bead-id]` command** — Orchestrates the full bead close-out:
   - Validates all changes are committed (clean worktree via `git status --porcelain`)
   - Closes the bead (`bd close <id>`)
   - Removes the worktree (`bd worktree remove worktree-<bead-id>`)
   - Advances state (next bead, back to plan, or idle per ADR-0005)
   - Reports what was done
   - The bead ID defaults to the `activeBead` from state if not provided.
4. **`implement.md` template update** — The completion checklist references `mindspec complete` as the single close-out step, replacing the current multi-step checklist.

### Molecule-based plan decomposition

5. **Replace `CreatePlanBeads()` with molecule creation** — `mindspec bead plan <spec-id>` creates a Beads molecule from the plan's `work_chunks` YAML instead of creating individual beads with `bd create` + `bd dep add`. The plan's spec bead becomes the molecule parent. Work chunks become molecule children with dependencies expressed via Beads' native DAG model.
6. **Formula or direct creation** — Two implementation options (to be decided during planning):
   - **Formula approach**: Convert `work_chunks` YAML to a Beads formula file, then `bd mol pour <formula>` to instantiate. Reusable if the same plan shape recurs.
   - **Direct approach**: Create the molecule parent, then `bd create --parent` for each child + `bd dep add` for dependencies. Simpler, no formula file needed. Same end result.
   - Either way, `WriteGeneratedBeadIDs()` writes molecule child IDs back to plan frontmatter.
7. **Molecule-aware status and visualization** — `bd mol show <id> --parallel` replaces custom work-chunk status reporting. `bd mol ready` replaces custom ready-work filtering.

### Deprecation

8. **Deprecate `mindspec bead worktree`** — Print deprecation notice pointing to `bd worktree list` and `mindspec complete`.
9. **Deprecate `mindspec bead plan`** — Replace with molecule-based creation. Print deprecation notice during transition.
10. **Remove custom worktree parsing** — `internal/bead/worktree.go` no longer parses `git worktree list --porcelain` or calls `git worktree add` directly.
11. **Remove custom DAG creation** — `internal/bead/plan.go`'s `CreatePlanBeads()` loop-and-wire logic replaced by molecule creation.

### Conventions

12. **Worktree naming** — Path: `../worktree-<bead-id>`. Branch: `bead/<bead-id>`. Passed as arguments to `bd worktree create`.
13. **Beads daemon** — Worktrees should use `--no-daemon` mode or the sync-branch feature. Document this requirement.

## Scope

### In Scope
- `cmd/mindspec/complete.go` — new `mindspec complete` command
- `cmd/mindspec/next.go` — add worktree creation, switch to molecule-aware ready-work discovery
- `internal/next/beads.go` — replace `QueryReady()` with `bd mol ready` / `bd ready`
- `internal/bead/plan.go` — replace `CreatePlanBeads()` with molecule creation
- `internal/bead/worktree.go` — replace with `bd worktree` calls
- `internal/bead/bdcli.go` — add wrappers for `bd worktree create/list/remove` and `bd mol pour/ready/show`
- `internal/instruct/worktree.go` — simplify to use `bd worktree info`
- `cmd/mindspec/bead.go` — deprecate `beadWorktreeCmd` and `beadPlanCmd`
- `internal/instruct/templates/implement.md` — update completion checklist
- `cmd/mindspec/root.go` — register `complete` command
- Updates to `CLAUDE.md`, `docs/core/CONVENTIONS.md`

### Out of Scope
- Worktrees for spec/plan phases (pending ADR-0007)
- Changes to Beads CLI itself
- Formula library / reusable templates (future enhancement)
- Human gates for approval (see 008b)
- `bd prime` composition into `mindspec instruct` (see 008c)

## Non-Goals

- Building a standalone `mindspec worktree` command tree — Beads already provides `bd worktree`
- Building a standalone `mindspec mol` command tree — Beads already provides `bd mol`
- Worktree-per-spec or worktree-per-plan (pending ADR-0007)
- Branch protection / PR-based merging to main (see ADR-0006, Proposed)

## Acceptance Criteria

### Workflow entry (`mindspec next`)
- [ ] `mindspec next` creates a worktree via `bd worktree create worktree-<bead-id> --branch bead/<bead-id>` and prints the path
- [ ] `mindspec next` reuses an existing worktree if one already exists for the claimed bead
- [ ] `mindspec next` discovers ready work via `bd mol ready` when beads are molecule children
- [ ] `mindspec next` falls back to `bd ready` for standalone beads

### Workflow exit (`mindspec complete`)
- [ ] `mindspec complete` closes the bead, removes the worktree via `bd worktree remove`, and advances state
- [ ] `mindspec complete` with no argument uses the `activeBead` from state
- [ ] `mindspec complete` refuses if the worktree has uncommitted changes, with exit code 1
- [ ] The `implement.md` instruction template references `mindspec complete` as the single close-out step

### Molecule-based plan decomposition
- [ ] `mindspec bead plan <spec-id>` creates a Beads molecule from work_chunks (not individual beads with manual dep wiring)
- [ ] Molecule children have correct dependencies matching work_chunks `depends_on`
- [ ] Spec bead is the molecule parent
- [ ] `WriteGeneratedBeadIDs()` writes molecule child IDs back to plan frontmatter
- [ ] Idempotent: re-running does not create duplicates
- [ ] `bd mol show <molecule-id>` displays the work DAG

### Beads integration
- [ ] Worktree creation uses `bd worktree create` (not raw `git worktree add`)
- [ ] Worktree removal uses `bd worktree remove` (not raw `git worktree remove`)
- [ ] `bd worktree list` and `bd worktree info` work with MindSpec-created worktrees

### Deprecation
- [ ] `mindspec bead worktree` prints a deprecation notice
- [ ] `mindspec bead plan` prints a deprecation notice (molecule creation replaces it)
- [ ] `internal/bead/worktree.go` no longer contains custom `git worktree list --porcelain` parsing
- [ ] `internal/bead/plan.go` no longer contains loop-and-wire bead creation

### General
- [ ] All new code has unit tests; `make test` passes
- [ ] Doc-sync: CLAUDE.md and CONVENTIONS.md updated

## Validation Proofs

- `./bin/mindspec bead plan <spec-id>`: Creates a molecule; `bd mol show <id>` displays the DAG
- `./bin/mindspec next`: Claims a molecule child, creates worktree, prints path
- `bd worktree list`: Shows the worktree created by `mindspec next`
- `./bin/mindspec complete`: Closes bead, removes worktree, advances state
- `make test`: All tests pass

## Open Questions

None — all resolved.

### Resolved

- ~~Should worktrees be created implicitly by workflow commands?~~ **Resolved**: Yes. `mindspec next` creates, `mindspec complete` cleans.
- ~~Should MindSpec implement its own worktree CRUD?~~ **Resolved**: No. Delegate to `bd worktree`.
- ~~Should MindSpec implement its own DAG creation?~~ **Resolved**: No. Use Beads molecules.
- ~~Formula vs direct molecule creation?~~ **Resolved**: Deferred to planning. Either approach achieves the same result.
- ~~Should MindSpec state be stored in Beads?~~ **Resolved**: No. Different concerns, different stores. See ADR-0007.
- ~~Parallel spec/plan work?~~ **Resolved**: Deferred to ADR-0007.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via /spec-approve workflow. Merged from original 008 (worktree lifecycle) + 008a (molecule-based plan decomposition).
