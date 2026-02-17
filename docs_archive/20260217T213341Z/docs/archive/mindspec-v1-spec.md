# MindSpec v1 — High-Level Product Spec

## Purpose

MindSpec is a **spec-driven development + context management framework** (Claude Code first) that uses **Beads** as its durable work graph and uses **git worktrees** to safely run agent execution in parallel. It standardizes how teams move from *user value* → *validated specification* → *bounded plans* → *implemented change*, while keeping architecture and documentation coherent over time.

## Goals (v1)

1. **Roadmap management** that is executable and auditable (not a slide deck).
2. **Spec + context management** that is concise, deterministic, and portable with the repo.
3. **Architecture decisions lifecycle** as a first-class primitive (ADR-aware, divergence-gated).
4. **DDD-aware documentation structure** where domains are explicit and govern where specs/ADRs/docs live.
5. **Explicit Context Map** that documents boundaries, integrations, and source-of-truth ownership.
6. **Claude Code-first integration** that feels native and minimizes operator friction.
7. **Worktree-first execution model** so work can be isolated, parallelized, and reviewed cleanly.
8. **CLI-first, minimal IDE glue** — enabling MindSpec in an agentic coding IDE should require minimal static markdown/skills that need to be maintained. Two facets:
   - **Dynamic over static**: agent instructions are emitted at runtime by `mindspec instruct` based on current state (mode, active bead, worktree), not maintained as static workspace files that try to anticipate every scenario.
   - **Logic in CLI, not skills**: workflow operations (`approve`, `next`, `complete`) are CLI commands (testable, versionable Go code). IDE skills/commands are thin shims that call the CLI.

## Non-Goals (v1)

* Full multi-agent orchestration (parallel sub-agents) as an automated feature. (v1 lays plumbing + conventions; orchestration comes later.)
* Vector RAG / embeddings infrastructure. (MindSpec focuses on deterministic context packs.)

---

## Core Dependencies

### Beads (first-class)

Beads is the canonical system of record for:

* roadmap hierarchy (epics/specs → plans → tasks)
* status/progress
* durable “what/why/how we validated” memory

MindSpec treats Beads objects as **primary artifacts**, not “optional issue tracking”.

### Git worktrees (first-class)

All agent execution (especially implementation) should run in **isolated worktrees** to:

* prevent unreviewed changes in the main working tree
* enable safe parallel execution later
* keep context + diffs clean per bead/task

---

## Primary User Outcomes

* A user can ask for a feature and MindSpec reliably produces:

  1. a spec framed around **user value + validation**
  2. a bounded, chunked plan with explicit **verification**
  3. implementation work that is isolated, reviewable, and traceable back to the spec
* Architecture stays coherent: divergence from ADRs is detected and handled via explicit user approval + new ADR superseding old ones.
* Any mode (spec/plan/implement) gets **exact, concise, relevant context** without manual “go read X” prompting.

---

## Key Concepts (v1 primitives)

### 1) Modes

MindSpec operates in explicit modes. Mode controls *allowed outputs*, *required context*, and *gates*.

#### Spec Mode

**Objective:** discuss user-facing value and how to validate it.
**Output:** a **Spec Bead** (parent bead) containing:

* problem statement + target user outcome
* acceptance criteria and validation plan (manual + automated where applicable)
* non-goals / constraints
* impacted domains (see Domain primitive)
* required architecture touchpoints (ADRs/docs to follow)
* “open questions” that must be resolved before planning

Spec Mode is intentionally **implementation-light**: no deep design unless necessary to define what “done” means.

#### Plan Mode

**Objective:** turn a spec bead into bounded work chunks.
**Output:** child beads (Implementation beads) each with:

* small scope (“one slice of value”)
* 3–7 step micro-plan
* explicit verification steps
* dependencies between beads

Plan Mode must review:

* applicable ADRs and domain docs
* existing constraints
* whether the architecture is fit for purpose (see ADR lifecycle)

#### Implementation Mode

**Objective:** execute one implementation bead in an isolated worktree.
**Output:** code changes + evidence + bead updates:

* proof (commands, screenshots, test outputs)
* documentation updates/refactors
* status progression and closure notes in Beads

Implementation Mode is not allowed to widen scope: discovered work becomes new beads and dependencies.

---

### 2) Roadmap as Beads hierarchy

MindSpec’s roadmap is expressed as Beads structure:

* Roadmap (release / milestone) → Spec beads → Implementation beads → (optional) sub-tasks

This makes the roadmap:

* queryable (“what’s ready?”)
* dependency-aware (“what’s blocked?”)
* durable across sessions

---

### 3) Architecture Decision Records (ADR) lifecycle

MindSpec maintains ADRs as a governed system:

**Rules:**

