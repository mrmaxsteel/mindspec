# MindSpec Happy Path: Developing a Feature

This guide walks through the complete lifecycle of developing a feature with MindSpec — from idea to completion. It covers what the human does, what the agent does, and what CLI commands orchestrate each step.

## Overview

MindSpec enforces a three-phase gated lifecycle: **Spec → Plan → Implement → Review**. Every phase transition requires explicit human approval. The agent cannot skip ahead.

```
Idle ──→ Spec Mode ──human gate──→ Plan Mode ──human gate──→ Implementation Mode ──→ Review Mode ──human gate──→ Idle
```

All agent operating guidance is emitted dynamically by `mindspec instruct` (run automatically on session start). Static files like CLAUDE.md and AGENTS.md are minimal bootstraps — the CLI is the source of truth for what the agent should do in each mode.

---

## Phase 0: Project Bootstrap

If the project has not been set up yet, run `mindspec init` to scaffold the full directory structure, starter files (GLOSSARY.md, CLAUDE.md, context-map, policies, state), and domain templates. All creation is additive — existing files are never overwritten. After init, `mindspec doctor` should report zero errors.

## Phase 0.5: Idle

**State**: `mode: idle` (or no `.mindspec/state.json`)

On session start, the SessionStart hook runs `mindspec instruct`, which emits idle-mode guidance. The agent is directed to greet the user, list available specs, and suggest next steps:
- `/spec-init` to draft a new specification
- Resuming an existing spec
- `mindspec doctor` to check project health

---

## Phase 1: Spec Mode

### Human says
"I want to build feature X" or invokes `/spec-init`

### Agent does
1. Asks for spec ID (e.g. `009-feature-name`), title, and context
2. Creates `docs/specs/009-feature-name/spec.md` from template
3. Creates placeholder `context-pack.md`
4. Runs `mindspec state set --mode=spec --spec=009-feature-name`
5. Tells the human they're in Spec Mode

### Iterative collaboration
Agent and human fill in the spec — Goal, Requirements, Acceptance Criteria, Impacted Domains, ADR Touchpoints, Open Questions. **Only markdown artifacts are permitted** — no code, no tests.

### Available CLI commands
| Command | Purpose |
|---------|---------|
| `mindspec instruct` | Re-emit spec mode guidance |
| `mindspec validate spec <id>` | Structural quality check |
| `mindspec context pack <id>` | Generate context pack on demand |

---

## Phase 2: Spec Approval (Human Gate)

### Human invokes
`/spec-approve` (typing the command is the approval — no second confirmation)

### Agent does
Runs `mindspec approve spec <id>`

### What the CLI does (single command)
1. **Validates** — `validate.ValidateSpec()` checks required sections, acceptance criteria quality
2. **Updates frontmatter** — sets `Status: APPROVED` in spec.md
3. **Closes molecule step** — closes the `spec-approve` step in the spec-lifecycle molecule (created at `spec-init`), which unblocks the `plan` step
4. **Generates context pack** — runs `contextpack.Build()` automatically (best-effort)
5. **Sets state** — `{mode: "plan", activeSpec: "<id>"}`
6. **Instruct-tail** — emits Plan Mode guidance

### Agent immediately begins planning
The spec approval **is** the authorization to start planning — no second confirmation needed.

---

## Phase 3: Plan Mode

### Agent does
1. Reviews domain docs and accepted ADRs for impacted domains
2. Reviews Context Map for neighbor contracts
3. Creates `docs/specs/<id>/plan.md` with YAML frontmatter (`status: Draft`)
4. Decomposes the spec into `work_chunks` — each with id, title, scope, verify steps, and dependencies
5. Writes the plan body using `## Bead N:` headings with `**Steps**`, `**Verification**`, and `**Depends on**` subsections
6. Iteratively refines with the human

### What a plan looks like
```yaml
---
status: Draft
spec_id: 009-feature-name
work_chunks:
  - id: 1
    title: "Core data model"
    scope: "internal/pkg/model.go"
    verify: ["Unit tests pass", "Struct matches spec schema"]
    depends_on: []
  - id: 2
    title: "CLI command wiring"
    scope: "cmd/mindspec/feature.go"
    verify: ["--help output correct", "Integration test passes"]
    depends_on: [1]
---

## Bead 1: Core data model

**Steps**
1. Define struct in `internal/pkg/model.go`
2. Add unit tests

**Verification**
- [ ] Unit tests pass
- [ ] Struct matches spec schema

**Depends on**: (none)

## Bead 2: CLI command wiring

**Steps**
1. Create `cmd/mindspec/feature.go`
2. Register with root command

**Verification**
- [ ] `--help` output correct
- [ ] Integration test passes

**Depends on**: Bead 1
```

---

## Phase 4: Plan Approval (Human Gate)

### Human invokes
`/plan-approve` (typing the command is the approval — no second confirmation)

### Agent does
Runs `mindspec approve plan <id>`

### What the CLI does (single command)
1. **Validates** — `validate.ValidatePlan()` checks frontmatter, bead sections, verification steps
2. **Updates frontmatter** — sets `status: Approved`, `approved_at`, `approved_by`
3. **Closes molecule step** — closes the `plan-approve` step in the spec-lifecycle molecule, which unblocks the `implement` step
4. **Sets state** — stays `plan` mode (deliberately NOT implement — need to claim a bead first)
5. **Instruct-tail** — emits guidance telling user to run `mindspec next`

