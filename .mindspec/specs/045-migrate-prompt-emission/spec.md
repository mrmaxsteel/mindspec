---
approved_at: "2026-02-21T08:24:59Z"
approved_by: user
molecule_id: mindspec-mol-uvg
status: Approved
step_mapping:
    implement: mindspec-mol-5hz
    plan: mindspec-mol-szs
    plan-approve: mindspec-mol-2w2
    review: mindspec-mol-0ug
    spec: mindspec-mol-2tb
    spec-approve: mindspec-mol-srj
    spec-lifecycle: mindspec-mol-uvg
---




# Spec 045-migrate-prompt-emission: Replace migrate with prompt emission + agent setup

## Goal

Replace the expensive, complex `mindspec migrate` implementation with a prompt emitter, and add `mindspec setup <agent>` to bootstrap agent-specific configuration (hooks, slash commands, instruction file pointers).

## Background

The current `mindspec migrate` spans ~700 lines across `internal/brownfield/` (6 files). It does deterministic path classification, optional LLM subprocess calls to `claude` for low-confidence docs, transactional staging, archive/lineage tracking, and drift detection. This is over-engineered for the use case: the coding agent (Claude Code) that runs `mindspec migrate` is already capable of reading, moving, and merging files itself. Emitting a structured prompt is cheaper, simpler, and more flexible than reimplementing file operations in Go.

This aligns with MindSpec's design philosophy (Goal #8): logic in CLI, but delegate work the agent can do natively.

## Impacted Domains

- workflow: changes how brownfield onboarding works (simplified to single command)
- agent-integration: adds `mindspec setup` for bootstrapping agent-specific config

## ADR Touchpoints

None directly affected. The existing ADRs do not prescribe the migrate implementation.

## Requirements

1. `mindspec migrate` scans the repo for markdown files (gitignore-aware, skipping `.git`, `.beads`, `.claude`, `.mindspec/docs`, `.mindspec/migrations`, `docs_archive`, nested git repos)
2. The command lists what already exists in `.mindspec/docs/`
3. The command emits a structured prompt to stdout containing:
   - The canonical directory structure reference
   - A category rubric (adr, spec, domain, core, context-map, glossary, user-docs, agent)
   - The discovered source file list
   - The existing canonical file list
   - Step-by-step instructions for the agent to categorize, move, merge, or create files
4. `mindspec migrate --json` outputs just the file inventory as JSON (source files + existing canonical files)
5. The `internal/brownfield/` package is deleted entirely
6. The `plan` and `apply` subcommands are removed
7. `mindspec setup claude` creates Claude Code-specific configuration:
   - `.claude/settings.json` with hooks (SessionStart → `mindspec instruct`, PreToolUse plan gates)
   - `.claude/commands/*.md` slash commands (spec-init, spec-approve, plan-approve, impl-approve, spec-status) as thin shims calling CLI
   - CLAUDE.md with pointer to AGENTS.md and MindSpec guidance block (idempotent append via `<!-- mindspec:managed -->` marker)
   - Optionally chains `bd setup claude` if beads CLI is installed
8. `mindspec setup claude` is idempotent — re-running it skips files/sections that already exist
9. `mindspec setup claude --check` reports what would be created/updated without writing
10. `mindspec init` output suggests running `mindspec setup <agent>` as next step

## Scope

### In Scope
- `cmd/mindspec/migrate.go` — full rewrite
- `cmd/mindspec/migrate_test.go` — full rewrite
- `cmd/mindspec/init.go` — update help text reference
- `internal/brownfield/` — delete all 6 files
- `cmd/mindspec/setup.go` — new command: `mindspec setup claude`
- `cmd/mindspec/setup_test.go` — tests for setup command
- `internal/setup/claude.go` — Claude Code setup logic (hooks, commands, CLAUDE.md)
- `internal/setup/claude_test.go` — unit tests

### Out of Scope
- `internal/doctor/migration.go` — keeps its own types for validating legacy migration artifacts; no brownfield import
- `internal/workspace/workspace.go` — `LegacyPoliciesPath` stays (comment-only reference)
- `cmd/mindspec/init_test.go` — tests init flags, not migrate
- `mindspec setup copilot` / `mindspec setup codex` — future work, not this spec

## Non-Goals

- Building any file-moving or archiving logic in Go
- LLM classification of documents
- Migration state tracking, lineage, or drift detection
- Backwards compatibility with existing migration artifacts (doctor still validates them independently)
- Setup commands for Copilot CLI or Codex (future specs)
- `--remove` flag for setup (future enhancement)

## Acceptance Criteria

- [ ] `make build` succeeds with no references to `internal/brownfield`
- [ ] `make test` passes
- [ ] `mindspec migrate` outputs a prompt containing: canonical structure reference, category rubric, discovered source files, existing canonical files, and agent instructions
- [ ] `mindspec migrate --json` outputs JSON with `source_files` and `canonical_files` arrays
- [ ] `mindspec migrate plan` and `mindspec migrate apply` no longer exist (exit with unknown command)
- [ ] `mindspec init --help` references `mindspec migrate` (not `migrate plan`/`migrate apply`)
- [ ] `mindspec setup claude` creates `.claude/settings.json` with SessionStart and PreToolUse hooks
- [ ] `mindspec setup claude` creates 5 slash command files in `.claude/commands/`
- [ ] `mindspec setup claude` appends MindSpec block to CLAUDE.md (or creates it) with AGENTS.md pointer
- [ ] `mindspec setup claude --check` reports status without writing files
- [ ] Running `mindspec setup claude` twice is idempotent (second run reports all items present)

## Validation Proofs

- `make build && make test`: both pass
- `./bin/mindspec migrate`: emits agent-ready prompt to stdout
- `./bin/mindspec migrate --json | jq .`: valid JSON with file arrays
- `./bin/mindspec migrate plan`: unknown command error
- `./bin/mindspec setup claude` in temp dir: creates `.claude/settings.json`, `.claude/commands/*.md`, `CLAUDE.md`
- `./bin/mindspec setup claude --check` in already-setup dir: reports all items present
- `./bin/mindspec setup claude` twice: second run is no-op

## Open Questions

None — requirements are clear.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-21
- **Notes**: Approved via mindspec approve spec