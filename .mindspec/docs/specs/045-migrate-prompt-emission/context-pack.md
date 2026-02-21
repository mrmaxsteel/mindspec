# Context Pack

- **Spec**: 045-migrate-prompt-emission
- **Mode**: plan
- **Commit**: 25b6da3ea7deca73df983fd3ef46c06f1c5a0eb4
- **Generated**: 2026-02-21T08:25:00Z

---

## Goal

Replace the expensive, complex `mindspec migrate` implementation (plan/apply/LLM classification/staging/archiving) with a single command that emits a prompt instructing the coding agent to reorganize docs into the canonical MindSpec structure.

## Impacted Domains

- workflow

## 1-Hop Neighbors

- context-system
- core

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

## Neighbor Domain: core — Interfaces

# Core Domain — Interfaces

## Provided Interfaces

### Workspace

```go
package workspace

// FindRoot walks up from startDir looking for .mindspec/ or .git.
func FindRoot(startDir string) (string, error)

// DocsDir returns the docs directory path under root.
func DocsDir(root string) string

// GlossaryPath returns the GLOSSARY.md path under root.
func GlossaryPath(root string) string
```

Used by context-system (for glossary location) and workflow (for spec/bead resolution).

### Health Check Report

```go
package doctor

type Status int // OK, Missing, Error, Warn

type Check struct {
    Name    string
    Status  Status
    Message string
}

type Report struct {
    Checks []Check
}

func (r *Report) HasFailures() bool  // true if any Error or Missing
func Run(root string) *Report        // execute all checks
```

### CLI Command Registration

Other domains register subcommands via cobra in `cmd/mindspec/`. Core owns the top-level `mindspec` command group.

## Consumed Interfaces

- **context-system**: Glossary parsing (for broken-link validation in doctor)
- **workflow**: None currently

## Events

None defined yet. Future: health check completion events for observability.

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

## ADR: ADR-0010

# ADR-0010: Automatic Per-Spec Agent Telemetry Recording

- **Date**: 2026-02-15
- **Status**: Accepted
- **Domain(s)**: viz, workflow, observability
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: ADR-0011

## Context

MindSpec orchestrates a multi-phase spec lifecycle: spec → plan → implement → review → idle. Each phase involves extensive agent activity — reading files, calling tools, making API requests, spawning sub-agents. ADR-0009 established AgentMind as the embedded real-time visualization system, and the bench system already collects OTLP telemetry into NDJSON files for post-hoc analysis.

However, recording is currently a manual, opt-in process: the operator must run `mindspec bench collect`, configure the OTLP endpoint, and manage the resulting files. There is no connection between a recording and the spec it belongs to. If an operator wants to replay the full journey of how a feature was built — from initial spec drafting through implementation — they must manually stitch together ad-hoc recordings.

This creates two gaps:

1. **No automatic capture**: Agent activity during spec development is lost by default. Valuable data about how features are built (what was read, what was tried, how many tokens were consumed per phase) is ephemeral.
2. **No spec-scoped replay**: Even when recordings exist, there's no way to say "show me everything that happened for spec 027" — recordings are disconnected from the artifacts they produced.

## Decision

Automatically record agent telemetry for the full lifecycle of every spec, from `spec-init` through `impl-approve`. Recordings are:

1. **Spec-scoped** — stored alongside the spec artifacts at `docs/specs/<id>/recording/`
2. **Single-file** — one append-only NDJSON file (`events.ndjson`) per spec, with inline phase-marker events delineating lifecycle boundaries
3. **Managed by a background collector** — a lightweight OTLP/HTTP receiver process that starts automatically and persists across Claude Code sessions
4. **Zero-friction** — recording starts and stops automatically with the spec lifecycle; no manual setup required

## Decision Details

### Storage Layout

```
docs/specs/<id>/recording/
  events.ndjson      # All OTLP events + lifecycle markers, append-only
  manifest.json      # Recording metadata, collector state
```

Co-locating recordings with specs follows the existing convention (spec.md, plan.md, context-pack.md are all in the spec directory). Recordings are versioned artifacts like any other spec output.

### Manifest Schema

```json
{
  "spec_id": "027-feature",
  "started_at": "2026-02-15T10:00:00Z",
  "collector_pid": 12345,
  "collector_port": 4318,
  "status": "recording | stopped | complete",
  "phases": [
    {"phase": "spec", "started_at": "...", "ended_at": "..."},
    {"phase": "plan", "started_at": "...", "ended_at": "..."},
    {"phase": "implement", "started_at": "...", "ended_at": "...", "beads": ["T-abc", "T-def"]},
    {"phase": "review", "started_at": "...", "ended_at": "..."}
  ]
}
```

### Single File with Phase Markers

Lifecycle events are emitted as NDJSON lines alongside OTLP events:

```json
{"ts":"...","event":"lifecycle.start","data":{"spec_id":"027-feature","phase":"spec"}}
{"ts":"...","event":"lifecycle.phase","data":{"from":"spec","to":"plan","spec_id":"027-feature"}}
{"ts":"...","event":"lifecycle.bead.start","data":{"bead_id":"T-abc","spec_id":"027-feature"}}
{"ts":"...","event":"lifecycle.bead.complete","data":{"bead_id":"T-abc","spec_id":"027-feature"}}
{"ts":"...","event":"lifecycle.end","data":{"spec_id":"027-feature"}}
```

