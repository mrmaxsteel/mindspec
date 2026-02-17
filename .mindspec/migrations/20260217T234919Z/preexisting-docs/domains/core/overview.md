# Core Domain — Overview

## What This Domain Owns

The **core** domain owns the foundational infrastructure of MindSpec:

- **CLI entry point** (`mindspec`) and command routing via cobra
- **Project health validation** (`mindspec doctor`) — structure checks, broken-link detection, Beads hygiene
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
| `cmd/mindspec/main.go` | CLI entry point |
| `cmd/mindspec/root.go` | Root command + subcommand registration |
| `cmd/mindspec/doctor.go` | Doctor command wiring |
| `cmd/mindspec/stubs.go` | Stub commands (instruct, next, validate) |
| `internal/workspace/workspace.go` | Project root detection |
| `internal/doctor/` | Health check logic (docs, beads) |
| `architecture/policies.yml` | Machine-checkable policies |

## Current State

Go CLI skeleton implemented (Spec 001). Doctor command validates docs structure and Beads hygiene.

### Doctor Checks (Spec 000)

The `doctor` command validates:
- **Docs structure**: `docs/` directory, `GLOSSARY.md`, domain directories
- **Glossary links**: broken link detection for glossary targets
- **Beads hygiene**: `.beads/` exists, durable state present (`issues.jsonl`, `config.yaml`, `metadata.json`), no runtime artifacts (`bd.sock`, `*.db`, locks) tracked by git
  - Exits non-zero if runtime artifacts are git-tracked
