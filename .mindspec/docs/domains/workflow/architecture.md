# Workflow Domain — Architecture

## Key Patterns

### Five-Mode Lifecycle

```
Explore -> [Spec Mode] -> approval -> [Plan Mode] -> approval -> [Implement Mode] -> [Review] -> Done
```

Each mode gates:
- **Allowed outputs** — what artifacts can be created/modified
- **Required context** — what must be reviewed before proceeding
- **Transition gates** — what conditions must hold to advance

### Beads as Single State Store (ADR-0023)

All lifecycle state is derived from Beads — no filesystem state files (no `focus`, no `lifecycle.yaml`):

| Concern | Owner |
|:--------|:------|
| Execution tracking (issues, dependencies) | Beads |
| Workflow orchestration (modes, gates) | MindSpec (this domain) |
| Phase derivation (spec lifecycle stage) | MindSpec, from Beads statuses |
| Long-form specs, ADRs, domain docs | Documentation system |

Phase is derived from epic metadata and child bead statuses:

| Condition | Derived phase |
|:----------|:-------------|
| No epic for spec | spec |
| Epic exists, no children | plan |
| Any child in_progress | implement |
| All children closed | review |
| Epic closed with done marker | done |

### Workflow/Execution Boundary (Spec 077)

Workflow packages determine *what* should happen and delegate *how* to the Executor:

```
approve/spec.go   ──▶ exec.InitSpecWorkspace()
approve/plan.go   ──▶ exec.HandoffEpic(), exec.DispatchBead()
complete/         ──▶ exec.CompleteBead(), exec.CommitAll()
approve/impl.go   ──▶ exec.FinalizeEpic()
cleanup/          ──▶ exec.Cleanup()
```

**Import rule**: Workflow packages (`approve/`, `complete/`, `next/`, `cleanup/`, `specinit/`) call `executor.Executor` methods. They MUST NOT import `internal/gitutil/` directly.

### ADR Governance

- Plans must cite ADRs they rely on
- Divergence detected at any mode triggers the ADR divergence protocol
- New superseding ADRs require human approval before work resumes

## Invariants

1. No code changes without an approved spec AND approved plan.
2. Implementation scope cannot widen — discovered work becomes new beads.
3. ADR divergence always triggers a human gate.
4. Bead closure requires proof-of-done + doc-sync.
5. Beads is the single state store — no filesystem state files.
6. Workflow packages never import `internal/gitutil/` — all execution goes through `Executor`.
