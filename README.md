# MindSpec

**See what your AI agent is thinking. Then structure how it works.**

MindSpec is a CLI framework with two capabilities: **AgentMind**, a real-time 3D visualization of AI agent activity, and a **spec-driven development workflow** that structures how agents move from idea to implementation.

## AgentMind — Real-Time Agent Visualization

AgentMind renders your AI agent's activity as an interactive 3D force-directed graph. Agents, tools, MCP servers, and data sources appear as nodes; calls between them appear as edges — all updating in real time.

<!-- TODO: Add screenshot or GIF -->

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

**Full guide:** [docs/guides/agentmind.md](docs/guides/agentmind.md)

---

## Spec-Driven Development

AI coding agents are powerful but unstructured. Without guardrails they drift from intent, ignore existing architecture, lose context between sessions, skip documentation, and resist scope discipline.

MindSpec treats these as system design problems, not prompting problems. It enforces a gated lifecycle where each transition requires explicit human approval:

```
Idle ──→ Spec Mode ──human gate──→ Plan Mode ──human gate──→ Implementation ──→ Review ──human gate──→ Idle
```

**Spec Mode** — Define what "done" looks like. Problem statement, acceptance criteria, impacted domains, ADR touchpoints. No code allowed.

**Plan Mode** — Decompose the spec into bounded work chunks. Review applicable ADRs. Check architectural fitness. Wire up dependencies.

**Implementation Mode** — Execute in an isolated git worktree. One bead per worktree, scoped to exactly what the plan defined. Doc-sync is mandatory.

**Review Mode** — Validate against the original spec's acceptance criteria. Human approves to return to idle.

### Getting Started

| Goal | Guide |
|:-----|:------|
| **Just visualize agent activity** | [AgentMind guide](docs/guides/agentmind.md) |
| **Full workflow with Claude Code** | [Claude Code guide](docs/guides/claude-code.md) |
| **Full workflow with Codex** | [Codex guide](docs/guides/codex.md) |
| **Complete reference** | [USAGE.md](docs/core/USAGE.md) |

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

### AgentMind

| Command | Description |
|:--------|:------------|
| `mindspec agentmind serve` | Start OTLP receiver + web UI for real-time visualization |
| `mindspec agentmind replay <file>` | Replay a recorded NDJSON session |

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
| `mindspec init` | Bootstrap project structure |
| `mindspec spec-init <id>` | Create new specification |
| `mindspec doctor` | Project health checks |

### Observability

| Command | Description |
|:--------|:------------|
| `mindspec trace summary <file>` | Summarize NDJSON trace events |
| `mindspec bench setup\|collect\|report` | Benchmark agent sessions |

## Project Structure

```
your-project/
├── .mindspec/
│   └── state.json              # Current mode, active spec/bead (committed)
├── .beads/                     # Beads work graph (committed)
├── docs/
│   ├── core/                   # Architecture, modes, conventions, usage
│   ├── domains/<name>/         # Domain-scoped documentation
│   ├── adr/                    # Cross-cutting architecture decisions
│   ├── specs/<id>/             # Specifications with plans and context packs
│   ├── guides/                 # Quick start guides
│   ├── context-map.md          # Bounded context relationships
│   └── templates/              # Templates for specs, plans, ADRs
├── architecture/
│   └── policies.yml            # Machine-checkable architectural policies
├── GLOSSARY.md                 # Term → doc section mapping
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
- Claude Code or Codex (for agent integration; MindSpec is CLI-first and works standalone)

## Building

```bash
make build      # Build to ./bin/mindspec
make test       # Run all tests
make install    # Install to $GOPATH/bin
```

## License

MIT
