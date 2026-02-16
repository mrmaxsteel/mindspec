# Spec 032-beads-formula-gates: Native Beads Integration

## Goal

Rearchitect mindspec's beads integration from a deep wrapping layer (39 exported Go functions shelling out to `bd`) to a thin composition layer that uses beads as the workflow engine. Mindspec becomes an opinionated agentic workflow that sits on top of beads — owning spec artifacts, validation, context engineering, and guidance emission while delegating all work tracking, dependency enforcement, and molecule orchestration to beads natively.

## Background

### The wrapping problem

Mindspec maintains `internal/bead/` — effectively a Go SDK for the `bd` CLI. Every function shells out to `bd`, parses JSON, and returns Go structs. This creates a fragile coupling surface:

- **gate.go** has been broken since inception (`bd create --type=gate` was never valid — gates are formula step primitives, not standalone issue types). Silent fallbacks hid the failure, meaning approval gates were never enforced.
- **propagate.go** reimplements parent-child status propagation that beads molecules handle natively.
- **plan.go** manually creates molecules by calling `Create()` + `DepAdd()` in loops — `bd pour` does this in one command.
- **next/beads.go** reimplements molecule-aware work discovery — `bd ready --parent` already does this.

### What mindspec actually owns

Analysis shows mindspec is ~40% pure domain logic, ~60% beads wrapping. The pure value is:

1. **Spec artifact lifecycle** — spec.md, plan.md, context-pack.md templates and validation
2. **Workflow state machine** — 5-mode lifecycle (idle → spec → plan → implement → review)
3. **Context engineering** — mode-aware context pack curation from domain docs, ADRs, policies, glossary
4. **Dynamic guidance** — `mindspec instruct` emits phase-appropriate operating instructions
5. **Validation gates** — structural quality checks on specs and plans
6. **ADR management** — domain-aware filtering, supersession chains, template creation (beads has no native ADR support)
7. **Workset hygiene** — stale/orphan/oversized detection rules specific to spec-driven development

None of this requires wrapping beads. It requires *composing* with beads.

### The beads-native approach

Beads provides everything mindspec needs for work tracking:

| Mindspec needs | Beads provides |
|---|---|
| Lifecycle phases with gates | Formulas with `type = "human"` steps + `needs` dependencies |
| Molecule creation from plan | `bd pour <formula> --var spec_id=<id>` |
| Work discovery | `bd ready` (respects `needs` dependencies) |
| Approval enforcement | Step dependencies — downstream steps blocked until predecessor closes |
| Parent-child status sync | Molecule parent auto-progression |
| Work claiming | `bd pin <id> --start` |
| Bead close + next | `bd close <id>` then `bd ready` |

## Impacted Domains

- **bead**: `internal/bead/` — delete gate.go, spec.go, plan.go, propagate.go; reduce bdcli.go to a minimal `bd` exec helper (JSON parsing, error handling) used only where multi-step orchestration genuinely benefits from Go
- **approve**: `internal/approve/*.go` — simplify to: validate artifact → update frontmatter → `bd close <step-id>` → transition state
- **specinit**: `internal/specinit/specinit.go` — add `bd pour spec-lifecycle --var spec_id=<id>`
- **complete**: `internal/complete/complete.go` — simplify to `bd close` + `bd ready` + state transition
- **next**: `internal/next/beads.go` — replace molecule-aware discovery with `bd ready`
- **state**: `internal/state/state.go` — add `ActiveMolecule` field
- **instruct**: `internal/instruct/worktree.go` — inline the one `bd worktree list` call

## ADR Touchpoints

- [ADR-0012](../../adr/ADR-0012.md): Compose with external CLIs, don't wrap them — this spec is the primary implementation of that decision

## Requirements

### Formula & Molecule

1. Define `.beads/formulas/spec-lifecycle.formula.toml` with steps:
   - `spec` (task) — write the spec
   - `spec-approve` (human, needs: spec) — human approval gate
   - `plan` (task, needs: spec-approve) — write the plan
   - `plan-approve` (human, needs: plan) — human approval gate
   - `implement` (task, needs: plan-approve) — implementation work
   - `review` (human, needs: implement) — final review
2. `mindspec spec-init` pours the formula: `bd pour spec-lifecycle --var spec_id=<id> --json`
3. Store molecule root ID in `.mindspec/state.json` (`activeMolecule` field)
4. `mindspec approve spec` = validate spec + update frontmatter + `bd close <spec-approve-step>` + set state to plan
5. `mindspec approve plan` = validate plan + update frontmatter + `bd close <plan-approve-step>` + set state to implement
6. `mindspec complete` = `bd close <current-bead>` + `bd ready` to find next + state transition
7. `mindspec next` = `bd ready` + `bd pin <id> --start` + set state

### Delete the wrapper layer