These marker events use the same `CollectedEvent` schema as OTLP events, so AgentMind replay processes them without special handling. The viz layer can render phase transitions as visual events (e.g., graph reset, phase label overlay).

### Collector Lifecycle

The recording collector is a background process managed via PID file:

**Start (on `spec-init`):**
1. `spec-init` creates the recording directory and manifest
2. Starts the collector as a background process (reuses `bench.Collector` internals)
3. Writes PID and port to manifest
4. Configures Claude Code's OTLP endpoint by writing to the project's `.claude/settings.local.json`

**Keep-alive (on SessionStart hook):**
1. Hook reads `.mindspec/state.json` — if mode ≠ idle, check for active recording
2. If manifest exists and `status == "recording"`:
   - Check if PID is alive (`kill -0`)
   - If dead, restart collector with same port and output file (append mode)
   - Ensure OTLP config is set
3. If no recording exists, do nothing (backwards-compatible)

**Stop (on `impl-approve`):**
1. Send SIGTERM to collector PID
2. Update manifest: `status: "complete"`, final phase end times
3. Remove OTLP endpoint from Claude Code settings

**Graceful degradation:** If the collector crashes and isn't restarted (e.g., hook doesn't fire), events are simply lost for that gap. The recording remains valid — NDJSON is append-only, so partial recordings are still replayable.

### OTLP Configuration

Claude Code reads OTLP environment variables at session startup only — there is no hot-reload. Rather than toggling OTLP config per-spec (which would require a session restart each time), **OTLP is configured once, permanently** during project bootstrap.

