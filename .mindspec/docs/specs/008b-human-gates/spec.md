# Spec 008b: Human Gates + Instruct-Tail Convention

## Goal

Model spec and plan approval as Beads human gates, consolidate approval mechanics into CLI commands (`mindspec approve spec/plan <id>`), and establish the **instruct-tail convention**: every state-changing command (`approve`, `next`, `complete`) emits `mindspec instruct` output after transitioning, so the agent always gets fresh guidance for its new mode. The `/spec-approve` and `/plan-approve` skills become thin wrappers that just call the CLI.

## Background

### Current state

Approval is tracked in markdown only — Beads has no awareness of it:

1. **Spec approval**: `spec.md` → `## Approval` → `Status: APPROVED`. The `/spec-approve` skill reads the spec, presents a summary, asks the user, edits the markdown, and sets MindSpec state. All of this is procedural logic embedded in a skill markdown file.
2. **Plan approval**: `plan.md` → YAML frontmatter `status: Approved`. The `/plan-approve` skill does similar multi-step coordination in markdown.

This means:
- `bd ready` may show implementation beads as "ready" before the plan is actually approved.
- The approval workflow is entirely soft — no machine-queryable gate in the execution graph.
- The skills are complex multi-step procedures that are hard to test, hard to version, and fragile.

### Instruct-tail gap

Every state-changing command transitions to a new mode, but the agent doesn't automatically get guidance for that mode:

- **`mindspec next`** already emits instruct output after claiming a bead (good).
- **`mindspec complete`** prints a brief ad-hoc summary but does NOT emit instruct guidance for the new mode. The agent must call `mindspec instruct` separately to get operating context.
- **`/spec-approve`** and **`/plan-approve`** are skill markdown — they print their own messages but don't emit instruct.

The pattern should be: every command that changes state emits instruct as its tail. The session-start hook just covers the cold-start case.

### Beads gate support

Beads provides first-class `human` gate support:

- Gates are beads with `--type=gate` — they participate in the dependency graph
- `bd gate resolve <id> --reason="..."` — closes a gate, unblocking dependents
- `bd gate list` / `bd gate check` — query gate state
- When a gate is resolved, any bead that depends on it becomes unblocked in `bd ready`

### Design principle

