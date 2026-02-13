# MindSpec Product Backlog

> **Principle**: Prioritize features that enable MindSpec to assist in building MindSpec itself (dogfooding).
> **Language**: Go (per ADR-0004, accepted). All CLI work targets the Go binary.

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
- [x] Agent rules (.claude/rules/mindspec-modes.md) aligned with three modes
- [x] GLOSSARY.md rebuilt with v1 primitives
- [x] CONVENTIONS.md updated with domain/worktree/Beads conventions
- [x] policies.yml expanded for Plan mode, ADR governance, domains, Beads, worktrees
- [x] ADR-0001: DDD Enablement + DDD-Informed Context Packs (proposed)
- [x] ADR-0002: Beads Integration Strategy (proposed → accepted)
- [x] ADR-0003: Centralized Agent Instruction Emission (proposed → accepted)
- [x] ADR-0004: Go as v1 CLI Implementation Language (accepted)
- [x] INIT.md archived (superseded by mindspec.md)

### 000: Repo + Beads Hygiene ✓
- [x] Beads initialized in repo (`.beads/` with durable state)
- [x] Selective `.beads/` gitignore (runtime ignored, durable tracked)
- [x] Packaging excludes (`MANIFEST.in`)
- [x] `mindspec doctor` Beads hygiene checks (Python prototype)

---

## P0: Immediate Value (Use While Building MindSpec)

### 001: Go CLI Skeleton + Doctor ✓
### 002: Glossary-Based Context Injection ✓
### 003: Context Pack Generation (with DDD Routing) ✓

---

## P1: Core Workflow Support

### 004: `mindspec instruct` — Mode-Aware Guidance Emission ✓
### 005: `mindspec next` — Work Selection + Claiming ✓
### 006: `mindspec validate` — Workflow Checks ✓
### 007: Beads Integration Conventions + Tooling ✓

### 008: Workflow Lifecycle — Worktrees + Molecules ✓
### 008b: Human Gates for Approval Workflow ✓
### 008c: Compose `bd prime` into `mindspec instruct` ✓

### 009: Workflow Happy-Path Gap Fixes ✓
### 010: `mindspec spec-init` CLI Command ✓

### 011: Domain Scaffold + Context Map
**Why P1**: DDD primitives need tooling support.

**Scope**:
- `mindspec domain add <name>`: scaffold `/docs/domains/<domain>/` with template files
- `mindspec domain list`: show registered domains
- Domain operations produce ADR drafts

**Partial**: Initial domain structure and `docs/context-map.md` created manually.

### 012: ADR Lifecycle Tooling
**Why P1**: ADR governance needs tooling support.

**Scope**:
- `mindspec adr create <title>`: generate ADR template with metadata
- `mindspec adr list`: show ADRs by status
- Superseding workflow: create new ADR linking to superseded one
- Validate ADR citations in plans

### 013: Proof Runner (MVP)
**Why P1**: Foundation for "proof-of-done" invariant.

**Scope**:
- Parse `Validation Proofs` section from spec.md
- Execute listed commands and capture output
- Report pass/fail with artifacts
- CLI: `mindspec proof run <spec-id>`

---

## P2: Project Health + Memory

### 014: Memory Service (Basic)
**Why P2**: Persist decisions, gotchas, debugging outcomes across sessions.

**Scope**:
- Local persistent store
- CLI: `mindspec memory save`, `mindspec memory recall`
- Tag by spec-id, domain, keywords
- Memory entries reference canonical beads or specs (per ADR-0002)

### 015: `mindspec init` — Project Bootstrap
**Why P2**: Scaffolds a new MindSpec project from scratch.

**Scope**:
- Create missing docs folders, templates, context-map placeholders
- Check for Beads presence and instruct how to init
- Generate starter GLOSSARY.md, AGENTS.md, CLAUDE.md

**Note**: Deferred from P0 — manual setup is fine while dogfooding on MindSpec itself.

---

## P3: Advanced Features

### 016: Architecture Divergence Detection
- Compare implementation against documented architecture
- Auto-trigger ADR divergence protocol when violations detected

### 017: Parallel Task Dispatch
- Identify ready beads (no unresolved dependencies)
- Generate per-bead context packets for parallel agent execution

### 018: Observability / Telemetry
- Glossary hit/miss rates
- Token budgets and cache rates
- OTel-friendly event shaping for future Agent Mind Visualization

### 019: Cross-Platform Release Automation
- CI/CD pipeline for Go binary builds
- Multi-arch binaries (darwin/linux, amd64/arm64)
- GitHub Releases or homebrew tap

---

## Implementation Order

```
P0: 000 ✓ → 001 ✓ (Go skeleton + doctor)
    → 002 ✓ (glossary) → 003 ✓ (context packs)

P1: 004 ✓ (instruct) → 005 ✓ (next) → 006 ✓ (validate) → 007 ✓ (Beads tooling)
    → 008 ✓ (worktree lifecycle) → 008b ✓ (human gates) → 008c ✓ (prime compose)
    → 009 ✓ (workflow gap fixes) → 010 ✓ (spec-init CLI)
    → 011 (domains) → 012 (ADRs) → 013 (proofs)

P2: 014 (memory) → 015 (init)
```

**Rationale**:
- 001–010 are done. 011+ resume with domain tooling.
