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
