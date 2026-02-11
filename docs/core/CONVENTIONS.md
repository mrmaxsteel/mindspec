# MindSpec Conventions

This document outlines the file organization, naming, and structural conventions for MindSpec-managed projects.

## File Organization

| Path | Purpose |
|:-----|:--------|
| `docs/core/` | Permanent architectural and convention documents |
| `docs/domains/<domain>/` | Domain-scoped documentation (overview, architecture, interfaces, runbook, ADRs) |
| `docs/specs/` | Historical and active specifications |
| `docs/context-map.md` | Bounded context relationships and integration contracts |
| `docs/adr/` | Cross-cutting architecture decision records |
| `architecture/` | Machine-readable policies |
| `GLOSSARY.md` | Concept-to-doc-section mapping for context injection |
| `docs/templates/` | Templates for specs, ADRs, domain docs |
| `AGENTS.md` | Agent behavioral instructions |
| `CLAUDE.md` | Claude Code project instructions |
| `mindspec.md` | Product specification (source of truth) |

## Domain Doc Structure

Each domain lives at `/docs/domains/<domain>/` with:

| File | Purpose |
|:-----|:--------|
| `overview.md` | What the domain owns, its boundaries |
| `architecture.md` | Key patterns, invariants |
| `interfaces.md` | APIs, events, contracts (published language) |
| `runbook.md` | Ops/dev workflows |
| `adr/ADR-xxxx.md` | Domain-scoped architecture decision records |

## Specification Naming

Specs follow the pattern `NNN-slug-name`:
- `001-skeleton`
- `002-glossary`
- `003-context-pack`

## ADR Naming

ADRs follow the pattern `ADR-NNNN.md`:
- Cross-cutting: `docs/adr/ADR-NNNN.md`
- Domain-scoped: `docs/domains/<domain>/adr/ADR-NNNN.md`

ADR metadata must include: domain(s), status (proposed/accepted/superseded), supersedes/superseded-by links, decision + rationale + consequences.

## Beads Conventions

- Spec beads contain a **concise summary** and **link to the canonical spec file**. No long-form content.
- Implementation beads contain: scope, micro-plan (3-7 steps), verification steps, dependencies.
- Keep the active workset intentionally small. Regularly clean up completed beads.
- Rely on git history + documentation for historical traceability, not Beads as archive.

## Worktree Conventions

- Worktrees are named with the bead ID: `worktree-<bead-id>`
- One worktree per implementation bead
- Changes are isolated per bead
- Closing a bead requires clean state sync from worktree

## Glossary Conventions

- **Pathing**: Always use **relative paths** from the project root for glossary targets (e.g., `docs/core/ARCHITECTURE.md#section-id`). Do not use absolute paths.
- **Format**: Use the standard table format: `| **Term** | [label](relative/path#anchor) |`.
- **Coverage**: Every new concept introduced in a spec or domain doc should have a glossary entry.

## Documentation Anchors

Use stable Markdown header anchors for deterministic section retrieval:
`## Component X {#component-x}`

## Tooling Interface (Tentative)

The primary interface will be a CLI. Usage pattern:

- `mindspec context pack <spec-id>`: Generate context for an agent session
- `mindspec validate spec <id>`: Check acceptance criteria quality
- `mindspec validate docs`: Verify doc-sync compliance
- `mindspec doctor`: Project structure health check
