# MindSpec + Codex

A guide to using MindSpec's spec-driven development workflow with OpenAI Codex CLI.

## How It Works

MindSpec's core is a standalone Go CLI. All workflow logic lives in CLI commands, not IDE-specific hooks. Codex can use MindSpec by calling the CLI directly.

## Prerequisites

- Go 1.22+
- [Beads](https://github.com/steveyegge/beads) CLI (`bd`)
- Git
- Codex CLI

## Setup

### 1. Build MindSpec

```bash
make build && make install
```

### 2. Bootstrap Your Project

```bash
mindspec init
```

### 3. Configure Codex

`mindspec init` creates an `AGENTS.md` file. Add the following instruction to it:

```markdown
On session start, run: mindspec instruct
For workflow commands, use the mindspec CLI directly.
```

Unlike Claude Code, Codex doesn't have SessionStart hooks. The agent reads `AGENTS.md` and follows the instruction to call `mindspec instruct` on each session.

### 4. Verify

```bash
mindspec doctor
```

### 5. Optional: Enable AgentMind Observability

[AgentMind](agentmind.md) is a standalone companion product installed from [its own repo](https://github.com/mrmaxsteel/agentmind). Start it, then configure Codex's OTEL export:

```bash
agentmind serve                                                # the standalone binary, not a mindspec verb
mindspec otel setup --endpoint http://localhost:4318 --codex
```

This configures `~/.codex/config.toml` to use OTLP/HTTP at `http://localhost:4318` and always writes `otel.log_user_prompt = false` (prompt text stays redacted; a hand-set `true` does not survive a re-run — see the [AgentMind guide](agentmind.md) for the full merge-semantics caveat).

## The Workflow

The same gated lifecycle applies — Spec, Plan, Implement, Review — but without custom slash commands. Use the CLI directly:

```
Idle ──→ Spec Mode ──human gate──→ Plan Mode ──human gate──→ Implementation ──→ Review ──human gate──→ Idle
```

### Your First Feature

**1. Start a specification**

```bash
mindspec spec create 001-my-feature
```

**2. Draft the spec collaboratively**

You and the agent fill in the spec. Validate with:

```bash
mindspec validate spec 001-my-feature
```

**3. Approve the spec**

```bash
mindspec spec approve 001-my-feature
```

**4. Draft the plan**

The agent creates `.mindspec/specs/001-my-feature/plan.md` with work chunks and dependencies.

**5. Approve the plan**

```bash
mindspec plan approve 001-my-feature
```

**6. Claim work and implement**

```bash
mindspec next                 # Claim first ready bead, create worktree
# ... implement ...
mindspec complete <bead-id>   # Close bead, advance state
mindspec next                 # Repeat until all beads done
```

**7. Approve the implementation**

```bash
mindspec impl approve 001-my-feature
```

## Differences from Claude Code Integration

| Feature | Claude Code | Codex |
|:--------|:-----------|:------|
| Session guidance | Auto (SessionStart hook) | Manual (agent reads AGENTS.md) |
| Custom commands | `/ms-spec-approve` etc. | Direct CLI: `mindspec spec approve <id>` |
| OTLP telemetry | `mindspec otel setup` | `mindspec otel setup --codex` (writes `~/.codex/config.toml`) |
| AgentMind | OTLP export | OTLP export |

## Limitations

- **No automatic SessionStart hook** — the agent must call `mindspec instruct` based on the AGENTS.md instruction
- **Telemetry is opt-in** — Codex OTEL export is disabled until configured
- **No custom slash commands** — use CLI commands directly
- **Worktree support** depends on Codex's ability to change directories

## Reference

- [USAGE.md](../../../.mindspec/core/USAGE.md) — Full happy-path walkthrough
- [MODES.md](../../../.mindspec/core/MODES.md) — Detailed mode definitions and transitions
- [CONVENTIONS.md](../../../.mindspec/core/CONVENTIONS.md) — File layout and naming conventions
