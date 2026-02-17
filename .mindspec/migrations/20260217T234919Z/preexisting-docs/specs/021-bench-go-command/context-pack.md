# Context Pack

- **Spec**: 021-bench-go-command
- **Mode**: plan
- **Commit**: eb84949643651da963115a370db2e854f1236936
- **Generated**: 2026-02-14T14:56:21Z

---

## Goal

Replace `scripts/bench-e2e.sh` with a native `mindspec bench run` Go command that runs 3-session A/B/C benchmarks, produces N-way side-by-side quantitative reports (instead of 3 pairwise comparisons), and stores all benchmark artifacts in `docs/specs/<id>/benchmark/`.

## Impacted Domains

- workflow
- core

## 1-Hop Neighbors

- context-system

## Domain: workflow — Overview

# Workflow Domain — Overview

## What This Domain Owns

The **workflow** domain owns the spec-driven development lifecycle:

- **Mode system** — Spec/Plan/Implement mode enforcement and transitions
- **Spec lifecycle** — spec creation, approval gates, status tracking
- **Plan lifecycle** — plan decomposition, bead creation, plan approval gates
- **Beads integration** — adapter layer between MindSpec and the Beads work graph
- **Worktree management** — creating, naming, and cleaning up bead-specific worktrees
- **Validation gates** — human-in-the-loop approval, ADR compliance checks, doc-sync enforcement

## Boundaries

Workflow does **not** own:
- CLI infrastructure or project health checks (core)
- Glossary parsing, context pack assembly, or provenance tracking (context-system)

Workflow **uses** context packs (from context-system) to provide mode-appropriate context during planning and implementation.

## Beads Integration Note

Beads is a **passive, execution-oriented tracking substrate** (ADR-0002). The Beads adapter and worktree operations are scoped to this domain. If Beads integration grows complex enough to warrant its own domain, that split should be proposed via ADR with human approval.

## Key Files

| File | Purpose |
|:-----|:--------|
| `docs/core/MODES.md` | Mode definitions and transitions |
| `.claude/rules/mindspec-modes.md` | Agent-facing mode enforcement rules |
| `.claude/commands/spec-init.md` | Spec initialization workflow |

## Current State

Mode system is documented. Beads integration conventions are defined (ADR-0002). Implementation tooling (Specs 004-009) is planned.

## Domain: workflow — Architecture

# Workflow Domain — Architecture

## Key Patterns

### Three-Mode Lifecycle

```
Intent -> [Spec Mode] -> approval -> [Plan Mode] -> approval -> [Implementation Mode] -> validation -> Done
```

Each mode gates:
- **Allowed outputs** — what artifacts can be created/modified
- **Required context** — what must be reviewed before proceeding
- **Transition gates** — what conditions must hold to advance

### Beads as Tracking Substrate (ADR-0002)

MindSpec layers its structured operating model on top of Beads:

| Concern | Owner |
|:--------|:------|
| Execution tracking (issues, dependencies) | Beads |
| Workflow orchestration (modes, gates) | MindSpec (this domain) |
| Long-form specs, ADRs, domain docs | Documentation system |

Beads entries must remain **concise and execution-oriented**. Spec beads contain a summary + link to the canonical spec file.

### Worktree Isolation

All implementation work runs in isolated git worktrees:
- Named with bead ID: `worktree-<bead-id>`
- One worktree per implementation bead
- Closing a bead requires evidence + doc updates + clean state sync

### ADR Governance

- Plans must cite ADRs they rely on
- Divergence detected at any mode triggers the ADR divergence protocol
- New superseding ADRs require human approval before work resumes

## Invariants

1. No code changes without an approved spec AND approved plan.
2. Implementation scope cannot widen — discovered work becomes new beads.
3. ADR divergence always triggers a human gate.
4. Bead closure requires proof-of-done + doc-sync.
5. Active Beads workset must be kept intentionally small.

## Domain: core — Overview

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

## Domain: core — Architecture

# Core Domain — Architecture

## Key Patterns

### Workspace Resolution

The `Workspace` class finds the project root by walking up from the current directory looking for `mindspec.md` or `.git`. All path resolution is relative to this root.

### Health Checks

`mindspec doctor` validates project structure. Checks are categorized:

- **Errors**: Missing critical files (e.g., `GLOSSARY.md`, `docs/core/`)
- **Warnings**: Missing optional structure (e.g., `docs/domains/`, `docs/context-map.md`)

The distinction allows fresh projects to pass basic checks while still surfacing incomplete scaffolding.

### Policy Framework

Policies in `architecture/policies.yml` are declarative rules with:
- `id`, `description`, `severity` (error/warning)
- Optional `scope` (file glob) and `mode` (spec/plan/implementation)
- `reference` pointing to the authoritative doc section

## Invariants

1. Workspace resolution must be deterministic — same directory always resolves to same root.
2. Health checks must never hard-fail on optional structure in a fresh project.
3. Policy evaluation is read-only — policies describe constraints, they don't enforce them at runtime (yet).

## Neighbor Domain: context-system — Interfaces

# Context-System Domain — Interfaces

## Provided Interfaces

### Glossary Parsing

```go
// internal/glossary/glossary.go
glossary.Parse(root string) ([]glossary.Entry, error)
// Returns all glossary entries with Term, Label, Target, FilePath, Anchor
```

### Glossary Matching

```go
// internal/glossary/match.go
glossary.Match(entries []glossary.Entry, text string) []glossary.Entry
// Returns matched terms, longest-match-first, case-insensitive
```

### Section Extraction

```go
// internal/glossary/section.go
glossary.ExtractSection(root, filePath, anchor string) (string, error)
// Extracts a specific section from a markdown file by anchor
```

### Context Pack Generation (Spec 003)

