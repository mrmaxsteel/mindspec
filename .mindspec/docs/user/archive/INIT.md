> **ARCHIVED**: This document is superseded by [mindspec-v1-spec.md](mindspec-v1-spec.md), [ADR-0001](../adr/ADR-0001.md), and [ADR-0002](../adr/ADR-0002.md). Retained for historical context only.

# Antigravity Spec-Driven Development + Memory + Self-Documentation System (Workspace-Aware)

## 1) Goals

### Primary goals

1. **Spec-driven development inside the IDE**

* Convert intent → structured spec → task graph → guided execution → validation gates.

2. **Self-documentation as a first-class outcome**

* Every feature/change updates/refactors docs so future work can automatically pull the right architectural context.
* Docs are treated as a living contract, not an afterthought.

3. **Explicit human-in-the-loop for architecture divergence**

* If implementation deviates from documented architecture, the agent must raise a proposal artifact and stop for confirmation.

4. **Deterministic, budget-conscious context**

* Prefer deterministic retrieval (keyword → anchored doc section) with explicit provenance and bounded token budgets.

5. **Durable “project brain” memory**

* Persist decisions, gotchas, debugging outcomes, and rationale across sessions in a local store with queryable recall.

6. **Multi-repo workspace support**

* Support workspaces containing multiple independent repos (e.g., frontend + backend), with shared specs/docs feeding context to each.

### Non-goals (initially)

* Fully autonomous end-to-end delivery without human review.
* Replacing semantic RAG for huge corpora (hybrid/fallback can come later).
* Depending on IDE-level prompt interception hooks.

---

## 2) Inspirations and what we’re borrowing

### `sighup/claude-workflow`

