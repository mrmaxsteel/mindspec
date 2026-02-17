# Spec 034-brownfield-init-onboarding: Brownfield Project Onboarding for `mindspec init`

## Goal

Enable teams to adopt MindSpec in an existing repository with one command that:
- discovers and analyzes existing Markdown docs,
- produces a canonical MindSpec documentation corpus in an optimal deterministic layout, and
- archives legacy docs with full provenance and reproducibility.

This spec also explicitly covers dual-role execution for this repository:
- MindSpec product behavior must change (bootstrap and canonical path expectations), and
- the MindSpec repository must dogfood the migration by applying the structural change to itself.

## Background

`mindspec init` currently scaffolds a greenfield structure additively. That is useful for empty projects, but insufficient for brownfield repositories where documentation already exists across many directories with mixed quality, overlap, and stale artifacts.

Adoption friction is currently high because users must manually decide:
- which docs are canonical versus stale,
- where canonical docs should live long-term,
- how to preserve old files without losing traceability.

This spec introduces a brownfield onboarding mode for `mindspec init` that performs deterministic-first migration with constrained LLM assistance where rules are insufficient.

Because this repository builds MindSpec through MindSpec, implementation is incomplete until the new bootstrap/migration behavior is applied to this repo itself and validated as a real migration outcome.

## Impacted Domains

- **core**: Extends `mindspec init`, moves templates to binary-embedded assets, and introduces migration orchestration.
- **context-system**: Defines canonical corpus shape, lineage/provenance, and extraction/synthesis rules.
- **workflow**: Migrates path resolution to canonical doc roots and updates `doctor` for new layout checks.

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): DDD principles remain, but canonical storage-path sections require a scoped superseding ADR.
- [ADR-0005](../../adr/ADR-0005.md): Migration state and checkpoints remain explicit under `.mindspec/`.
- [ADR-0012](../../adr/ADR-0012.md): Brownfield implementation composes capabilities directly and avoids deep wrapper abstractions.

## Requirements

1. Add brownfield mode: `mindspec init --brownfield`.
2. Keep greenfield mode available: `mindspec init` without `--brownfield`.
3. Treat templates as binary-internal assets only (embedded in Go binary).
4. Remove runtime dependency on workspace `docs/templates/` in both greenfield and brownfield flows.
5. Canonical documentation root is `.mindspec/docs/`.
6. Canonical docs include `.mindspec/docs/specs/`, `.mindspec/docs/adr/`, `.mindspec/docs/domains/`, `.mindspec/docs/core/`, `.mindspec/docs/context-map.md`, and `.mindspec/docs/glossary.md`.
7. Canonical policy location is `.mindspec/policies.yml` (migrated from legacy `architecture/policies.yml`).
8. Legacy `docs/` tree becomes migration input and archive source, not long-term canonical storage.
9. Path APIs in `internal/workspace/workspace.go` must migrate to canonical roots (or canonical-first resolvers), including policies path migration to `.mindspec/policies.yml`.
10. Existing read paths may use legacy fallback only when canonical docs/policies are absent; write paths must target canonical roots.
11. Brownfield apply archives legacy Markdown docs to `docs_archive/<run-id>/<original-relative-path>.md`.
12. Discovery scans Markdown files deterministically (stable path order; explicit ignore list).
13. Classification uses deterministic rules first; only low-confidence docs are routed to LLM classification.
14. LLM outputs must follow strict JSON schema; model prompt/version is pinned per run artifact.
15. LLM runtime must support explicit provider/config resolution (for example via env vars) and deterministic request ordering.
16. Brownfield `--report-only` must work without LLM credentials.
17. Brownfield `--apply` must fail with actionable diagnostics when low-confidence docs require LLM and no LLM provider is available.
18. Deep extraction and synthesis happen per category with chunked deterministic map-reduce for large corpora.
19. Every generated canonical claim must preserve source citations to original docs.
20. Conflicting source statements must be surfaced explicitly in canonical conflict sections and validation artifacts.
21. Migration must emit a machine-readable lineage manifest mapping `source -> canonical -> archive` with source hashes.
22. Migration must be resumable from checkpoints via `--resume <run-id>`.
23. Migration must be idempotent for unchanged inputs (same hashes + same model config produce byte-identical outputs).
24. Brownfield flag semantics must be explicit and deterministic:
   - `--brownfield` selects migration mode
   - default behavior is report-only if neither `--apply` nor `--report-only` is provided
   - `--apply` and `--report-only` are mutually exclusive
   - `--archive` is valid only with `--apply`