### The agent then tells the human
> Run `mindspec next` to claim the first ready bead and enter Implementation Mode.

---

## Phase 5: Claiming Work

### Agent (or human) runs
`mindspec next`

### What the CLI does
1. **Clean tree check** — fails if uncommitted changes
2. **Query ready work** — reads `ActiveMolecule` from state, queries `bd ready --parent <mol-id>` for molecule children, falls back to `bd ready` if no molecule
3. **Display & select** — shows available beads, picks first (or `--pick=N`)
4. **Claim** — `bd update <id> in_progress`
5. **Create worktree** — `bd worktree create worktree-<beadID> bead/<beadID>`
6. **Resolve mode** — maps bead type to MindSpec mode (extracts spec ID from bracket-prefix titles like `[IMPL 009-feature.1] Chunk title`)
7. **Set state** — `{mode: "implement", activeSpec: "<id>", activeBead: "<beadID>"}`
8. **Instruct-tail** — emits Implementation Mode guidance with bead scope and obligations

The instruct-tail checks if a worktree exists for the active bead and tells the agent to switch to it if needed.

---

## Phase 6: Implementation

### Agent does (within worktree)
1. Writes code **within the bead's declared scope**
2. Creates tests
3. Updates documentation (**doc-sync is mandatory** — "done" includes doc-sync)
4. Follows cited ADRs (divergence → stop + inform human)
5. Uses commit convention: `impl(<bead-id>): <summary>`
6. Runs verification steps from the plan

### Constraints
- **Scope discipline**: new work becomes new beads, not scope creep
- **Worktree isolation**: work happens in the bead worktree, not main
- **ADR compliance**: divergence triggers a human gate

---

## Phase 7: Bead Completion

### Agent runs
`mindspec complete`

### What the CLI does
1. Reads `activeBead` from state
2. Finds matching worktree
3. **Clean tree check** — all changes must be committed (no stashing)
4. **Close bead** — `bd close <beadID>`
5. **Remove worktree** — `bd worktree remove`
6. **Advance state**:
   - If more ready beads → stays `implement`, sets next bead
   - If beads exist but blocked → transitions to `plan`
   - If all beads done → transitions to `review`
7. **Instruct-tail** — emits guidance for the new state

---

## Phase 8: Loop or Review

If `mindspec complete` found another ready bead → run `mindspec next` again, repeat Phases 5–7.

If all beads are done → state transitions to `review` mode (see Phase 9).

---

## Phase 9: Implementation Review (Human Gate)

**State**: `mode: review, activeSpec: "<id>"`

When all implementation beads are complete, the agent enters review mode. The agent verifies the work against the spec's acceptance criteria and presents a summary.

### Agent does
1. Reads the spec's acceptance criteria from `docs/specs/<id>/spec.md`
2. Runs `make test` and `make build` to confirm quality gates pass
3. Verifies doc-sync is complete
4. Presents a summary: what was built and how each acceptance criterion is satisfied

### Human invokes
`/impl-approve` (typing the command is the approval)

### Agent does
Runs `mindspec approve impl <id>`

### What the CLI does
1. **Verifies** review mode is active for the given spec
2. **Closes molecule step** — closes the `review` step in the spec-lifecycle molecule, completing the entire lifecycle
3. **Sets state** → `idle`
4. **Instruct-tail** — emits idle mode guidance

The feature is now complete.

---

## Session Close Protocol

At the end of every session, regardless of mode:

1. Commit all changes
2. Run quality gates if code changed (tests, build)
3. Update bead status
4. Run `bd sync --flush-only` to export beads to JSONL
5. Push to remote (if configured)

Work is not complete until changes are committed.

---

## Summary: Who Does What

| Step | Human | Agent | CLI Command |
|------|-------|-------|-------------|
| Start feature | "Build X" | Creates spec, sets state | `mindspec state set --mode=spec` |
| Write spec | Reviews, guides | Writes markdown | — |
| Approve spec | `/spec-approve` | Runs approval | `mindspec approve spec <id>` |
| Write plan | Reviews plan | Decomposes into beads | — |
| Approve plan | `/plan-approve` | Runs approval | `mindspec approve plan <id>` |
| Claim work | — | Claims bead | `mindspec next` |
| Implement | Reviews code | Codes + tests + doc-sync | `impl(bead): ...` commits |
| Complete bead | — | Closes bead | `mindspec complete` |
| Next bead | — | Claims next | `mindspec next` (loop) |
| Review | — | Verifies acceptance criteria | — |
| Approve impl | `/impl-approve` | Transitions to idle | `mindspec approve impl <id>` |

---

## Quick Reference: Key Commands

| Command | When to Use |
|---------|-------------|
| `mindspec init` | Bootstrap a new MindSpec project |
| `mindspec instruct` | See current mode guidance (auto-runs on session start) |
| `mindspec state show` | Check current mode, spec, and bead |
| `mindspec validate spec <id>` | Pre-check spec quality before approval |
| `mindspec validate plan <id>` | Pre-check plan quality before approval |
| `mindspec doctor` | Project health check |
| `/spec-init` | Start a new specification |
| `/spec-approve` | Approve spec → plan transition |
| `/plan-approve` | Approve plan → implement transition |
| `/impl-approve` | Approve implementation → idle |
| `mindspec next` | Claim the next ready bead |
| `mindspec complete` | Close current bead and advance |
