# Core Domain — Architecture

## Key Patterns

### Workspace Resolution

The `Workspace` package finds the project root by walking up from the current directory looking for `.mindspec/` or `.git`. All path resolution is relative to this root.

### Health Checks

`mindspec doctor` validates project structure. Checks are categorized:

- **Errors**: Missing critical files (e.g., `GLOSSARY.md`, `docs/core/`)
- **Warnings**: Missing optional structure (e.g., `docs/domains/`, `docs/context-map.md`)

The distinction allows fresh projects to pass basic checks while still surfacing incomplete scaffolding.

### Policy Framework

Policies in `architecture/policies.yml` are declarative rules with:
- `id`, `description`, `severity` (error/warning)
- Optional `scope` (file glob) and `mode` (spec/plan/implementation)
- `reference` pointing to the authoritative doc section

## Invariants

1. Workspace resolution must be deterministic — same directory always resolves to same root.
2. Health checks must never hard-fail on optional structure in a fresh project.
3. Policy evaluation is read-only — policies describe constraints, they don't enforce them at runtime (yet).

## Phase detail derivation and guard context (spec 092)

`internal/phase` exposes the stored-vs-derived phase split behind the
spec-092 gate hardening:

- `PhaseDetail{EpicID, Stored, Derived}` — the metadata-cached
  `mindspec_phase` alongside the child-derived ground truth
  (ADR-0023 §3/§5, ADR-0034).
- `DerivePhaseDetail(epicID)` / `DerivePhaseDetailWithCache(c, epicID)`
  — read-only derivation. Callers (`mindspec impl approve`,
  `mindspec repair phase`) decide whether to reconcile the cache
  forward; derivation itself never writes.

`internal/workspace.ContextLine(dir, checkedPath)` renders the
fixed-format worktree-context line that guard failures emit
immediately before their final `recovery:` line (spec 092 Req 8).
