---
adr_citations:
    - id: ADR-0001
      sections:
        - Decision Details
        - Project DDD Artifacts
        - Validation / Rollout
    - id: ADR-0005
      sections:
        - Decision
        - State File Schema (v1)
        - Validation / Rollout
    - id: ADR-0012
      sections:
        - Decision
        - Principles
        - Consequences
approved_at: "2026-02-17T17:39:41Z"
approved_by: user
last_updated: "2026-02-17"
spec_id: 034-brownfield-init-onboarding
status: Approved
version: "0.3"
work_chunks:
    - depends_on: []
      id: 1
      scope: docs/adr/, docs/core/ARCHITECTURE.md, docs/core/CONVENTIONS.md
      title: Scoped ADR supersession for canonical path semantics
      verify:
        - New ADR supersedes only ADR-0001 storage-path semantics (not DDD principles)
        - Canonical docs path guidance is unambiguous and consistent in core docs
    - depends_on:
        - 1
      id: 2
      scope: cmd/mindspec/init.go, internal/bootstrap/, init flags and mode semantics
      title: Init mode contract and embedded template migration
      verify:
        - '`mindspec init` remains greenfield-safe without workspace templates'
        - '`--brownfield` defaults to report-only and enforces `--apply`/`--report-only` exclusivity'
    - depends_on:
        - 1
      id: 3
      scope: internal/workspace/workspace.go and docs-path consumers (context-pack, domain tooling, validation, doctor)
      title: Canonical workspace path migration across consumers
      verify:
        - Docs-related operations resolve canonical-first paths
        - Legacy read fallback works only when canonical docs are absent
        - 'Architecture artifact mapping is preserved: ARCHITECTURE/ADRs move to canonical docs root; policies.yml moves to .mindspec/policies.yml'
    - depends_on:
        - 2
      id: 4
      scope: internal/brownfield/discovery/, internal/brownfield/classify/, LLM provider/config integration
      title: Deterministic discovery/classification with LLM provider fallback
      verify:
        - Unchanged corpus yields byte-identical inventory/classification outputs
        - No-LLM report-only succeeds and apply fails with actionable unresolved diagnostics
    - depends_on:
        - 3
        - 4
      id: 5
      scope: internal/brownfield/extract/, internal/brownfield/synthesize/, internal/brownfield/archive/, .mindspec/docs/, .mindspec/lineage/
      title: Canonical synthesis, lineage, and transactional archive apply
      verify:
        - Canonical docs include source coverage and conflicts sections
        - Apply is transactional (stage -> validate -> atomic promote -> archive)
        - Archive output is `docs_archive/<run-id>/<original-relative-path>.md`
    - depends_on:
        - 5
      id: 6
      scope: internal/doctor/, internal/brownfield/*_test.go, cmd/mindspec/init*_test.go
      title: Resume, doctor updates, and end-to-end migration tests
      verify:
        - '`--resume <run-id>` continues from checkpoints and completes successfully'
        - '`mindspec doctor` passes on migrated canonical layout'
        - Repeat apply on unchanged corpus is idempotent
    - depends_on:
        - 6
      id: 7
      scope: Apply brownfield migration outputs to this repository and validate workflow continuity
      title: Dogfood migration in the mindspec repository
      verify:
        - This repository is migrated with canonical docs, lineage manifest, and archive outputs
        - Core workflow checks pass after migration (doctor/validate and key docs-path consumers)
        - Architecture artifact locations are reconciled in this repo (core/ARCHITECTURE + ADRs under canonical docs; policies at .mindspec/policies.yml)
        - Policy reference fields point to canonical .mindspec/docs/* paths
        - Migration artifacts and structural changes are committed as proof
---

# Plan: Spec 034 — Brownfield Project Onboarding for `mindspec init`

**Spec**: [spec.md](spec.md)

---

## Context

The approved direction is an optimal canonical MindSpec root under `.mindspec/` with legacy docs archived after migration. This plan adds explicit architecture and execution details for:
- scoped ADR supersession (storage-path semantics only),
- workspace path migration (`internal/workspace/workspace.go` and consumers),
- LLM provider/fallback behavior,
- deterministic flag semantics, and
- transactional apply/rollback behavior.

This plan also addresses the repo's dual role:
1. MindSpec product implementation changes.
2. MindSpec repository dogfooding migration as a required final delivery step.

---

## ADR Fitness

| ADR | Verdict | Notes |
|-----|---------|-------|
| ADR-0001 (DDD Enablement + DDD-Informed Context Packs) | **Diverge (Scoped Supersession Required)** | Only canonical storage-path semantics are superseded. DDD principles, context boundaries, and ADR governance remain in force. |
| ADR-0005 (Explicit state file) | **Conform** | Brownfield migration checkpoints and lineage metadata under `.mindspec/` align with explicit persisted state. |
| ADR-0012 (Compose with external CLIs) | **Conform** | Brownfield pipeline remains a thin orchestration layer with deterministic stage boundaries and minimal abstraction overhead. |

---

## Bead 034-A: Scoped ADR supersession for canonical path semantics

**Scope**: Introduce a narrowly scoped ADR that supersedes only ADR-0001's canonical storage-location semantics.

**Steps**:
1. Draft a new ADR defining `.mindspec/docs/` as canonical root and marking superseded ADR-0001 sections precisely
2. Explicitly document what is unchanged from ADR-0001 (DDD boundaries, domain contracts, context-pack philosophy)
3. Reconcile architecture artifact mapping in architecture docs:
   - `docs/core/ARCHITECTURE.md` and `docs/adr/*.md` migrate to canonical docs root
   - `architecture/policies.yml` migrates to `.mindspec/policies.yml`
4. Update `docs/core/ARCHITECTURE.md` canonical path references to the new root
5. Update `docs/core/CONVENTIONS.md` path conventions and archive behavior
6. Obtain human acceptance of the superseding ADR before implementation beads proceed

**Verification**:
- [ ] New ADR references partial supersession scope (storage paths only)
- [ ] Core architecture/conventions docs have no conflicting canonical path statements
- [ ] DDD requirements from ADR-0001 remain explicitly preserved
- [ ] Architecture artifact mapping is explicit and unambiguous (docs artifacts migrated, policies file moved to `.mindspec/policies.yml`)

**Depends on**: nothing

---

## Bead 034-B: Init mode contract and embedded template migration

**Scope**: Define and implement precise `init` mode semantics while removing workspace template dependency.

**Steps**:
1. Refactor `mindspec init` into explicit greenfield and brownfield flows
2. Move templates to embedded binary assets and remove read/write dependency on `docs/templates/`
3. Move policy bootstrap target from `architecture/policies.yml` to `.mindspec/policies.yml`
4. Implement brownfield flag contract:
   - `--brownfield` selects migration mode
   - default action is report-only
   - `--apply` and `--report-only` are mutually exclusive
   - `--archive` is only valid with `--apply`
5. Ensure greenfield `mindspec init` continues to bootstrap required artifacts without workspace templates
6. Add CLI help and error messaging to make flag interactions deterministic and discoverable

**Verification**:
- [ ] `mindspec init` still succeeds for greenfield bootstrapping
- [ ] `mindspec init --brownfield` performs report-only behavior with no canonical writes
- [ ] Greenfield bootstrap writes `.mindspec/policies.yml` (not `architecture/policies.yml`)
- [ ] Invalid flag combinations fail with clear diagnostics

**Depends on**: 034-A

---

## Bead 034-C: Canonical workspace path migration across consumers

**Scope**: Migrate docs path resolution from legacy `docs/` assumptions to canonical-first path APIs.

**Steps**:
1. Update `internal/workspace/workspace.go` with canonical-first docs path helpers/resolvers for docs/specs/adr/domains/context-map/glossary
2. Add legacy read fallback resolution for non-migrated repos where canonical root is absent
3. Migrate `PoliciesPath()` from `architecture/policies.yml` to `.mindspec/policies.yml`, with legacy fallback for pre-migration repos
4. Migrate docs-path consumers (context pack assembly, domain commands, validators, doctor, and related path call sites) to canonical-first helpers
5. Ensure write paths target canonical docs root for docs artifacts and `.mindspec/policies.yml` for policy writes
6. Add/adjust workspace and consumer tests for canonical-first and legacy-fallback behaviors, including policy-path migration invariants

**Verification**:
- [ ] Canonical docs are used automatically when `.mindspec/docs/` exists
- [ ] Legacy repos without canonical docs remain readable through fallback
- [ ] No write path targets legacy `docs/` canonical locations post-migration
- [ ] Policy reads/writes target `.mindspec/policies.yml` after migration, with legacy fallback only before migration

**Depends on**: 034-A

---

## Bead 034-D: Deterministic discovery/classification with LLM provider fallback

**Scope**: Implement deterministic inventory and classification pipeline with explicit LLM integration contract.

**Steps**:
1. Implement deterministic markdown discovery with stable ordering, ignore globs, and per-file hashes
2. Implement rule-based category scoring and confidence calculation
3. Add LLM classification provider interface/config resolution and record provider/model metadata in run artifacts
4. Route only low-confidence docs to LLM classification using strict JSON schema outputs
5. Implement no-LLM behavior: report-only succeeds with unresolved diagnostics; apply fails when unresolved classifications remain

**Verification**:
- [ ] Two identical runs produce byte-identical inventory/classification artifacts
- [ ] High-confidence docs skip LLM classification
- [ ] No-LLM apply run fails with explicit remediation output

**Depends on**: 034-B

---

## Bead 034-E: Canonical synthesis, lineage, and transactional archive apply

**Scope**: Generate canonical docs with citations/conflicts and archive legacy docs through a transactional apply flow.

**Steps**:
1. Implement per-category extraction and deterministic synthesis with chunked merge for large categories
2. Write canonical output to run staging area (`.mindspec/migrations/<run-id>/staging/`)
3. Validate coverage/conflicts/lineage completeness before promotion
4. Atomically promote staged canonical docs to `.mindspec/docs/`
5. Migrate legacy `architecture/policies.yml` to `.mindspec/policies.yml` and rewrite `reference:` links to canonical `.mindspec/docs/*` paths
6. Execute archive policy (`copy`/`move`) only after successful promotion; emit lineage manifest with source/canonical/archive mapping

**Verification**:
- [ ] Canonical docs include source coverage and conflict sections
- [ ] Failed synthesis/validation does not mutate source docs destructively
- [ ] Successful apply yields canonical docs + archive tree + lineage manifest
- [ ] Policy migration outputs `.mindspec/policies.yml` with canonicalized references

**Depends on**: 034-C, 034-D

---

## Bead 034-F: Resume, doctor updates, and end-to-end migration tests

**Scope**: Ensure migration is resumable, idempotent, and validated by `doctor`.

**Steps**:
1. Add stage checkpoints in `.mindspec/migrations/<run-id>/state.json`
2. Implement `--resume <run-id>` to continue incomplete runs without replaying completed deterministic stages
3. Extend `internal/doctor` checks for canonical docs layout and migration metadata integrity
4. Add integration tests for resume, no-LLM fallback semantics, and transactional apply behavior
5. Add end-to-end fixture tests covering mixed-category brownfield repositories and unchanged-corpus idempotency

**Verification**:
- [ ] Interrupted runs resume and complete successfully
- [ ] `mindspec doctor` passes after successful brownfield apply
- [ ] Re-applying unchanged corpus produces byte-identical canonical outputs

**Depends on**: 034-E

---

## Bead 034-G: Dogfood migration in the mindspec repository

**Scope**: Apply the finished brownfield migration flow to this repository itself and keep it as the canonical proof-of-adoption artifact.

**Steps**:
1. Run the new brownfield flow in this repository (`report-only` then `--apply`) and review generated migration diagnostics
2. Promote and inspect canonical docs output under `.mindspec/docs/`, lineage under `.mindspec/lineage/`, and archive under `docs_archive/<run-id>/`
3. Verify architecture artifact end-state in this repo:
   - `.mindspec/docs/core/ARCHITECTURE.md` exists
   - `.mindspec/docs/adr/` contains canonical ADR corpus
   - `.mindspec/policies.yml` is canonical policy source
4. Update any remaining repository-specific references that still assume legacy canonical `docs/` or `architecture/policies.yml` paths
5. Verify `.mindspec/policies.yml` `reference:` fields target canonical `.mindspec/docs/*` paths
6. Execute workflow checks (`mindspec doctor`, relevant `mindspec validate` commands, and targeted consumer checks) against migrated layout
7. Commit the migration artifacts and structural updates as explicit dogfooding evidence

**Verification**:
- [ ] This repository contains committed canonical docs, lineage manifest, and archive outputs from brownfield apply
- [ ] Doctor and validation checks pass on the migrated repository layout
- [ ] No unresolved repository-local path assumptions remain in core workflow docs/config
- [ ] Architecture artifacts resolve exactly as intended: canonical docs migrated and policy file canonical at `.mindspec/policies.yml`

**Depends on**: 034-F

---

## Dependency Graph

```text
034-A (Scoped ADR supersession)
  ├── 034-B (Init mode + flag contract + embedded templates)
  │     └── 034-D (Discovery/classification + LLM fallback)
  └── 034-C (Canonical workspace path migration)

034-D + 034-C
  └── 034-E (Synthesis + transactional apply + archive)
        └── 034-F (Resume + doctor + tests)
              └── 034-G (Dogfood migration in this repository)
```