```go
// internal/contextpack/builder.go
contextpack.Build(root, specID, mode string) (*ContextPack, error)
// Assembles a context pack for the given spec and mode

// internal/contextpack/spec.go
contextpack.ParseSpec(specDir string) (*SpecMeta, error)
// Parses spec.md to extract goal and impacted domains

// internal/contextpack/domaindoc.go
contextpack.ReadDomainDocs(root, domain string) (*DomainDoc, error)
// Reads 4 standard domain doc files (overview, architecture, interfaces, runbook)

// internal/contextpack/contextmap.go
contextpack.ParseContextMap(path string) ([]Relationship, error)
contextpack.ResolveNeighbors(rels []Relationship, impactedDomains []string) []string
// Parses Context Map relationships and resolves 1-hop neighbors

// internal/contextpack/adr.go
contextpack.ScanADRs(root string) ([]ADR, error)
contextpack.FilterADRs(adrs []ADR, domains []string) []ADR
// Scans and filters ADRs by status and domain

// internal/contextpack/policy.go
contextpack.ParsePolicies(path string) ([]Policy, error)
contextpack.FilterPolicies(policies []Policy, mode string) []Policy
// Parses policies.yml and filters by mode
```

## Consumed Interfaces

- **core**: `workspace.FindRoot()`, `workspace.GlossaryPath()`, `workspace.DocsDir()`, `workspace.SpecDir()`, `workspace.ContextMapPath()`, `workspace.ADRDir()`, `workspace.PoliciesPath()`, `workspace.DomainDir()`
- **workflow**: Spec bead metadata (impacted domains, ADR citations) for context pack routing

## Events

None defined yet. Future: context pack generation events for observability (tokens injected, cache hits).

## ADR: ADR-0001

# ADR-0001: Project DDD Enablement + DDD-Informed Context Packs

- **Date**: 2026-02-11
- **Status**: Accepted
- **Domain(s)**: core, context-system, workflow
- **Deciders**: MindSpec maintainers
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Context

MindSpec’s v1 goal is to make spec-driven development reproducible while delivering **concise, deterministic, portable context** to coding agents, and to keep architecture/documentation coherent over time. :contentReference[oaicite:0]{index=0}

MindSpec already treats:
- ADRs as governed, divergence-gated primitives (with superseding) :contentReference[oaicite:1]{index=1}
- Domains as first-class documentation primitives with a standard structure :contentReference[oaicite:2]{index=2}
- Context Packs as mode-specific, budgeted, deduped, provenance-preserving bundles assembled from beads + domain docs + accepted ADRs :contentReference[oaicite:3]{index=3}
- “Docs-first” / “done includes doc-sync” as a core invariant 

However, many projects struggle to consistently apply DDD (clear boundaries, contracts, ownership) and agents frequently fail when they lack an explicit “map” of bounded contexts and integrations. This causes:
- accidental cross-boundary changes
- incomplete awareness of downstream consumers
- brittle context injection (“go read X”) instead of systematic, deterministic selection :contentReference[oaicite:5]{index=5}

## Decision

MindSpec will explicitly help the **project-under-development** apply DDD by:
1) creating/maintaining DDD artifacts (Context Map, domain docs, ADRs) as first-class repo primitives, and  
2) using those DDD artifacts as **inputs to Context Pack assembly** (routing + scoping + contract selection).

This makes DDD actionable (not “docs theatre”) and improves agent correctness by turning boundaries and contracts into deterministic context inputs. 

## Decision Details

### A) Project DDD Artifacts (required primitives)

For any project using MindSpec, the following artifacts are canonical inputs for work planning and context assembly:

1. **Project Context Map**
   - File: `/docs/context-map.md`
   - Purpose: declare bounded contexts/domains, ownership, and integration relationships (upstream/downstream).
   - Must include pointers to “published language” / contracts (e.g., which `interfaces.md` defines the contract).  
   - Changes to boundaries/relationships require a governed update (see Governance). :contentReference[oaicite:7]{index=7}

2. **Domain Docs**
   - Folder: `/docs/domains/<domain>/`
   - Files: `overview.md`, `architecture.md`, `interfaces.md`, `runbook.md`, `adr/ADR-xxxx.md` :contentReference[oaicite:8]{index=8}
   - Domain docs define the owning context’s responsibilities, invariants, and contracts.

3. **ADRs**
   - Governed lifecycle: proposed → accepted → superseded
   - Plans must cite ADRs; divergence requires stopping and creating a superseding ADR with explicit human approval. :contentReference[oaicite:9]{index=9}

4. **Glossary / Registry**
   - Used for deterministic mapping from concepts to doc sections for context injection. 

### B) How MindSpec helps a project apply DDD

MindSpec will enforce that DDD artifacts stay accurate via its mode system and gates:

1. **Spec Mode**
   - Specs must declare “impacted domains” and relevant architecture touchpoints. :contentReference[oaicite:11]{index=11}
   - If a spec introduces/changes cross-domain integration or ownership, it must update `/docs/context-map.md` and/or propose an ADR.

2. **Plan Mode**
   - Plans and implementation beads are authored per bounded context (where possible), with explicit “integration/contract” beads when cross-context work is required.
   - Plans must cite applicable ADRs and domain docs. 

3. **Implementation Mode**
   - “Done” requires proof + doc updates + bead updates, including documentation refactors where needed. 
   - If implementation reveals a missing/incorrect boundary or contract, the agent stops, returns to Spec Mode, and updates the Context Map / ADRs accordingly. 

4. **Doc Sync Validation**
   - MindSpec will validate that code changes have corresponding doc updates (planned as a core workflow validator). :contentReference[oaicite:15]{index=15}

### C) How Context Packs use DDD artifacts (deterministic assembly rules)

Context Packs will use the project’s DDD artifacts as a routing layer.

**Baseline inputs** (already required):
- relevant bead(s), domain docs, accepted + not-superseded ADRs, explicitly referenced files/sections :contentReference[oaicite:16]{index=16}

**DDD-informed selection rules** (new):
1. Start from `impacted domains` declared in the spec bead. :contentReference[oaicite:17]{index=17}
2. For each impacted domain, include:
   - `overview.md` (ownership/boundaries)
   - `architecture.md` (invariants)
   - `interfaces.md` (contracts)
   - accepted ADRs under that domain :contentReference[oaicite:18]{index=18}
