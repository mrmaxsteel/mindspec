# Mindspec Conventions

This document outlines the coding and documentation conventions for the `mindspec` project.

## File Organization

*   `docs/core/`: Permanent architectural and convention documents.
*   `docs/features/`: Living documentation for specific features.
*   `docs/specs/`: Historical and active specifications.
*   `architecture/`: Machine-readable policies and configuration.

## Specification Naming

Specs should follow the pattern `NNN-slug-name`:
*   `001-skeleton`
*   `002-memory-service`

## Tooling Interface (Tentative)

The primary interface will be a CLI implemented in Python. Usage pattern:

*   `python -m mindspec context pack <spec-id>`: Generates context for an agent.
*   `python -m mindspec task execute <task-id>`: Starts execution phase for a task.
*   `python -m mindspec validate run <spec-id>`: Runs validation gates for a spec.
*   `python -m mindspec arch propose <title>`: Generates an ACP template.

## Documentation Anchors

Use stable HTML-style comment anchors or Markdown headers for deterministic section retrieval:
`## Component X {#component-x}`

## Glossary Conventions

*   **Pathing**: Always use **relative paths** from the project root for glossary targets (e.g., `docs/core/ARCHITECTURE.md#section-id`). Do not use absolute paths or environment-specific URLs.
*   **Format**: Use the standard table format: `| **Term** | [label](relative/path#anchor) |`.

