---
molecule_id: mindspec-mol-nlf
---
# Spec 038-beads-native-multi-spec-state: Beads-Native Multi-Spec State

## Goal

Enable MindSpec to work on multiple specs in parallel (including from the same git worktree) by making workflow state derive from Beads molecules per spec, rather than from a single global active mode pointer in `.mindspec/state.json`.

## Background

MindSpec currently stores one global active workflow pointer (`mode`, `activeSpec`, `activeBead`) in `.mindspec/state.json` (ADR-0005). This works for serial work but breaks down when multiple specs are active concurrently in one worktree.

Beads already models work as a graph:
- Spec lifecycle is represented by a molecule root and step children.
- Dependencies and readiness are resolved from issue status, not from a separate global mode variable.

This spec aligns MindSpec with that model so it can behave as a thin workflow orchestrator over Beads formulas and molecules while preserving MindSpec's docs/validation/guide value.

## Impacted Domains

- workflow: mode resolution, command targeting, state model
- bead integration: molecule lookups and step status resolution
- docs/ADR governance: updates to state and lifecycle ADRs
- instruct/context: guidance must be per-target-spec, not globally singular

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Beads as execution tracking substrate
- [ADR-0003](../../adr/ADR-0003.md): MindSpec owns orchestration semantics
- [ADR-0005](../../adr/ADR-0005.md): Current single-state model to be amended/superseded
- [ADR-0007](../../adr/ADR-0007.md): Parallel workstream direction and Beads shared-state constraint
- [ADR-0013](../../adr/ADR-0013.md): Formula-driven lifecycle as the execution backbone

## Requirements

1. **Per-spec mode derivation**  
   MindSpec must derive workflow mode per spec from Beads molecule step state, not from a single global `mode` field.

2. **Spec-to-molecule binding is explicit and durable**  
   Each spec must have a durable reference to its lifecycle molecule ID (and optional step ID mapping), stored with spec artifacts (e.g., spec frontmatter/metadata).

3. **Command targeting must be explicit when ambiguous**  
   Commands that depend on "current work" (`instruct`, `approve`, `next`, `complete`, status-like flows) must accept explicit `--spec` or equivalent targeting and fail with actionable guidance when multiple candidates exist and no target is provided.

4. **Global state becomes optional UX cursor, not source of truth**  
   If `.mindspec/state.json` remains, it must no longer be the canonical lifecycle signal. It may keep convenience state (e.g., last focused spec) only.

5. **Backward compatibility for existing specs**  
   Existing specs and molecules created before this change must continue to work. Migration must be additive and deterministic.

6. **Parallel specs in one worktree must be supported**  
   Two specs in active refinement/planning in the same worktree must not conflict at the mode/state layer.

7. **Instruct output must be per target**  
   `mindspec instruct` must render guidance for the selected spec lifecycle state. If no target is selected and multiple active specs exist, it must show a concise chooser summary instead of guessing.

8. **Approval and transition commands use molecule semantics**  
   Approval flows (`approve spec`, `approve plan`, `approve impl`) must resolve and act on the target spec's molecule steps directly.

## Scope

### In Scope
- State model redesign from global-single to per-spec-derived lifecycle
- Command UX for target resolution and ambiguity handling
- Metadata placement for spec↔molecule binding
- Migration/backfill strategy for existing specs
- Tests for multi-spec parallel behavior in same worktree
- Docs and ADR updates for new state model

### Out of Scope
- Replacing Beads itself or changing Beads core semantics
- New workflow phases beyond current lifecycle
- Full UI redesign beyond what is needed for target selection clarity
- Mandatory worktree creation for spec/plan phases

## Non-Goals

- Building a second task graph outside Beads
- Maintaining compatibility with heuristic-only implicit mode guessing
- Preserving `state.json` as the primary source of truth

## Acceptance Criteria

- [ ] It is possible to have at least two active specs in one worktree without state conflicts.
- [ ] `mindspec instruct --spec <id>` produces correct mode guidance for each active spec independently.
- [ ] Untargeted `mindspec instruct` with multiple active specs does not guess; it returns a clear ambiguity prompt/list.
- [ ] `mindspec approve spec <id>` and `mindspec approve plan <id>` transition only the targeted spec's molecule steps.
- [ ] `mindspec next --spec <id>` resolves ready work for the targeted spec molecule and does not cross-select from another active spec unless explicitly requested.
- [ ] Existing specs initialized under prior model continue to function after migration/backfill.
- [ ] Docs explain canonical source-of-truth changes and command targeting behavior.
- [ ] ADR-0005 is amended/superseded to reflect the new canonical state model.

## Validation Proofs

- `mindspec spec-init 038-a` and `mindspec spec-init 038-b` in same worktree:
  both specs have independent molecule references, no overwrite conflict.
- `mindspec instruct --spec 038-a` and `mindspec instruct --spec 038-b`:
  mode/guidance differ correctly by molecule step state.
- `mindspec approve spec 038-a`:
  only 038-a's molecule progresses; 038-b remains unchanged.
- `mindspec instruct` (no args) with multiple active specs:
  returns deterministic ambiguity output requiring selection.
- `make test`:
  includes coverage for parallel multi-spec targeting behavior and passes.

## Open Questions

None.

### Resolved

- [x] **Spec↔molecule binding location**: store canonical binding in `spec.md` frontmatter. Rationale: keeps lifecycle identity with the spec artifact and avoids a second canonical mapping file.
- [x] **Global state role**: retain `.mindspec/state.json` only as a non-canonical compatibility/cursor surface during migration; lifecycle truth is derived per-spec from Beads + spec metadata.
- [x] **Default targeting behavior**: if exactly one active spec is detectable, untargeted commands may auto-select it; if multiple are active, commands must refuse to guess and require explicit `--spec`.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-19
- **Notes**: Approved via mindspec approve spec