3. Use `/docs/context-map.md` to expand context **one hop** to neighboring contexts:
   - include **only** neighbor `interfaces.md` + any explicitly referenced contract notes
   - do **not** pull full neighbor internals unless the bead scope explicitly requires it
4. Record provenance back to the bead: “these sections were loaded because of relationship X in context-map + contract Y.” :contentReference[oaicite:19]{index=19}
5. Respect Context Pack properties: mode-specific, budgeted, deduped, provenance-preserving. :contentReference[oaicite:20]{index=20}

### D) Governance & Continuous Updating

- DDD artifacts are not “set-and-forget”; they are part of the project’s executable system.
- Updates are required when:
  - a new integration is introduced
  - ownership changes (domain split/merge/add)
  - contracts/events/APIs change
  - an ADR is superseded
- Domain operations (add/split/merge) remain human-in-the-loop and must produce ADR-style records because they change the codebase mental model. :contentReference[oaicite:21]{index=21}
- “Docs-first” and “done includes doc-sync” remain non-negotiable. 

## Consequences

### Positive
- Agents operate with explicit boundaries and contracts, reducing accidental cross-context coupling.
- Context packs become more correct with less noise (contracts for neighbors, internals for owners).
- Architecture drift is surfaced earlier via Context Map + ADR governance.
- Documentation stays coherent because it is required for bead closure and validated. 

### Negative / Tradeoffs
- Requires discipline: teams must keep Context Map and domain docs updated.
- Adds overhead to changes that impact boundaries (but this is deliberate and gated).
- Early projects may not have a clean domain split; MindSpec must support iterative refinement via human-in-the-loop domain operations. :contentReference[oaicite:24]{index=24}

## Alternatives Considered

1. **Rely on ad-hoc reading / tribal knowledge**
   - Rejected: non-deterministic, doesn’t scale, fails with agents.

2. **Use embeddings/vector RAG for “automatic” context**
   - Rejected for v1: MindSpec explicitly prioritizes deterministic context packs over vector infrastructure. :contentReference[oaicite:25]{index=25}

3. **Only domains + ADRs, no project context map**
   - Rejected: lacks explicit integration topology; causes over-injection or missed downstream contracts.

## Validation / Rollout

1. Add `/docs/context-map.md` template for projects adopting MindSpec.
2. Update spec template to require “impacted domains” and “integration/contract impact” fields. :contentReference[oaicite:26]{index=26}
3. Implement doc-sync validation checks (P1) to enforce updates alongside code changes. :contentReference[oaicite:27]{index=27}
4. Add Context Pack builder logic to:
   - read context-map for 1-hop neighbor contract inclusion
   - write provenance back to beads. :contentReference[oaicite:28]{index=28}

## ADR: ADR-0002

# ADR-0002: MindSpec Integration Strategy with Beads (Tracking Substrate, Not Planning System)

* **Date**: 2026-02-11
* **Status**: Accepted
* **Domain(s)**: workflow, tracking, context-system
* **Deciders**: MindSpec maintainers
* **Supersedes**: n/a
* **Superseded-by**: n/a

---

## Context

MindSpec’s goal is to provide a deterministic, spec-driven development operating model for AI-assisted engineering. It introduces:

* Mode-based workflow (Spec → Plan → Implement)
* ADR governance with divergence gating
* Domain-based documentation primitives
* Deterministic Context Packs
* “Docs-first” and “done includes doc-sync” invariants

Beads is a git-native issue/dependency graph tool designed to:

* Track execution work (issues, epics, dependencies)
* Support multi-branch / multi-agent workflows
* Remain intentionally minimal (tracking-only substrate)
* Avoid becoming a planning tool, UI layer, or orchestrator

A decision is required on how MindSpec should use Beads: whether Beads is the canonical planning system, the durable memory store, or a narrower execution substrate.

---

## Decision

MindSpec will treat Beads as a **passive, execution-oriented tracking substrate**, not as a planning system or long-form specification store.

Specifically:

1. **Beads will store execution work state (epics, issues, dependencies).**
2. **Long-form specifications, ADRs, and domain docs remain canonical in the documentation system.**
3. **Beads entries must remain concise and execution-oriented.**
4. **MindSpec orchestration (modes, gates, context assembly) remains outside Beads.**
5. **The active Beads workset must be kept intentionally small and regularly cleaned up.**

This preserves alignment with Beads’ architectural intent while allowing MindSpec to layer a structured operating model on top.

---

## Decision Details

### A) Clear Responsibility Boundaries

| Concern                                      | System of Record                |
| -------------------------------------------- | ------------------------------- |
| Execution tracking                           | Beads                           |
| Long-form specs                              | `/docs/specs/`                  |
| Domain architecture                          | `/docs/domains/`                |
| ADR lifecycle                                | `/docs/.../adr/`                |
| Deterministic context assembly               | MindSpec Context Pack builder   |
| Workflow orchestration                       | MindSpec modes                  |
| Optional recall memory (gotchas/debug notes) | Separate memory store (if used) |

Beads is not responsible for:

* Planning workflows
* Spec authoring
* Context routing
* Architectural governance

It is responsible for:

* Tracking tasks and dependencies
* Representing work graph state
* Enabling multi-branch/multi-agent concurrency

---

### B) Spec Beads Pattern (Index, Not Canon)

If MindSpec creates “spec beads,” they must:

* Contain a concise summary
* Link to the canonical spec file
* Avoid embedding long-form documentation
* Reference acceptance criteria and impacted domains

Beads entries must not become the primary home for full specifications.

---

### C) Active Workset Discipline

Beads is optimized for a bounded active issue set.

MindSpec will adopt the following hygiene rules:

1. Keep only active and near-term issues open.
2. Regularly run cleanup / compaction.
3. Rely on git history + documentation for historical traceability.
4. Avoid using Beads as an archival knowledge base.

This ensures:

* Agent scanning remains performant.
* `issues.jsonl` does not grow unbounded.
* Context Packs remain lean and deterministic.

