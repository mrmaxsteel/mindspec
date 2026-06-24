---
adr_citations:
    - id: ADR-0003
      sections:
        - ADR Fitness
    - id: ADR-0012
      sections:
        - ADR Fitness
    - id: ADR-0017
      sections:
        - ADR Fitness
approved_at: "2026-02-26T07:17:32Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-26T07:15:00Z"
spec_id: 042-copilot-cli-support
status: Approved
version: 1
---

# Plan: 042-copilot-cli-support — Copilot Integration Support (CLI + VS Code)

## Overview

Add first-class Copilot support to MindSpec covering three areas:

1. **Bootstrap**: `mindspec init` creates `.github/copilot-instructions.md` (pointing to AGENTS.md per ADR-0017)
2. **Setup command**: `mindspec setup copilot` — creates `.github/copilot-instructions.md` + prompt files (`.github/prompts/*.prompt.md`) with feature parity to Claude's slash commands
3. **User guide**: `project-docs/user/guides/copilot.md` covering both Copilot CLI and VS Code Chat

### Brownfield migrate provider — descoped

Spec requirements 8-12 assumed `mindspec migrate` runs LLM subprocess calls for classification. Per Spec 045 / ADR-0017, migrate now emits a prompt to stdout — the agent does the classification natively. There are no LLM subprocess calls to route, so provider-based classification routing is moot. The acceptance criteria referencing `MINDSPEC_LLM_PROVIDER` are satisfied by design: migrate has no provider dependency.

If LLM-assisted classification is reintroduced in the future, provider routing would be a separate spec.

## ADR Fitness

