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

Single implementation bead: delete `internal/brownfield/`, rewrite `cmd/mindspec/migrate.go` as a prompt emitter, update tests and help text.

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

## Testing Strategy

Unit tests in `cmd/mindspec/migrate_test.go`:
- Test scan function with a temp dir containing sample `.md` files in various locations
- Test that `.git`, `.beads`, `.mindspec/docs` dirs are skipped
- Test JSON output mode produces valid JSON
- Test prompt output contains required sections (canonical structure, category rubric, file lists)

No integration or e2e tests needed — the command is a pure scanner + text emitter with no side effects.

## Provenance

| Acceptance Criterion | Bead | Verification |
|---|---|---|
| `make build` succeeds with no brownfield references | Bead 1 | Step 1 (delete) + `make build` |
| `make test` passes | Bead 1 | `make test` |
| `mindspec migrate` outputs prompt with structure/rubric/files/instructions | Bead 1 | Step 2 verification |
| `mindspec migrate --json` outputs JSON with file arrays | Bead 1 | Step 2 verification |
| `mindspec migrate plan` no longer exists | Bead 1 | Step 5 verification |
| `mindspec init --help` references `mindspec migrate` | Bead 1 | Step 4 |
