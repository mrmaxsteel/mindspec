# Spec 015-project-bootstrap: mindspec init — Project Bootstrap

## Goal

Enable new adopters to bootstrap a MindSpec-compatible project with a single command. `mindspec init` creates the required directory structure, starter files, and state so that `mindspec doctor` passes and the spec-driven workflow is immediately usable. The initial target is Claude Code; other agent runtimes (Codex, Cursor, etc.) are out of scope for now.

## Background

Today, adopting MindSpec requires manually creating `docs/`, `architecture/`, `.mindspec/`, templates, a glossary, a context map, and domain scaffolding. This friction defeats Goal #8 (CLI-first, minimal IDE glue) — the tool should bring you to a working state, not a README full of mkdir instructions.

The `doctor` command already codifies what a healthy project looks like (required dirs, files, domains). `mindspec init` is the constructive counterpart: it produces the structure that `doctor` validates.

## Impacted Domains

- **core**: New CLI command (`cmd/mindspec/init.go`) and bootstrap logic (`internal/init/`)
- **workflow**: Initializes `.mindspec/state.json` to idle, enabling the mode lifecycle

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): DDD Enablement — init must create `docs/context-map.md` and domain scaffolding matching ADR-0001 expectations
- [ADR-0005](../../adr/ADR-0005.md): Explicit State Tracking — init creates `.mindspec/state.json` committed to git

## Requirements

1. `mindspec init` creates the full directory tree expected by `mindspec doctor`
2. Generates starter `GLOSSARY.md` with core MindSpec terms if not already present
3. Generates minimal `CLAUDE.md` (Claude Code agent instructions) pointing to `mindspec instruct` if not already present
4. Generates `docs/context-map.md` placeholder if not already present
5. Copies spec/plan/ADR/domain templates into `docs/templates/`
6. Initializes `.mindspec/state.json` to `{mode: "idle"}` if not already present
7. Generates starter `architecture/policies.yml` with baseline policies if not already present
8. Checks for `bd` / `beads` in PATH; if absent, prints advisory (does not fail)
9. All file creation is additive — existing files are never overwritten
10. Prints a summary of created vs skipped items on completion

## Scope

### In Scope

- `cmd/mindspec/init.go` — cobra command wiring
- `internal/init/init.go` — bootstrap logic (dir creation, template copying, starter file generation)
- `internal/init/templates/` — embedded starter file content (glossary, CLAUDE.md, context-map, policies, state)
- Updates to `cmd/mindspec/root.go` to register the new command
- `docs/templates/` — reference templates already exist; init copies them if target project lacks them

### Out of Scope

- Running `beads init` automatically (advisory only — user controls their Beads setup)
- Domain doc content generation beyond empty templates (that's spec 011)
- Git repository initialization (`git init`) — assumes git already exists
- Interactive prompts or TUI — pure CLI with flags

## Non-Goals

- Migrating existing non-MindSpec projects (no "upgrade" path in this spec)
- Generating project-specific domain content (init produces templates, not filled-in docs)
- Supporting agent runtimes other than Claude Code (Codex, Cursor, Antigravity, etc. — future specs)

## Acceptance Criteria

- [ ] Running `mindspec init` in an empty directory (with git) creates all required dirs and files
- [ ] Running `mindspec init` in an already-bootstrapped project skips existing files and prints "skipped" for each
- [ ] After `mindspec init`, `mindspec doctor` reports zero errors on the created structure
- [ ] Beads advisory is printed when `bd` is not in PATH, but command exits 0
- [ ] `mindspec init --dry-run` prints what would be created without writing anything
- [ ] State file is initialized to `{"mode":"idle"}` and is valid for `mindspec state show`

## Validation Proofs

- `mindspec init --dry-run` in a temp dir: lists all files/dirs that would be created
- `mindspec init` in a temp dir followed by `mindspec doctor`: zero errors
- `mindspec init` run twice: second run reports all items skipped, no file content changes
- `mindspec state show` after init: reports mode=idle

## Open Questions

*None — all resolved.*

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-14
- **Notes**: Approved via mindspec approve spec