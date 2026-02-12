# CLAUDE.md — Claude Code Project Instructions

This file is for Claude Code (claude.ai/code), the AI-powered CLI tool. It provides project-specific context so Claude Code can work effectively in this repository.

## What is MindSpec?

MindSpec is a spec-driven development + context management framework (Claude Code-first). See [mindspec.md](mindspec.md) for the full product specification.

## Behavioral Rules

All behavioral rules for agents (including Claude Code) are defined in [AGENTS.md](AGENTS.md). Read it before doing any work. Key points:

- **Three-mode system**: Spec → Plan → Implement. Never write code without an approved spec AND plan.
- **ADR governance**: Divergence from accepted ADRs requires stopping and getting human approval.
- **Doc-sync**: Every code change must update corresponding documentation. "Done" includes doc-sync.

## Custom Commands

| Command | Purpose |
|:--------|:--------|
| `/spec-init` | Initialize a new specification (enters Spec Mode) |
| `/spec-approve` | Request Spec → Plan transition |
| `/plan-approve` | Request Plan → Implementation transition |
| `/spec-status` | Check current mode and active spec/bead state |

## Key Files

| File | Purpose |
|:-----|:--------|
| [mindspec.md](mindspec.md) | Product specification (source of truth) |
| [AGENTS.md](AGENTS.md) | Agent behavioral rules and mode system |
| [GLOSSARY.md](GLOSSARY.md) | Term-to-doc-section mapping for context injection |
| [docs/core/ARCHITECTURE.md](docs/core/ARCHITECTURE.md) | System design and invariants |
| [docs/core/MODES.md](docs/core/MODES.md) | Mode definitions and transitions |
| [docs/core/CONVENTIONS.md](docs/core/CONVENTIONS.md) | File organization and naming |
| [docs/context-map.md](docs/context-map.md) | Bounded context relationships |
| [architecture/policies.yml](architecture/policies.yml) | Machine-checkable policies |

## Project Layout

```
cmd/mindspec/          CLI entry point (Go, cobra)
internal/workspace/    Project root detection
internal/doctor/       Health check logic
internal/glossary/     Glossary parsing and matching
internal/contextpack/  Context pack generation
internal/state/        Workflow state management (.mindspec/state.json)
internal/instruct/     Mode-aware guidance emission (embedded templates)
internal/next/         Work selection, claiming, mode resolution
docs/core/             Permanent architectural context
docs/domains/          Domain-scoped documentation (DDD)
docs/specs/            Versioned feature specifications
docs/adr/              Cross-cutting architecture decision records
docs/templates/        Templates for specs, ADRs, domain docs
architecture/          Machine-readable policies
```

## Build & Run

```bash
make build                   # Build binary to ./bin/mindspec
./bin/mindspec --help        # CLI usage
./bin/mindspec doctor        # Project health check
./bin/mindspec next          # Claim next ready bead and get guidance
./bin/mindspec state show    # Check current mode/spec/bead
./bin/mindspec instruct      # Emit mode-aware guidance
./bin/mindspec validate spec <id>   # Validate spec quality
./bin/mindspec validate plan <id>   # Validate plan quality
./bin/mindspec validate docs        # Check doc-sync compliance
make test                    # Run all tests
```
