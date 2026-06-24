---
adr_citations: []
approved_at: "2026-02-21T09:17:27Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-21"
spec_id: 045-migrate-prompt-emission
status: Approved
version: 1
---

# Plan: 045-migrate-prompt-emission

## Summary

Two beads: (1) delete `internal/brownfield/`, rewrite `cmd/mindspec/migrate.go` as a prompt emitter; (2) add `mindspec setup claude` command to bootstrap Claude Code-specific configuration (hooks, slash commands, CLAUDE.md pointer).

## ADR Fitness

No ADRs govern the migrate implementation. The change aligns with the CLI-first design philosophy (Goal #8 in mindspec.md): delegate work the agent can do natively rather than reimplementing it in Go.

## Beads

## Bead 1: Replace migrate with prompt emission

**Scope**: Delete brownfield package, rewrite migrate command, update tests and references.

**Steps**:

1. Delete `internal/brownfield/` (all 6 files: `discovery.go`, `discovery_test.go`, `plan.go`, `apply.go`, `llm.go`, `llm_test.go`)
2. Rewrite `cmd/mindspec/migrate.go`:
   - Remove all imports of `internal/brownfield`, remove `plan`/`apply` subcommands, remove all helper functions (`resolveArchiveMode`, `validateMigrateApplyFlags`, `readMigrationArtifactJSON`, `writeMigratePlanOverview`, `newMigratePlanProgressWriter`, `requireCleanGitTree`, `isGitRepository`)
   - New `migrateCmd` is a leaf command (no subcommands) with `--json` flag
   - `RunE` does: scan repo for `.md` files (gitignore-aware walk, skip `.git`, `.beads`, `.claude`, `.mindspec/docs`, `.mindspec/migrations`, `docs_archive`, nested `.git` dirs, `beads/`, `worktree-*`); list existing `.mindspec/docs/` contents; emit prompt or JSON
   - Prompt includes: canonical structure, category rubric (adr, spec, domain, core, context-map, glossary, user-docs, agent), source files, canonical files, agent instructions
   - The `agent` category covers agent instruction files (e.g. `CLAUDE.md`, `agents.md`, `.cursorrules`) with canonical target `.mindspec/docs/agent/`
3. Rewrite `cmd/mindspec/migrate_test.go`: remove old tests, add test for scan (temp dir with sample `.md` files) and prompt output verification
4. Update `cmd/mindspec/init.go` line 21: change `"Use 'mindspec migrate plan' and 'mindspec migrate apply' to onboard\nan existing brownfield repository."` → `"Use 'mindspec migrate' to onboard an existing brownfield repository."`
5. Remove subcommand registration from `cmd/mindspec/migrate.go` `init()` — the new `migrateCmd` has no subcommands (just `--json` flag)

**Depends on**: none

**Verification**:

- [ ] `make build` succeeds with no references to `internal/brownfield`
- [ ] `make test` passes (includes `cmd/mindspec/migrate_test.go`)
- [ ] `./bin/mindspec migrate` outputs prompt with discovered files
- [ ] `./bin/mindspec migrate --json | jq .` outputs valid JSON
- [ ] `./bin/mindspec migrate plan` errors (unknown command)

## Bead 2: Add `mindspec setup claude` command

**Scope**: New command that bootstraps Claude Code-specific configuration: hooks, slash commands, and CLAUDE.md with AGENTS.md pointer.

**Steps**:

1. Create `internal/setup/claude.go`:
   - `Run(root string, check bool) (*Result, error)` — main entry point
   - `Result` struct with `Created`, `Skipped`, `Existing` string slices
   - Creates `.claude/settings.json` with hooks:
     - `SessionStart`: runs `mindspec instruct`
     - `PreToolUse` (ExitPlanMode matcher): plan gate check
   - If `.claude/settings.json` exists, merges hooks (adds missing ones, preserves existing)
   - Creates `.claude/commands/*.md` (5 files): `spec-init.md`, `spec-approve.md`, `plan-approve.md`, `impl-approve.md`, `spec-status.md`
   - Each command file has YAML frontmatter (`allowed-tools`, `description`) and body that calls the CLI
   - Appends MindSpec block to `CLAUDE.md` using `<!-- mindspec:managed -->` marker (same pattern as bootstrap)
   - CLAUDE.md block includes pointer to AGENTS.md
   - If beads CLI is found (`bd` in PATH), runs `bd setup claude` (best-effort, warns on failure)
   - `--check` mode: reports what would be created/updated, writes nothing
2. Create `internal/setup/claude_test.go`:
   - Test fresh setup creates all expected files
   - Test idempotent re-run skips existing files
   - Test `--check` mode writes nothing
   - Test existing `settings.json` with custom hooks is preserved (merge, not overwrite)
3. Create `cmd/mindspec/setup.go`:
   - `setupCmd` parent command with `claude` subcommand
   - `--check` flag on `claude` subcommand
   - Register in root command
4. Create `cmd/mindspec/setup_test.go` — basic wiring test
5. Update `internal/bootstrap/bootstrap.go` `FormatSummary()`: append hint `"Run 'mindspec setup claude' to configure Claude Code integration."` after successful init

**Depends on**: Bead 1

**Verification**:

- [ ] `make build && make test` pass
- [ ] `mindspec setup claude` in clean temp dir creates `.claude/settings.json`, 5 command files, `CLAUDE.md`
- [ ] `mindspec setup claude --check` reports status without writing
- [ ] Second `mindspec setup claude` run is idempotent
- [ ] Existing `.claude/settings.json` with custom hooks is preserved after setup

## Testing Strategy

**Bead 1** — Unit tests in `cmd/mindspec/migrate_test.go`:
- Test scan function with a temp dir containing sample `.md` files in various locations
- Test that `.git`, `.beads`, `.mindspec/docs` dirs are skipped
- Test JSON output mode produces valid JSON
- Test prompt output contains required sections (canonical structure, category rubric, file lists)

**Bead 2** — Unit tests in `internal/setup/claude_test.go`:
- Test fresh setup creates all expected files with correct content
- Test idempotent re-run reports all items as existing
- Test `--check` mode creates nothing
- Test settings.json hook merging preserves existing hooks
- Test CLAUDE.md append uses marker for idempotency

No integration or e2e tests needed — both commands are pure file creators with no external side effects (except optional `bd setup claude` which is best-effort).

## Provenance

| Acceptance Criterion | Bead | Verification |
|---|---|---|
| `make build` succeeds with no brownfield references | Bead 1 | Step 1 (delete) + `make build` |
| `make test` passes | Bead 1+2 | `make test` |
| `mindspec migrate` outputs prompt with structure/rubric/files/instructions | Bead 1 | Step 2 verification |
| `mindspec migrate --json` outputs JSON with file arrays | Bead 1 | Step 2 verification |
| `mindspec migrate plan` no longer exists | Bead 1 | Step 5 verification |
| `mindspec init --help` references `mindspec migrate` | Bead 1 | Step 4 |
| `mindspec setup claude` creates `.claude/settings.json` with hooks | Bead 2 | Step 1 verification |
| `mindspec setup claude` creates 5 slash command files | Bead 2 | Step 1 verification |
| `mindspec setup claude` appends MindSpec block to CLAUDE.md | Bead 2 | Step 1 verification |
| `mindspec setup claude --check` reports without writing | Bead 2 | Step 1 verification |
| Idempotent re-run of setup | Bead 2 | Step 2 verification |
