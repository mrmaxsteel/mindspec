---
molecule_id: mindspec-mol-p32
status: Draft
approved_at:
approved_by:
step_mapping:
    implement: mindspec-mol-v5m
    plan: mindspec-mol-bpm
    plan-approve: mindspec-mol-gf8
    review: mindspec-mol-0h2
    spec: mindspec-mol-i98
    spec-approve: mindspec-mol-54w
    spec-lifecycle: mindspec-mol-p32
---

# Spec 043-lifecycle-closeout: Lifecycle Close-Out Reconciliation

## Goal

Ensure that `mindspec approve impl` is the single authoritative ceremony for completing a spec lifecycle — closing the entire molecule (epic + all steps), and that bypassing CLI approval commands (editing frontmatter directly) is detected by validation. This prevents stale beads from accumulating when work is done but tracking wasn't updated.

## Background

In practice, the 041-explore-mode spec was fully implemented and committed, but all pipeline beads (write-spec, approve-spec, write-plan, approve-plan, implement, review, and the epic) remained open. The root cause is twofold:

1. **`approve impl` only closes the review step** — it doesn't reconcile the rest of the molecule. If earlier steps were skipped or the workflow was compressed into a single session, those beads linger forever.
2. **No validation link between approval frontmatter and bead gates** — a spec or plan can show approved status in YAML frontmatter while the corresponding gate bead is still open. Nothing catches this drift.

The result: `bd ready` reports no available work (everything is blocked by stale open beads), and the project appears stuck when it isn't.

## Impacted Domains

- **workflow**: `approve impl` gains molecule reconciliation; validation gains frontmatter↔gate consistency checks
- **core**: documentation updates to USAGE.md and MODES.md to clarify the close-out contract

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Beads as passive tracking substrate — molecule close-out is MindSpec-initiated, Beads remains passive. Conforms.
- [ADR-0008](../../adr/ADR-0008.md): Human gates as dependency markers — this spec tightens the contract that gates and frontmatter must stay in sync. Conforms + extends.
- [ADR-0013](../../adr/ADR-0013.md): Beads formulas for lifecycle orchestration — molecule close-out operates on the formula-poured molecule. Conforms.
- [ADR-0015](../../adr/ADR-0015.md): Per-spec molecule-derived state — closing all molecule steps is consistent with molecule-derived state (all closed = lifecycle complete).

## Requirements

1. **Molecule-wide reconciliation on impl approval**: `mindspec approve impl <id>` must close the parent epic (`molecule_id`) and every unique bead ID in `step_mapping` (including `spec` and `plan`, not only gates) before transitioning state to idle. Closure target resolution must be data-driven from frontmatter/state metadata, not a hardcoded subset of step names.
2. **Idempotent close semantics**: Already-closed molecule members are treated as success (skip silently). Unexpected closure failures for individual members are warning-only; the ceremony continues and remains best-effort.
3. **Spec approval frontmatter canonicalization**: `mindspec approve spec <id>` must write/update YAML frontmatter fields in `spec.md` as the source of truth (`status: Approved`, `approved_at`, `approved_by`). Legacy specs that only have the markdown `## Approval` section must be handled gracefully (migrated or warned with actionable remediation).
4. **Frontmatter↔gate consistency validation**: `mindspec validate spec <id>` must warn if `spec.md` frontmatter says `status: Approved` while the `spec-approve` gate bead is open. `mindspec validate plan <id>` must do the same for `plan.md` (`status: Approved` vs `plan-approve` gate).
5. **Doc/template updates**: USAGE.md Phase 9 and MODES.md Implementation Mode Exit Gate must document molecule-wide close-out and the `complete` (per-bead) vs `approve impl` (lifecycle) distinction; the spec template must document approval status in YAML frontmatter.

## Scope

### In Scope
- `internal/approve/impl.go` — add molecule reconciliation logic
- `internal/approve/spec.go` — write spec approval status to YAML frontmatter
- `internal/validate/spec.go` — add frontmatter↔gate consistency check
- `internal/validate/plan.go` — add frontmatter↔gate consistency check
- `.mindspec/docs/user/templates/spec.md` — add canonical approval frontmatter fields
- `.mindspec/docs/core/USAGE.md` — clarify Phase 9 close-out contract
- `.mindspec/docs/core/MODES.md` — clarify `complete` vs `approve impl` distinction
- Tests for the new behavior

### Out of Scope
- Changing `mindspec complete` behavior (it correctly handles per-bead close-out already)
- Git hooks or pre-push enforcement (could be a future spec)
- Retroactive cleanup of already-stale beads from other specs

## Non-Goals

- This spec does not add automated detection of "code committed but beads not closed" — that's a broader observability concern. It focuses on making the happy path (running the CLI commands) sufficient and the bypass path (editing frontmatter directly) detectable.
- This spec does not change the molecule formula or step structure — it only changes what happens at close-out.

## Acceptance Criteria

- [ ] `mindspec approve impl <id>` closes the parent epic (`molecule_id`) plus every unique ID in `step_mapping` (all lifecycle steps), not just the review step
- [ ] If a targeted molecule member is already closed, `approve impl` treats it as success and continues without warning/error
- [ ] If closing a targeted molecule member fails unexpectedly, `approve impl` emits a warning and continues processing remaining members
- [ ] `mindspec approve spec <id>` writes `status: Approved`, `approved_at`, and `approved_by` in `spec.md` YAML frontmatter
- [ ] `mindspec validate spec <id>` emits a warning when `spec.md` frontmatter says `status: Approved` but the `spec-approve` gate bead is still open
- [ ] `mindspec validate plan <id>` emits a warning when `plan.md` frontmatter says `status: Approved` but the `plan-approve` gate bead is still open
- [ ] Legacy specs without spec approval frontmatter are handled gracefully (migration path or actionable warning)
- [ ] USAGE.md Phase 9 documents that `approve impl` reconciles the full molecule
- [ ] MODES.md documents the distinction between `complete` (per-bead) and `approve impl` (lifecycle close-out)
- [ ] `.mindspec/docs/user/templates/spec.md` documents canonical approval fields in YAML frontmatter

## Validation Proofs

- Create a test spec with a poured molecule, close some members manually, leave others open. Run `mindspec approve impl <id>` (in review mode). Verify the parent epic and every ID in `step_mapping` are closed afterward.
- Re-run `mindspec approve impl <id>` on the same already-closed molecule. Verify idempotent success (no hard failure).
- Simulate one targeted closure failure (mock or fixture). Verify warning emission and continued reconciliation of remaining molecule members.
- Create a spec with `status: Approved` in YAML frontmatter but an open `spec-approve` gate. Run `mindspec validate spec <id>`. Verify it emits a consistency warning.
- Create a plan with `status: Approved` in YAML frontmatter but an open `plan-approve` gate. Run `mindspec validate plan <id>`. Verify it emits a consistency warning.
- Create a legacy spec using only markdown `## Approval` status (no approval frontmatter). Run approval/validation and verify migration or actionable warning behavior.

## Open Questions

(none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-20
- **Notes**: Approved via mindspec approve spec