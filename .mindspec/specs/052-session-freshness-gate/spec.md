---
approved_at: "2026-02-27T07:35:10Z"
approved_by: user
molecule_id: mindspec-mol-6jbbv
status: Approved
step_mapping:
    implement: mindspec-mol-wrx3o
    plan: mindspec-mol-0v8a9
    plan-approve: mindspec-mol-j7bv2
    review: mindspec-mol-5zvii
    spec: mindspec-mol-cek4d
    spec-approve: mindspec-mol-0vyys
    spec-lifecycle: mindspec-mol-6jbbv
---






# Spec 052-session-freshness-gate: Session Freshness Gate for Bead Starts

## Goal

Ensure every `mindspec next` invocation (including the first bead) begins with a clean context window by gating on session freshness rather than a flag set by the prior bead's completion. This eliminates the dependency on `mindspec complete` and covers all bead start scenarios — first bead after plan approval, sequential beads, and multi-agent handoffs.

## Background

Spec 047 introduced clean context enforcement with a `needs_clear` flag set by `mindspec complete` when a successor bead is ready. This approach has three confirmed failure modes:

1. **Bypass via `bd close`**: If beads are closed directly through the beads CLI instead of `mindspec complete`, the flag is never set. This was observed during the spec 051 implementation session where all three beads were worked in parallel and closed via `bd close`.

2. **First bead not covered**: The first implementation bead starts after spec drafting, plan discussions, and approval — all of which pollute the context window. Spec 047 only triggers on bead-to-bead transitions, so the first bead inherits a context full of irrelevant planning artifacts.

3. **Multi-agent parallel work**: When beads are worked concurrently by spawned agents, there is no sequential complete→next transition. Each agent starts fresh by design, but the flag mechanism assumes serial execution.

The fix is to invert the enforcement: instead of the *completion* path setting a flag, the *start* path checks session freshness. Claude Code's SessionStart hook provides a `source` field (`startup`, `clear`, `resume`, `compact`) that tells us exactly how the current session began. This is the only reliable signal available — context token counts are not accessible from hooks or CLI tools.

### Prior art

- **Spec 047 (clean-context-enforcement)**: Established the `needs_clear` flag, bead context primer, `--emit-only`, `--force`, and PreToolUse hook enforcement. The primer and flags remain valid; only the trigger mechanism changes.
- **ADR-0019 (three-layer enforcement)**: The hook-based enforcement pattern stays the same.
- **Claude Code SessionStart hook**: Receives JSON on stdin with `source` field and `session_id`. This is documented by Anthropic and is the basis for the new gate.

## Impacted Domains

- **agent-lifecycle**: Gate trigger changes from completion-driven to session-freshness-driven
- **state**: New `sessionSource` and `sessionStartedAt` fields; `beadClaimedAt` timestamp
- **instruct**: SessionStart hook updated to write session metadata to state

## ADR Touchpoints

- [ADR-0019](../../adr/ADR-0019.md): Three-layer enforcement — the PreToolUse hook layer remains; only the condition it checks changes

## Requirements

1. **SessionStart writes freshness metadata (R1)**: The SessionStart hook must parse the `source` field from stdin JSON and write `sessionSource` and `sessionStartedAt` to `.mindspec/state.json`. Valid clean sources are `startup` and `clear`.

2. **`mindspec next` records bead claim time (R2)**: When `mindspec next` successfully claims a bead, it must write `beadClaimedAt` to state.

3. **Session freshness gate (R3)**: `mindspec next` must refuse to proceed if `beadClaimedAt` exists and is newer than `sessionStartedAt` — meaning a bead was already claimed in this session without a `/clear` in between. If `sessionStartedAt` is missing (non-Claude-Code environment), the gate is skipped.

4. **First bead coverage (R4)**: The gate must also block the first bead if `sessionSource` is `resume` — meaning the session was resumed (not fresh). A session that started as `startup` with no prior `beadClaimedAt` passes.

