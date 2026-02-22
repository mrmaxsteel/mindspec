# Canonical Documentation Layout

This document is the reference for MindSpec's documentation structure: what each artifact is, where it lives, how it gets created, and how it's consumed by the MindSpec workflow.

---

## Directory Structure

```
.mindspec/
  docs/
    adr/                   # Cross-cutting Architecture Decision Records
      ADR-NNNN.md
    core/                  # Project-wide architecture, conventions, modes
      ARCHITECTURE.md
      CONVENTIONS.md
      MODES.md
      USAGE.md
      DOCS-LAYOUT.md       # This file
    domains/               # Bounded domain documentation
      <domain>/
        overview.md
        architecture.md
        interfaces.md
        runbook.md
        adr/               # Domain-scoped ADRs
          ADR-NNNN.md
    specs/                 # Feature specifications
      NNN-slug/
        spec.md
        plan.md
        context-pack.md    # Generated
    context-map.md         # Bounded-context relationships
  state.json               # Convenience cursor (not source of truth)
AGENTS.md                  # Cross-agent instruction file
CLAUDE.md                  # Claude Code-specific instructions
```

---

## Artifact Reference

### Domain Docs (`domains/<domain>/`)

**What**: Documentation for a bounded domain — what it owns, how it's built, its public surface, and how to operate it.

**Files**:

| File | Content |
|:-----|:--------|
| `overview.md` | What this domain does, why it exists, key concepts, boundaries |
| `architecture.md` | Internal structure, patterns, key types, data flow, invariants |
| `interfaces.md` | Public API surface, exported functions, contracts with other domains |
| `runbook.md` | Build, test, debug, and operate this domain |
| `adr/ADR-NNNN.md` | Architecture decisions scoped to this domain |

**Created by**: `mindspec domain add <slug>` scaffolds the directory and all four files with placeholder content. Also registers the domain in context-map.md.

**Populated by**:
- `mindspec migrate` guides the agent through codebase analysis and domain doc population (Phase 4) for brownfield repos
- During Implementation Mode, the agent updates domain docs when code changes affect domain architecture or interfaces (doc-sync convention)

**Consumed by**:
- **Context Pack builder** (`mindspec context pack`): assembles domain docs into mode-tiered context packs. In Spec Mode, only `overview.md` is included. In Plan Mode, `overview.md` + `architecture.md`. In Implement Mode, all four files.
- **Doctor** (`mindspec doctor`): checks that each domain directory has the expected files
- **Instruct** (`mindspec instruct`): references domain boundaries in mode guidance

### Context Map (`context-map.md`)

**What**: Declares bounded contexts, their ownership, upstream/downstream relationships, and integration contracts.

**Created by**: `mindspec domain add` auto-updates it when adding a new domain. For greenfield projects, the initial `mindspec init` does not create it — it appears when the first domain is added.

**Populated by**:
- `mindspec migrate` guides the agent through relationship mapping (Phase 3) for brownfield repos
- Manual updates when domain relationships change

**Consumed by**:
- **Context Pack builder**: uses it for 1-hop neighbor expansion — includes neighbor `interfaces.md` and referenced contracts, but not full neighbor internals
- **Domain list** (`mindspec domain list`): reads it to show registered domains

### Architecture Decision Records (`adr/`)

**What**: Records of significant architectural decisions with context, rationale, and consequences.

**Locations**:
- Cross-cutting ADRs: `.mindspec/docs/adr/ADR-NNNN.md`
- Domain-scoped ADRs: `.mindspec/docs/domains/<domain>/adr/ADR-NNNN.md`

**Created by**: `mindspec adr create --title "..." --domains "..."` assigns the next ID and scaffolds the ADR from a template.

**Status lifecycle**: Proposed -> Accepted -> Superseded

**Supersession**: `mindspec adr create --supersedes ADR-NNNN` creates a new ADR that explicitly replaces the old one. The old ADR's `Superseded-by` field is automatically updated.

**Consumed by**:
- **Context Pack builder**: includes accepted (non-superseded) ADRs for impacted domains
- **Plan validation** (`mindspec validate plan`): checks that plans cite relevant ADRs and include an ADR fitness section
- **Plan Mode guidance**: agents are instructed to check ADR fitness during planning — if a plan needs to deviate from an existing ADR, the agent must stop and propose a new ADR before proceeding