* Plans must cite the ADRs they rely on.
* If a plan/implementation detects that an ADR blocks progress or is unfit:

  * the agent must stop and inform the user
  * present a divergence option set (continue-as-is vs propose change)
  * if user accepts divergence, MindSpec creates a **new ADR** that **supersedes** prior ADR(s)

**ADR metadata (v1)**

* domain(s)
* status: proposed / accepted / superseded
* supersedes / superseded-by links
* decision + rationale + consequences
* validation / rollout notes when relevant

---

### 4) Domains as first-class primitives (DDD-inspired)

MindSpec treats “domain” as an explicit entity that governs:

* where docs/specs live
* how context packs are assembled
* which ADRs are relevant by default

**Domain operations are human-in-the-loop:**

* `add-domain`
* `split-domain`
* `merge-domain` (optional)

These operations produce ADRs (or a “Domain Decision Record”) because they change the mental model of the codebase.

**Doc structure (v1)**

* `/docs/domains/<domain>/`

  * `overview.md` (what it owns, boundaries)
  * `architecture.md` (key patterns, invariants)
  * `interfaces.md` (APIs/events/contracts)
  * `runbook.md` (ops/dev workflows)
  * `adr/ADR-xxxx.md`

---

### 5) Context Map (integration blueprint)

MindSpec maintains a lightweight **Context Map** to make boundaries, ownership, and integrations explicit.

**Location (v1):** `/docs/context-map.md`

**What it captures (v1):**

* the major MindSpec contexts (e.g., Roadmap/Work Graph, Spec, Plan, Implement, Knowledge/Governance, Context Delivery, Telemetry)
* upstream/downstream relationships between contexts (who produces truth vs who consumes it)
* integration contracts and translation points (e.g., published language/event schemas, adapters/ACLs around Beads/Git/agent tooling)
* source-of-truth notes per concept (e.g., where status, validation evidence, ADR state, and domain boundaries are authoritative)

**Maintenance rule (v1):** any change that introduces a new context, changes ownership, or adds a new integration contract must update the Context Map in the same bead.

## Context System (the “hero feature” for v1)

MindSpec’s most important capability is **efficient, deterministic context delivery** per mode.

### Context Packs

A Context Pack is a concise bundle constructed from:

* the relevant bead(s): spec bead + implementation bead
* domain docs
* ADRs (accepted + not superseded)
* any explicitly referenced files/sections
* optional cached summaries (generated and stored as docs)

**Properties**

* mode-specific (Spec vs Plan vs Implement)
* budgeted (hard token budget)
* deduped (don’t re-inject the same sections repeatedly)
* provenance-preserving (“these exact sections were loaded, because of X”)

**Mechanism (v1 direction)**

* glossary/registry mapping from concepts → doc sections (Memex-like)
* section-level extraction, not full-file dumps
* session dedupe cache
* explicit provenance recorded back to the bead (what was loaded)

---

## Claude Code-first integration (v1)

MindSpec should feel like a Claude Code-native workflow:

* conventions + commands that Claude Code can follow consistently
* artifacts stored in-repo alongside code
* Beads + worktrees as the execution substrate

(Implementation specifics can come later, but v1 should be designed so Claude Code can operate it with minimal glue.)

---

## Worktree-first execution model (v1)

Implementation Mode executes in a worktree tied to the bead:

* worktree naming convention includes bead id
* changes isolated per bead
* closing a bead requires evidence + doc updates + clean state sync

This is also the enabling foundation for “parallel sub-agents later”.

---

## Human-in-the-loop gates (v1)

MindSpec requires explicit confirmation for:

* architecture divergence / new ADR creation
* domain add/split/merge
* scope expansions that change the user value definition
* acceptance of “cannot be verified automatically” items

---

## Observability plumbing for future “Agent Mind Visualization”

v1 should emit structured events (even if minimal) so that later we can build the 3D mind map:

* nodes: agents, tools, MCP servers, domains, beads, docs, ADRs
* edges: tool calls, context pack injections, bead transitions, verification runs
* metrics: tokens injected, cache hits, latency/errors, budget utilization

(OTel-friendly event shaping is a good direction, but v1 can start with a simple event stream that can later be normalized into spans.)

---

## v1 Deliverables (high-level)

* Beads-based roadmap + spec/task primitives and conventions
* Three-mode workflow semantics (Spec/Plan/Implement)
* Context Map (`/docs/context-map.md`) defining major contexts, integrations, and source-of-truth ownership
* ADR lifecycle + superseding workflow with explicit user gate
* Domain-first docs/ADR structure and domain change governance
* Deterministic, budgeted Context Packs (mode-specific) with provenance
* Worktree execution conventions for implementation beads
* Minimal event model to support future visualization plumbing
