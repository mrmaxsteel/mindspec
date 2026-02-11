# Core Domain — Overview

## What This Domain Owns

The **core** domain owns the foundational infrastructure of MindSpec:

- **CLI entry point** (`python -m mindspec`) and command routing
- **Project health validation** (`mindspec doctor`) — structure checks, broken-link detection
- **Policy framework** — loading and evaluating machine-readable policies from `architecture/policies.yml`
- **Workspace resolution** — finding the project root, locating standard directories

## Boundaries

Core does **not** own:
- Glossary parsing, context pack assembly, or provenance tracking (context-system)
- Mode enforcement logic, spec/plan lifecycle, or Beads/worktree integration (workflow)

Core provides the CLI shell and health infrastructure that other domains plug into.

## Key Files

| File | Purpose |
|:-----|:--------|
| `src/mindspec/__main__.py` | CLI entry point |
| `src/mindspec/cli.py` | Command definitions |
| `src/mindspec/doctor.py` | Health check logic |
| `src/mindspec/workspace.py` | Project root detection |
| `architecture/policies.yml` | Machine-checkable policies |

## Current State

Skeleton implementation exists (Spec 001). CLI entry point and basic doctor command are in progress.
