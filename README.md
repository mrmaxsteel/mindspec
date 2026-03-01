# MindSpec

**Spec-driven development and real-time observability for AI coding agents.**

AI coding agents are powerful but unstructured. Without guardrails they:

- **Drift from intent** — the agent builds what it infers, not what you specified
- **Ignore architecture** — existing design decisions and ADRs get steamrolled
- **Lose context between sessions** — every conversation starts from scratch
- **Skip documentation** — code ships, docs rot
- **Resist scope discipline** — a "small feature" becomes a refactor of three subsystems

MindSpec treats these as system design problems, not prompting problems. It provides a **gated development lifecycle** where architecture divergence is detected and blocked until explicitly resolved, **bounded contexts** borrowed from domain-driven design to manage what the agent sees — deterministic, token-budgeted context packs assembled from domain docs, ADRs, and the Context Map so the agent gets exactly the right context without manual prompt engineering — and an **observability layer** (AgentMind) that shows you exactly what your agent is doing, spending, and how efficiently it's working.

<p align="center">
  <img src="agentmind.png" alt="AgentMind — AI Agent Observability UI" width="800" />
  <br />
  <em>AgentMind — real-time observability for AI coding agents</em>
</p>

## The Workflow

Every phase transition requires explicit human approval:

```
             ┌─ dismiss ─→ Idle
Idle ──→ [Explore Mode]
             └─ promote ─→ Spec Mode ──gate──→ Plan Mode ──gate──→ Implementation ──→ Review ──gate──→ Idle
```

**Explore Mode** (optional) — Evaluate whether an idea is worth pursuing. The agent clarifies the problem, checks prior art, assesses feasibility, and recommends whether to proceed or dismiss. No specs or code — just structured conversation.

**Spec Mode** — Define what "done" looks like. Problem statement, acceptance criteria, impacted domains, ADR touchpoints. No code allowed.

**Plan Mode** — Decompose the spec into bounded work chunks. Review applicable ADRs. Check architectural fitness. If implementation needs to deviate from a cited ADR, the agent stops and escalates — you approve a superseding ADR or reject the divergence.

**Implementation Mode** — Execute in an isolated git worktree. One bead per worktree, scoped to exactly what the plan defined. Doc-sync is mandatory. Discovered work becomes new beads, not scope creep.

**Review Mode** — Validate against the original spec's acceptance criteria. Human approves to return to idle.

The work graph is tracked by [Beads](https://github.com/steveyegge/beads), a git-native issue tracker that survives across sessions without external services.

Documentation stays current because the system won't let you skip it — beads can't close without doc-sync, architecture decisions are tracked as ADRs that plans must cite, and every spec produces versioned artifacts that persist alongside the code.

---

## Quickstart

```bash
# 1. Install (download from GitHub Releases)
# https://github.com/mrmaxsteel/mindspec/releases
# or build from source: make build && cp ./bin/mindspec /usr/local/bin/

# 2. Bootstrap your project
cd your-project
mindspec init
mindspec setup claude   # Configure Claude Code hooks + slash commands
```

`mindspec init` scaffolds the `.mindspec/` directory, `GLOSSARY.md`, `AGENTS.md`, and the project structure. `mindspec setup claude` adds Claude Code-specific integration (SessionStart hook, plan gates, `/ms:spec-init` and other skills). From here, your coding agent picks up the workflow automatically — the SessionStart hook runs `mindspec instruct` and the agent knows what to do.

Tell the agent what you want to build. It will walk you through the lifecycle:

1. **Explore** — "I have an idea about X" (agent evaluates feasibility, you decide go/no-go)
2. **Spec** — Agent drafts the spec, you approve with `/ms:spec-approve`
3. **Plan** — Agent decomposes into work chunks, you approve with `/ms:plan-approve`
4. **Implement** — Agent codes in isolated worktrees, scoped to the plan
5. **Review** — Agent verifies acceptance criteria, you approve with `/ms:impl-approve`

### Guides

| Goal | Guide |
|:-----|:------|
| **Full workflow with Claude Code** | [Claude Code guide](.mindspec/docs/guides/claude-code.md) |
| **Full workflow with Codex** | [Codex guide](.mindspec/docs/guides/codex.md) |
| **Visualize & benchmark agent activity** | [AgentMind guide](.mindspec/docs/guides/agentmind.md) |
| **Complete reference** | [USAGE.md](.mindspec/docs/core/USAGE.md) |

---

## AgentMind — AI Agent Observability UI

AgentMind gives you real-time visibility into what your agent is doing, what it's spending, and how efficiently it's working.

- **3D Activity Graph** — Agents, tools, MCP servers, and LLM endpoints rendered as an interactive force-directed constellation, updating live
- **Token & Cost Tracking** — Input tokens, output tokens, cache reads, cache creation tokens, and estimated USD cost — broken down per model
- **Tool & MCP Analytics** — Every tool call and MCP server interaction counted and categorized, with frequency histograms
- **Model Statistics** — Per-model breakdown of API calls, token usage, and cost across multi-model sessions
- **Session Recording & Replay** — Capture full sessions as NDJSON, replay at any speed, filter by lifecycle phase
- **Benchmarking** — Compare agentic workflows side-by-side with automated A/B/C testing, delta reporting, and qualitative analysis

### Quick Start

```bash
# 1. Build MindSpec
make build

# 2. Start AgentMind
./bin/mindspec agentmind serve
# OTLP receiver on :4318, UI at http://localhost:8420

# 3. Configure Claude Code
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json

# 4. Open http://localhost:8420
```

Any OTLP-compatible agent works — point the standard `OTEL_EXPORTER_OTLP_ENDPOINT` to `http://localhost:4318`.

**Full guide:** [.mindspec/docs/guides/agentmind.md](.mindspec/docs/guides/agentmind.md)

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

### AgentMind & Observability

| Command | Description |
|:--------|:------------|
| `mindspec agentmind serve` | Start OTLP receiver + web UI (tokens, cost, tool analytics, 3D graph) |
| `mindspec agentmind replay <file>` | Replay a recorded NDJSON session at any speed |
| `mindspec bench setup\|collect\|report` | A/B/C benchmark agent workflows with comparative reporting |
| `mindspec trace summary <file>` | Summarize NDJSON trace events |

### Workflow

| Command | Description |
|:--------|:------------|
| `mindspec explore [description]` | Enter Explore Mode to evaluate an idea |
| `mindspec explore promote <id>` | Promote exploration to a spec |
| `mindspec explore dismiss [--adr]` | Exit exploration (optionally record as ADR) |
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
├── .claude/                    # Claude Code config (created by mindspec setup claude)
│   ├── settings.json           # Hooks (SessionStart, PreToolUse plan gates)
│   └── skills/                 # Skills (/ms:spec-init, /ms:spec-approve, etc.)
├── AGENTS.md                   # Cross-agent workflow conventions (read by all agents)
└── CLAUDE.md                   # Claude Code-specific config (points to AGENTS.md)
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
- Claude Code or Codex (for agent integration; MindSpec is CLI-first and works standalone)

## Building

```bash
make build      # Build to ./bin/mindspec
make test       # Run all tests
make install    # Install to $GOPATH/bin
```

## License

MIT
