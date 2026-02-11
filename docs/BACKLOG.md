# Mindspec Product Backlog

> **Principle**: Prioritize features that enable mindspec to assist in building mindspec itself (dogfooding).

## Priority Tiers

| Tier | Description |
| :--- | :---------- |
| **P0** | Immediately useful for the next development session |
| **P1** | Needed within the first few specs |
| **P2** | Important for scaled usage |
| **P3** | Nice-to-have / future enhancements |

---

## P0: Immediate Value (Use While Building Mindspec)

### ✅ Done: Mode System + Antigravity Integration
- [x] Spec Mode vs Implementation Mode documentation
- [x] `/spec-init`, `/spec-approve`, `/spec-status` workflows
- [x] Agent rules for mode enforcement
- [x] `AGENTS.md` repository instructions

### 🔲 001: CLI Skeleton + Doctor
**Why P0**: Establishes CLI foundation and provides immediate project health validation.

**Scope**:
- CLI entry point: `python -m mindspec`
- `mindspec doctor` command for project structure health checks
- Validate: docs/core/, GLOSSARY.md, docs/specs/ exist

**Immediate Use**: Validate mindspec's own project structure.

### 🔲 002: Glossary-Based Context Injection
**Why P0**: Enables deterministic doc retrieval based on keywords.

**Scope**:
- Parse `GLOSSARY.md` into keyword → target mapping
- Match keywords from input text
- Extract targeted documentation sections
- CLI: `mindspec glossary list|match|show`

**Immediate Use**: Agent can pull architectural context when working on specs.

### 🔲 003: Context Pack Generation
**Why P0**: Reproducible context bundles for agent sessions.

**Scope**:
- CLI command: `mindspec context pack <spec-id>`
- Include: spec, matched docs, policies, commit tuple
- Output: `context-pack.md` in spec directory

**Immediate Use**: Consistent context for every implementation session.

---

## P1: Core Workflow Support

### 🔲 004: Proof Runner (MVP)
**Why P1**: Foundation for "proof-of-done" invariant; enables trust in completion.

**Scope**:
- Parse `Validation Proofs` section from spec.md
- Execute listed commands and capture output
- Report pass/fail with artifacts
- CLI: `mindspec proof run <spec-id>`

**Use**: Verify specs before marking complete; dogfood immediately.

### 🔲 005: Task Graph Generation
**Why P1**: Convert spec requirements into structured task graph.

**Scope**:
- Parse spec.md requirements and scope
- Generate `tasks.json` with dependencies
- Support multi-file tasks

**Use**: Break down specs into actionable, trackable tasks.

### 🔲 006: Doc Sync Validation
**Why P1**: Verify that code changes have corresponding doc updates.

**Scope**:
- CLI command: `mindspec validate docs`
- Compare changed files against doc requirements
- Flag missing doc updates

**Use**: Enforce "done includes doc-sync" rule.

### 🔲 007: ACP Workflow
**Why P1**: Formalizes architecture divergence handling with templated proposals.

**Scope**:
- CLI: `mindspec acp create <title>`
- Generate ACP template in `docs/architecture/proposals/`
- Include: summary, motivation, options, impact, required doc updates, approval question

**Use**: Standardize divergence decisions; human gate for architecture changes.

---

## P2: Project Health + Memory

### 🔲 008: Spec Validation
**Why P2**: Enables `/spec-approve` to verify acceptance criteria quality.

**Scope**:
- CLI command: `mindspec validate spec <id>`
- Check: All sections filled, criteria count, measurability
- Output: Pass/fail with specific feedback

### 🔲 009: Memory Service (Basic)
**Why P2**: Persist decisions, gotchas, debugging outcomes.

**Scope**:
- SQLite-based local store
- CLI: `mindspec memory save`, `mindspec memory recall`
- Tag by spec-id, keywords

**Use**: Cross-session continuity.

### 🔲 010: Workspace Provider
**Why P2**: Multi-repo support, submodule resolution.

**Scope**:
- `workspace.yml` configuration
- Resolve repo roots and aliases
- **Default implementation**: Meta-repo + submodules provider
- Commit tuple generation for reproducibility

---

## P3: Advanced Features

### 🔲 011: Architecture Divergence Detection
- Compare implementation against documented architecture
- Auto-trigger ACP workflow when divergence detected

### 🔲 012: Parallel Task Dispatch
- Identify ready tasks (no dependencies)
- Generate per-task context packets

### 🔲 013: Observability / Telemetry
- Glossary hit/miss rates
- Token budgets and cache rates

---

## Implementation Order

Based on feedback interpretation:

```
P0: 001 → 002 → 003 (CLI foundation → glossary → context packs)
P1: 004 → 007 → 005 → 006 (proof runner → ACP → tasks → doc-sync)
P2: 008 → 009 → 010 (spec validation → memory → workspace)
```

**Rationale**:
- Proof Runner (004) comes early to match "proof-of-done" invariant
- ACP Workflow (007) supports the ACP-first divergence gate
- Workspace Provider (010) has meta-repo/submodules as target implementation
