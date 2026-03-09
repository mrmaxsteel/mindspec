---
approved_at: "2026-03-09T00:41:25Z"
approved_by: user
status: Approved
---
# Spec 080-epic-lifecycle-metadata: Epic Lifecycle Metadata

## Goal

Store the canonical spec lifecycle phase (`spec`, `plan`, `implement`, `review`, `done`) directly in epic metadata, replacing the current approach of deriving phase by analyzing child bead statuses. Child bead analysis remains as a consistency validation check. Additionally, remove the auto-next behavior after plan approval — the CLI should stop and tell the agent how to proceed (`/clear` then `mindspec next`), not silently start implementation in a stale context.

## Background

### Phase derivation is fragile

`DerivePhase()` in `internal/phase/derive.go` currently infers the lifecycle phase from epic children:

- No children → `plan`
- Any child `in_progress` → `implement`
- All children `closed` → `review`
- Mix of open/closed, none in_progress → `plan`

This is indirect, O(children), and produces ambiguous results in edge cases:
- A closed epic with all children closed could be `review` or `done` — we added a `mindspec_done` marker to disambiguate, but this is a band-aid
- The `spec` vs `plan` distinction can't be derived from children at all — both look like "epic exists, no children"
- Phase transitions happen implicitly as side effects of bead status changes, not as explicit workflow events

### Auto-next pollutes agent context

After `mindspec approve plan`, the CLI auto-calls `mindspec next` (in `plan_cmd.go` lines 76-90). This:
1. Claims a bead and creates a worktree in the same agent session
2. The agent's context is already full of spec/plan discussion — starting implementation in this state leads to worse code quality
3. The `--no-next` flag exists but is not the default
4. The skill file (`ms-plan-approve/SKILL.md`) also instructs agents to "run `mindspec next`" after approval

The correct pattern: plan approval is a stopping point. The agent should `/clear` to reset context, then `mindspec next` starts fresh with bead-specific context.

## Impacted Domains

- `internal/phase/derive.go`: Read phase from epic metadata instead of deriving from children; keep child analysis as validation
- `internal/approve/spec.go`: Write `mindspec_phase: plan` to epic metadata on spec approval
- `internal/approve/plan.go`: Write `mindspec_phase: implement` to epic metadata on plan approval
- `internal/approve/impl.go`: Write `mindspec_phase: done` to epic metadata on impl approval (replaces `mindspec_done` marker)
- `internal/complete/complete.go`: No phase write needed — phase doesn't change when a bead completes (stays `implement` until all done → `review`)
- `internal/next/beads.go`: No phase write needed — claiming a bead doesn't change the phase
- `cmd/mindspec/plan_cmd.go`: Remove auto-next after plan approval, emit stop-and-proceed guidance
- `.claude/skills/ms-plan-approve/SKILL.md`: Remove instruction to run `mindspec next`
- `internal/specinit/specinit.go`: Write `mindspec_phase: spec` to epic metadata on spec creation

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): State derivation from beads — this spec modifies the derivation approach (metadata-first instead of child-analysis-first) while keeping the same external contract

## Requirements

### Epic metadata as source of truth

1. **Explicit phase storage**: Each lifecycle transition MUST write `mindspec_phase` to the epic's metadata via `bd update <epic-id> --metadata '{"mindspec_phase": "<phase>"}'`.
2. **Phase values**: `spec`, `plan`, `implement`, `review`, `done` — matching the existing `state.Mode*` constants.
3. **Read path**: `DerivePhase()` MUST first check `metadata.mindspec_phase`. If present and valid, use it directly (O(1)).
4. **Consistency validation**: After reading the stored phase, `DerivePhase()` SHOULD also derive phase from children and compare. If they disagree, emit a warning to stderr but trust the stored phase. This catches data corruption or manual `bd close` without `mindspec complete`.
5. **Migration**: For epics without `mindspec_phase` metadata (pre-080 epics), fall back to the current child-based derivation. This preserves backward compatibility with no migration step.
6. **Replace `mindspec_done`**: The existing `mindspec_done: true` marker becomes redundant — `mindspec_phase: done` replaces it. `hasDoneMarker()` should check both for backward compat during transition.

### Plan validation enforcement