- **ADR-0003** (Centralized instruction emission): Sound. `copilot-instructions.md` will point to AGENTS.md, which points to `mindspec instruct`. No divergence.
- **ADR-0012** (Compose, don't wrap): Sound. No Copilot CLI wrapper needed — the guide instructs users to call `mindspec` directly. No divergence.
- **ADR-0017** (Agent onboarding — AGENTS.md layering): Sound. This spec is a direct application of ADR-0017's design: agent-specific file → AGENTS.md → `mindspec instruct`. The Copilot integration follows the same pattern established for CLAUDE.md. No divergence.

## Testing Strategy

- **Unit tests**: `internal/bootstrap/bootstrap_test.go` — verify `.github/copilot-instructions.md` appears in manifest, additive creation, idempotent append. `internal/setup/copilot_test.go` — verify setup creates correct artifacts.
- **Integration**: `make test` passes after all changes.
- **Manual verification**: `mindspec init` in a temp dir confirms Copilot artifacts created; `mindspec setup copilot` confirms setup works.

## Bead 1: Add `.github/copilot-instructions.md` to init manifest

**Steps**
1. Add a `copilot-instructions.md` manifest item to `internal/bootstrap/bootstrap.go` under the `.github/` path
2. Content: pointer to AGENTS.md per ADR-0017, plus Copilot-specific note about running `mindspec instruct` via terminal
3. Include `appendBlock` for idempotent append when `.github/copilot-instructions.md` already exists
4. Update `FormatSummary()` next-step text to mention `mindspec setup copilot` alongside `mindspec setup claude`
5. Add tests in `internal/bootstrap/bootstrap_test.go`: greenfield creation, existing-file append, idempotent skip

**Verification**
- [ ] `go test ./internal/bootstrap/...` passes with new Copilot tests
- [ ] `make test` passes

**Depends on**
None

## Bead 2: Add `mindspec setup copilot` command

**Steps**
1. Create `internal/setup/copilot.go` with `RunCopilot(root, check)` following the pattern in `claude.go`
2. Setup creates/updates `.github/copilot-instructions.md` (idempotent via marker, same as CLAUDE.md pattern)
3. Create prompt files in `.github/prompts/` — Copilot's equivalent of Claude's `.claude/commands/` slash commands. Each `.prompt.md` file uses YAML frontmatter (`description`, `agent: "agent"`) and a markdown body that instructs the agent to call the corresponding `mindspec` CLI command. Create these 5 files mirroring Claude parity:
   - `spec-init.prompt.md` — `mindspec spec-init`
   - `spec-approve.prompt.md` — `mindspec approve spec`
   - `plan-approve.prompt.md` — `mindspec approve plan`
   - `impl-approve.prompt.md` — `mindspec approve impl`
   - `spec-status.prompt.md` — `mindspec state show` + `mindspec instruct`
4. Prompt file content should be functionally equivalent to the Claude command files in `commandFiles()` but use the `.prompt.md` format with appropriate frontmatter
5. Register the `copilot` subcommand under `setup` in `cmd/mindspec/setup.go`
6. Add tests in `internal/setup/copilot_test.go`: creation, prompt file existence, idempotent re-run, `--check` dry-run

**Verification**
- [ ] `go test ./internal/setup/...` passes
- [ ] 5 prompt files created in `.github/prompts/`
- [ ] `make test` passes

**Depends on**
Bead 1

## Bead 3: Write Copilot user guide

**Steps**
1. Create `project-docs/user/guides/copilot.md` covering both Copilot CLI and VS Code Chat
2. Structure: prerequisites, setup (`mindspec init` + `mindspec setup copilot`), session-start pattern, the full workflow (spec → plan → implement → review), differences from Claude/Codex
3. Document VS Code Chat specifics: `.github/copilot-instructions.md` is read automatically, use integrated terminal for `mindspec` commands, `@workspace` for codebase awareness
4. Document Copilot CLI specifics: reads AGENTS.md directly, terminal-native command execution
5. Add comparison table (like the Codex guide has) showing Claude vs Codex vs Copilot CLI vs Copilot Chat differences
6. Update guide index references (if any exist) to list Copilot alongside Claude and Codex

**Verification**
- [ ] Guide file exists at `project-docs/user/guides/copilot.md`
- [ ] Guide covers both CLI and VS Code Chat workflows
- [ ] `mindspec doctor` passes (no broken doc structure)
- [ ] `make test` passes

**Depends on**
Bead 1, Bead 2 (guide references setup command)

## Bead 4: Update entry points and docs references

**Steps**
1. Update any README or guide index that lists supported agents to include Copilot (CLI + VS Code Chat)
2. Update `FormatSummary()` in bootstrap to mention `mindspec setup copilot`
3. Verify `project-docs/user/guides/` index (if any) lists the new copilot guide
4. Verify `mindspec doctor` doesn't flag the new guide as orphaned

**Verification**
- [ ] Copilot listed in user-facing entry points alongside Claude and Codex
- [ ] `mindspec doctor` passes
- [ ] `make test` passes

**Depends on**
Bead 3

## Provenance

| Acceptance Criterion | Bead | Verification |
|:---------------------|:-----|:-------------|
| Copilot guide at `copilot.md` covering CLI + VS Code | Bead 3 | Guide file exists, covers both workflows |
| Guide documents VS Code Chat workflow | Bead 3 | VS Code-specific sections present |
| `mindspec init` creates `.github/copilot-instructions.md` additively | Bead 1 | Bootstrap tests: greenfield + append + idempotent |
| `copilot-instructions.md` points to AGENTS.md (ADR-0017) | Bead 1 | Content check in test |
| `mindspec setup copilot` creates prompt files with slash command parity | Bead 2 | 5 `.prompt.md` files in `.github/prompts/`, setup tests pass |
| `MINDSPEC_LLM_PROVIDER=copilot-cli` migrate routing | N/A | Descoped — migrate has no LLM subprocess (Spec 045) |
| `MINDSPEC_LLM_PROVIDER=off` deterministic mode | N/A | Descoped — migrate has no LLM subprocess (Spec 045) |
| Provider auto-detection for Copilot | N/A | Descoped — migrate has no LLM subprocess (Spec 045) |
| Existing Claude migrate tests green | N/A | No migrate code changed |
| New Copilot provider tests | N/A | Descoped — no provider routing to test |
| `make test` passes | Bead 1-4 | `make test` in each bead verification |