---

### D) Parallelism Compatibility

MindSpec’s worktree-first execution model aligns with Beads’ design for:

* Hash-based IDs
* Git-native sync/merge
* Multi-agent collaboration

Beads remains the execution coordination layer; MindSpec governs how work is staged and validated.

---

### E) Multiple Memory Systems Boundary

If MindSpec introduces a separate memory store (e.g., SQLite for debug notes or decision recall):

* That store must never duplicate task state.
* Every memory entry must reference a canonical bead or spec.
* Beads remains the only source of truth for execution graph state.

This prevents split-brain “truth” systems.

---

### F) Orchestration Stays Outside Beads

MindSpec will not:

* Encode workflow modes into Beads itself.
* Depend on Beads for orchestration logic.
* Extend Beads into a planning engine.

Modes, gating, doc validation, and context assembly are implemented in MindSpec’s command layer.

Beads remains passive infrastructure.

---

## Consequences

### Positive

* Strong architectural alignment with Beads’ intended design.
* Clear separation of planning vs tracking.
* Deterministic context assembly remains controlled by MindSpec.
* Reduced risk of token explosion from large Beads datasets.
* Compatible with multi-agent workflows.
* Clean layering: Beads as substrate, MindSpec as operating system.

### Negative / Tradeoffs

* Requires discipline to keep Beads concise.
* Requires documentation hygiene outside Beads.
* Teams may initially prefer embedding specs directly in issues; this must be discouraged.
* Cleanup must be a cultural norm, not optional.

---

## Alternatives Considered

### 1. Make Beads the canonical planning + spec system

Rejected.
Would violate Beads’ minimal design intent and lead to bloated issue state.

### 2. Use Beads as full historical knowledge base

Rejected.
Unbounded growth harms agent performance and increases cognitive load.

### 3. Avoid Beads entirely and build a custom tracking layer

Rejected.
Reinventing a git-native dependency graph adds unnecessary complexity.

### 4. Encode workflow modes directly inside Beads

Rejected.
Blurs separation of concerns and makes orchestration dependent on tracking substrate.

---

## Validation / Rollout

1. Define Beads usage conventions in MindSpec documentation.
2. Update spec and plan templates to:

   * Require linking to canonical spec files.
   * Enforce concise Beads entries.
3. Add workflow checks to:

   * Prevent long-form spec duplication inside Beads.
   * Encourage cleanup after completion.
4. Establish Beads hygiene guidelines as part of “done.”

---

## Summary

MindSpec will use Beads as a **minimal, git-native execution graph** and not as a planning engine or documentation store.

MindSpec provides the structured operating model (modes, ADR governance, context assembly).
Beads provides the durable work graph.

This layered architecture preserves clarity, performance, and alignment with Beads’ design philosophy.

## ADR: ADR-0003

# ADR-0003: Centralized Agent Instruction Emission via MindSpec CLI

- **Date**: 2026-02-11
- **Status**: Accepted
- **Domain(s)**: workflow, agent-interface, context-system
- **Deciders**: MindSpec maintainers
- **Supersedes**: n/a
- **Superseded-by**: n/a

---

## Context

MindSpec's v1 workflow depends on strong, consistent operational guidance for agents across modes (Spec → Plan → Implement), including:

- deterministic Context Pack rules (budgeting, dedupe, provenance)
- ADR governance and divergence gating
- docs-first / "done includes doc-sync"
- worktree-first execution and parallelism hygiene
- Beads integration as the durable execution work graph (tracking substrate)

In practice, agents are bootstrapped via tool-specific instruction files (e.g., repo-level markdowns), which creates several problems:

- **Drift**: multiple instruction sources diverge over time.
- **Tool coupling**: different agent runtimes prefer different file names and conventions.
- **Mode ambiguity**: static instructions struggle to reflect current mode and current work.
- **Operational friction**: updating instructions requires editing prose across multiple docs rather than changing a single governed mechanism.

A decision is required on whether operational guidance should remain embedded in static instruction markdown files, or be emitted dynamically from MindSpec based on the active work and mode.

## Decision

MindSpec will provide a CLI-driven, centralized mechanism to emit agent-operating guidance, with a minimal "bootstrap" instruction in repo-facing instruction files.

Specifically:

1. MindSpec will ship a `mindspec instruct` command that prints the authoritative, mode-appropriate operating guidance for the current working context.
2. Repo-facing instruction files will be reduced to a minimal entrypoint that directs agents to the MindSpec CLI guidance, plus a small offline fallback.
3. MindSpec will determine the active mode and work context using local state (including Beads work state and/or worktree conventions) and will tailor guidance accordingly.
4. Beads remains a passive tracking substrate; MindSpec may call Beads to discover/claim work, but Beads will not invoke MindSpec.

## Decision Details

### A) Responsibilities and Boundaries

- **MindSpec owns**: mode semantics, orchestration rules, gating, instruction emission, context-pack assembly directives, and worktree conventions.
- **Beads owns**: execution work tracking (ready work, dependencies, ownership/claiming), as a minimal durable work graph.
- **Docs own**: canonical long-form artifacts (specs, ADRs, domain docs, context maps), with Beads entries remaining concise and execution-oriented.

### B) CLI Contract (v1)

MindSpec will expose the following commands (names indicative; final CLI may evolve):

- `mindspec instruct`
  - emits the authoritative operating guidance for the current state
  - includes: current mode, active work item, required outputs, and hard gates
  - **read-only by default** (no side effects)

- `mindspec next`
  - selects/claims the next ready work item (via Beads)
  - ensures correct worktree association
  - then emits guidance (or instructs the agent to run `mindspec instruct`)

- `mindspec validate`
  - runs workflow checks (e.g., doc-sync expectations, ADR divergence gates, context-pack invariants)

### C) Instruction Sources

- Instruction templates/rules will be **stored in-repo** (versioned), and the CLI will assemble them into the final emitted guidance.
- Emitted guidance may be available in:
  - human-readable text/markdown (default)
  - optional machine-readable form (e.g., JSON) for future integrations

