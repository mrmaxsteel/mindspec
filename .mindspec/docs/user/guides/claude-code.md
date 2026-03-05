# MindSpec + Claude Code

A guide to adopting MindSpec as your full spec-driven development workflow in Claude Code.

## Prerequisites

- Go 1.22+
- [Beads](https://github.com/steveyegge/beads) CLI (`bd`)
- Git
- Claude Code

## Setup

### 1. Build MindSpec

```bash
make build && make install
```

### 2. Bootstrap Your Project

```bash
mindspec init
```

This scaffolds the full directory structure: `.mindspec/`, `docs/` (core, domains, specs, templates), `GLOSSARY.md`, `CLAUDE.md`, `.claude/` hooks, and more. All creation is additive — existing files are never overwritten.

### 3. Verify

```bash
mindspec doctor
```

Should report zero errors.

## The Workflow

MindSpec enforces a gated lifecycle. Every phase transition requires explicit human approval.

```
Idle ──→ Spec Mode ──human gate──→ Plan Mode ──human gate──→ Implementation ──→ Review ──human gate──→ Idle
```

### Your First Feature

**1. Start a specification**

Use the `/ms-spec-create` custom command (or `mindspec spec create 001-my-feature`). This creates `docs/specs/001-my-feature/spec.md` from a template and sets the workflow state to Spec Mode.

**2. Draft the spec collaboratively**

You and the agent fill in the spec: goal, requirements, acceptance criteria, impacted domains, ADR touchpoints, open questions. Only markdown artifacts — no code. Run `mindspec validate spec 001-my-feature` to check quality.

**3. Approve the spec**

Type `/ms-spec-approve`. This validates the spec, updates its frontmatter to `APPROVED`, closes the spec-approve molecule step, generates a context pack, and transitions to Plan Mode.

**4. Draft the plan**

The agent reviews domain docs and ADRs, then creates `docs/specs/001-my-feature/plan.md` — decomposing the spec into bounded work chunks with dependencies and verification steps.

**5. Approve the plan**

Type `/ms-plan-approve`. This validates the plan, updates frontmatter, closes the plan-approve molecule step (unblocking the implement step), and transitions toward Implementation Mode.

**6. Claim work**

```bash
mindspec next
```

This claims the first ready bead, creates an isolated git worktree, and sets the state to Implementation Mode.

**7. Implement**

The agent writes code within the bead's declared scope, creates tests, and updates documentation. Doc-sync is mandatory — "done" includes documentation.

**8. Complete the bead**

```bash
mindspec complete
```

This closes the bead, removes the worktree, and advances state. If more beads are ready, run `mindspec next` again. When all beads are done, the state transitions to Review Mode.

**9. Approve the implementation**

Type `/ms-impl-approve`. This verifies the work against acceptance criteria and returns to Idle.

## Skills

| Skill | What It Does |
|:------|:-------------|
| `/ms-explore` | Enter, promote, or dismiss an Explore Mode session |
| `/ms-spec-create` | Create a new specification (enters Spec Mode) |
| `/ms-spec-approve` | Approve spec → Plan Mode |
| `/ms-plan-approve` | Approve plan → Implementation Mode |
| `/ms-impl-approve` | Approve implementation → Idle |
| `/ms-spec-status` | Check current mode and active spec/bead state |

## How Guidance Works

The SessionStart hook runs `mindspec instruct` automatically at the start of every conversation. This emits mode-appropriate guidance based on current state — the agent knows what phase it's in, what spec is active, what bead it's working on, and what it should do next.

Every state-changing command (`approve`, `next`, `complete`) also emits fresh guidance as its tail output. The agent never lacks context about its operating mode.

No need to maintain sprawling static instruction files. The CLI is the source of truth.

## Observability

Use [AgentMind](agentmind.md) to visualize agent activity in real time:

```bash
mindspec agentmind serve    # Start the visualization server
```

Then configure Claude Code's OTLP export to point to `http://localhost:4318`.

## Reference

- [USAGE.md](../core/USAGE.md) — Full happy-path walkthrough
- [MODES.md](../core/MODES.md) — Detailed mode definitions and transitions
- [CONVENTIONS.md](../core/CONVENTIONS.md) — File layout and naming conventions