5. **Remove completion-driven flag (R5)**: Remove the `needs_clear` flag-setting logic from `complete.go`. The `NeedsClear` field in state and the `state clear-flag` command can be removed.

6. **PreToolUse hook update (R6)**: The `needs-clear` hook in `dispatch.go` must check the session freshness ordering instead of the `NeedsClear` boolean.

7. **`--force` bypass preserved (R7)**: `mindspec next --force` continues to bypass the gate with a warning.

8. **Bead context primer unchanged (R8)**: The `BuildBeadPrimer`, `--emit-only`, and token estimation from spec 047 are unaffected.

## Scope

### In Scope

- `internal/state/state.go` — add `SessionSource`, `SessionStartedAt`, `BeadClaimedAt` fields; remove `NeedsClear`
- `internal/hook/dispatch.go` — update `NeedsClear()` to check session freshness ordering
- `internal/complete/complete.go` — remove `needs_clear` flag-setting logic (lines 149-155)
- `cmd/mindspec/next.go` — write `beadClaimedAt` on successful claim; CLI-level freshness gate
- `cmd/mindspec/state.go` — remove `clear-flag` subcommand; add `write-session` subcommand for SessionStart hook
- SessionStart hook command — update to parse stdin JSON and call `mindspec state write-session`

### Out of Scope

- Bead context primer changes (spec 047, working correctly)
- Automatic `/clear` invocation (not supported by Claude Code)
- Multi-agent orchestration (spawned agents start fresh by definition)
- Context token count monitoring (not available in hooks)

## Non-Goals

- **Detecting context pollution by other means**: We don't parse transcript files or estimate token usage. Session source is the invariant.
- **Backward compatibility with `needs_clear`**: The old flag is removed entirely. No migration path needed — it's internal state.

## Acceptance Criteria

- [ ] SessionStart hook writes `sessionSource` and `sessionStartedAt` to state
- [ ] `mindspec next` writes `beadClaimedAt` to state on successful bead claim
- [ ] `mindspec next` blocks when `beadClaimedAt > sessionStartedAt` (bead already claimed, no `/clear` since)
- [ ] `mindspec next` blocks when `sessionSource` is `resume` (first bead in a resumed session)
- [ ] `mindspec next` passes when `sessionSource` is `startup` or `clear` and no prior bead claimed
- [ ] `mindspec next --force` bypasses the gate with a warning
- [ ] `mindspec next` in non-Claude-Code environments (no `sessionStartedAt`) skips the gate
- [ ] `needs_clear` flag removed from state, `complete.go`, and `state clear-flag` command
- [ ] PreToolUse `needs-clear` hook checks session freshness ordering
- [ ] Bead context primer continues to work unchanged
- [ ] All existing tests pass (`make test`)
- [ ] New unit tests cover session freshness gate logic

## Validation Proofs

- `make test`: All tests pass
- `mindspec next` after fresh SessionStart (`source: startup`) with no prior bead → passes, claims bead
- `mindspec next` after claiming a bead without `/clear` → blocks with instruction to run `/clear`
- `mindspec next --force` after claiming a bead without `/clear` → proceeds with warning
- `mindspec next` after `/clear` (SessionStart `source: clear`) → passes, claims bead
- `mindspec next` in a resumed session (`source: resume`) → blocks

## Open Questions

- [x] Should the gate check context size/percentage instead of session source? **Decision: No. Context metrics are only available in the status line (display-only), not in hooks or CLI. Session source is the only reliable signal from hooks.**
- [x] Should we back out spec 047 entirely? **Decision: No. The bead context primer, `--emit-only`, `--force`, and token estimation are all valid. Only the trigger mechanism (how the gate fires) changes.**
- [x] Does this need an ADR? **Decision: No. This refines spec 047's enforcement mechanism, not a new architectural decision. ADR-0019 (three-layer enforcement) still applies.**

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-27
- **Notes**: Approved via mindspec approve spec