### Specifications (`specs/NNN-slug/`)

**What**: Feature specifications that capture user value, acceptance criteria, and implementation plans.

**Files**:

| File | Content |
|:-----|:--------|
| `spec.md` | Problem statement, goal, impacted domains, acceptance criteria |
| `plan.md` | Implementation decomposition with work chunks (Draft -> Approved) |
| `context-pack.md` | Generated context bundle for agent sessions |

**Created by**: `mindspec spec-init <slug>` scaffolds the spec directory, creates `spec.md` from a template, and pours the spec-lifecycle molecule in Beads.

**Lifecycle**: Spec Mode (write spec) -> `/spec-approve` -> Plan Mode (write plan) -> `/plan-approve` -> Implementation Mode -> `/impl-approve` -> Idle

**Consumed by**:
- **Context Pack builder**: parses spec metadata (goal, impacted domains) as the starting point for context assembly
- **Validation** (`mindspec validate spec/plan`): checks structural quality
- **State system**: `state.json` tracks the active spec as a convenience cursor

### Core Docs (`core/`)

**What**: Project-wide architectural reference documents that are not scoped to a single domain.

**Created by**: Manual authoring. Not scaffolded by `mindspec init` — authors create files here as the project matures.

**Consumed by**: Human reference and agent context. Not directly parsed by any MindSpec command, but included in context packs when explicitly referenced.

### Instruction Files (`AGENTS.md`, `CLAUDE.md`)

**What**: Agent behavioral instructions. `AGENTS.md` is cross-agent (read by Codex, Copilot, Cursor, etc.). `CLAUDE.md` is Claude Code-specific (hooks, slash commands).

**Created by**: `mindspec init` creates both. If they already exist, init appends a MindSpec block using the `<!-- mindspec:managed -->` marker.

**Updated by**: `mindspec setup claude` manages Claude-specific configuration. Re-runnable after MindSpec upgrades.

---

## How Docs Get Populated

### Greenfield (new project)

1. `mindspec init` creates the `.mindspec/` directory structure, `state.json`, `AGENTS.md`, and `CLAUDE.md`
2. `mindspec setup claude` configures Claude Code hooks and slash commands
3. `mindspec domain add <slug>` for each identified domain — scaffolds domain docs and updates context-map
4. Author populates domain docs with real content
5. Specs, plans, and ADRs are created through the normal workflow

### Brownfield (existing project)

1. `mindspec init` creates missing structure, appends to existing instruction files
2. `mindspec migrate` emits a multi-phase prompt that guides the agent through:
   - **Phase 1**: Codebase analysis (scan source structure, identify boundaries)
   - **Phase 2**: Domain identification (propose domains, create via `mindspec domain add`)
   - **Phase 3**: Context map population (relationships, contracts)
   - **Phase 4**: Domain doc population (fill scaffolded files from codebase)
   - **Phase 5**: File classification (move stray docs to canonical locations)
3. `mindspec setup claude` configures agent integration

### Continuous maintenance

During the normal spec-driven workflow, documentation is maintained by convention:

- **Spec Mode**: creates `spec.md` in the spec directory
- **Plan Mode**: creates `plan.md`, may propose new ADRs if architectural decisions are needed
- **Implementation Mode**: updates domain docs when code changes affect architecture or interfaces (doc-sync). Context packs are regenerated. Bead closure requires documentation to be current.
- **Doctor** (`mindspec doctor`) continuously validates structure health

---

## See Also

- [CONVENTIONS.md](CONVENTIONS.md) — Naming rules and structural conventions
- [ARCHITECTURE.md](ARCHITECTURE.md) — System architecture and responsibility boundaries
- [MODES.md](MODES.md) — Mode definitions and transitions
- [ADR-0001](../adr/ADR-0001.md) — DDD enablement
- [ADR-0017](../adr/ADR-0017.md) — Agent onboarding architecture
- [ADR-0018](../adr/ADR-0018.md) — Lean bootstrap and glossary/policies removal
