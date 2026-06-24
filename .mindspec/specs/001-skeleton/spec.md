# Spec 001: Go CLI Skeleton + Doctor

## Goal

Establish the MindSpec CLI as a Go binary with workspace detection, `doctor` for project health validation, and the foundation for `instruct`/`next`/`validate` commands defined in ADR-0003.

## Background

ADR-0003 (accepted) defines centralized instruction emission via the CLI. ADR-0004 (accepted) decided Go as the v1 implementation language for cross-platform binary distribution. This replaces the existing Python prototype in `src/mindspec/`.

A working Python prototype exists (`main.py`, `workspace.py`, `docs.py`) with CLI group, doctor command, and Beads hygiene checks (Spec 000). This spec ports the doctor functionality to Go and establishes the Go project structure that all subsequent specs build on.

The Python code in `src/mindspec/` will be retired once the Go CLI reaches feature parity on `doctor`.

## Impacted Domains

- core: CLI entry point, project health validation, workspace resolution

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): Centralized instruction emission — defines CLI command surface (`instruct`, `next`, `validate`)
- [ADR-0004](../../adr/ADR-0004.md): Go as v1 implementation language
- [ADR-0002](../../adr/ADR-0002.md): Doctor validates documentation system structure; Beads hygiene checks (from Spec 000) must be ported

## Requirements

1. **Go project scaffolding**: Go module, directory layout, build configuration
2. **CLI entry point**: `mindspec` binary with subcommand routing (cobra or similar)
3. **Workspace detection**: Find project root by walking up from cwd looking for `mindspec.md` or `.git`
4. **Doctor command**: `mindspec doctor` validates project structure health
5. **Beads hygiene checks**: Port Spec 000's checks — `.beads/` existence, durable state, git-tracked runtime artifacts
6. **Exit codes**: 0 on success, 1 on validation failures
7. **Command stubs**: `mindspec instruct`, `mindspec next`, `mindspec validate` registered as commands (stub output, not implemented — establishes the ADR-0003 command surface)

## Scope

### In Scope
- `cmd/mindspec/main.go` (binary entry point)
- `internal/workspace/` (project root detection)
- `internal/doctor/` (health check logic)
- `go.mod`, `go.sum` (Go module definition)
- `Makefile` or `justfile` (build/install convenience)
- Stub commands for `instruct`, `next`, `validate`
- Retire `src/mindspec/` Python package (mark as deprecated or remove)

### Out of Scope
- `instruct` implementation (future spec)
- `next` implementation (future spec — requires Beads adapter)
- `validate` implementation (future spec)
- Glossary parsing (see Spec 002)
- Context pack generation (see Spec 003)
- Worktree management
- Cross-compilation / release automation

## Non-Goals

- Full `instruct` / `next` / `validate` implementation (stubs only)
- Plugin system for doctor checks
- Colored/rich terminal output (plain text is fine for v1)
- Multi-repo support

## Acceptance Criteria

- [ ] `go build ./cmd/mindspec` produces a working binary
- [ ] `mindspec --help` displays usage and available commands including `doctor`, `instruct`, `next`, `validate`
- [ ] `mindspec doctor` validates: `docs/core/`, `docs/domains/`, `docs/specs/`, `architecture/`, `GLOSSARY.md`, `.beads/` (existence + durable state + no tracked runtime artifacts)
- [ ] Doctor reports each check with `[OK]`/`[MISSING]`/`[ERROR]` and actionable messages
- [ ] Doctor exits 0 when all checks pass, 1 when any check fails
- [ ] `mindspec instruct` prints a stub message indicating future implementation
- [ ] `mindspec next` prints a stub message indicating future implementation
- [ ] `mindspec validate` prints a stub message indicating future implementation
- [ ] Workspace detection finds project root by walking up to `mindspec.md` or `.git`
- [ ] `src/mindspec/` Python code is marked deprecated or removed

## Validation Proofs

- `mindspec --help`: displays usage with all subcommands
- `mindspec doctor`: reports project health status with all checks (docs structure + Beads hygiene)
- `mindspec doctor` on a repo missing `docs/specs/`: exits 1 with actionable message
- `mindspec instruct`: prints stub without error

## Open Questions

- (none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-11
- **Notes**: Approved via /spec-approve workflow. Replaces Python prototype.
