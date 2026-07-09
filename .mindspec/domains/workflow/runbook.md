# Workflow Domain — Runbook

## Common Operations

### Start a New Spec

Use `/spec-init` or create manually:
```
.mindspec/specs/<NNN-slug>/
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

## Maintenance Notes

- **2026-07-02 (spec 107 wave 1):** The hidden `spec init` alias
  (`cmd/mindspec/spec_init.go`) was de-duplicated to reuse `specCreateCmd.RunE`
  instead of carrying a byte-identical copy of the create flow, so future
  `spec create` changes propagate to the alias automatically. Behavior of
  `mindspec spec init` is unchanged; the alias still registers its own `--title`
  flag.
- **2026-07-02 (spec 108 wave 2, Bead 4):** `mindspec doctor`'s dead-manifest
  check (`internal/doctor/ownership.go`) now walks the workspace tree **once per
  ownership check** instead of once per domain. A single enumeration collects the
  live file list (still skipping `.git/`, `.worktrees/`, and `.beads/`, V2-6), and
  every domain's `paths:` globs are tested against that cached list. The walk is
  routed through the package-level `walkWorkspaceFn` seam so a test can count its
  invocations. Doctor output is unchanged: the same dead-manifest Warn/pass result
  per domain, just fewer directory walks on the `doctor` hot path.
- **2026-07-09 (spec 110 Bead 5):** The panel operator procedure is now
  mechanized behind `mindspec panel create|verify|tally` (spec 110 Bead 4) — a
  thin CLI layer over `internal/panel`'s single-home writer + `PanelGateDecision`.
  `/ms-panel-run` Step 0 registers (or re-panels) with one `panel create` call
  instead of a hand-typed `panel.json` schema and a skill-re-authored
  verdict-JSON template; `/ms-panel-tally` renders its decision and the
  aggregated `concrete_changes_required` with one `panel tally` call instead of
  a hand-tabulated decision matrix. The judgment sections both skills retain —
  `/ms-panel-run`'s **Launch the panel**, **Codex failure detection**,
  **Working directory matters**, **Slot lens defaults**, and
  **Anti-patterns**; `/ms-panel-tally`'s **Artifact gates** (the HARD-vs-soft `hard_block`
  judgment), **Consolidate** (semantic dedup + criticality ranking), and
  halt-recovery/escape-hatch procedure — are unchanged: the verbs mechanize the
  decision function and the artifact registration, not the human judgment
  layered on top of them.