Move approval mechanics from skills into the CLI (Goal #8: CLI-first, minimal IDE glue). The CLI commands handle: validation → frontmatter update → gate resolution → state transition → instruct emission. Skills just run the command. Beads gates are the **execution signal** (controls `bd ready`); markdown frontmatter is the **document record** (captures who/when/notes). Both are updated atomically by the CLI. Consistent with ADR-0002 (Beads as execution substrate, docs as canonical record).

## Impacted Domains

- workflow: approval transitions become CLI commands with gate side effects; instruct-tail becomes a convention for all state-changing commands
- tracking: gates integrate into Beads dependency graph, affecting `bd ready` results

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Beads as passive tracking substrate. Gates are execution signals; canonical approval lives in docs. MindSpec CLI orchestrates both.
- [ADR-0003](../../adr/ADR-0003.md): MindSpec owns orchestration. Approval logic moves from skill markdown into Go code, where it can be tested and versioned. Instruct-tail ensures agents always get dynamic guidance from the CLI.
- [ADR-0005](../../adr/ADR-0005.md): State file tracks mode. The approve commands handle state transitions as part of the approval flow.

## Requirements

### Gate creation

1. **Spec approval gate** — When `mindspec bead spec <id>` creates a spec bead, it also creates a human gate bead titled `[GATE spec-approve <id>]` as a child of the spec bead. Implementation beads (created later by `mindspec bead plan`) will depend on this gate.

2. **Plan approval gate** — When `mindspec bead plan <id>` creates the molecule parent (plan epic), it also creates a human gate bead titled `[GATE plan-approve <id>]` as a child of the molecule parent. All implementation chunk beads depend on this gate, blocking them from `bd ready` until the plan is approved.

3. **Gate idempotency** — Gate creation is idempotent: if a gate with the expected title prefix already exists, reuse it.

### Approval commands

4. **`mindspec approve spec <id>`** — Single command that:
   - Runs `mindspec validate spec <id>` and fails if there are errors
   - Updates `docs/specs/<id>/spec.md` → `## Approval` section to `Status: APPROVED`, with date and `Approved By: user`
   - Resolves the spec gate via `bd gate resolve <gate-id> --reason="Spec approved"` (discovered by searching for `[GATE spec-approve <id>]` prefix)
   - Sets MindSpec state to plan mode: `mode=plan, spec=<id>`
   - Emits `mindspec instruct` output for the new mode (plan mode guidance)
   - If no gate exists (legacy beads), warns but proceeds (graceful degradation)

5. **`mindspec approve plan <id>`** — Single command that:
   - Runs `mindspec validate plan <id>` and fails if there are errors
   - Updates `docs/specs/<id>/plan.md` YAML frontmatter: `status: Approved`, `approved_at`, `approved_by`
   - Resolves the plan gate via `bd gate resolve <gate-id> --reason="Plan approved"` (discovered by searching for `[GATE plan-approve <id>]` prefix)
   - Sets MindSpec state to implement mode: `mode=implement, spec=<id>`
   - Emits `mindspec instruct` output for the new mode (implement mode guidance)
   - If no gate exists, warns but proceeds

6. **Skills become trivial** — `/spec-approve` and `/plan-approve` skills are updated to: (a) confirm spec ID with the user, (b) run `mindspec approve spec/plan <id>`, (c) done. All procedural logic (frontmatter editing, validation, state management, guidance emission) is removed from the skill files.

### Instruct-tail convention

7. **`mindspec complete` emits instruct** — After closing the bead, removing the worktree, and advancing state, `mindspec complete` emits `mindspec instruct` output for the new mode. Replaces the current ad-hoc `FormatResult()` summary. The agent gets guidance for whatever comes next (next bead in implement mode, plan mode if blocked, or idle).

8. **`mindspec next` already compliant** — `mindspec next` already reads state and emits instruct output after claiming a bead (lines 101-118 in `cmd/mindspec/next.go`). No changes needed, but the implementation should be aligned to use the same shared pattern as the other commands.

9. **Shared instruct-tail helper** — Extract a shared function (e.g., `emitInstruct(root)`) that reads current state, builds context, checks worktree if in implement mode, and renders guidance. Used by `approve`, `complete`, and `next` to avoid duplicating the instruct-tail logic.

### Dependency wiring

10. **Impl beads depend on plan gate** — When `CreatePlanBeads()` creates implementation chunk beads, each chunk bead gets a dependency on the plan approval gate. `bd ready` will not show any impl bead until the plan gate is resolved.

11. **Plan gate depends on spec gate** — The plan approval gate depends on the spec approval gate (if one exists). This enforces ordering: spec approval must happen before plan approval can unblock work.

### Backward compatibility

12. **Existing beads without gates still work** — `mindspec next`, `mindspec complete`, and other workflow commands do not require gates to exist. Gates enhance `bd ready` accuracy but are not a hard requirement.

13. **Frontmatter remains canonical for document state** — The approve commands update both frontmatter and gates. Code that reads frontmatter (e.g., `isSpecApproved()`, plan status checks) continues to work unchanged.

14. **`mindspec bead plan` requires spec gate resolved** — `mindspec bead plan <id>` checks that the spec approval gate (`[GATE spec-approve <id>]`) is resolved before creating impl beads. If the gate is open, the command fails with a clear error. This prevents orphaned beads if a spec is never approved.

## Scope

### In Scope
- `cmd/mindspec/approve.go` — new `mindspec approve spec` and `mindspec approve plan` commands
- `cmd/mindspec/root.go` — register approve command
- `cmd/mindspec/complete.go` — add instruct-tail emission after state advance
- `cmd/mindspec/next.go` — refactor to use shared instruct-tail helper
- `cmd/mindspec/instruct_tail.go` — shared `emitInstruct(root)` helper (new file)
- `internal/complete/complete.go` — `FormatResult()` replaced or supplemented by instruct output
- `internal/bead/spec.go` — create spec approval gate during `CreateSpecBead()`
- `internal/bead/plan.go` — create plan approval gate during `CreatePlanBeads()`, wire impl beads to depend on it
- `internal/bead/bdcli.go` — add gate creation and resolution wrappers
- `internal/bead/gate.go` — gate creation/discovery/resolution helpers (new file)
- `.claude/commands/spec-approve.md` — simplify to just call `mindspec approve spec <id>`
- `.claude/commands/plan-approve.md` — simplify to just call `mindspec approve plan <id>`
- Doc-sync: `CLAUDE.md`, `docs/core/CONVENTIONS.md`

### Out of Scope
- Timer gates, GitHub gates, or other gate types
- Per-worktree state changes (ADR-0007 scope)
- `mindspec approve` for other artifacts (ADRs, etc.)

## Non-Goals

- Replacing frontmatter-based approval tracking — gates complement, not replace
- Building a `mindspec gate` command tree — Beads already provides `bd gate`
- Making gates hard blockers in state transitions — `mindspec approve` does both; state checks still use frontmatter

## Acceptance Criteria

### Gate creation
- [ ] `mindspec bead spec <id>` creates a human gate bead titled `[GATE spec-approve <id>]` as a child of the spec bead
- [ ] `mindspec bead plan <id>` creates a human gate bead titled `[GATE plan-approve <id>]` as a child of the molecule parent
- [ ] Gate creation is idempotent: re-running does not create duplicates
- [ ] Implementation chunk beads depend on the plan approval gate
- [ ] Plan approval gate depends on spec approval gate (when spec gate exists)

### Approval commands
- [ ] `mindspec approve spec <id>` validates, updates frontmatter, resolves gate, sets state, and emits instruct
- [ ] `mindspec approve plan <id>` validates, updates plan frontmatter, resolves gate, sets state, and emits instruct
- [ ] Both commands fail with clear error if validation fails
- [ ] Both commands warn but proceed if no gate exists (backward compat)

### Instruct-tail
- [ ] `mindspec complete` emits instruct output for the new mode after advancing state
- [ ] `mindspec next` uses the shared instruct-tail helper (existing behavior preserved)
- [ ] `mindspec approve spec` emits plan mode instruct after approval
- [ ] `mindspec approve plan` emits implement mode instruct after approval
- [ ] Shared `emitInstruct()` helper is used by all four commands

### Skills
- [ ] `/spec-approve` skill is simplified to call `mindspec approve spec <id>`
- [ ] `/plan-approve` skill is simplified to call `mindspec approve plan <id>`

### Gate enforcement
- [ ] `mindspec bead plan <id>` refuses to create impl beads if the spec gate is still open

### Integration
- [ ] `bd ready` does not show implementation beads until both spec and plan gates are resolved
- [ ] `bd gate list` shows open gates for specs/plans awaiting approval
- [ ] Existing beads without gates continue to work with `mindspec next` and `mindspec complete`

### General
- [ ] All new code has unit tests; `make test` passes
- [ ] Doc-sync: CLAUDE.md, CONVENTIONS.md updated

## Validation Proofs

- `./bin/mindspec bead spec <id>`: Creates spec bead + gate; `bd gate list` shows the open gate
- `./bin/mindspec bead plan <id>`: Creates plan beads + gate; `bd gate list` shows the open gate; `bd ready` does NOT show impl beads
- `./bin/mindspec approve spec <id>`: Updates frontmatter, resolves gate, emits plan mode guidance
- `./bin/mindspec approve plan <id>`: Updates frontmatter, resolves gate, emits implement mode guidance; `bd ready` now shows impl beads
- `./bin/mindspec complete`: Closes bead, emits guidance for next state (not just ad-hoc summary)
- `make test`: All tests pass

## Open Questions

None — all resolved.

### Resolved

- ~~Should `mindspec bead plan` refuse to create impl beads if the spec gate is still open?~~ **Resolved**: Yes, refuse. `mindspec bead plan` checks that the spec gate is resolved before creating impl beads. Prevents orphaned beads if spec is never approved. Added as requirement 14.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via /spec-approve workflow