### D) Bootstrap and Fallback

- Repo-facing bootstrap content will:
  - direct agents to run the CLI for authoritative guidance
  - include a minimal fallback path if the binary is unavailable (e.g., a short bootstrap doc that explains how to install/run MindSpec locally)

### E) Control Flow

- MindSpec may invoke Beads commands/APIs to:
  - list ready work
  - claim/lock an item
  - read work metadata needed for mode selection
- Beads will not be extended to call MindSpec as a hook/plugin in v1.

## Consequences

### Positive

- **Single source of truth** for operational guidance across tools and agents.
- **Mode-accurate guidance** (Spec/Plan/Implement) without duplicating static instruction sets.
- **Reduced drift** between workflow rules and what agents actually follow.
- **Clear control plane**: MindSpec orchestrates; Beads tracks.
- **Better parallelism hygiene** by consistently emitting worktree and claiming conventions.

### Negative / Tradeoffs

- **Binary dependency**: requires local installation/availability for the best experience.
- **CLI stability requirement**: the command surface becomes part of the contract and must be versioned carefully.
- **Bootstrap risk**: if the CLI is missing or broken, the fallback path must remain sufficient to recover.

## Alternatives Considered

### 1) Keep large, tool-specific instruction markdown files as canonical

- Pros: no binary dependency; simple to start.
- Cons: instruction drift, mode mismatch, and tool coupling persist; governance becomes document-edit heavy.

### 2) Encode mode and operating rules directly into Beads workflow

- Pros: single work system; "ready" flow is native.
- Cons: turns Beads into an orchestrator/control plane; increases coupling and conflicts with "tracking substrate" intent.

### 3) Have Beads invoke MindSpec via hooks/plugins

- Pros: automatic handoff when work is claimed.
- Cons: tight coupling, versioning complexity, and harder local determinism; requires extending Beads and maintaining integration surfaces.

### 4) No CLI; store instructions purely as templates and select manually

- Pros: fewer moving parts.
- Cons: reintroduces ambiguity and friction; loses dynamic mode/work awareness.

## Validation / Rollout

1. Add minimal bootstrap guidance to the repo's agent entrypoint docs:
   - direct agents to the CLI guidance
   - include offline fallback
2. Implement `mindspec instruct` with:
   - deterministic mode detection (local-only)
   - no side effects by default
   - clear "hard gates" (when to stop and ask for a decision)
3. Implement `mindspec next` integrating with Beads ready-work discovery/claiming and worktree association.
4. Add `mindspec validate` checks for core workflow invariants (ADR divergence gate, doc-sync expectations, context-pack invariants).
5. Dogfood on a small repo set with limited parallelism (local-first), then expand.

## Summary

MindSpec will become the authoritative emitter of agent-operating guidance via a CLI command, keeping repo instruction files minimal and avoiding duplicated, drifting rule sets. Beads remains the durable work-tracking substrate; MindSpec orchestrates modes, gating, and instruction emission.

## ADR: ADR-0004

# ADR-0004: Go as MindSpec v1 CLI Implementation Language

- **Date**: 2026-02-11
- **Status**: Accepted
- **Domain(s)**: core
- **Deciders**: MindSpec maintainers
- **Supersedes**: n/a
- **Superseded-by**: n/a

---

## Context

ADR-0003 established that MindSpec will provide a CLI binary for centralized instruction emission, work orchestration, and workflow validation. A language choice is required for the v1 implementation.

The current prototype is in Python (`src/mindspec/`), using Click for CLI, with workspace detection, glossary parsing, and doctor health checks. The codebase is small (~200 lines across 4 files).

Key constraints:
- MindSpec is a local-first CLI tool that agents and developers invoke frequently
- It must work reliably across developer machines and CI without environment assumptions
- Beads (the execution tracking substrate) is implemented in Go
- Distribution simplicity matters as adoption grows beyond dogfooding

## Decision

MindSpec's v1 CLI will be implemented in **Go (Golang)**.

### Rationale

1. **Single binary distribution**: `go build` produces a self-contained binary with no runtime dependencies. No Python version management, virtualenvs, or pip installs required.
2. **Cross-platform**: Go cross-compiles trivially (`GOOS=linux GOARCH=arm64 go build`).
3. **Ecosystem alignment**: Beads is Go. Sharing the language enables potential future library-level integration (rather than shelling out to `bd`), though v1 will use CLI invocation.
4. **CLI ergonomics**: Go has mature CLI libraries (cobra, viper) and fast startup time — important for a tool invoked on every agent session.
5. **Low rewrite cost**: The Python prototype is ~200 lines. Porting now is cheap; porting later after building glossary, context packs, and validation would be significantly more expensive.

### What this does NOT decide

- Future UI layers, plugins, or integrations may use other languages.
- This is a v1 distribution and ergonomics choice, not a permanent language commitment.

## Consequences

### Positive

- Simple, portable binary distribution from day one.
- No runtime environment assumptions (Python version, virtualenv, dependencies).
- Fast startup time for a frequently-invoked CLI tool.
- Ecosystem alignment with Beads.
- Rewrite cost is minimal given the small prototype.

### Negative / Tradeoffs

- **Go toolchain required** for contributors building from source (mitigated by prebuilt releases).
- **Python prototype discarded**: existing `src/mindspec/` code is retired. The Beads hygiene checks (Spec 000) and doctor logic must be re-implemented in Go.
- **Iteration speed**: Go is more verbose than Python for prototyping. Acceptable given the CLI-focused workload.

## Alternatives Considered

### 1) Continue with Python

- Pros: existing code, fast prototyping, rich ecosystem.
- Cons: distribution complexity (pip, virtualenvs, Python version matrix), slower startup, environment assumptions.

### 2) Rust

- Pros: single binary, excellent performance, strong type system.
- Cons: steeper learning curve, slower compile times, no ecosystem alignment with Beads.

### 3) TypeScript/Deno

