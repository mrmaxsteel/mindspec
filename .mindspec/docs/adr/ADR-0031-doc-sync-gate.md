# ADR-0031: Doc-Sync as an Enforcement Gate with Per-Domain OWNERSHIP.yaml

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: validation, doc-sync, lifecycle, ownership
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: [ADR-0036](ADR-0036-ownership-discovery.md) (in part — fallback semantics only)
- **Related**: [ADR-0030](ADR-0030-executor-boundary.md) (executor-boundary; doc-sync now uses `Executor.ChangedFiles`), [ADR-0011](ADR-0011.md) (lifecycle states)

---

> **Superseded in part by [ADR-0036](ADR-0036-ownership-discovery.md)**
> (spec 091): the silent `internal/<domain>/**` fallback this ADR
> records as live behavior — in Decision 2 ("Fallback to
> `internal/<domain>/**` when the file is missing") and in the
> Consequences ("…or accept the `internal/<domain>/**` fallback
> heuristic") — is REMOVED. A domain whose `OWNERSHIP.yaml` is absent
> now claims nothing (`Paths: []`, `Source() == "missing"`); see
> ADR-0036 for the replacement semantics and migration path. The
> manifest schema and the warning-to-error promotion recorded here
> remain authoritative.

> **Amended by spec 095 (mindspec-vvs9) — attribution reads the diffed
> ref, not the ambient working tree.** Decision 2 records OWNERSHIP
> resolution but left the TREE it reads from implicit; in practice both
> the per-domain manifest load (`LoadOwnership(root, domain)`) and the
> domain-directory enumeration (`listDomainDirs(root)`) did `os.ReadFile`
> / `os.ReadDir` on the ambient working tree at `root`. Run from the
> main checkout, the gate then evaluated a changed file against *main's*
> OWNERSHIP, which lacks any claim the branch added — so an OWNERSHIP
> claim committed on a bead/spec branch could not satisfy its own gate,
> forcing an `--override-adr` on every on-branch claim. The gates now
> resolve attribution input — BOTH the manifests AND the domain
> enumeration — from the SAME git ref they diff, via the executor
> (`Executor.FileAtRefOrAbsent` / `Executor.TreeDirsAtRef`, wrapping
> `git ls-tree`/`git show`): `beadHead` for the per-bead gates in
> `complete`, the spec-branch tip for the whole-branch backstop in
> `impl approve`. The ownership ref is threaded as an explicit parameter
> INDEPENDENT of the diff range. This is a FIDELITY REFINEMENT of the
> same attribution decision (no new ADR; spec 095 plan DQ2). The on-disk
> `LoadOwnership`/`listDomainDirs` remain for the working-tree consumers
> (`mindspec validate docs`, doctor, `ownership populate`) and for the
> `ownerRef == ""` (working-tree) call sites; `source_globs` stays a
> working-tree read (operator config, not a per-bead gate input). The
> absent-manifest-claims-nothing semantics (ADR-0036) and the HC-5
> excluded-first-segment rejection are preserved under the ref read.
> **Example (placeholder):** a bead branch that commits
> `.mindspec/docs/domains/widget/OWNERSHIP.yaml` claiming
> `internal/widget/**` together with `internal/widget/foo.go` passes its
> own doc-sync + ADR-divergence gates at `mindspec complete` run from the
> main root with no override.

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

1. **Warnings → Errors at the doc-sync call sites; operator-docs lane
   stays advisory.** `AddWarning` at `internal/validate/docsync.go:37`
   and `:127` become `AddError`. `AddWarning` at `:154` (the operator-docs
   lane — `cmd/` changes without `CLAUDE.md`/`CONVENTIONS.md`/
   `.mindspec/docs/user/` touches) deliberately REMAINS `AddWarning`,
   per spec 086 Requirement 7: the operator-docs lane is intentionally
   advisory so cross-cutting `cmd/` edits don't require operator-doc
   churn on every commit. `complete.Run` and `ApproveImpl` exit non-zero
   on doc-sync errors only — operator-docs warnings continue to surface
   as advisories. Rejected alternatives: warn-only across all three sites
   (preserves the status quo and lets drift compound); promote all three
   sites uniformly (contradicts the operator-docs lane policy and would
   block routine `cmd/` refactors); feature-flag rollout (defers the
   decision without forcing a resolution).

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