8. Delete `internal/bead/gate.go` — replaced by molecule step dependencies
9. Delete `internal/bead/spec.go` — replaced by `bd pour` at spec-init
10. Delete `internal/bead/plan.go` — plan decomposition moves to approve/plan.go as direct `bd` calls (or replaced entirely by formula sub-steps if plan work chunks map to formula variables)
11. Delete `internal/bead/propagate.go` — molecules handle this natively
12. Reduce `internal/bead/bdcli.go` — keep only: `Preflight()`, `RunBD()` (generic exec helper with JSON parsing), and the `BeadInfo` struct. Delete all single-purpose wrapper functions.
13. Keep `internal/bead/hygiene.go` — genuine domain logic (stale/orphan/oversized rules) that happens to query beads

### Keep ADRs as-is

14. Keep `internal/adr/` unchanged — beads has no native ADR support; mindspec's domain-aware filtering, supersession chains, and context pack integration are unique value

### Backward Compatibility

15. Pre-032 specs without molecules continue to work — approve commands check for molecule first, fall back to direct state transition if none exists

## Scope

### In Scope

- `.beads/formulas/spec-lifecycle.formula.toml` — new formula definition
- `internal/bead/gate.go` — delete
- `internal/bead/spec.go` — delete
- `internal/bead/plan.go` — delete
- `internal/bead/propagate.go` — delete
- `internal/bead/bdcli.go` — reduce to minimal exec helper
- `internal/bead/hygiene.go` — keep, minor updates to use new exec helper
- `internal/approve/spec.go` — simplify
- `internal/approve/plan.go` — simplify
- `internal/approve/impl.go` — simplify
- `internal/complete/complete.go` — simplify
- `internal/next/beads.go` — simplify
- `internal/specinit/specinit.go` — add `bd pour`
- `internal/state/state.go` — add `ActiveMolecule`
- `internal/instruct/worktree.go` — inline bd call
- Test files for all changed packages
- New ADR documenting compose-don't-wrap principle

### Out of Scope

- Timer or GitHub gates (only human gates needed)
- Formula aspects, bond points, or wisps
- Changing the spec folder layout or state machine modes
- `cmd/mindspec/bead.go` CLI subcommands (keep as manual escape hatches)
- `internal/adr/` (no changes — unique mindspec value)
- `internal/validate/` (no changes — pure mindspec logic)
- `internal/contextpack/` (no changes — pure mindspec logic)
- `internal/glossary/` (no changes — pure mindspec logic)

## Non-Goals

- Zero beads code in Go — `Preflight()`, hygiene reporting, and a generic `RunBD()` helper are worth keeping
- Changing the mindspec CLI UX — same commands, same interface, different internals
- Migrating existing closed/completed specs to formulas
- Replacing mindspec's ADR system with beads labels/decisions

## Acceptance Criteria

- [ ] `.beads/formulas/spec-lifecycle.formula.toml` exists with 6 steps and correct `needs` chain
- [ ] `mindspec spec-init 999-test` creates a beads molecule via `bd pour`; `bd mol list --json` confirms it
- [ ] `bd dep tree <mol-id>` shows 6 steps with correct dependency chain
- [ ] `mindspec approve spec 999-test` closes the spec-approve step; `bd ready` shows the plan step
- [ ] `mindspec approve plan 999-test` fails if spec-approve step is still open
- [ ] `mindspec approve plan 999-test` succeeds after spec approval, closes plan-approve step
- [ ] `.mindspec/state.json` contains `activeMolecule` after spec-init
- [ ] `internal/bead/gate.go`, `spec.go`, `plan.go`, `propagate.go` are deleted
- [ ] `internal/bead/` exports fewer than 10 functions (down from 39)
- [ ] Pre-032 specs without molecules still work through approve flow
- [ ] `internal/adr/` is unchanged
- [ ] ADR-0012 (compose, don't wrap) is cited in plan frontmatter
- [ ] `make test` passes with no new failures

## Validation Proofs

- `mindspec spec-init 999-test && bd mol list --json`: molecule exists with spec-lifecycle formula
- `bd dep tree <mol-id>`: shows step hierarchy with human gate steps
- `mindspec approve spec 999-test && bd ready`: plan step appears in ready queue
- `mindspec approve plan 999-test` (before spec approval): non-zero exit
- `wc -l internal/bead/*.go`: significant line count reduction (target: <200 lines total, down from ~800+)
- `make test`: all tests pass

## Open Questions

- [ ] Does `bd pour` return the molecule root ID in `--json` output? Need to verify the exact output format before implementation.
- [ ] What is the step ID format — `<mol-id>.1`, `<mol-id>.2`? Are step numbers assigned in formula declaration order?
- [ ] Can the formula define sub-steps for implementation work (one step per plan work chunk), or should implementation remain a single step with mindspec managing granularity via separate beads issues?

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: —
