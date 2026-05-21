# ADR-0034: Ceremony Collapse — Single-Bead Lifecycle Epic + Legacy Auto-Migrator

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0030](ADR-0030-executor-boundary.md), [ADR-0032](ADR-0032-adr-semantic-gates.md)

---

## Status

Stub created during spec 089-ceremony-collapse drafting. Finalized in
spec 089 Bead 3 alongside the auto-migrator implementation.

## Context

`mindspec approve spec` historically created multiple ceremony beads
(molecule-step scaffolding) under the lifecycle epic to represent the
spec → plan → impl → complete progression. After the spec 080+
refactors, the lifecycle epic itself carries `mindspec_phase` metadata
and is the single source of truth. Legacy specs created before that
refactor still have 7-bead ceremony children hanging off the epic, and
`phase.DerivePhase` reads `mindspec_phase` first, falling back to a
children-derivation path only when the metadata key is absent.

F5 of the converged transformation plan resolves the drift
mechanically: auto-migrate legacy specs to metadata on the first
lifecycle command touching them (writing the derived `mindspec_phase`
once and stamping `mindspec_migrated_at` for audit), surface a
`mindspec doctor --dry-run-migration` reporter for pre-mutation
visibility, and remove any vestigial molecule scaffolding paths left
over from the old ceremony model.

## Decision

Three sub-decisions:

1. **Auto-migrate on first lifecycle command (no opt-in).** On entry
   to `approve plan`, `approve impl`, or `complete`: if the target
   spec's lifecycle epic lacks the `mindspec_phase` metadata key,
   derive the phase from existing ceremony children once, write
   `mindspec_phase` plus `mindspec_migrated_at` to the epic, then
   proceed with the original command. The migration is transparent
   and idempotent — second invocations see the metadata and skip the
   derive step. Rejected alternatives: require an explicit
   `--migrate` flag (operator friction, indefinite drift in practice);
   leave legacy specs alone forever (the children-derivation fallback
   stays load-bearing and the metadata-first invariant never becomes
   true across the fleet).

2. **Ceremony children kept, ignored after migration.** The migration
   does not delete the legacy ceremony beads — it only writes the
   metadata. The children-derived fallback in `phase.DerivePhase`
   remains in the binary so that legacy specs which never trigger a
   lifecycle command (archived, abandoned, read-only history) still
   resolve to a sensible phase. Rejected: delete ceremony children on
   migration (destructive, irreversible, blocks any historical audit
   that walks children); delete the children-derived fallback in
   `DerivePhase` (breaks every unmigrated spec the instant the binary
   ships).

3. **`mindspec doctor --dry-run-migration`** reports every spec that
   would migrate on its next lifecycle command, showing the derived
   phase value and the source children, without writing any state.
   Operators get visibility into the upcoming migration surface
   before any production lifecycle command runs.

## Consequences

- (+) Mechanically removes ceremony drift as lifecycle commands
  naturally touch each spec — no broadcast migration or operator
  coordination required.
- (+) Operator gets pre-migration visibility via
  `doctor --dry-run-migration`.
- (+) `phase.DerivePhase` becomes metadata-first for the active
  working set within one cycle of normal use, shrinking the
  children-derivation fallback to truly cold paths.
- (−) Legacy specs that never touch a lifecycle command never migrate
  and remain dependent on the children-derived fallback indefinitely.
  Acceptable per the transformation plan — those specs are de facto
  read-only and the fallback is correct, just slower.
- (−) The one-shot migration runs on the cold path of `approve plan`,
  `approve impl`, and `complete`. Cost is one metadata read plus one
  metadata write on first hit per spec — negligible against the
  surrounding command work.
- (−) Ceremony children remain in the tree for migrated specs,
  visible to `bd list` and similar queries. Documented as
  intentionally-vestigial in the spec 089 release notes.

## Rollback

Revert the spec 089 PR. The auto-migrate code is purely additive —
removing it leaves `phase.DerivePhase` with its children-derived
fallback exactly as before, and migrated specs simply have an extra
`mindspec_phase` key on their epic that the older binary reads
preferentially (matching post-migration behavior). No data migration
or schema rollback is required. ADR-0034 itself remains harmless in
the tree.

## Related

- [ADR-0030](ADR-0030-executor-boundary.md) — executor surface; the
  auto-migrator writes metadata through the same executor boundary
  used by lifecycle commands.
- [ADR-0032](ADR-0032-adr-semantic-gates.md) — ADR semantic gates;
  ceremony collapse simplifies the lifecycle-phase signal that
  several semantic gates depend on.
