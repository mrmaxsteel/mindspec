# Spec 001: MindSpec Skeleton

## Goal

Establish the minimal CLI foundation with a `doctor` command to validate project structure health.

## Background

This is the bootstrap spec for MindSpec. It creates the bare minimum CLI scaffolding and a single useful command (`doctor`) that can immediately validate the MindSpec project structure.

## Impacted Domains

- core: CLI entry point and project health validation

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Doctor should validate that Beads conventions are followable (docs structure exists)

## Requirements

1. **CLI Entry Point**: CLI accessible via `python -m mindspec`
2. **Doctor Command**: Health check that validates project structure
3. **Exit Codes**: Return 0 on success, non-zero on validation failures

## Scope

### In Scope
- `src/mindspec/__init__.py` (package init)
- `src/mindspec/__main__.py` (CLI entry point)
- `src/mindspec/cli.py` (CLI commands)
- `src/mindspec/doctor.py` (health check logic)

### Out of Scope
- Glossary parsing (see Spec 002)
- Context pack generation (see Spec 003)
- Beads integration tooling
- Worktree management
- Memory service

## Non-Goals

- Full workspace resolution
- Multi-repo support

## Acceptance Criteria

- [ ] `python -m mindspec` displays help and available commands
- [ ] `python -m mindspec doctor` runs without error on valid project
- [ ] Doctor validates: `docs/core/` exists, `docs/domains/` exists, `GLOSSARY.md` exists, `docs/specs/` exists, `architecture/` exists
- [ ] Doctor reports missing directories/files with actionable messages
- [ ] Exit code is 0 when all checks pass, 1 when any check fails

## Validation Proofs

- `python -m mindspec --help`: Should display usage and available commands
- `python -m mindspec doctor`: Should report project health status

## Open Questions

- (none)

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: Narrowed scope from original; glossary and context pack moved to separate specs.
