# MindSpec Product Backlog

> **Principle**: Prioritize features that enable MindSpec to assist in building MindSpec itself (dogfooding).

## Priority Tiers

| Tier | Description |
|:-----|:-----------|
| **P0** | Immediately useful for the next development session |
| **P1** | Needed within the first few specs |
| **P2** | Important for scaled usage |
| **P3** | Nice-to-have / future enhancements |

---

## Done

### Documentation Alignment
- [x] Three-mode system (Spec/Plan/Implement) documented in MODES.md
- [x] ARCHITECTURE.md rewritten for Beads + worktrees + domains + Claude Code
- [x] AGENTS.md updated for three-mode system + Beads + ADR governance
- [x] Agent rules (.agent/rules/mindspec-modes.md) aligned with three modes
- [x] GLOSSARY.md rebuilt with v1 primitives
- [x] CONVENTIONS.md updated with domain/worktree/Beads conventions
- [x] policies.yml expanded for Plan mode, ADR governance, domains, Beads, worktrees
- [x] ADR-0001: DDD Enablement + DDD-Informed Context Packs (proposed)
- [x] ADR-0002: Beads Integration Strategy (proposed)
- [x] INIT.md archived (superseded by mindspec.md)

---

## P0: Immediate Value (Use While Building MindSpec)

### 001: CLI Skeleton + Doctor
**Why P0**: Establishes CLI foundation and provides immediate project health validation.

**Scope**:
- CLI entry point
- `mindspec doctor` command for project structure health checks
- Validate: `docs/core/`, `docs/domains/`, `GLOSSARY.md`, `docs/specs/`, `architecture/` exist

**Immediate Use**: Validate MindSpec's own project structure.

### 002: Glossary-Based Context Injection
**Why P0**: Enables deterministic doc retrieval based on keywords.

**Scope**:
- Parse `GLOSSARY.md` into keyword-to-target mapping
- Match keywords from input text
- Extract targeted documentation sections
- CLI: `mindspec glossary list|match|show`

**Immediate Use**: Agent can pull architectural context when working on specs.

### 003: Context Pack Generation (with DDD Routing)
**Why P0**: Reproducible context bundles for agent sessions.

**Scope**:
- CLI command: `mindspec context pack <spec-id>`
- Include: spec, matched domain docs, accepted ADRs, policies, commit tuple
- DDD-informed assembly: start from impacted domains, 1-hop neighbor expansion via Context Map (per ADR-0001)
- Output: `context-pack.md` in spec directory with provenance

**Immediate Use**: Consistent, domain-aware context for every session.

---

## P1: Core Workflow Support

### 004: Beads Integration Conventions + Tooling
**Why P1**: Beads is central to the execution model; conventions must be codified.

**Scope**:
- Spec bead creation from approved spec (concise summary + link)
- Implementation bead creation from plan
- Active workset hygiene commands
- Bead-to-worktree mapping

### 005: Worktree Lifecycle Management
**Why P1**: Implementation Mode requires worktree isolation.

**Scope**:
- Create worktree for a bead: `mindspec worktree create <bead-id>`
- Naming convention: `worktree-<bead-id>`
- Clean state sync on bead closure
- List active worktrees: `mindspec worktree list`

### 006: Domain Scaffold + Context Map
**Why P1**: DDD primitives need tooling support.

**Scope**:
- `mindspec domain add <name>`: scaffold `/docs/domains/<domain>/` with template files
- `mindspec domain list`: show registered domains
- Context Map template at `/docs/context-map.md`
- Domain operations produce ADR drafts

**Partial**: Initial domain structure (`docs/domains/{core,context-system,workflow}/`) and `docs/context-map.md` created manually. CLI tooling (`mindspec domain add/list`) still needed.

### 007: ADR Lifecycle Tooling
**Why P1**: ADR governance needs tooling support.

**Scope**:
- `mindspec adr create <title>`: generate ADR template with metadata
- `mindspec adr list`: show ADRs by status
- Superseding workflow: create new ADR linking to superseded one
- Validate ADR citations in plans

### 008: Proof Runner (MVP)
**Why P1**: Foundation for "proof-of-done" invariant.

**Scope**:
- Parse `Validation Proofs` section from spec.md
- Execute listed commands and capture output
- Report pass/fail with artifacts
- CLI: `mindspec proof run <spec-id>`

### 009: Doc Sync Validation
**Why P1**: Enforce "done includes doc-sync" rule.

**Scope**:
- CLI: `mindspec validate docs`
- Compare changed files against doc requirements
- Flag missing doc updates

---

## P2: Project Health + Memory

### 010: Spec Validation
**Why P2**: Enables `/spec-approve` to verify acceptance criteria quality.

**Scope**:
- CLI: `mindspec validate spec <id>`
- Check: all sections filled, criteria count, measurability, impacted domains declared

### 011: Plan Validation
**Why P2**: Ensures plan quality before Implementation Mode.

**Scope**:
- Verify implementation beads have verification steps
- Verify ADR citations
- Verify dependency graph is acyclic
- Verify scope coverage against spec requirements

### 012: Memory Service (Basic)
**Why P2**: Persist decisions, gotchas, debugging outcomes across sessions.

**Scope**:
- Local persistent store
- CLI: `mindspec memory save`, `mindspec memory recall`
- Tag by spec-id, domain, keywords
- Memory entries reference canonical beads or specs (per ADR-0002)

---

## P3: Advanced Features

### 013: Architecture Divergence Detection
- Compare implementation against documented architecture
- Auto-trigger ADR divergence protocol when violations detected

### 014: Parallel Task Dispatch
- Identify ready beads (no unresolved dependencies)
- Generate per-bead context packets for parallel agent execution

### 015: Observability / Telemetry
- Glossary hit/miss rates
- Token budgets and cache rates
- OTel-friendly event shaping for future Agent Mind Visualization

---

## Implementation Order

```
P0: 001 → 002 → 003 (CLI foundation → glossary → context packs)
P1: 004 → 005 → 006 → 007 → 008 → 009 (Beads → worktrees → domains → ADRs → proofs → doc-sync)
P2: 010 → 011 → 012 (spec validation → plan validation → memory)
```

**Rationale**:
- CLI + glossary + context packs are immediately useful for dogfooding
- Beads and worktree conventions codify the execution model from ADR-0002
- Domain scaffold enables the DDD model from ADR-0001
- ADR tooling supports the governance model
- Proof runner and doc-sync enforce core invariants
