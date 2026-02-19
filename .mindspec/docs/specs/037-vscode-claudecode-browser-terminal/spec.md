# Spec 037-vscode-claudecode-browser-terminal: VS Code plugin for browser Claude Code terminal with AgentMind visualization

## Goal

Create a VS Code extension that acts as an alternative to the Claude Code plugin by launching a Claude Code terminal in a web browser page that also hosts AgentMind, so users can type in the terminal and see the agent's live reasoning/activity visualization in the same browser experience.

## Background

AgentMind already provides live visualization of agent telemetry and is the unified OTLP collector (ADR-0011). Current workflows do not provide a tight "type + visualize" loop from VS Code in a single browser surface. The desired workflow is:

1. Start from VS Code.
2. Open a browser terminal connected to a Claude Code session.
3. Watch AgentMind visualization update live as the session runs.

This spec defines that integrated workflow with local-first security and minimal setup friction.

## Impacted Domains

- `viz`: Browser UI composition for terminal + live graph view.
- `observability`: Session-level OTEL wiring from Claude Code to AgentMind collector.
- `core`: Extension/launcher command path and local process orchestration.
- `docs`: Operator setup and troubleshooting guidance.

## ADR Touchpoints

- [ADR-0009](../../adr/ADR-0009.md): AgentMind graph model and real-time web visualization behavior remain authoritative.
- [ADR-0011](../../adr/ADR-0011.md): AgentMind remains the single OTLP receiver on `localhost:4318`.
- [ADR-0012](../../adr/ADR-0012.md): Integration should compose with external CLIs/processes directly rather than introducing unnecessary wrapper abstractions.

## Requirements

### 1. VS Code Launch Workflow

1. Provide a VS Code command (command palette + optional activity entry point) to start a browser-based Claude Code session for AgentMind.
2. Validate local prerequisites before launch (at minimum: `claude` CLI available; AgentMind endpoint reachable or auto-startable).
3. Start or attach to AgentMind if not already running, preserving ADR-0011 single-collector behavior.
4. Open the browser page automatically after launch and return actionable errors in VS Code if startup fails.

### 2. Browser Terminal Experience

5. Provide an interactive terminal surface in the browser backed by a live Claude Code PTY/session.
6. Stream terminal input/output bidirectionally with ANSI/line-editing compatibility sufficient for normal Claude Code usage.
7. Keep terminal session lifecycle controls explicit (start, reconnect, stop/terminate).

### 3. AgentMind Co-Visualization

8. Render AgentMind in the same browser experience as the terminal (same page or tightly coupled adjacent page flow).
9. Ensure Claude session telemetry is emitted to AgentMind during this flow so the graph updates while users type and the agent acts.
10. If visualization data is unavailable, keep terminal usable and show a clear degraded-mode notice.

### 4. Local Security and Privacy Defaults

11. Expose terminal bridge endpoints on localhost only by default.
12. Use per-session access protection (for example session token) to reduce accidental local cross-tab/session hijacking.
13. Do not force persistent transcript or telemetry export beyond existing AgentMind behavior unless explicitly enabled.

### 5. Documentation and UX Clarity

14. Document install/setup/use flow for the extension and browser session.
15. Document failure modes and remediation (missing CLI, occupied ports, AgentMind unavailable).

## Scope

### In Scope

- VS Code extension command(s) and launch orchestration for browser session start.
- Local terminal bridge needed to connect browser terminal to Claude Code process.
- Browser UI composition that includes terminal interaction and AgentMind visualization together.
- OTEL/environment wiring required for live AgentMind updates in this workflow.
- User-facing documentation for setup and troubleshooting.

### Out of Scope

- Replacing all features of the upstream Claude Code extension beyond this integrated visualization workflow.
- Cloud-hosted or remote multi-user terminal serving.
- Changes to AgentMind's core graph semantics or visual design system.
- Non-local authentication/authorization infrastructure.

## Non-Goals

- Building a full remote IDE service.
- Introducing a new telemetry protocol separate from OTLP/AgentMind.
- Solving every possible shell compatibility edge case in the first iteration.

## Acceptance Criteria

- [ ] From VS Code, invoking the new command launches a browser session with an interactive Claude terminal and visible AgentMind surface.
- [ ] User input in the browser terminal reaches Claude Code and output returns interactively without blocking normal prompt/tool flow.
- [ ] During a live session, AgentMind graph/activity updates reflect Claude session behavior in near real time.
- [ ] If AgentMind is not available, the user sees an explicit warning and can still use the terminal session.
- [ ] Startup and runtime errors are surfaced with actionable remediation in VS Code and/or browser UI.
- [ ] Documentation includes a reproducible quick-start and troubleshooting section for this feature.

## Validation Proofs

- `mindspec agentmind serve` + extension launch command: browser opens with terminal + AgentMind view.
- Run a short Claude prompt that triggers at least one tool call: terminal output appears and AgentMind graph updates.
- Kill AgentMind during session: terminal remains functional and degraded-mode warning appears.
- Restore AgentMind and continue session: visualization resumes without full restart.

## Open Questions

*None currently. Resolved assumptions for this draft:*
- Browser experience is local-first and launched from VS Code.
- AgentMind remains the visualization backend and unified collector.
- The feature is an alternative workflow, not a full replacement of all existing Claude extension capabilities.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