The first `spec-init` (or `mindspec init`) writes OTLP env vars to `.claude/settings.local.json` (project-local, gitignored):

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/json",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4319"
  }
}
```

This is a **one-time, permanent** configuration. The user restarts Claude Code once after first setup. From then on:
- When a collector is running (active spec) → telemetry flows into the recording
- When no collector is running (idle) → OTLP exporter silently drops batches (connection refused, no retry storm, no visible errors)

Port 4319 is used for the recording collector, leaving 4318 free for `mindspec agentmind serve` (live visualization).

If OTLP env vars are already configured with a different endpoint (e.g., pointing to an external collector), the bootstrap warns and does not override.

### Integration Points

| Command | Recording Action |
|:--------|:-----------------|
| `spec-init` | Create recording dir + manifest, start collector, configure OTLP, emit `lifecycle.start` |
| `approve spec` | Emit `lifecycle.phase` marker (spec → plan) |
| `approve plan` | Emit `lifecycle.phase` marker (plan → plan-approved) |
| `next` | Emit `lifecycle.bead.start` marker |
| `complete` | Emit `lifecycle.bead.complete` marker |
| `approve impl` | Emit `lifecycle.end` marker, stop collector, clean up OTLP config |
| SessionStart hook | Health-check collector, restart if dead |

### Replay Integration

AgentMind replay already accepts NDJSON files. Two additions:

1. **`--spec <id>` convenience flag**: resolves to `docs/specs/<id>/recording/events.ndjson`
2. **Phase filtering**: `--phase plan` filters the NDJSON stream to events between the matching `lifecycle.phase` markers

## Consequences

### Positive

- Every spec's development journey is captured automatically — no manual setup
- Recordings are co-located with specs, making them discoverable and versionable
- Full replay capability: watch how any feature was built, from spec through implementation
- Phase-level granularity enables targeted analysis (e.g., "how many tokens did planning consume?")
- Builds on existing infrastructure (bench collector, AgentMind replay, NDJSON format)
- Backwards-compatible: specs without recordings work exactly as before

### Negative / Tradeoffs

- Background collector process adds operational complexity (PID management, health checks)
- NDJSON files can grow large for complex specs with many beads (mitigated: NDJSON compresses well, can be pruned)
- Single-port collector means only one spec can record at a time on the default port (acceptable: MindSpec is single-spec-active by design)
- Fan-out not supported in v1 — can't simultaneously record and visualize live without manual setup
- Recordings include all OTLP events, including ones unrelated to the spec work (e.g., if the user does other things in the same session)

## Alternatives Considered

### 1. Per-Phase Files

Store each phase in a separate NDJSON file (`spec-phase.ndjson`, `plan-phase.ndjson`, etc.). Rejected because:
- Requires stitching for full replay
- Phase boundaries don't always align with session boundaries (a session might span spec-approve + early plan work)
- More files to manage with no meaningful benefit — filtering a single file by marker events is trivial

### 2. No Background Collector (Direct File Write)

Have Claude Code write telemetry directly to a file instead of sending OTLP to a collector. Rejected because:
- Claude Code emits OTLP over HTTP; there's no file-write mode
- Would require patching Claude Code or adding a custom exporter
- OTLP/HTTP is the standard protocol; the collector approach is the expected pattern

### 3. Opt-In Recording

Only record when the user explicitly requests it (e.g., `mindspec record start`). Rejected because:
- The whole point is zero-friction capture — if it requires manual action, recordings won't exist for most specs
- "Record everything by default" is cheap (disk is cheap, NDJSON compresses well) and enables analysis that wasn't planned in advance

### 4. Central Recording Database

Store all recordings in a single database or directory (e.g., `.mindspec/recordings/`) rather than per-spec. Rejected because:
- Breaks the co-location convention (spec artifacts live with the spec)
- Makes it harder to find a spec's recording
- Can't version recordings alongside the spec they belong to

## Validation / Rollout

1. `spec-init` creates recording directory and starts collector — verify PID file, port, OTLP config
2. Claude Code telemetry appears in `events.ndjson` during normal spec work
3. Phase markers appear at each lifecycle transition (`approve spec`, `approve plan`, `next`, `complete`, `approve impl`)
4. SessionStart hook restarts a dead collector without data loss
5. `impl-approve` stops collector and cleans up OTLP config
6. `mindspec agentmind replay --spec <id>` replays the full recording
7. `mindspec agentmind replay --spec <id> --phase plan` replays only the planning phase
8. Specs without recordings (created before this feature) work normally with no errors

## ADR: ADR-0012

# ADR-0012: Compose with External CLIs, Don't Wrap Them

- **Date**: 2026-02-16
- **Status**: Accepted
- **Domain(s)**: bead, workflow, core
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Context

MindSpec integrates with Beads (`bd`) for work tracking, dependency management, and molecule orchestration. The current integration maintains `internal/bead/` — a 39-function Go wrapper layer that shells out to `bd` for every operation, parses JSON responses into Go structs, and re-exports them to callers.

This wrapping approach has proven fragile:

1. **Silent breakage goes undetected.** `gate.go` used `bd create --type=gate`, which was never a valid beads operation (gates are formula step primitives, not standalone issue types). The code included fallback logic ("legacy beads — proceeding without gate") that silently skipped gate enforcement. Approval gates were never actually enforced — the entire feature was broken from inception.

2. **Feature reimplementation creates drift.** `propagate.go` reimplements parent-child status propagation that beads molecules handle natively. `next/beads.go` reimplements molecule-aware work discovery that `bd ready --parent` already provides. `plan.go` manually constructs molecules via loops of `Create()` + `DepAdd()` calls when `bd mol pour` does this in a single command.

3. **Abstraction layers obscure intent.** A call chain like `ApproveSpec()` → `bead.CreateSpecBead()` → `bead.FindOrCreateGate()` → `bead.Search()` → `exec.Command("bd", "search", ...)` is four layers of indirection for what is ultimately one CLI call. When the underlying CLI changes, every layer must be audited.

4. **Maintenance cost scales with surface area.** Each new beads feature (formulas, molecules, wisps, pinning) would require new wrapper functions, new Go structs, new tests — duplicating work that beads already validates.

Meanwhile, beads is explicitly designed for CLI composition: every command supports `--json` output, hash-based IDs prevent collision without coordination, and `bd ready` provides dependency-aware task selection. The tool is built to be called, not wrapped.

## Decision

**MindSpec composes with external CLIs via direct `exec.Command()` calls at the call site. It does not maintain wrapper packages that abstract external tools into Go function libraries.**

### Principles

1. **Call, don't wrap.** When mindspec needs to interact with `bd`, it calls `exec.Command("bd", args...)` directly where the interaction happens. No intermediate Go function that exists solely to proxy a CLI call.

2. **Parse at the boundary.** JSON parsing happens at the call site using a minimal shared helper (`RunBD()` or equivalent). The parsed data is used immediately — not re-exported as package-level Go types that mirror the external tool's data model.

3. **Own your logic, delegate the plumbing.** MindSpec owns spec validation, context pack assembly, guidance emission, state machine transitions, ADR lifecycle. It delegates work tracking, dependency enforcement, and molecule orchestration to beads. The boundary is: if beads does it, don't reimplement it.

4. **Keep escape hatches.** `cmd/mindspec/bead.go` subcommands (`bead spec`, `bead plan`, `bead hygiene`) remain as manual escape hatches for operators. These are thin CLI pass-throughs, not abstraction layers.

5. **Allowed exceptions.** A Go helper is justified when it provides genuine multi-step orchestration (e.g., `Preflight()` checks git repo + `.beads/` dir + `bd` on PATH) or domain-specific reporting (e.g., hygiene audit interprets beads data through mindspec-specific rules). The test: would inlining the `bd` call at the call site be clearer? If yes, inline it.

### Applied to Beads

| Before (wrapping) | After (composing) |
|---|---|
| `bead.CreateGate(title, parent)` | Eliminated — formulas define gates declaratively |
| `bead.ResolveGate(id, reason)` | `exec.Command("bd", "close", stepID)` at call site |
| `bead.CreatePlanBeads(root, specID)` | `exec.Command("bd", "pour", "spec-lifecycle", "--var", ...)` at call site |
| `bead.PropagateStart(specID)` | Eliminated — molecules handle parent status natively |
| `bead.Search(query)` | `exec.Command("bd", "search", query, "--json")` at call site |
| `bead.MolReady(parentID)` | `exec.Command("bd", "ready", "--parent", parentID, "--json")` at call site |

### Applied Generally

This principle extends beyond beads. If mindspec integrates with other CLIs in the future (e.g., `gh` for GitHub, `git` for repository operations), the same rule applies: call directly, parse at the boundary, don't build Go wrapper packages.

## Consequences

### Positive

- **Reduced maintenance surface.** Deleting `gate.go`, `spec.go`, `plan.go`, `propagate.go` eliminates ~600 lines of wrapper code and their corresponding tests.
- **Breakage becomes visible.** No fallback logic to hide failures. If `bd close` fails, the error propagates directly. No silent degradation.
- **Beads upgrades are free.** New beads features (formulas, wisps, pinning) can be used via `exec.Command()` without writing new Go wrappers.
- **Intent is clearer.** Reading `exec.Command("bd", "close", stepID)` at the call site is immediately understandable. Reading `bead.ResolveGate(gate.ID, reason)` requires tracing through the wrapper to understand what it actually does.
- **Fewer abstraction layers.** Approval flow goes from 4 layers to 2: `ApproveSpec()` → `exec.Command("bd", ...)`.

### Negative / Tradeoffs

- **Duplicated exec boilerplate.** Multiple call sites may have similar `exec.Command` + JSON parse patterns. Mitigated by a shared `RunBD(args...) ([]byte, error)` helper that handles stderr separation and error wrapping — but this is a utility, not an abstraction layer.
- **No compile-time type safety for beads data.** Callers work with `map[string]interface{}` or minimal structs rather than comprehensive Go types. In practice, most call sites only need 1-2 fields (ID, status), so comprehensive types were over-engineering.
- **Harder to mock in tests.** Direct `exec.Command` calls require `execCommand` variable injection or integration tests. Mitigated by the existing pattern of package-level `var execCommand = exec.Command` for test substitution.
- **Scattered bd calls.** Without a central package, it's harder to grep for "all places that call beads." Mitigated by a consistent pattern: search for `"bd"` in exec.Command calls.

## Alternatives Considered

### 1. Keep the wrapper, fix gate.go

Fix `--type=gate` to use formulas, keep the rest of `internal/bead/` as-is. Rejected because the gate breakage is a symptom, not the disease. The wrapping pattern itself creates a maintenance burden and drift risk that grows with every beads release.

### 2. Use beads as a Go library

Import `github.com/steveyegge/beads/internal/storage` directly. Rejected because:
- Beads internal packages are not a stable API
- Couples mindspec to beads internals rather than the stable CLI interface
- Defeats beads' CLI-first design philosophy

### 3. Build an MCP integration instead of CLI calls

Use beads' MCP server (`beads-mcp`) for structured tool access. Rejected because:
- MCP adds 10-50k tokens of context overhead per the beads docs
- CLI calls via `exec.Command` are zero-overhead and well-understood
- MCP is designed for environments without shell access (Claude Desktop, Amp) — mindspec always has shell access

## Validation / Rollout

Spec 032 implements this decision:

1. Delete `internal/bead/gate.go`, `spec.go`, `plan.go`, `propagate.go`
2. Reduce `internal/bead/bdcli.go` to a minimal exec helper
3. Create `.beads/formulas/spec-lifecycle.formula.toml` (declarative, not imperative)
4. Inline `bd` calls at call sites in `approve/`, `complete/`, `next/`, `specinit/`
5. Verify: `internal/bead/` exports <10 functions (down from 39), `make test` passes

## ADR: ADR-0013

# ADR-0013: Use Beads Formulas for Spec Lifecycle Orchestration

- **Date**: 2026-02-16
- **Status**: Accepted
- **Domain(s)**: bead, workflow, core
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Context

MindSpec's spec lifecycle follows a fixed sequence: spec → spec-approve → plan → plan-approve → implement → review. Each transition is a human approval gate.

Currently, mindspec constructs this workflow imperatively in Go:

1. `bead/spec.go` creates a spec bead, then manually creates a gate issue via `bd create --type=gate` (which is broken — see ADR-0012)
2. `bead/plan.go` parses plan frontmatter for work chunks, creates an epic (molecule parent), creates child task issues in a loop, wires dependencies via `DepAdd()` calls, creates a plan approval gate, and writes generated bead IDs back to the plan file
3. `approve/spec.go` and `approve/plan.go` search for gate issues by title convention, then call `bd gate resolve` to close them

This is ~400 lines of imperative Go that constructs beads molecules by hand. It reimplements what beads formulas do declaratively: define steps with types, dependencies, and gates in a TOML file, then instantiate with `bd mol pour`.

Beads formulas are purpose-built for this:

```toml
formula = "spec-lifecycle"

