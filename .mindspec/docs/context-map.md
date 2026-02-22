# MindSpec Context Map

> Declares the bounded contexts in MindSpec, their relationships, and integration contracts.

## Bounded Contexts

### Core

**Owns**: CLI entry point, project health validation, workspace resolution.

**Domain docs**: [`docs/domains/core/`](domains/core/overview.md)

### Context-System

**Owns**: Context pack assembly, DDD-informed routing, provenance tracking.

**Domain docs**: [`docs/domains/context-system/`](domains/context-system/overview.md)

### Workflow

**Owns**: Mode system (Spec/Plan/Implement), spec and plan lifecycle, Beads adapter, worktree management, validation gates.

**Domain docs**: [`docs/domains/workflow/`](domains/workflow/overview.md)

---

## Relationships

```
┌────────────┐       ┌──────────────────┐       ┌────────────┐
│    Core     │◄──────│  Context-System  │──────►│  Workflow   │
│             │       │                  │       │            │
│ CLI, health,│       │ Context packs,   │       │ Modes, spec│
│ workspace   │       │ DDD routing,     │       │ lifecycle, │
│ workspace   │       │                  │       │ Beads,     │
└──────┬──────┘       └──────────────────┘       │ worktrees  │
       │                                         └─────┬──────┘
       │                                               │
       └───────────────────────────────────────────────┘
                    (both consume Core)
```

### Core → Context-System (upstream)

Core provides workspace resolution and path infrastructure. Context-system consumes `workspace.FindRoot()`, `workspace.DocsDir()`, `workspace.SpecDir()`.

**Contract**: [`docs/domains/core/interfaces.md`](domains/core/interfaces.md)

### Core → Workflow (upstream)

Core provides CLI shell and workspace resolution. Workflow registers subcommands and uses workspace paths to locate specs and beads.

**Contract**: [`docs/domains/core/interfaces.md`](domains/core/interfaces.md)

### Workflow → Context-System (upstream)

Workflow provides spec bead metadata (impacted domains, ADR citations) that context-system uses for DDD-informed context pack assembly.

**Contract**: [`docs/domains/workflow/interfaces.md`](domains/workflow/interfaces.md) (spec metadata), [`docs/domains/context-system/interfaces.md`](domains/context-system/interfaces.md) (context pack builder)

### Context-System → Workflow (downstream)

Context-system delivers assembled context packs that workflow uses during planning and implementation modes.

---

## Source of Truth

| Concept | Authoritative Location |
|:--------|:----------------------|
| Project structure health | Core (`mindspec doctor`) |
| Mode state and transitions | Workflow (spec/bead status) |
| Execution tracking (issues, dependencies) | Beads (external, accessed via workflow adapter) |
| Long-form specifications | `.mindspec/docs/specs/` |
| Domain architecture | `.mindspec/docs/domains/<domain>/` |
| ADR lifecycle | `.mindspec/docs/adr/` + `.mindspec/docs/domains/<domain>/adr/` |
| Context pack content and provenance | Context-system (generated artifacts) |

---

## Integration Notes

- **Beads** is an external dependency accessed only through the workflow domain's adapter. Other domains do not interact with Beads directly.
- **Git worktrees** are managed by the workflow domain. Context-system and core are worktree-agnostic.
- **Context Map maintenance rule**: any change that introduces a new context, changes ownership, or adds a new integration contract must update this file.
