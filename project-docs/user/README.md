# MindSpec

**Spec-driven development and real-time observability for AI coding agents.**

AI coding agents are powerful but unstructured. Without guardrails they:

- **Drift from intent** — the agent builds what it infers, not what you specified
- **Ignore architecture** — existing design decisions and ADRs get steamrolled
- **Lose context between sessions** — every conversation starts from scratch
- **Skip documentation** — code ships, docs rot
- **Resist scope discipline** — a "small feature" becomes a refactor of three subsystems

MindSpec treats these as system design problems, not prompting problems. It provides a **gated development lifecycle** where architecture divergence is detected and blocked until explicitly resolved, **bounded contexts** borrowed from domain-driven design to manage what the agent sees — deterministic, token-budgeted context packs assembled from domain docs, ADRs, and the Context Map so the agent gets exactly the right context without manual prompt engineering — and **OTLP telemetry export** that pairs with [AgentMind](https://github.com/mrmaxsteel/agentmind), a standalone observability dashboard that shows you exactly what your agent is doing, spending, and how efficiently it's working.

## The Workflow

Every phase transition requires explicit human approval:

```
Idle ──→ Spec Mode ──human gate──→ Plan Mode ──human gate──→ Implementation ──→ Review ──human gate──→ Idle
```

**Spec Mode** — Define what "done" looks like. Problem statement, acceptance criteria, impacted domains, ADR touchpoints. No code allowed.

**Plan Mode** — Decompose the spec into bounded work chunks. Review applicable ADRs. Check architectural fitness. If implementation needs to deviate from a cited ADR, the agent stops and escalates — you approve a superseding ADR or reject the divergence.

**Implementation Mode** — Execute in an isolated git worktree. One bead per worktree, scoped to exactly what the plan defined. Doc-sync is mandatory. Discovered work becomes new beads, not scope creep.

**Review Mode** — Validate against the original spec's acceptance criteria. Human approves to return to idle.

The work graph is tracked by [Beads](https://github.com/steveyegge/beads), a git-native issue tracker that survives across sessions without external services.

Documentation stays current because the system won't let you skip it — beads can't close without doc-sync, architecture decisions are tracked as ADRs that plans must cite, and every spec produces versioned artifacts that persist alongside the code.

---

## AgentMind — AI Agent Observability UI

[AgentMind](https://github.com/mrmaxsteel/agentmind) is a **standalone companion product** (its own repo, install, and docs — extracted from mindspec per ADR-0026/ADR-0027) that gives you real-time visibility into what your agent is doing, what it's spending, and how efficiently it's working: a 3D activity graph, per-model token and cost tracking, tool/MCP analytics, and session recording/replay — all from standard OpenTelemetry data.

MindSpec's side of the integration is OTLP export configuration, nothing more:

```bash
# 1. Install AgentMind from its repo, then start it
agentmind serve
# OTLP receiver on :4318, UI at http://localhost:8420

# 2. Point your agent's telemetry at it
mindspec otel setup --endpoint http://localhost:4318   # Claude Code (default)
mindspec otel setup --endpoint http://localhost:4318 --codex

# 3. Open http://localhost:8420
```

Any OTLP-compatible agent works — point the standard `OTEL_EXPORTER_OTLP_ENDPOINT` at `http://localhost:4318` — and any OTLP/HTTP receiver can stand in for AgentMind.

**Full guide:** [AgentMind guide](guides/agentmind.md)

---

### Getting Started

| Goal | Guide |
|:-----|:------|
| **Full workflow with Claude Code** | [Claude Code guide](guides/claude-code.md) |
| **Full workflow with GitHub Copilot** | [Copilot guide](guides/copilot.md) |
| **Full workflow with Codex** | [Codex guide](guides/codex.md) |
| **Observability (OTLP export + the standalone AgentMind)** | [AgentMind guide](guides/agentmind.md) |
| **Complete reference** | [USAGE.md](../../.mindspec/core/USAGE.md) |

---

## How It Works

### Context Packs

MindSpec assembles deterministic, token-budgeted context for each phase. A context pack pulls from the spec, relevant domain docs, applicable ADRs, glossary terms, neighboring bounded contexts (via the Context Map), and active policies — then deduplicates and respects token budgets.

```bash
mindspec context pack 009-my-feature
```

### Architecture Decision Records

ADRs are a governed primitive. Plans must cite the ADRs they rely on. If implementation needs to deviate from a cited ADR, the agent stops and escalates — you approve a new superseding ADR or reject the divergence.

```bash
mindspec adr create --title "Use WebSockets for real-time updates" --domain viz
mindspec adr list --status accepted
```

### Dynamic Agent Guidance

Instead of maintaining sprawling static instruction files, MindSpec emits agent guidance at runtime based on current state (mode, active spec, active bead, worktree status):

```bash
mindspec instruct
```

### Domain-Driven Design

Bounded contexts reduce ambiguity. Specs declare impacted domains. Context packs route through the Context Map, expanding one hop to include neighboring bounded contexts. Domain-scoped ADRs live alongside domain docs.

---

## CLI Reference

### Observability

| Command | Description |
|:--------|:------------|
| `mindspec otel setup --endpoint <url>` | Write OTLP exporter config for Claude Code, Codex, or env exports |
| `mindspec otel status` | Show the currently configured OTEL endpoint (read-only) |
| `mindspec trace summary <file>` | Summarize NDJSON trace events |

(The old `mindspec agentmind|viz|bench` verbs moved to the standalone [agentmind repo](https://github.com/mrmaxsteel/agentmind); they remain for one release as hidden deprecation stubs that print a pointer and exit 2.)

### Workflow

| Command | Description |
|:--------|:------------|
| `mindspec instruct` | Emit mode-appropriate agent guidance |
| `mindspec state show` | Show current mode and active work |
| `mindspec next` | Claim next ready bead, create worktree |
| `mindspec complete` | Close bead, remove worktree, advance state |
| `mindspec approve spec <id>` | Approve spec, transition to Plan Mode |
| `mindspec approve plan <id>` | Approve plan, transition to Implementation |
| `mindspec approve impl <id>` | Approve implementation, return to Idle |

### Context & Documentation

| Command | Description |
|:--------|:------------|
| `mindspec context pack <id>` | Generate token-budgeted context pack |
| `mindspec glossary list\|match\|show` | Term lookup and section extraction |
| `mindspec adr create\|list\|show` | ADR lifecycle management |
| `mindspec validate spec\|plan\|docs` | Pre-flight validation checks |

### Project Management

| Command | Description |
|:--------|:------------|
| `mindspec init` | Bootstrap project structure and AGENTS.md |
| `mindspec setup claude` | Configure Claude Code integration (hooks, commands, CLAUDE.md) |
| `mindspec setup copilot` | Configure GitHub Copilot integration (instructions, prompt files) |
| `mindspec migrate` | Emit prompt to reorganize existing docs into canonical structure |
| `mindspec spec-init <id>` | Create new specification |
| `mindspec doctor` | Project health checks |

## Project Structure

```
your-project/
├── .mindspec/
│   ├── docs/                   # Canonical docs (core, domains, adr, specs, guides)
│   ├── policies.yml            # Canonical architecture policies
│   └── state.json              # Current mode, active spec/bead (committed)
├── .beads/                     # Beads work graph (committed)
├── docs_archive/               # Migration archive outputs by run-id
├── AGENTS.md                   # Minimal bootstrap (points to CLI)
└── CLAUDE.md                   # Minimal bootstrap (points to CLI)
```

## Design Principles

1. **Docs-first** — every code change updates documentation, enforced by the system
2. **Spec-anchored** — all implementation traces back to a versioned specification
3. **Human gates for divergence** — architecture deviations require approval and a new ADR
4. **Proof of done** — beads close only with verification evidence
5. **Scope discipline** — discovered work becomes new beads, never scope creep
6. **Dynamic over static** — runtime guidance beats static files that drift
7. **CLI-first** — logic lives in testable, versionable Go; IDE integrations are thin shims
8. **Deterministic context** — token-budgeted context packs, not "go read this file" prompting

## Requirements

- Go 1.22+
- [Beads](https://github.com/steveyegge/beads) CLI (`bd`)
- Git (for worktree support)
- Claude Code, GitHub Copilot, or Codex (for agent integration; MindSpec is CLI-first and works standalone)

## Building

```bash
make build      # Build to ./bin/mindspec
make test       # Run all tests
make install    # Install to $GOPATH/bin
```

## License

MIT