- Pros: fast prototyping, good CLI libraries.
- Cons: runtime dependency (Node/Deno), no ecosystem alignment with Beads.

## Validation / Rollout

1. Spec 001 establishes the Go project structure (`cmd/mindspec/`, `internal/`, `go.mod`).
2. Port Python doctor and Beads hygiene checks to Go.
3. Retire `src/mindspec/` Python prototype once Go binary reaches feature parity on `doctor`.
4. All subsequent specs (002+) build on the Go codebase.

## Summary

Go is selected for MindSpec's v1 CLI to enable simple binary distribution, cross-platform support, and ecosystem alignment with Beads. The small Python prototype makes the rewrite cost minimal now, whereas deferring would increase it as functionality grows.

## ADR: ADR-0005

# ADR-0005: Explicit MindSpec State Tracking via Committed State File

- **Date**: 2026-02-12
- **Status**: Accepted
- **Domain(s)**: workflow, core
- **Deciders**: MindSpec maintainers
- **Supersedes**: n/a
- **Superseded-by**: n/a

---

## Context

MindSpec's three-mode lifecycle (Spec/Plan/Implement) requires that tooling knows the current mode and active work item. ADR-0003 established that `mindspec instruct` should emit mode-appropriate guidance based on "local state," but did not prescribe how mode is determined.

Without explicit state tracking, mode must be inferred heuristically from multiple signals:

- Spec file approval status (`Status: APPROVED` in `spec.md`)
- Plan frontmatter (`status: Approved` in `plan.md`)
- Beads issue state (`bd list --status=in_progress`)
- Current git worktree name

This creates problems:

- **Ambiguity**: Multiple specs or beads may exist in various states; heuristics must guess which is "active"
- **Conflicts**: An in-progress spec bead AND an in-progress implementation bead create contradictory signals
- **Fragility**: Each new command (`instruct`, `next`, `validate`, `spec-status`) must reimplement the same inference logic
- **No session continuity**: A fresh session has no way to know "where we left off" without scanning all artifacts

A prototype `.mindspec/current-spec.json` already exists with `mode`, `activeSpec`, and `lastUpdated` fields, confirming the need for explicit state.

## Decision

MindSpec will maintain an explicit state file at `.mindspec/state.json` as the **primary source of truth** for current mode and active work. The file is **committed to git** as project-level workflow state.

### State File Schema (v1)

```json
{
  "mode": "idle|spec|plan|implement",
  "activeSpec": "004-instruct",
  "activeBead": "beads-xxx",
  "lastUpdated": "2026-02-12T10:00:00Z"
}
```

### Write Surface

State is written exclusively through the MindSpec CLI:

```
mindspec state set --mode=spec --spec=004-instruct
mindspec state set --mode=plan --spec=004-instruct
mindspec state set --mode=implement --spec=004-instruct --bead=beads-xxx
mindspec state set --mode=idle
```

The CLI validates inputs (mode must be one of `idle|spec|plan|implement`, spec must exist if provided). Skill hooks (`/spec-init`, `/spec-approve`, `/plan-approve`) call `mindspec state set` at each transition.

### Commit Ordering

State writes happen **before** the milestone commit at each transition, so `state.json` is co-committed with the transition artifacts:

1. Update artifacts (spec approval, plan frontmatter, etc.)
2. `mindspec state set --mode=X ...`
3. `git add` artifacts + `.mindspec/state.json`
4. Milestone commit

### Completion and Reset

When a bead's implementation is accepted, the state file must be advanced **before** the milestone commit:

- **More beads remain for the active spec**: Set state to the next bead (`--mode=implement --bead=<next>`), or back to `plan` if the next bead isn't ready yet
- **All beads for the spec are done**: Reset to idle (`mindspec state set --mode=idle`)

This follows the same commit ordering as forward transitions: state write first, then co-commit with the closure artifacts. The state file must never be left pointing at a closed bead or completed spec across a commit boundary.

### Cross-Validation

Commands that read state (e.g., `mindspec instruct`) cross-validate `state.json` against artifact state. If they disagree, the command emits an advisory warning with recovery guidance. `state.json` remains the primary signal; artifact state is the sanity check.

### Graceful Degradation

If `state.json` is missing (e.g., first clone, deleted accidentally), commands fall back to artifact-based inference with a warning recommending `mindspec state set` to initialize.

## Decision Details

### A) Committed vs Gitignored

The state file is **committed to git** because:

- It represents project-level workflow state ("this project is in Plan Mode for spec 004"), not personal preferences
- It enables session continuity: `mindspec instruct` works immediately in a fresh session
- It changes infrequently (only at mode transitions), so merge conflicts are rare and trivial
- It serves as a "bookmark" for where the project left off

For future multi-developer scenarios, the state file may need splitting into project-level state (committed) and developer-level state (gitignored). That decision is deferred.

### B) CLI-Mediated Writes Only

State is written through `mindspec state set` rather than direct JSON file manipulation because:

- The CLI validates mode values and spec existence
- A single write path prevents inconsistency
- It's testable and auditable
- Agents call a command rather than constructing JSON

### C) State as Primary, Artifacts as Secondary

Rather than treating artifacts as authoritative and state as cache, `state.json` is the primary signal. This avoids the "which artifact wins?" ambiguity that motivated this ADR. Cross-validation catches staleness but does not override state.

## Consequences

### Positive

- **Single source of truth** for current mode and active work — no heuristic guessing
- **Session continuity** — fresh sessions know exactly where the project left off
- **Simpler tooling** — `instruct`, `next`, `validate`, `spec-status` all read one file instead of scanning artifacts
- **Commit-atomic** — state transitions are co-committed with their artifacts, keeping git history coherent
- **Validated writes** — CLI prevents invalid mode values or nonexistent spec references

### Negative / Tradeoffs

