# mindspec

**Spec-Driven Development + Context Management Framework (Claude Code-first)**

MindSpec standardizes how teams move from user value to validated specification to bounded plans to implemented change, while keeping architecture and documentation coherent over time. It uses **Beads** as its durable work graph and **git worktrees** to safely isolate and parallelize agent execution.

## Core Principles

1. **Spec-Driven**: Every feature starts with a formal specification.
2. **Self-Documenting**: Implementation automatically updates and refactors documentation.
3. **Deterministic Context**: Progressive disclosure of architectural context via keyword-anchored docs.
4. **Durable Memory**: Locally-persisted project brain for cross-session rationale and decisions.

## Getting Started

Review the [Architecture Documentation](docs/core/ARCHITECTURE.md) to understand the system design.

The project is currently in the **Skeleton Initialization** phase. See [Spec 001: Skeleton](docs/specs/001-skeleton/spec.md) for current implementation goals.

## Project Structure

```
src/mindspec/          Python package (src layout)
docs/
  core/                Permanent architectural context (ARCHITECTURE, MODES, CONVENTIONS)
  domains/             Domain-scoped documentation (DDD)
    core/              CLI, health, policies
    context-system/    Glossary, context packs, provenance
    workflow/          Modes, spec lifecycle, Beads, worktrees
  adr/                 Cross-cutting architecture decision records
  specs/               Versioned feature specifications
  templates/           Templates for specs, ADRs, domain docs
  context-map.md       Bounded context relationships
architecture/          Machine-readable policies
GLOSSARY.md            Concept-to-doc-section mapping
AGENTS.md              Agent behavioral instructions
CLAUDE.md              Claude Code project instructions
mindspec.md            Product specification (source of truth)
```

## Usage

```bash
# Verify project health
python -m mindspec doctor

# Show available commands
python -m mindspec --help
```

## Key Documentation

| Document | Purpose |
|:---------|:--------|
| [mindspec.md](mindspec.md) | Product specification |
| [AGENTS.md](AGENTS.md) | Agent behavioral rules |
| [CLAUDE.md](CLAUDE.md) | Claude Code instructions |
| [docs/core/ARCHITECTURE.md](docs/core/ARCHITECTURE.md) | System design |
| [docs/core/MODES.md](docs/core/MODES.md) | Mode definitions |
| [docs/context-map.md](docs/context-map.md) | Bounded context relationships |
| [GLOSSARY.md](GLOSSARY.md) | Term definitions |