* **Spec → plan → execute → validate** discipline, with proofs and gates defining “done”.
* Task metadata as a contract (scope, requirements, proofs, checks).
* Dependency-aware parallel dispatch.
  Reference: [https://github.com/sighup/claude-workflow](https://github.com/sighup/claude-workflow)

### `johnpsasser/memex`

* Keyword-driven **section-level** doc injection (progressive disclosure).
* Budget enforcement + session dedupe cache.
* Repo-portable documentation “memory surface” + telemetry mindset.
  Reference: [https://github.com/johnpsasser/memex](https://github.com/johnpsasser/memex)

### `hjertefolger/cortex`

* Local persistent memory DB (SQLite) and pragmatic recall patterns (keyword + optional vector/hybrid).
* Durable memory across sessions without external infra.
  Reference: [https://github.com/hjertefolger/cortex](https://github.com/hjertefolger/cortex)

---

## 3) System overview

### Architectural shape

A **workspace-aware workflow and memory layer**, exposed to Antigravity as a set of tools (CLI and/or MCP server). The system reads/writes:

* **versioned docs/specs/proofs** in one or more repos
* a local **persistent memory store** (SQLite)
* **policy/gate** configuration
* optional **telemetry**

```text
┌──────────────────────────────────────────┐
│ Antigravity IDE Workspace                │
│  - Repo A (frontend)                     │
│  - Repo B (backend)                      │
│  - Repo C (knowledge/specs) (optional)   │
└───────────────────┬──────────────────────┘
                    │ tool calls (CLI/MCP)
┌───────────────────▼─────────────────────────────┐
│ Workflow+Memory Service (workspace-aware)        │
│  - Workspace registry + repo resolution          │
│  - Context packs (docs + memory + policies)      │
│  - Spec → task graph (multi-repo tasks)          │
│  - Execution protocol + proof runner             │
│  - Validation gates + coverage matrix            │
│  - Architecture divergence detector + ACP writer │
│  - Persistent memory DB + recall                 │
│  - Session cache + budgets + (optional) OTEL     │
└──────────────┬───────────────────┬──────────────┘
               │                   │
     versioned repo writes         │ local durable state
┌──────────────▼──────────────┐   ┌▼──────────────────────────┐
│ docs/specs/proofs/glossary   │   │ ~/.toolname/              │
│ policies + proposals         │   │  memory.db, cache, logs   │
└─────────────────────────────┘   └───────────────────────────┘
```

---

## 4) Workspace and repo-structure agnosticism

### Core principle

**Tooling is workspace-native, not repo-structure-native.**
It should work whether repos are:

* separate folders,
* nested folders,
* git submodules,
* or checked out by CI.

### Workspace abstraction

A workspace is a set of **repo targets**:

* `repo_alias` (e.g., `frontend`, `backend`, `knowledge`)
* `root_path`
* optional `role` (frontend/backend/shared) for default rules
* `vcs_adapter` (Git initially)
* `docs_scope` (which docs are injectable into which repos)

### Workspace providers

Repo discovery/version coupling is encapsulated behind a provider interface:

* **Meta-repo + submodules provider** (reads `.gitmodules`, resolves submodule roots + SHAs)
* **Config-only provider** (reads `workspace.yml` for repo roots)
* **CI provider** (reads environment variables/paths)

This keeps the core workflow logic agnostic, while allowing meta-repo/submodules to give you deterministic version coupling.

### Commit tuple in every run

Every Context Pack and validation report records a **commit tuple**:

* `repo_alias → commit SHA`
* injected doc sections (file + anchor)
* memory fragments used (IDs)

This makes executions reproducible and reviewable across machines and time.

---

## 5) Repo contract (what lives in Git)

### Shared documentation taxonomy (injectable)

Recommended minimal structure (whether in a dedicated knowledge repo or distributed):

* `docs/core/ARCHITECTURE.md` (north star + invariants)
* `docs/core/DATABASE.md` (schema + migration rules)
* `docs/core/API_CONVENTIONS.md`
* `docs/core/FRONTEND_PATTERNS.md`
* `docs/features/<feature>.md` (feature invariants/behavior)
* `docs/architecture/proposals/ACP-xxxx.md` (human gate for divergence)

### Glossary mapping (Memex-style)

* `GLOSSARY.md`: keyword/synonym → doc#anchor mappings

  * Example: `database schema -> docs/core/DATABASE.md#schema`
  * Example: `plan generation -> docs/features/coach.md#plan-generation`

### Specs + task graph + proofs (claude-workflow-style)

* `docs/specs/006-refactor-coach/`

  * `spec.md` (structured spec)
  * `tasks.json` (dependency graph; tasks tagged by repo target)
  * `context-pack.md` (sources injected + SHAs)
  * `proofs/` (captured artifacts)
  * `reports/validate.md` (coverage matrix + gate results)

### Architecture invariants (machine-checkable)

* `architecture/policies.yml` (or `.json`)

  * layering rules, forbidden deps, required doc updates for certain change types, etc.

---

## 6) Smart features (with Antigravity implementation notes)

### Feature A: Context Pack generation (docs + memory + policies)

**What it does**

* Given `(spec_id, prompt, target_repo)`:

  1. match glossary keywords
  2. extract anchored doc sections (section-level)
  3. recall relevant memory snippets
  4. attach relevant policies/invariants
  5. output a Context Pack with provenance + commit tuple

**Antigravity notes**

* Enforce via `AGENTS.md`: before planning/executing, call `context.pack(...)` and base the plan on it.

---

### Feature B: Spec → multi-repo task graph

**What it does**

* Converts a spec into tasks with:

  * `target_repo` (repo alias)
  * `scope` (paths relative to repo root)
  * dependencies (can cross repos)
  * `proofs` (commands per repo)
  * `doc_delta` (docs/glossary updates required)

**Antigravity notes**

* In Plan mode: generate tasks, then review the graph rather than a long narrative plan.

---

### Feature C: Execution protocol (proof-first, doc-inclusive)

**What it does**

* Executes exactly one task with consistent phases:

  * baseline checks
  * implement within scope
  * update/refactor docs (required)
  * run proofs + capture artifacts
  * run gates
  * produce completion report linked to proofs/docs

**Antigravity notes**

* Tool `task.execute(task_id)` returns an ordered checklist and refuses completion until doc/proof gates pass.

---

### Feature D: Doc Sync + Doc Refactoring (self-documentation loop)

**What it does**

* Ensures docs stay aligned and injectable:

  * require doc updates for certain changes
  * refactor docs when sections become too large
  * suggest glossary additions for new concepts
  * keep anchors stable and consistent

**Antigravity notes**

* Tooling should provide `docs.sync(task_id)` and `docs.lint()`; validation fails if doc sync expectations aren’t met.

---

### Feature E: Architecture divergence detection + ACP gate

**What it does**

* Detects violations or proposed deviations from invariants.
* Writes an ACP (Architecture Change Proposal) and halts until explicit approval.

**ACP includes**

* change summary, motivation, options, proposed approach, impact, required doc updates, proof plan, explicit approval question.

**Antigravity notes**

* `arch.check()` runs before “finalize”.
* If divergence: `arch.propose_acp()` writes `ACP-xxxx.md` and blocks downstream tasks.

---

### Feature F: Persistent memory (workspace-scoped)

**What it does**

* Stores durable “project brain” fragments:

  * decisions, gotchas, debugging outcomes, rationale
  * links to commits/specs/doc anchors

**Recall**

* Hybrid retrieval: keyword/FTS + optional embeddings + recency weighting.

**Antigravity notes**

* `memory.save()` runs at task completion; `memory.recall()` feeds into Context Pack.

---

### Feature G: Dispatch (parallelism across repos)

**Two modes**

1. **Assisted dispatch (works now)**

* selects ready tasks (no deps, no scope conflicts)
* generates per-task packets (context + instructions + proofs)
* you paste into multiple Antigravity agent tabs

2. **True dispatch (future)**

* if Antigravity exposes session/job APIs, spawn agents programmatically

**Antigravity notes**

* Start with Assisted dispatch; it removes the briefing overhead and stays structure-agnostic.

---

### Feature H: Validation + coverage matrix (workspace-aware)

**What it does**

* Runs gates and proofs per repo + cross-repo checks.
* Produces a matrix:

  * spec requirement → tasks → proofs → docs updated → pass/fail

**Antigravity notes**

* Require `validate.run(spec_id)` prior to declaring the spec complete.

---

### Feature I: Observability (optional OTEL)

**What it measures**

* glossary hit/miss, injected tokens, cache hit rate, proof pass/fail, gate failures, task cycle time.

---

## 7) Distribution mechanisms

### Primary (MVP): CLI/MCP service + repo templates

Deliver as a toolkit repo (template) containing:

* local service (Node/Python) implementing tools
* install/doctor scripts
* templates (spec, tasks schema, ACP)
* docs taxonomy + example glossary + policies starter

Adoption:

* works with any repo layout by configuring a workspace provider (`workspace.yml` or meta-repo auto-detect).

### UX upgrade (later): Thin VS Code extension + local service

Extension provides UI:

* specs/tasks sidebar
* context pack provenance viewer
* ACP approval workflow
* validation results panel
  Core logic remains in the local service (keeps the extension thin and less brittle).

---

## 8) Recommended workspace coupling strategy (meta-repo + submodules)

Decision: **meta-repo + submodules** for frontend/backend/knowledge pinning.

Important: the tooling does **not** require submodules; it simply benefits from the provider:

* resolves repo roots and SHAs deterministically
* enables reproducible Context Packs and coherent feature branches
* supports safe cross-repo dispatch

---

## 9) Antigravity “rules of engagement” (minimal)

In each repo’s `AGENTS.md` (or workspace-level policy):

* Always call `context.pack(...)` before plan/execute.
* Every task must have Doc Delta; “done” includes doc sync.
* If `arch.check` flags divergence, create ACP and stop for explicit confirmation.
* No completion without proofs and `validate.run(spec_id)`.