25. Brownfield writes must be transactional: stage outputs first, validate, then promote atomically.
26. No source-doc destructive mutation may occur before successful synthesis/validation.
27. `mindspec doctor` must validate canonical layout and migration metadata.
28. Run artifacts (inventory, classification, extraction, synthesis, validation, state) must be persisted under `.mindspec/migrations/<run-id>/`.
29. Dogfooding requirement: once implementation is complete, this repository must be migrated using the new brownfield flow and committed as a first-class proof artifact.
30. Post-dogfood, core MindSpec workflows in this repository (init/doctor/validate/context assembly as applicable) must operate against canonical-first path expectations.
31. Architecture artifact reconciliation must be explicit:
   - `docs/core/ARCHITECTURE.md` migrates to `.mindspec/docs/core/ARCHITECTURE.md`
   - `docs/adr/*.md` migrates to `.mindspec/docs/adr/*.md`
   - `architecture/policies.yml` migrates to `.mindspec/policies.yml`
32. Policy `reference:` links in migrated `.mindspec/policies.yml` must be updated to canonical doc paths during migration/dogfooding.

## Scope

### In Scope
- `cmd/mindspec/init.go` mode/flag contract and execution semantics.
- `internal/bootstrap/` greenfield flow updates (embedded templates, no workspace template dependency).
- New brownfield pipeline modules: discovery, classification, extraction, synthesis, lineage, archive, validation.
- `internal/workspace/workspace.go` canonical path migration for docs-related path helpers.
- `internal/workspace/workspace.go` policies path migration from `architecture/policies.yml` to `.mindspec/policies.yml`.
- Downstream consumer migration for canonical path helpers (`context-pack`, domain tooling, validators, doctor, init/spec-init where applicable).
- Brownfield checkpointing, resume, and transactional apply behavior.
- Dogfooding rollout in this repository as a required final implementation outcome.

### Out of Scope
- Ingesting non-Markdown source formats (PDF, DOCX, external wiki APIs).
- Interactive TUI conflict resolution.
- User-overridable template bundles (future enhancement).
- Monthly/continuous background re-migration automation.

## Non-Goals

- Keeping legacy `docs/` and canonical `.mindspec/docs/` as co-equal long-term sources of truth.
- Perfect sentence-level rewriting of every legacy artifact.
- Broad architectural supersession of ADR-0001 beyond storage-path semantics.
- Deferring dogfooding to a separate future epic.

## Path Migration Strategy

- Authoritative docs are stored under `.mindspec/docs/` after migration.
- Legacy `docs/` files are discovery inputs and then archived to `docs_archive/<run-id>/...`.
- During transition, read fallback from legacy paths is allowed only for non-migrated projects where canonical docs do not yet exist.
- All write operations for docs/specs/ADRs/domains/context-map/glossary are canonical-root writes.
- Architecture artifact mapping is explicit:
  - `docs/core/ARCHITECTURE.md` -> `.mindspec/docs/core/ARCHITECTURE.md`
  - `docs/adr/*.md` -> `.mindspec/docs/adr/*.md`
  - `architecture/policies.yml` -> `.mindspec/policies.yml`.
  - `reference:` links in `.mindspec/policies.yml` are rewritten to canonical doc paths.

## LLM Runtime and Fallback

- Brownfield LLM calls are an internal migration capability, not user-authored prompts.
- Provider/model/config values are recorded in run artifacts for reproducibility.
- If no LLM is available:
  - report-only mode completes with unresolved low-confidence diagnostics.
  - apply mode fails fast with actionable remediation instructions.

## Flag Semantics

- `mindspec init --brownfield` defaults to report-only behavior.
- `--apply` enables writes to canonical docs and archive paths.
- `--report-only` forces no-write mode.
- `--apply` and `--report-only` are mutually exclusive.
- `--archive=copy|move` defaults to `copy` and is valid only in apply mode.
- `--resume <run-id>` continues from `.mindspec/migrations/<run-id>/state.json`.

## Failure and Rollback Model

