# Workflow Domain — Architecture

## Key Patterns

### Three-Mode Lifecycle

```
Intent -> [Spec Mode] -> approval -> [Plan Mode] -> approval -> [Implementation Mode] -> validation -> Done
```

Each mode gates:
- **Allowed outputs** — what artifacts can be created/modified
- **Required context** — what must be reviewed before proceeding
- **Transition gates** — what conditions must hold to advance

### Beads as Tracking Substrate (ADR-0002)

MindSpec layers its structured operating model on top of Beads:

| Concern | Owner |
|:--------|:------|
| Execution tracking (issues, dependencies) | Beads |
| Workflow orchestration (modes, gates) | MindSpec (this domain) |
| Long-form specs, ADRs, domain docs | Documentation system |

Beads entries must remain **concise and execution-oriented**. Spec beads contain a summary + link to the canonical spec file.

### Worktree Isolation

All implementation work runs in isolated git worktrees:
- Named with bead ID: `worktree-<bead-id>`
- One worktree per implementation bead
- Closing a bead requires evidence + doc updates + clean state sync

### ADR Governance

- Plans must cite ADRs they rely on
- Divergence detected at any mode triggers the ADR divergence protocol
- New superseding ADRs require human approval before work resumes

## Invariants

1. No code changes without an approved spec AND approved plan.
2. Implementation scope cannot widen — discovered work becomes new beads.
3. ADR divergence always triggers a human gate.
4. Bead closure requires proof-of-done + doc-sync.
5. Active Beads workset must be kept intentionally small.
