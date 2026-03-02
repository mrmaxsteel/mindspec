# MindSpec Core Architecture

MindSpec is a **spec-driven development and context management framework** (Claude Code-first) that uses **Beads** as its durable work graph and **git worktrees** to safely isolate and parallelize agent execution. It standardizes how teams move from user value to validated specification to bounded plans to implemented change, while keeping architecture and documentation coherent over time.

---

## System Shape

```
┌─────────────────────────────────────────────────────┐
│ MindSpec Operating Model                            │
│                                                     │
│  Modes: Explore (optional) → Spec → Plan → Implement │
│  Approvals: human gates, ADR compliance, doc-sync   │
│  Context: deterministic, budgeted Context Packs     │
│                                                     │
├──────────────┬──────────────┬───────────────────────┤
│ Beads        │ Git Worktrees│ Documentation System  │
│ (work graph) │ (execution)  │ (specs/ADRs/domains)  │
└──────────────┴──────────────┴───────────────────────┘
```

---

## Responsibility Boundaries {#responsibility-boundaries}

| Concern | System of Record |
|:--------|:-----------------|
| Execution tracking (epics, issues, dependencies) | Beads |
| Long-form specifications | `/.mindspec/docs/specs/` |
| Domain architecture and documentation | `/.mindspec/docs/domains/<domain>/` |
| ADR lifecycle | `/.mindspec/docs/adr/` and `/.mindspec/docs/domains/<domain>/adr/` |
| Deterministic context assembly | MindSpec Context Pack builder |
| Workflow orchestration (modes, approvals) | MindSpec mode system + spec-lifecycle formula |
| Roadmap hierarchy | Beads (release/milestone → spec beads → implementation beads) |
| Isolated execution | Git worktrees |

Canonical MindSpec docs live under `/.mindspec/docs/`. Legacy `docs/` paths are read-only compatibility fallbacks for pre-migration repositories.

Beads is a **passive, execution-oriented tracking substrate** (see [ADR-0002](../adr/ADR-0002.md)). It is not responsible for planning, spec authoring, context routing, or architectural governance.

---

## Core Primitives

### 1. Modes {#modes}

MindSpec operates in four explicit modes. Mode controls allowed outputs, required context, and gates.

- **Explore Mode** (optional): evaluate whether an idea is worth pursuing before committing to the spec workflow
- **Spec Mode**: capture user value, define acceptance criteria, declare impacted domains
- **Plan Mode**: decompose spec into bounded implementation beads, review ADRs and domain docs
- **Implementation Mode**: execute one bead in an isolated worktree with proof and doc-sync

Full definitions: [MODES.md](MODES.md)

### 2. Beads (Work Graph) {#beads}

Beads is the canonical system of record for execution tracking:

- **Roadmap hierarchy**: release/milestone → spec beads → implementation beads → sub-tasks
- **Dependency graph**: queryable ("what's ready?"), dependency-aware ("what's blocked?")
- **Durable across sessions**: git-native, hash-based IDs

Beads entries must remain **concise and execution-oriented**. Long-form content lives in the documentation system. Spec beads contain a summary and link to the canonical spec file (see [ADR-0002](../adr/ADR-0002.md)).

### 3. Git Worktrees {#worktrees}

All implementation work runs in isolated worktrees tied to beads:

- Worktree naming convention includes bead ID
- Changes isolated per bead
- Prevents unreviewed changes in the main working tree
- Enables safe parallel execution
- Closing a bead requires evidence + doc updates + clean state sync

### 4. ADR Lifecycle {#adr-lifecycle}

Architecture Decision Records are a governed system:

- **Status lifecycle**: proposed → accepted → superseded
- **Superseding**: new ADRs explicitly link to the ADR(s) they supersede
- **Plan citation**: plans must cite the ADRs they rely on
- **Divergence gating**: if a plan/implementation detects an ADR is unfit, the agent must stop, inform the user, and present options (continue-as-is vs. propose new ADR)