10. **Per-bead acceptance criteria required**: The plan validator MUST error (not warn) when a bead section is missing `**Acceptance Criteria**`. The current behavior (Spec 078) is a warning that is skipped for already-approved plans, which means plans can be approved without per-bead AC via the fallback to spec-level AC. This silently degrades plan quality.
11. **No skip for approved plans**: The `!isApproved` guard on the missing-AC warning in `internal/validate/plan.go` should be removed for the AC check — per-bead AC is a structural requirement, not something that becomes optional once approved.

### Plan approval stops, doesn't start implementation

7. **Remove auto-next**: `plan_cmd.go` MUST NOT auto-call `nextCmd.RunE()` after plan approval. Remove the `--no-next` flag (it becomes the only behavior).
8. **Emit proceed guidance**: After plan approval, the CLI MUST output:
   ```
   Plan approved. Implementation beads created.

   Next steps:
     1. Run /clear to reset your context
     2. Run `mindspec next` to claim your first bead
   ```
9. **Update skill file**: `ms-plan-approve/SKILL.md` MUST instruct agents to stop after approval and tell the user how to proceed, not to start implementing.

## Scope

### In Scope

- `internal/phase/derive.go` — metadata-first phase read, child-based validation
- `internal/approve/spec.go` — write `mindspec_phase: plan` on spec approval
- `internal/approve/plan.go` — write `mindspec_phase: implement` on plan approval
- `internal/approve/impl.go` — write `mindspec_phase: done`, deprecate `mindspec_done`
- `internal/specinit/specinit.go` — write `mindspec_phase: spec` on epic creation
- `cmd/mindspec/plan_cmd.go` — remove auto-next, emit stop guidance
- `.claude/skills/ms-plan-approve/SKILL.md` — update agent instructions
- `internal/bead/` — helper to write phase metadata if needed
- `internal/validate/plan.go` — promote missing per-bead AC from warning to error

### Out of Scope

- Changing the beads CLI (`bd`) itself — we use existing metadata features
- Changing the epic structure or parent-child relationships
- Adding new CLI commands — phase writes happen inside existing approval commands
- Changing `mindspec next` or `mindspec complete` behavior (covered by Spec 079)

## Non-Goals

- This spec does not add a `mindspec phase` command to manually set the phase — phase transitions are always through approval commands.
- This spec does not change the external contract of `DerivePhase()` — callers still get a mode string, they don't need to know whether it came from metadata or child analysis.
- This spec does not remove child-based derivation — it demotes it to a validation check.

## Acceptance Criteria

- [ ] `mindspec approve spec <id>` writes `mindspec_phase: plan` to the epic's metadata
- [ ] `mindspec approve plan <id>` writes `mindspec_phase: implement` to the epic's metadata
- [ ] `mindspec approve plan <id>` does NOT auto-call `mindspec next` — outputs stop guidance instead
- [ ] `mindspec approve impl <id>` writes `mindspec_phase: done` to the epic's metadata
- [ ] `DerivePhase()` reads `mindspec_phase` from metadata and returns it directly when present
- [ ] `DerivePhase()` emits a warning when stored phase disagrees with child-derived phase
- [ ] Epics without `mindspec_phase` metadata fall back to child-based derivation (backward compat)
- [ ] `ms-plan-approve` skill tells agents to stop and output proceed instructions
- [ ] Plan validator errors (not warns) when a bead section is missing `**Acceptance Criteria**`
- [ ] All existing LLM harness tests pass

## Validation Proofs

- `bd show <epic-id> --json` after spec approval: metadata contains `mindspec_phase: plan`
- `bd show <epic-id> --json` after plan approval: metadata contains `mindspec_phase: implement`
- `mindspec approve plan <id>` output: contains "/clear" and "mindspec next", does NOT claim a bead
- `DerivePhase()` with metadata set: returns stored phase without querying children
- LLM harness: `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM -timeout 10m`

## Open Questions

*All resolved:*

- ~~Should `mindspec complete` write `mindspec_phase: review`?~~ → No. Phase stays `implement` until `approve impl` writes `done`. `complete` would need O(children) to detect last bead, defeating the purpose. `DerivePhase` can warn "all beads closed, consider `approve impl`" via the consistency check. Phase writes stay 1:1 with approval commands.
- ~~Warning vs error for consistency disagreement?~~ → Warning. An error would block workflows when someone runs `bd close` manually. The stored phase is the deliberate workflow state; child status is emergent. Trust the deliberate state, warn on disagreement.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-09
- **Notes**: Approved via mindspec approve spec