- Write canonical outputs to staging under `.mindspec/migrations/<run-id>/staging/`.
- Promote staged canonical outputs atomically only after validation succeeds.
- Perform archive operations only after canonical promotion succeeds.
- On failure, leave diagnostics/checkpoints; do not delete source docs.

## Canonical Layout

```text
.mindspec/
  policies.yml
  docs/
    core/
    domains/
    adr/
    specs/
    context-map.md
    glossary.md
    index.json
  lineage/
    manifest.json
  migrations/
    <run-id>/
      inventory.json
      classification.json
      extraction.json
      synthesis.json
      validation.json
      state.json
      staging/
docs_archive/
  <run-id>/
    <original-relative-path>.md
```

## Acceptance Criteria

- [ ] `mindspec init --brownfield` (without `--apply`) runs in report-only mode and writes no canonical docs.
- [ ] `mindspec init --brownfield --apply` writes canonical docs under `.mindspec/docs/`.
- [ ] `mindspec init --brownfield --apply` archives discovered legacy markdown docs to `docs_archive/<run-id>/...`.
- [ ] Workspace path resolution for docs-related operations is canonical-first after migration.
- [ ] Existing read operations still work for non-migrated repos through legacy fallback.
- [ ] Greenfield `mindspec init` succeeds without writing `docs/templates/`.
- [ ] Canonical docs include source coverage sections with citations.
- [ ] Lineage manifest exists at `.mindspec/lineage/manifest.json` and includes source hash, category, canonical target, archive target.
- [ ] Re-running unchanged corpus with identical model config yields byte-identical canonical outputs.
- [ ] `--resume <run-id>` resumes from checkpoint and completes without rerunning completed deterministic stages.
- [ ] High-confidence docs skip LLM classification; low-confidence docs are routed to LLM classification.
- [ ] `--apply` fails with explicit diagnostics when LLM is required but unavailable.
- [ ] Transactional apply prevents destructive source mutations on failed runs.
- [ ] `mindspec doctor` passes for migrated repositories using canonical docs layout.
- [ ] This repository is migrated via the new brownfield flow and committed with canonical docs/lineage/archive outputs.
- [ ] After dogfooding migration, key MindSpec workflows continue to pass in this repository.
- [ ] Architecture artifact reconciliation is preserved: `ARCHITECTURE.md` and ADRs are canonical under `.mindspec/docs/`, and policies are canonical at `.mindspec/policies.yml`.
- [ ] Migrated `.mindspec/policies.yml` contains canonicalized `reference:` paths (not archived legacy doc paths).

## Validation Proofs

- `mindspec init --brownfield --json`: report-only inventory/classification/plan output; no canonical writes.
- `mindspec init --brownfield --apply --archive=copy`: generates `.mindspec/docs/`, `.mindspec/lineage/manifest.json`, `docs_archive/<run-id>/...`.
- `mindspec init --brownfield --apply` twice on unchanged corpus: identical hashes and no canonical diffs.
- `mindspec init --brownfield --resume <run-id>` after interruption: resumes from checkpoint and completes.
- `MINDSPEC_LLM_PROVIDER=off mindspec init --brownfield --apply`: fails with actionable unresolved-classification diagnostics.
- `mindspec doctor`: zero errors after successful brownfield apply.
- `mindspec init --brownfield --apply` executed in this repository: canonical docs and archive artifacts are produced, then `mindspec doctor` and `mindspec validate spec 034-brownfield-init-onboarding` pass.
- `test -f .mindspec/docs/core/ARCHITECTURE.md && test -d .mindspec/docs/adr && test -f .mindspec/policies.yml`: architecture artifact mapping is correct post-migration.
- `rg -n \"reference: \\\"\\.mindspec/docs/\" .mindspec/policies.yml`: policy references are canonicalized after migration.

## Open Questions

All resolved for planning scope:
- [x] Canonical location: `.mindspec/docs/`.
- [x] Legacy docs handling: archive under `docs_archive/<run-id>/<original-relative-path>.md`.
- [x] Template ownership: binary-internal templates; no workspace template dependency.
- [x] Path migration: canonical-first path helpers with legacy read fallback only pre-migration.
- [x] LLM fallback behavior: report-only succeeds; apply fails when unresolved and no LLM is available.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-17
- **Notes**: Approved via `mindspec approve spec`; clarified during plan refinement without changing target user outcome
