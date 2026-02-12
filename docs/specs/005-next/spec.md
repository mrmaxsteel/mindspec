# Spec 005: Work Selection + Claiming (`mindspec next`)

## Goal

Give agents a single command to discover and claim their next piece of work. `mindspec next` queries Beads for ready work, claims it, updates MindSpec state, and emits guidance — so an agent can go from "what should I do?" to "here's your bead, here's the mode, here are your rules" in one step.

## Background

Today, finding the next work item requires multiple manual steps:
1. `bd ready` to see available beads
2. `bd update <id> --status=in_progress` to claim one
3. `mindspec state set --mode=implement --spec=X --bead=Y` to update state
4. `mindspec instruct` to get guidance

This friction slows down session starts and bead transitions. ADR-0003 defines `mindspec next` as the command that selects/claims work and emits guidance (or instructs the agent to run `mindspec instruct`).

## Impacted Domains

- **workflow**: Work selection, claiming, state transitions
- **core**: CLI command registration

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): Centralized Agent Instruction Emission — defines `mindspec next` as part of the CLI contract
- [ADR-0002](../../adr/ADR-0002.md): Beads Integration Strategy — `next` interacts with Beads as passive tracking substrate
- [ADR-0005](../../adr/ADR-0005.md): Explicit State Tracking — `next` writes to `.mindspec/state.json` on claim

## Requirements

1. **Ready work discovery**: Query Beads (`bd ready`) for work items with no unresolved blockers
2. **Interactive selection**: If multiple items are ready, list them and let the user pick. If only one, claim it directly
3. **Claim and lock**: Mark the selected bead as `in_progress` via `bd update <id> --status=in_progress`
4. **State update**: Write `.mindspec/state.json` with the appropriate mode and active bead (per ADR-0005)
5. **Guidance emission**: After claiming, emit mode-appropriate guidance (run `mindspec instruct` internally or print equivalent output)
6. **Mode-aware behavior**: Detect which mode the claimed work belongs to (spec bead → Spec/Plan Mode, implementation bead → Implementation Mode) based on bead metadata
7. **Clean tree check**: Verify `git status` is clean before claiming work. If dirty, emit recovery guidance instead of proceeding
8. **No-work handling**: If `bd ready` returns no items, report clearly and suggest next steps (create new spec, check blocked items)

## Scope

### In Scope

- `internal/next/` package: ready work query, selection logic, claiming, state transition
- `cmd/mindspec/next.go`: replace the stub in `cmd/mindspec/stubs.go`
- Integration with `internal/state/` for state writes
- Integration with `mindspec instruct` for post-claim guidance
- Shell out to `bd` CLI for Beads operations

### Out of Scope

- Worktree creation (Spec 008 — `next` reports the expected worktree but doesn't create it)
- Parallel work / multiple active beads (v1 is single-bead)
- `mindspec validate` (Spec 006)
- Auto-creating beads from specs or plans

## Non-Goals

- Replacing Beads as the work tracking system — `next` is a thin orchestration layer on top of `bd`
- Supporting non-Beads work items
- Automated bead prioritization (user picks if multiple are ready)

## Acceptance Criteria

- [ ] `mindspec next` with ready work available lists ready items and claims the selected one
- [ ] `mindspec next` with exactly one ready item claims it automatically
- [ ] `mindspec next` with no ready work reports "no ready work" and suggests next steps
- [ ] After claiming, `.mindspec/state.json` is updated with the correct mode and bead ID
- [ ] After claiming, mode-appropriate guidance is emitted (equivalent to `mindspec instruct`)
- [ ] `mindspec next` with a dirty git tree emits a warning and does not claim work
- [ ] `mindspec next` when Beads is unavailable fails gracefully with a diagnostic message
- [ ] `make test` passes with tests covering ready/no-ready/dirty-tree/claim paths
- [ ] Existing commands (`instruct`, `state`, `doctor`, `glossary`, `context`) are unaffected

## Validation Proofs

- `make build && ./bin/mindspec next`: Claims next ready bead and emits guidance
- `./bin/mindspec state show`: Confirms state was updated after claim
- `make test`: All tests pass including next package tests

## Open Questions

None — all resolved.

## Design Decisions (resolved during spec)

- **Bead type detection**: Parse `bd show <id>` output for the `type` field to distinguish spec beads (feature) from implementation beads (task).
- **Auto-claim**: Auto-claim when only one item is ready, with a confirmation message. Multiple items prompt the user to pick.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-12
- **Notes**: Approved via /spec-approve workflow