ADR metadata: domain(s), status, supersedes/superseded-by links, decision + rationale + consequences, validation notes.

### 5. Domains (DDD-Inspired) {#domains}

Domains are first-class primitives that govern where docs/specs live, how context packs are assembled, and which ADRs are relevant.

**Domain doc structure** (`/.mindspec/docs/domains/<domain>/`):

| File | Purpose |
|:-----|:--------|
| `overview.md` | What it owns, boundaries |
| `architecture.md` | Key patterns, invariants |
| `interfaces.md` | APIs/events/contracts |
| `runbook.md` | Ops/dev workflows |
| `adr/ADR-xxxx.md` | Domain-scoped ADRs |

**Domain operations** are human-in-the-loop: `add-domain`, `split-domain`, `merge-domain`. These produce ADRs because they change the mental model of the codebase.

See [ADR-0001](../adr/ADR-0001.md) for full DDD enablement decision.

### 6. Context Map {#context-map}

Location: `/.mindspec/docs/context-map.md`

The Context Map declares bounded contexts, ownership, upstream/downstream relationships, integration contracts, and source-of-truth notes. Any change that introduces a new context, changes ownership, or adds a new integration contract must update the Context Map.

Context Packs use the Context Map for **1-hop neighbor expansion**: include neighbor `interfaces.md` and referenced contracts, but not full neighbor internals.

---

## Context System {#context-system}

MindSpec's most important capability is **efficient, deterministic context delivery** per mode.

### Context Packs

A Context Pack is a concise bundle constructed from:

- Relevant bead(s): spec bead + implementation bead
- Domain docs (for impacted domains)
- Accepted ADRs (not superseded) for impacted domains
- Neighbor contracts (1-hop from Context Map)
- Explicitly referenced files/sections
- Explicitly referenced files/sections

### Properties

- **Mode-specific**: different content for Spec vs. Plan vs. Implement
- **Budgeted**: hard token budget
- **Deduped**: no repeated sections within a session
- **Provenance-preserving**: records exactly what was loaded and why

### DDD-Informed Assembly (ADR-0001)

1. Start from impacted domains declared in the spec bead
2. Include domain `overview.md`, `architecture.md`, `interfaces.md`, and accepted ADRs
3. Expand 1-hop via Context Map: neighbor `interfaces.md` + referenced contracts only
4. Record provenance back to the bead

---

## Core Invariants {#core-invariants}

1. **Docs-first**: every non-trivial change must update corresponding documentation
2. **Spec-anchored**: all code changes originate from a versioned specification
3. **Human gate for divergence**: ADR deviations require a new ADR and human approval
4. **Proof of done**: beads complete only when verification steps pass with evidence
5. **Done includes doc-sync**: bead closure requires documentation updates
6. **Scope discipline**: implementation beads cannot widen scope; discovered work becomes new beads

---

## Observability (Future) {#observability}

v1 emits structured events (minimal) for future "Agent Mind Visualization":

- Nodes: agents, tools, domains, beads, docs, ADRs
- Edges: tool calls, context pack injections, bead transitions, verification runs
- Metrics: tokens injected, cache hits, latency/errors, budget utilization

---

## See Also

- [WORKFLOW-STATE-MACHINE.md](WORKFLOW-STATE-MACHINE.md) — Detailed workflow state machine, transition matrix, and guard rules
- [mindspec-v1-spec.md](../archive/mindspec-v1-spec.md) — Original product specification (archived)
- [MODES.md](MODES.md) — Mode definitions and transitions
- [CONVENTIONS.md](CONVENTIONS.md) — File organization and naming
- [ADR-0001](../adr/ADR-0001.md) — DDD enablement + context packs
- [ADR-0014](../adr/ADR-0014.md) — Canonical docs/policies path supersession scope
- [ADR-0002](../adr/ADR-0002.md) — Beads integration strategy
- [DOCS-LAYOUT.md](DOCS-LAYOUT.md) — Canonical documentation layout reference
