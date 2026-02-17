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
