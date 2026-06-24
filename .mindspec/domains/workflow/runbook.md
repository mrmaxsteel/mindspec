# Workflow Domain — Runbook

## Common Operations

### Start a New Spec

Use `/spec-init` or create manually:
```
docs/specs/<NNN-slug>/
  spec.md
  context-pack.md (placeholder)
```

### Approve a Spec

1. Verify all acceptance criteria are defined and measurable
2. Verify impacted domains and ADR touchpoints are declared
3. Verify all open questions are resolved
4. Use `/spec-approve` or update the spec's Approval section to `Status: APPROVED`

### Create Implementation Plan

1. Review accepted ADRs for impacted domains
2. Review domain docs (overview, architecture, interfaces)
3. Check Context Map for neighbor contracts
4. Decompose spec into bounded implementation beads
5. Use `/plan-approve` when ready

### Execute an Implementation Bead

1. Create worktree: `worktree-<bead-id>`
2. Load context pack for the bead
3. Implement within the bead's scope
4. Capture proof (test outputs, command results)
5. Update documentation
6. Close bead with evidence

### Handle ADR Divergence

1. Stop work immediately
2. Identify the ADR and nature of divergence
3. Present options to user: continue-as-is vs propose new ADR
4. If user approves divergence: create superseding ADR
5. Resume only after new ADR is accepted

## Troubleshooting

### Mode Confusion

Check current mode with `/spec-status`. If unclear:
- No approved spec? You're in Spec Mode.
- Approved spec but no approved plan? You're in Plan Mode.
- Both approved + active bead? You're in Implementation Mode.
