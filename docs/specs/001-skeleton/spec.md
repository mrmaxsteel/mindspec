# Spec 001: Mindspec Skeleton

## Goal

Establish the minimal CLI foundation with a `doctor` command to validate project structure health.

## Background

This is the bootstrap spec for mindspec. It creates the bare minimum CLI scaffolding and a single useful command (`doctor`) that can immediately validate the mindspec project structure.

## Requirements

1. **CLI Entry Point**: Python CLI accessible via `python -m mindspec`
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
- Workspace resolution beyond basic project root detection
- Memory service
- Task graph generation

## Acceptance Criteria

- [ ] `python -m mindspec` displays help and available commands
- [ ] `python -m mindspec doctor` runs without error on valid project
- [ ] Doctor validates: `docs/core/` exists, `GLOSSARY.md` exists, `docs/specs/` exists
- [ ] Doctor reports missing directories/files with actionable messages
- [ ] Exit code is 0 when all checks pass, 1 when any check fails

## Validation Proofs

- `python -m mindspec --help`: Should display usage and available commands
- `python -m mindspec doctor`: Should report project health status

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: Narrowed scope from original; glossary and context pack moved to separate specs.