[[steps]]
id = "spec"
title = "Write spec {{spec_id}}"

[[steps]]
id = "spec-approve"
title = "Approve spec {{spec_id}}"
needs = ["spec"]
type = "human"

# ...etc
```

One `bd mol pour spec-lifecycle --var spec_id=032-native-beads` replaces the entire imperative construction sequence.

## Decision

**MindSpec defines its spec lifecycle as a beads formula (`.beads/formulas/spec-lifecycle.formula.toml`) and instantiates it via `bd mol pour` rather than constructing molecules imperatively in Go.**

### What the formula defines

A `spec-lifecycle` formula with 6 steps:

| Step ID | Type | Needs | Purpose |
|---------|------|-------|---------|
| `spec` | task | — | Write the spec |
| `spec-approve` | human | spec | Human approval gate |
| `plan` | task | spec-approve | Write the plan |
| `plan-approve` | human | plan | Human approval gate |
| `implement` | task | plan-approve | Implementation work |
| `review` | human | implement | Final review |

### How mindspec uses it

| Mindspec command | Beads operation |
|---|---|
| `mindspec spec-init 032-foo` | `bd mol pour spec-lifecycle --var spec_id=032-foo` |
| `mindspec approve spec 032-foo` | Validate spec → `bd close <spec-approve step>` |
| `mindspec approve plan 032-foo` | Validate plan → `bd close <plan-approve step>` |
| `mindspec next` | `bd ready` (beads enforces step ordering) |
| `mindspec complete` | `bd close <current step>` |

### What mindspec still owns

Mindspec retains all domain-specific logic:
- **Spec/plan validation** before closing approval steps
- **Artifact management** — spec.md, plan.md, context-pack.md frontmatter updates
- **State machine** — mode transitions (idle/spec/plan/implement/review) synchronized with molecule progression
- **Context engineering** — context pack assembly from domain docs, ADRs, policies
- **Guidance emission** — `mindspec instruct` emits mode-appropriate operating instructions

The formula defines *what steps exist and how they depend on each other*. Mindspec defines *what happens at each step* (validation, artifact updates, context assembly).

### Formula location

The formula lives in `.beads/formulas/spec-lifecycle.formula.toml` (project-level). This means:
- It travels with the repo (git-tracked)
- Teams can customize the lifecycle for their project
- It's inspectable — `cat .beads/formulas/spec-lifecycle.formula.toml` shows the full workflow
- Beads discovers it automatically (project-level formulas take priority)

## Consequences

### Positive

- **Declarative over imperative.** The lifecycle is defined in ~20 lines of TOML instead of ~400 lines of Go. The TOML is readable by anyone; the Go required understanding beads internals.
- **Beads enforces dependencies natively.** `bd ready` only shows steps whose `needs` are satisfied. No custom enforcement logic in mindspec.
- **Formula is customizable.** Teams can add project-specific steps (e.g., security review, load test) by editing the TOML. No Go changes required.
- **Molecule lifecycle is free.** Beads handles molecule creation, step ID assignment, parent-child relationships, status tracking, and `bd dep tree` visualization. Mindspec doesn't reimplement any of this.
- **Consistent with ADR-0012.** Composing with beads' declarative primitive (formulas) rather than wrapping its imperative API (create + dep-add loops).

### Negative / Tradeoffs

- **Formula must be present.** If `.beads/formulas/spec-lifecycle.formula.toml` is missing or malformed, `bd mol pour` fails. Mitigated by: `mindspec doctor` checks for the formula; `mindspec spec-init` can bootstrap it if missing.
- **Beads formula system is a dependency.** If beads changes its formula spec, mindspec's TOML may need updating. Mitigated by: formulas are versioned (`version = 1`) and beads maintains backward compatibility.
- **Less control over step ID assignment.** Beads assigns step IDs as `<mol-id>.1`, `<mol-id>.2`, etc. Mindspec must discover step IDs by querying the molecule rather than controlling them. Mitigated by: steps have stable `title` fields that mindspec can match on.
- **Implementation work chunks not modeled in formula.** The plan may decompose implementation into multiple work chunks, but the formula has a single `implement` step. Sub-decomposition (if needed) happens via separate beads issues under the implement step, not formula steps.

## Alternatives Considered

### 1. Keep imperative molecule construction, fix gate.go

Fix the `--type=gate` issue (e.g., use `--type=task` with naming conventions), keep the rest of the imperative construction. Rejected because:
- Still ~400 lines of Go reimplementing what `bd mol pour` does
- Every lifecycle change requires Go code changes, recompilation, and redeployment
- Inconsistent with ADR-0012's compose-don't-wrap principle

### 2. Embed the formula in the mindspec binary

Compile the formula TOML as an embedded Go resource, write it to `.beads/formulas/` on first use. Rejected because:
- Adds complexity for no benefit — the file is small and should be version-controlled with the project
- Makes customization harder (users would need to override an embedded default)
- Breaks the principle that beads formulas live in `.beads/formulas/`

### 3. Use beads molecules without formulas

Create molecules manually via `bd create --parent` without a formula template. Rejected because:
- This is what the current code does (imperatively), just without the Go wrapper
- No variable substitution, no formula versioning, no `bd mol pour` discoverability
- Loses the declarative benefit entirely

## Validation / Rollout

1. `.beads/formulas/spec-lifecycle.formula.toml` exists and `bd mol pour spec-lifecycle --var spec_id=test --dry-run` succeeds
2. `mindspec spec-init 999-test` creates a molecule; `bd mol show <id>` shows 6 steps
3. `bd dep tree <mol-id>` shows the correct dependency chain
4. `mindspec approve spec 999-test` closes the spec-approve step; `bd ready` shows the plan step
5. Imperative molecule construction code (`bead/spec.go`, `bead/plan.go`) is deleted

## ADR: ADR-0014

# ADR-0014: Canonical MindSpec Document Root Under .mindspec

- **Date**: 2026-02-17
- **Status**: Accepted
- **Domain(s)**: core, context-system, workflow
- **Deciders**: MindSpec maintainers
- **Supersedes**: ADR-0001 (storage-location semantics only; DDD semantics unchanged)
- **Superseded-by**: n/a

## Context

ADR-0001 established DDD-informed documentation artifacts and context-pack assembly rules. It also referenced canonical artifact locations under `docs/` and `architecture/`.

Brownfield onboarding introduces a canonical operational root under `.mindspec/` for deterministic migration, archival lineage, and reproducible tooling behavior. The old paths are still needed as compatibility fallbacks for pre-migration repositories, but they can no longer be the primary canonical locations.

## Decision

MindSpec canonical locations are:

1. Canonical docs root: `.mindspec/docs/`
2. Canonical policies file: `.mindspec/policies.yml`

Legacy locations (`docs/*`, `architecture/policies.yml`) are compatibility read fallbacks only when canonical locations are absent.

## Decision Details

### Supersession Scope (Narrow)

This ADR supersedes only ADR-0001's storage-path semantics. Specifically, it replaces assumptions that canonical docs and policies live under:

- `docs/*`
- `architecture/policies.yml`

### Explicit Non-Superseded ADR-0001 Semantics

This ADR does **not** supersede ADR-0001's DDD model, including:

- required project DDD artifacts (Context Map, domain docs, ADR lifecycle, glossary)
- DDD-informed context-pack assembly and 1-hop contract expansion
- governance expectations around ADR divergence and documentation sync

Those ADR-0001 semantics remain authoritative.

## Consequences

### Positive

- Canonical operational state is consistently scoped under `.mindspec/`
- Brownfield migration has a deterministic target layout
- Tooling can implement canonical-first path resolution with safe legacy fallback

### Negative / Tradeoffs

- During migration periods, repositories may contain both canonical and legacy trees
- Documentation and policy references must be updated to canonical paths to avoid drift

## Alternatives Considered

### 1. Keep canonical docs under `docs/` permanently

Rejected because brownfield migration and operational state separation become harder to reason about and validate.

### 2. Move policies only, keep docs canonical in `docs/`

Rejected because it preserves split-brain path conventions and weakens deterministic tooling behavior.

## Validation / Rollout

1. Update workspace path APIs to canonical-first resolution
2. Update docs and conventions to canonical path guidance
3. Keep legacy path read fallback for pre-migration repositories
4. Ensure brownfield migration outputs canonical docs and canonical policy references

## ADR: ADR-0015

# ADR-0015: Per-Spec Molecule-Derived Lifecycle State

- **Date**: 2026-02-19
- **Status**: Accepted
- **Domain(s)**: workflow
- **Deciders**: Max
- **Supersedes**: ADR-0005
- **Superseded-by**: n/a

---

## Context

MindSpec currently uses `.mindspec/state.json` as the single canonical lifecycle pointer (ADR-0005). The state file tracks exactly one `mode`, one `activeSpec`, and one `activeBead` at a time. This prevents parallel spec work within a single worktree — only one spec can be "active" at any moment.

ADR-0007 proposed per-worktree state files to enable parallelism, but this approach couples orchestration state to git branch topology and was never accepted.

ADR-0013 established that spec lifecycles are orchestrated by beads formulas: `mindspec spec-init` pours a `spec-lifecycle` molecule, and each lifecycle phase corresponds to a molecule step. The molecule already encodes the full lifecycle state for each spec — which steps are complete, which are in progress, which are blocked.

This means the authoritative lifecycle state for any spec already lives in Beads, not in `state.json`. The state file is a redundant, lossy projection of molecule state that breaks down when multiple specs are in flight.

## Decision

**Lifecycle state is derived per-spec from the spec-lifecycle molecule's step statuses, not from a global state file.**

Each spec's molecule (created at `spec-init` via `bd mol pour`) is the single source of truth for that spec's lifecycle position. The current mode for a spec is computed by examining which molecule steps are complete, in-progress, or blocked:

| Molecule state | Derived mode |
|:---|:---|
| `spec` step open | spec |
| `spec-approve` step ready/open | spec (awaiting approval) |
| `plan` step open | plan |
| `plan-approve` step ready/open | plan (awaiting approval) |
| `implement` step open | implement |
| `review` step ready/open | review |
| All steps closed | done |

### state.json becomes a convenience cursor

`.mindspec/state.json` is retained but demoted from "primary source of truth" (ADR-0005) to a **non-canonical "last focused spec" cursor** for UX convenience only. Its purpose:

- Track which spec the user was last working on, so `mindspec instruct` can default to showing guidance for that spec
- Store UI preferences (e.g., last-selected bead)
- Provide a fast hint to avoid querying all molecules on every command

**state.json is never consulted for mode derivation.** Mode is always derived from the molecule. If `state.json` disagrees with molecule state, molecule state wins silently.

### Command targeting

All lifecycle commands accept a `--spec` flag to target a specific spec. When `--spec` is omitted, commands use the `activeSpec` from `state.json` as a default, but derive mode from the molecule — not from `state.json`'s `mode` field.

### Spec-to-molecule binding

Each spec's `spec.md` frontmatter includes a `molecule_id` field that binds it to its lifecycle molecule. This is written by `mindspec spec-init` when the molecule is poured. The binding enables per-spec mode derivation without scanning all molecules.

## Decision Details

### A) Molecule is the source of truth

The beads molecule already tracks step completion, dependencies, and ordering. Re-deriving mode from molecule state eliminates the class of bugs where `state.json` drifts from reality (stale mode after a failed transition, forgotten state update, etc.).

### B) Multiple specs can progress independently

With per-spec molecule-derived state, there is no global mode lock. Spec A can be in `plan` mode while Spec B is in `implement` mode. The user focuses on one spec at a time (tracked by `state.json`'s cursor), but nothing prevents switching between specs.

### C) Backward compatibility

The `state.json` file continues to exist and commands continue to write to it as a cursor update. Existing workflows that rely on `mindspec state show` will see the same output for the focused spec. The change is internal: mode derivation reads from the molecule, not from `state.json`.

### D) Relationship to ADR-0013

ADR-0013 established formula-driven lifecycle orchestration. This ADR is the natural consequence: if the molecule defines the lifecycle, the molecule should be authoritative for lifecycle state. ADR-0013 is unchanged; this ADR conforms to it.

## Consequences

### Positive

- **Multiple specs can progress independently** — no global mode lock prevents parallel spec work
- **Single source of truth per spec** — the molecule is both the workflow definition and the state store, eliminating drift between `state.json` and reality
- **Simpler state management** — no need to carefully synchronize `state.json` writes with molecule step closures; the molecule is always correct
- **Commands become stateless queries** — mode derivation is a pure function of molecule state, not a mutable file
- **Aligns with ADR-0013** — formula-driven lifecycle implies formula-derived state

### Negative / Tradeoffs

- **Beads dependency is deeper** — mode derivation now requires querying Beads, not just reading a JSON file. If Beads is unavailable, mode cannot be derived. Mitigated by: Beads is already required for all lifecycle operations.
- **Migration required** — existing repos with `state.json` as the authority need backfill: each spec's `spec.md` needs a `molecule_id` binding, and existing molecules need to be discoverable. Mitigated by: a lazy backfill migration that binds specs to molecules on first access.
- **Slightly slower** — deriving mode requires a `bd mol show` call rather than reading a local JSON file. Mitigated by: the call is fast (SQLite query), and results can be cached within a command invocation.

## Alternatives Considered

### 1. Per-worktree state files (ADR-0007)

Each git worktree maintains its own `state.json` on its own branch.

- Pros: Enables parallelism; preserves state-file-as-authority model
- Cons: Couples orchestration state to git branch topology; merge conflicts on `state.json`; requires worktree creation for any parallel work; duplicates information already in molecules
- Rejected because molecule-derived state achieves the same parallelism goal without the git coupling

### 2. Multi-item state file

Extend `state.json` to track multiple active workstreams as an array.

- Pros: Single file; no Beads query needed
- Cons: Complex schema; must decide which workstream is "current"; still a redundant projection of molecule state; synchronization bugs multiply with each tracked workstream

### 3. Keep ADR-0005 as-is, accept single-spec limitation

- Pros: No changes needed
- Cons: Blocks parallel spec work; `state.json` drift bugs continue; does not leverage ADR-0013's molecule-driven model

## Validation / Rollout

1. Add `molecule_id` field to spec frontmatter schema; `mindspec spec-init` writes it when pouring the molecule
2. Implement `DeriveMode(moleculeID)` function that queries molecule step statuses and returns the current mode
3. Update all mode-reading code paths (`instruct`, `next`, `complete`, `approve`) to use `DeriveMode()` instead of reading `state.json`'s mode field
4. Demote `state.json` writes to cursor updates (still written, but not consulted for mode)
5. Lazy backfill: on first access, if a spec lacks `molecule_id`, search for its molecule by title convention and bind it
6. `mindspec doctor` checks for unbound specs and offers to run backfill

## ADR: ADR-0016

# ADR-0016: Bead Creation Timing Across Lifecycle Phases

- **Date**: 2026-02-20
- **Status**: Accepted
- **Domain(s)**: workflow, beads
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a

---

## Context

MindSpec's spec lifecycle has three phases where Beads artifacts are created: spec initialization, plan approval, and implementation. Each phase faces the same timing question: should beads be created eagerly (as soon as content is being drafted) or lazily (only when the phase is approved/finalized)?

The tradeoff is consistent across phases:
- **Eager creation** gives early bead IDs for cross-referencing and enables Beads-native dependency tracking during drafting, but requires reconciliation as content evolves.
- **Lazy creation** keeps the drafting loop simple (markdown is the sole artifact) and avoids sync complexity, at the cost of no bead IDs until the handoff point.

The right answer depends on how much the content changes during drafting and whether early bead IDs provide meaningful workflow value.

## Decision

**Spec molecule: created eagerly at `spec-init` time.**

`mindspec spec-init` pours the `spec-lifecycle` formula immediately, creating the full molecule with all lifecycle step beads (spec gate, plan gate, implementation steps, review gate). This is appropriate because:

- The molecule structure is fixed by the formula — it doesn't change as the spec is drafted.
- The step mapping (which bead ID corresponds to which lifecycle phase) is needed immediately so that `mindspec state`, `mindspec instruct`, and gate resolution can function throughout the lifecycle.
- There is no reconciliation problem because the molecule shape is determined by the formula, not by spec content.

**Implementation beads: created lazily at plan-approval time.**

`ApprovePlan()` calls `CreatePlanBeads()` and `WriteGeneratedBeadIDs()` as part of the approval flow, before resolving the plan gate. This is appropriate because:

- Work chunks change frequently during plan drafting — chunks get added, removed, renamed, reordered, and merged. Each edit would require reconciling plan YAML with existing beads.
- Even if beads were created early, `bd ready` would not surface them until the plan gate resolves (Spec 008b), so early creation provides no workflow benefit.
- The plan's `work_chunks` YAML is sufficient as the authoritative decomposition during drafting. Beads take over as the authoritative tracking at the approval boundary.

**The general principle: create beads eagerly when the structure is formula-driven (fixed shape), lazily when the structure is content-driven (evolving shape).**

## Consequences

- **Spec molecule available immediately** — `mindspec instruct`, `mindspec state`, and gate commands work from the moment a spec is initialized, with no "beads not yet created" edge cases.
- **No bead reconciliation during planning** — plans can be freely edited without syncing to Beads state, keeping the drafting loop fast and simple.
- **Single handoff point for impl beads** — the approval moment is the only time plan YAML is translated to Beads issues, reducing the surface area for sync bugs.
- **No early impl bead IDs in plan.md** — `bead_ids` in plan frontmatter are only populated after approval, not during drafting. This is acceptable because bead IDs are consumed by implementation tooling, not by the planning process.
- **Formula changes propagate differently than content changes** — if the `spec-lifecycle` formula is updated, existing molecules are unaffected (they were poured with the old formula). This is intentional: lifecycle shape is locked at init time.

## Applicable Policies

| ID | Severity | Description | Reference |
|:---|:---------|:------------|:----------|
| plan-mode-no-code | error | In Plan Mode, only Beads entries, plan documents, ADR proposals, and documentation may be modified. Code changes are forbidden until the plan is approved. | .mindspec/docs/core/MODES.md#plan-mode |
| spec-required | error | Every functional change must refer to a spec in docs/specs/ | — |
| doc-sync-required | warning | Changes to core logic must be accompanied by updates to docs/core/, docs/domains/, or docs/features/. Done includes doc-sync. | — |
| adr-divergence-gate | error | If implementation or planning detects that an accepted ADR blocks progress or is unfit, the agent must stop, inform the user, and present divergence options. A new superseding ADR requires human approval. | .mindspec/docs/core/MODES.md#implementation-mode |
| plan-must-cite-adrs | warning | Plans and implementation beads must cite the ADRs they rely on. Uncited ADR reliance is a policy violation. | .mindspec/docs/core/ARCHITECTURE.md#adr-lifecycle |
| domain-operations-require-approval | error | Adding, splitting, or merging domains requires explicit human approval and must produce an ADR. | .mindspec/docs/core/ARCHITECTURE.md#domains |
| spec-declares-impacted-domains | warning | Every spec must declare its impacted domains and relevant ADR touchpoints. | .mindspec/docs/core/MODES.md#spec-mode |
| beads-concise-entries | warning | Beads entries must remain concise and execution-oriented. Long-form specs, ADRs, and domain docs live in the documentation system, not in Beads. | .mindspec/docs/adr/ADR-0002.md |
| beads-active-workset | warning | Keep only active and near-term issues open in Beads. Regularly clean up completed work. Rely on git history + docs for archival traceability. | .mindspec/docs/adr/ADR-0002.md |
| clean-tree-before-transition | error | Working tree must be clean (no uncommitted changes) before starting new work, picking up a bead, or switching modes. If dirty: commit or revert. Never auto-stash. | .mindspec/docs/core/CONVENTIONS.md#clean-tree-rule |
| milestone-commit-at-transition | error | Mode transitions must produce a milestone commit: spec(<bead-id>) for Spec→Plan, plan(<bead-id>) for Plan→Implement, impl(<bead-id>) for Implement→Done. .beads/ changes must be co-committed. | .mindspec/docs/core/CONVENTIONS.md#milestone-commits |

---

## Provenance

| Source | Section | Reason |
|:-------|:--------|:-------|
| .mindspec/docs/domains/workflow/overview.md | Overview | Impacted domain overview |
| .mindspec/docs/domains/workflow/architecture.md | Architecture | Impacted domain architecture (plan/implement tier) |
| .mindspec/docs/domains/context-system/interfaces.md | Neighbor Interfaces | 1-hop neighbor via Context Map |
| .mindspec/docs/domains/core/interfaces.md | Neighbor Interfaces | 1-hop neighbor via Context Map |
| .mindspec/docs/adr/ADR-0001.md | ADR-0001 | Accepted ADR for domains: core, context-system, workflow |
| .mindspec/docs/adr/ADR-0002.md | ADR-0002 | Accepted ADR for domains: workflow, tracking, context-system |
| .mindspec/docs/adr/ADR-0003.md | ADR-0003 | Accepted ADR for domains: workflow, agent-interface, context-system |
| .mindspec/docs/adr/ADR-0008.md | ADR-0008 | Accepted ADR for domains: workflow, tracking |
| .mindspec/docs/adr/ADR-0010.md | ADR-0010 | Accepted ADR for domains: viz, workflow, observability |
| .mindspec/docs/adr/ADR-0012.md | ADR-0012 | Accepted ADR for domains: bead, workflow, core |
| .mindspec/docs/adr/ADR-0013.md | ADR-0013 | Accepted ADR for domains: bead, workflow, core |
| .mindspec/docs/adr/ADR-0014.md | ADR-0014 | Accepted ADR for domains: core, context-system, workflow |
| .mindspec/docs/adr/ADR-0015.md | ADR-0015 | Accepted ADR for domains: workflow |
| .mindspec/docs/adr/ADR-0016.md | ADR-0016 | Accepted ADR for domains: workflow, beads |
| .mindspec/policies.yml | Policies | Policies applicable to mode "plan" |
