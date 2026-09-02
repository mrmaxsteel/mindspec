# MindSpec + GitHub Copilot

A guide to using MindSpec's spec-driven development workflow with GitHub Copilot — both Copilot CLI (terminal) and Copilot Chat (VS Code).

## How It Works

MindSpec's core is a standalone Go CLI. All workflow logic lives in CLI commands, not IDE-specific hooks. Copilot can use MindSpec by calling the CLI directly — from the terminal (Copilot CLI) or from VS Code's integrated terminal (Copilot Chat).

Both surfaces share `.github/copilot-instructions.md` as the workspace instruction file, which points to `AGENTS.md` for project-wide conventions (per [ADR-0017](../../../.mindspec/adr/ADR-0017.md)).

## Prerequisites

- Go 1.22+
- [Beads](https://github.com/steveyegge/beads) CLI (`bd`)
- Git
- GitHub Copilot (CLI or VS Code extension)

## Setup

### 1. Build MindSpec

```bash
make build && make install
```

### 2. Bootstrap Your Project

```bash
mindspec init
```

This creates `AGENTS.md`, `.github/copilot-instructions.md`, and the `.mindspec/` directory structure.

### 3. Configure Copilot

```bash
mindspec setup copilot
```

This creates:
- `.github/copilot-instructions.md` — workspace instructions pointing to `AGENTS.md`
- `.github/hooks/mindspec.json` — a sessionStart hook that runs `mindspec hook session-start` for session-start guidance
- `.agents/skills/` — workflow skills (`/ms-spec-create`, `/ms-spec-approve`, etc.)

### 4. Verify

```bash
mindspec doctor
```

### 5. Optional: Enable AgentMind Observability

[AgentMind](agentmind.md) is a standalone companion product installed from [its own repo](https://github.com/mrmaxsteel/agentmind). Start it, then point your agent's OTLP export at it:

```bash
agentmind serve                                        # the standalone binary, not a mindspec verb
mindspec otel setup --endpoint http://localhost:4318
```

## Copilot CLI (Terminal)

Copilot CLI is a terminal agent that reads `AGENTS.md` directly and can run `mindspec` commands.

**Session start**: Run `mindspec instruct` at the beginning of each session for mode-appropriate guidance. Unlike Claude Code, Copilot CLI doesn't have automatic SessionStart hooks — the agent reads `AGENTS.md` and follows the instruction.

**Workflow commands**: Call `mindspec` directly or use prompt commands if available.

## Copilot Chat (VS Code)

Copilot Chat reads `.github/copilot-instructions.md` automatically for workspace context.

**Session start**: The instructions file tells Copilot to run `mindspec instruct` in the integrated terminal for mode-aware guidance.

**Running commands**: Use VS Code's integrated terminal to run `mindspec` CLI commands. Copilot Chat can execute terminal commands when you ask it to.

**Skills**: Use `/ms-spec-create`, `/ms-spec-approve`, etc. in the chat panel — these are defined in `.agents/skills/` and instruct Copilot to run the corresponding `mindspec` CLI command.

**Workspace awareness**: Use `@workspace` to give Copilot context about the codebase when drafting specs or plans.

## The Workflow

The same gated lifecycle applies — Spec, Plan, Implement, Review — using prompt commands or direct CLI:

```
Idle ──→ Spec Mode ──human gate──→ Plan Mode ──human gate──→ Implementation ──→ Review ──human gate──→ Idle
```

### Your First Feature

**1. Start a specification**

```bash
mindspec spec create 001-my-feature
```

Or use `/ms-spec-create` in Copilot Chat.

**2. Draft the spec collaboratively**

You and the agent fill in the spec. Validate with:

```bash
mindspec validate spec 001-my-feature
```

**3. Approve the spec**

```bash
mindspec spec approve 001-my-feature
```

Or use `/ms-spec-approve` in Copilot Chat.

**4. Draft the plan**

The agent creates `.mindspec/specs/001-my-feature/plan.md` with work chunks and dependencies.

**5. Approve the plan**

```bash
mindspec plan approve 001-my-feature
```

Or use `/ms-plan-approve` in Copilot Chat.

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

Or use `/ms-impl-approve` in Copilot Chat.

## Differences from Claude Code and Codex

| Feature | Claude Code | Codex | Copilot CLI | Copilot Chat (VS Code) |
|:--------|:-----------|:------|:------------|:----------------------|
| Instruction file | `CLAUDE.md` → `AGENTS.md` | `AGENTS.md` | `AGENTS.md` | `.github/copilot-instructions.md` → `AGENTS.md` |
| Session guidance | Auto (SessionStart hook) | Manual (reads AGENTS.md) | sessionStart hook installed (`.github/hooks/mindspec.json`) | Reads instructions on open + installed sessionStart hook |
| Skills | `.claude/skills/*/SKILL.md` | `.agents/skills/*/SKILL.md` | `.agents/skills/*/SKILL.md` | `.agents/skills/*/SKILL.md` |
| OTLP telemetry | `mindspec otel setup` | `mindspec otel setup --codex` | Not yet supported | Not yet supported |
| AgentMind | OTLP export | OTLP export | Not yet supported | Not yet supported |

## Instruction Layering

The instruction chain follows [ADR-0017](../../../.mindspec/adr/ADR-0017.md):

```
.github/copilot-instructions.md  →  AGENTS.md  →  mindspec instruct
CLAUDE.md                        →  AGENTS.md  →  mindspec instruct
```

Agent-specific files are thin pointers to the universal `AGENTS.md`, which holds shared workflow conventions and points to `mindspec instruct` for runtime guidance.

## Hooks

`mindspec setup copilot` installs one hook in `.github/hooks/mindspec.json`:

- **sessionStart** — runs `mindspec hook session-start --format copilot` when a Copilot session begins, emitting mode-appropriate guidance

This is the Copilot equivalent of Claude Code's `.claude/settings.json` SessionStart hook. There is no plan-mode gate hook for Copilot: nothing blocks code edits during plan mode.

## Limitations

- **Telemetry is not yet supported** — Copilot doesn't expose OTEL configuration hooks
- **Worktree support** depends on Copilot's ability to change directories

## Reference

- [USAGE.md](../../../.mindspec/core/USAGE.md) — Full happy-path walkthrough
- [MODES.md](../../../.mindspec/core/MODES.md) — Detailed mode definitions and transitions
- [CONVENTIONS.md](../../../.mindspec/core/CONVENTIONS.md) — File layout and naming conventions