- **Staleness risk** — if a transition hook or completion step fails to update state, it drifts from artifact reality. Mitigated by cross-validation warnings and the completion-reset rule (state must advance when beads close).
- **Merge conflicts** — unlikely but possible if two branches change state simultaneously. Conflicts are trivial to resolve (pick the branch's mode).
- **Single active work item** — v1 schema supports only one `activeSpec` and one `activeBead`. Multi-work scenarios are deferred.

## Alternatives Considered

### 1. Heuristic-only mode detection (no state file)

Infer mode entirely from artifact state (spec approval, plan frontmatter, Beads status).

- Pros: No new file to maintain; always reflects "real" state.
- Cons: Ambiguous when multiple specs/beads exist; each command must reimplement inference; no session continuity; heuristic conflicts require precedence rules.

### 2. Gitignored state file (local-only)

Same as the decision but `.mindspec/state.json` is not committed.

- Pros: No merge conflicts; purely personal state.
- Cons: Fresh sessions start blank; no project-level "where are we?"; state must be reconstructed from artifacts on every new session.

### 3. Store mode in Beads metadata

Use Beads as the state store (e.g., a special "mode" issue or metadata field).

- Pros: Single tracking system; reuses existing infrastructure.
- Cons: Couples mode semantics to Beads internals; violates ADR-0002 (Beads is a passive tracking substrate); adds complexity to Beads queries for a simple key-value lookup.

### 4. Store mode in spec/plan files themselves

Add a `current: true` field to the active spec or plan.

- Pros: State lives with its artifact; no new file.
- Cons: Requires scanning all specs to find the active one; "current" is a project concern, not a spec concern; multiple specs could be marked current.

## Validation / Rollout

1. Implement `internal/state/` package with `Read()`, `Write()`, `CrossValidate()` (Spec 004, Bead 004-A)
2. Implement `mindspec state set` and `mindspec state show` CLI commands (Spec 004, Bead 004-A)
3. Remove `.mindspec/` from `.gitignore` and commit initial `state.json`
4. Update skill hooks to call `mindspec state set` at each transition (Spec 004, Bead 004-C)
5. Verify cross-validation catches intentional drift in tests
6. Dogfood on Spec 004 implementation itself

## Summary

MindSpec will track current mode and active work in a committed `.mindspec/state.json` file, written exclusively through the CLI and cross-validated against artifact state. This replaces heuristic mode detection with an explicit, validated, git-tracked source of truth.

## ADR: ADR-0008

# ADR-0008: Human Gates as Beads Dependency Markers for Approval Workflow

- **Date**: 2026-02-13
- **Status**: Accepted
- **Domain(s)**: workflow, tracking
- **Deciders**: MindSpec maintainers
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Refines**: ADR-0002 (Section F), ADR-0003

---

## Context

Spec 008b introduced human gates into the approval workflow. Before a plan's implementation beads become available via `bd ready`, both the spec and plan must be explicitly approved. This is enforced by Beads gate beads — a first-class Beads type (`--type=gate`) that participates in the dependency graph.

This creates a potential tension with ADR-0002, which states:

> **Section F**: MindSpec will not encode workflow modes into Beads itself. Modes, gating, doc validation, and context assembly are implemented in MindSpec's command layer. Beads remains passive infrastructure.

The question is: does using Beads gates for approval violate the "passive substrate" principle?

---

## Decision

**Beads gates are used as dependency markers for approval ordering, not as the source of truth for approval status.**

Specifically:

1. **Gates are execution signals, not approval records.** A resolved gate means "this dependency is satisfied" in the Beads work graph. The canonical approval record remains in documentation: `spec.md` → `## Approval` section, `plan.md` → YAML frontmatter `status: Approved`.

2. **MindSpec owns the full gate lifecycle.** Gates are created by MindSpec (`mindspec bead spec/plan`), resolved by MindSpec (`mindspec approve spec/plan`), and queried by MindSpec (`IsGateResolved()`). Beads does not initiate, schedule, or decide when gates resolve. It merely stores the dependency edge and honors it in `bd ready`.

3. **Gates refine, not replace, ADR-0002's "passive substrate" principle.** Beads remains passive — it doesn't orchestrate. Gates are data in the dependency graph, no different from a `dep add` between two tasks. The difference is that gate resolution requires a human action mediated by MindSpec, rather than being resolved by task completion.

4. **Two complementary signals, one atomic operation.** `mindspec approve` updates both the document record (frontmatter) and the execution signal (gate) in a single command. Neither system is authoritative alone — they serve different audiences:
   - **Frontmatter** → human reviewers, git history, spec validators
   - **Gate** → `bd ready`, dependency graph, agent work selection

---

## Decision Details

### Gate Conventions

| Gate | Title Convention | Parent | Created By | Resolved By |
|:-----|:-----------------|:-------|:-----------|:------------|
| Spec approval | `[GATE spec-approve <id>]` | Spec bead | `mindspec bead spec` | `mindspec approve spec` |
| Plan approval | `[GATE plan-approve <id>]` | Molecule parent (plan epic) | `mindspec bead plan` | `mindspec approve plan` |

### Dependency Chain

```
[GATE spec-approve] ← [GATE plan-approve] ← [IMPL chunk beads]
```

Until the spec gate is resolved, the plan gate cannot be resolved (dependency). Until the plan gate is resolved, implementation beads don't appear in `bd ready`.

### Backward Compatibility

Gates are optional. If no gate exists for a spec or plan (legacy beads created before 008b), all approval commands warn but proceed. `mindspec next` and `mindspec complete` do not require gates to function.

### What This Is NOT

- Gates do **not** encode workflow modes. The mode system (`idle → spec → plan → implement`) remains entirely in `.mindspec/state.json`, managed by MindSpec.
- Gates do **not** make Beads an orchestrator. Beads doesn't know about MindSpec modes, specs, or plans. It sees gates as dependency nodes — nothing more.
- Gates do **not** replace frontmatter validation. Code that reads `isSpecApproved()` or plan `status: Approved` continues to work unchanged.

---

## Consequences

### Positive

- `bd ready` accurately reflects approval state — agents don't see implementation beads before the plan is approved.
- Approval ordering is enforced in the execution graph, not just in documentation conventions.
- MindSpec retains full control over orchestration — Beads remains a passive store of dependency edges.
- The approval CLI commands (`mindspec approve`) provide a single atomic operation that updates both the document and execution layers.

### Negative / Tradeoffs

- Introduces a coupling between MindSpec's approval concept and Beads' gate type. If Beads changes gate semantics, MindSpec must adapt.
- Two sources must stay in sync (frontmatter + gate). The `mindspec approve` command handles this atomically, but manual edits to frontmatter won't resolve gates (and vice versa).
- Slightly broadens Beads' role from pure task tracking to include approval-gating edges — though gates are a native Beads feature, not a MindSpec extension.

---

## Alternatives Considered

### 1. Keep approval entirely in frontmatter (no Beads gates)

- Pros: simpler; no Beads coupling; ADR-0002 untouched.
- Cons: `bd ready` shows impl beads before plan is approved; agents may start work prematurely; approval ordering is a soft convention, not enforced in the work graph.

### 2. Encode approval state as a MindSpec-side check in `mindspec next`

- Pros: no Beads involvement; pure MindSpec orchestration.
- Cons: `bd ready` (used directly or by other tools) would still show unapproved work; MindSpec would need to filter `bd ready` results rather than letting the dependency graph handle it naturally.

### 3. Use Beads gates as the sole approval record (no frontmatter)

- Pros: single source of truth.
- Cons: violates ADR-0002 (docs as canonical record); loses git-visible approval history; couples approval semantics entirely to Beads.

---

## Relationship to Other ADRs

- **ADR-0002**: This ADR refines Section F. The "passive substrate" principle holds — Beads doesn't orchestrate. Gates are dependency data that MindSpec creates and resolves. The refinement is: Beads may store human-gate dependency edges that affect `bd ready` visibility, as long as MindSpec owns the lifecycle.
- **ADR-0003**: Reinforced. MindSpec remains the sole orchestrator. Gate resolution is triggered by `mindspec approve`, which also handles validation, frontmatter, state transitions, and instruct emission.
- **ADR-0005**: Unaffected. State transitions remain in `.mindspec/state.json`. Gates don't influence mode — they influence work visibility.

---

## Summary

Beads gates are used as **passive dependency markers** in the approval workflow. MindSpec creates and resolves them; Beads stores them. Frontmatter remains the canonical approval record; gates are the execution signal that keeps `bd ready` accurate. This refines ADR-0002's "passive substrate" principle without violating it — Beads stores one more kind of dependency edge, but MindSpec still owns all orchestration.

## Applicable Policies

| ID | Severity | Description | Reference |
|:---|:---------|:------------|:----------|
| plan-mode-no-code | error | In Plan Mode, only Beads entries, plan documents, ADR proposals, and documentation may be modified. Code changes are forbidden until the plan is approved. | docs/core/MODES.md#plan-mode |
| spec-required | error | Every functional change must refer to a spec in docs/specs/ | — |
| doc-sync-required | warning | Changes to core logic must be accompanied by updates to docs/core/, docs/domains/, or docs/features/. Done includes doc-sync. | — |
| adr-divergence-gate | error | If implementation or planning detects that an accepted ADR blocks progress or is unfit, the agent must stop, inform the user, and present divergence options. A new superseding ADR requires human approval. | docs/core/MODES.md#implementation-mode |
| plan-must-cite-adrs | warning | Plans and implementation beads must cite the ADRs they rely on. Uncited ADR reliance is a policy violation. | docs/core/ARCHITECTURE.md#adr-lifecycle |
| domain-operations-require-approval | error | Adding, splitting, or merging domains requires explicit human approval and must produce an ADR. | docs/core/ARCHITECTURE.md#domains |
| spec-declares-impacted-domains | warning | Every spec must declare its impacted domains and relevant ADR touchpoints. | docs/core/MODES.md#spec-mode |
| beads-concise-entries | warning | Beads entries must remain concise and execution-oriented. Long-form specs, ADRs, and domain docs live in the documentation system, not in Beads. | docs/adr/ADR-0002.md |
| beads-active-workset | warning | Keep only active and near-term issues open in Beads. Regularly clean up completed work. Rely on git history + docs for archival traceability. | docs/adr/ADR-0002.md |
| clean-tree-before-transition | error | Working tree must be clean (no uncommitted changes) before starting new work, picking up a bead, or switching modes. If dirty: commit or revert. Never auto-stash. | docs/core/CONVENTIONS.md#clean-tree-rule |
| milestone-commit-at-transition | error | Mode transitions must produce a milestone commit: spec(<bead-id>) for Spec→Plan, plan(<bead-id>) for Plan→Implement, impl(<bead-id>) for Implement→Done. .beads/ changes must be co-committed. | docs/core/CONVENTIONS.md#milestone-commits |

---

## Provenance

| Source | Section | Reason |
|:-------|:--------|:-------|
| docs/domains/workflow/overview.md | Overview | Impacted domain overview |
| docs/domains/workflow/architecture.md | Architecture | Impacted domain architecture (plan/implement tier) |
| docs/domains/core/overview.md | Overview | Impacted domain overview |
| docs/domains/core/architecture.md | Architecture | Impacted domain architecture (plan/implement tier) |
| docs/domains/context-system/interfaces.md | Neighbor Interfaces | 1-hop neighbor via Context Map |
| docs/adr/ADR-0001.md | ADR-0001 | Accepted ADR for domains: core, context-system, workflow |
| docs/adr/ADR-0002.md | ADR-0002 | Accepted ADR for domains: workflow, tracking, context-system |
| docs/adr/ADR-0003.md | ADR-0003 | Accepted ADR for domains: workflow, agent-interface, context-system |
| docs/adr/ADR-0004.md | ADR-0004 | Accepted ADR for domains: core |
| docs/adr/ADR-0005.md | ADR-0005 | Accepted ADR for domains: workflow, core |
| docs/adr/ADR-0008.md | ADR-0008 | Accepted ADR for domains: workflow, tracking |
| architecture/policies.yml | Policies | Policies applicable to mode "plan" |
