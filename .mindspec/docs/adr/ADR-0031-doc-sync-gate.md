# ADR-0031: Doc-Sync as an Enforcement Gate with Per-Domain OWNERSHIP.yaml

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: validation, doc-sync, lifecycle, ownership
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0030](ADR-0030-executor-boundary.md) (executor-boundary; doc-sync now uses `Executor.ChangedFiles`), [ADR-0011](ADR-0011.md) (lifecycle states)

---

## Status

Stub created during spec 086-doc-sync drafting. Finalized in spec 086 Bead N
alongside the AddWarning→AddError promotion and OWNERSHIP.yaml machinery.

## Context

Today doc-sync in `internal/validate/docsync.go` emits `AddWarning` for:
source-without-doc-update, doc-without-source-update, and related drift
signals. Warnings don't block `mindspec complete` or `mindspec approve impl`
— they're advisory. Without enforcement, doc drift compounds silently as
beads land and specs close.

F2 of the converged transformation plan (`mindspec-transformation-plan.md`)
promotes these warnings to errors with an explicit override
(`--allow-doc-skew "<reason>"`) recorded in metadata. It also adds
per-domain `OWNERSHIP.yaml` co-located at
`.mindspec/docs/domains/<domain>/OWNERSHIP.yaml` for path→domain
resolution, replacing implicit `internal/<domain>/**` heuristics.

## Decision

Three sub-decisions:

1. **Warnings → Errors at the named call sites.** `AddWarning` at
   `internal/validate/docsync.go:37`, `:127`, and `:154` become `AddError`.
   `complete.Run` and `ApproveImpl` exit non-zero on doc-sync errors.
   Rejected alternatives: warn-only (preserves the status quo and lets
   drift compound); feature-flag rollout (defers the decision without
   forcing a resolution).

2. **Per-domain `OWNERSHIP.yaml`, co-located.**
   `.mindspec/docs/domains/<domain>/OWNERSHIP.yaml` with schema
   `{paths: [...], exclude: [...]}`. Fallback to `internal/<domain>/**`
   when the file is missing. First-match-wins, ties broken by
   lexicographic domain order. Rejected: a central `domain_map.yml`
   (ownership belongs next to the thing it owns; central files rot when
   domains split or merge).

3. **`--allow-doc-skew` override, recorded with `reason`+`by`+`at`.** On
   `complete`: bead metadata key `mindspec_doc_skew_reason`. On
   `approve impl`: spec epic metadata key `mindspec_impl_skew_reason`.
   Empty reason rejected. Rejected alternatives: no override (too rigid
   for cross-domain refactors that legitimately touch source without
   touching docs); env-var escape hatch (not auditable, leaves no trail
   in bead/spec metadata).

## Consequences

- (+) Doc-sync errors mechanically block merges via `complete` and
  `approve impl` exit codes.
- (+) Explicit override is auditable — every skew leaves a reason, an
  actor, and a timestamp in bead/spec metadata.
- (+) Ownership is co-located with the domain it owns; splitting or
  renaming a domain moves its `OWNERSHIP.yaml` with it.
- (−) Cross-domain refactors require thoughtful `OWNERSHIP.yaml`
  authoring or judicious override use.
- (−) `internal/approve/impl.go` call order must be reorganized so
  enforcement runs before side-effecting steps.
- (−) Existing repos must author `OWNERSHIP.yaml` for each domain
  directory or accept the `internal/<domain>/**` fallback heuristic.

## Rollback

Revert spec 086 PR's merge commit in a single git command
(`git revert -m 1 <merge-sha>`). `AddError` calls revert to `AddWarning`.
Override metadata keys (`mindspec_doc_skew_reason`,
`mindspec_impl_skew_reason`) are forward-compatible — older binaries
ignore them. `OWNERSHIP.yaml` files left in the tree remain harmless
under the reverted resolver.

## Related

- [ADR-0030](ADR-0030-executor-boundary.md) — executor-boundary; doc-sync
  now consumes `Executor.ChangedFiles` for its diff input.
- [ADR-0011](ADR-0011.md) — lifecycle states that doc-sync errors now
  gate (`complete`, `approve impl`).
