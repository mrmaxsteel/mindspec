---
approved_at: "2026-02-26T10:03:57Z"
approved_by: user
molecule_id: mindspec-mol-015
status: Approved
step_mapping:
    implement: mindspec-mol-9zt
    plan: mindspec-mol-9ws
    plan-approve: mindspec-mol-lq8
    review: mindspec-mol-e3m
    spec: mindspec-mol-qn6
    spec-approve: mindspec-mol-hfl
    spec-lifecycle: mindspec-mol-015
---









# Spec 047-clean-context-enforcement: Clean Context Enforcement for Bead Starts

## Goal

Ensure every agent starting a new implementation bead begins with a clean, focused context window — free of stale reasoning, resolved decisions, and file states from prior beads. This must work in both single-agent mode (one Claude Code session doing sequential beads) and multi-agent mode (team lead spawning fresh agents per bead).

## Background

When an agent implements multiple beads sequentially in a single session, its context window accumulates stale information: old file contents, resolved design decisions, debugging traces, and prior bead reasoning. This leads to:

- **Hallucinated state**: The agent references code patterns or decisions from bead N when working on bead N+1, even if they're unrelated
- **Context budget waste**: Thousands of tokens occupied by irrelevant prior work, reducing headroom for the current bead
- **Cross-contamination**: Reasoning about one subsystem leaks into another, producing subtly wrong implementations

In multi-agent mode (Claude Code agent teams), each spawned agent starts fresh by design — but still needs to be primed with the right context for the assigned bead. In single-agent mode, there is no mechanism to enforce or even encourage a context reset between beads.

The `mindspec complete` → `mindspec next` transition is the natural boundary between beads. This spec adds deterministic enforcement at that boundary.

### Prior art

- **Spec 046 (worktree enforcement)**: Established the pattern of three-layer enforcement (git hooks, CLI guards, agent hooks) for deterministic invariants
- **`mindspec instruct`**: Already emits mode-appropriate context at every state transition via the "instruct-tail" convention
- **`bd prime`**: Emits molecule status context, already called by SessionStart hook
- **Context packs** (`mindspec context`): Generate focused context documents for a spec — could be extended to bead-level granularity

## Impacted Domains

- **agent-lifecycle**: New enforcement layer for context hygiene at bead transitions
- **instruct**: Extended to emit bead-specific context primers
- **state**: New `needs_clear` flag to gate `mindspec next` in single-agent mode

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Zero-on-main — bead transitions already happen in worktrees; this spec adds context isolation on top of file isolation
- [ADR-0019](../../adr/ADR-0019.md): Three-layer enforcement — this spec adds a fourth concern (context hygiene) enforced through the same three layers

## Requirements

1. **Single-agent clear gate (R1)**: When `mindspec complete` closes a bead and another bead is ready, the CLI must set a `needs_clear` flag in state. `mindspec next` must refuse to proceed while `needs_clear` is set. The flag is cleared by the SessionStart hook (which runs after `/clear` resets the session).
2. **Bead context primer (R2)**: `mindspec next` must emit a focused context primer for the claimed bead, including: the bead's title and description (from beads), the relevant slice of the spec (requirements/acceptance criteria), the relevant slice of the plan (work chunk with verification steps), key file paths with line numbers the bead will touch, and cited ADR decisions. This replaces the generic implement-mode instruct output with bead-specific context.
3. **Multi-agent context handoff (R3)**: `mindspec next --emit-only` outputs the primer to stdout without claiming the bead, so a team lead can pass it to a spawned agent. Auto-selects the next ready bead by default; accepts an explicit bead ID if passed.
4. **Hook enforcement for clear gate (R4)**: A Claude Code `PreToolUse` hook on `Bash` commands matching `mindspec next` must block execution when `needs_clear` is set, emitting a message instructing the agent to run `/clear` first.
5. **Graceful degradation (R5)**: `mindspec next --force` bypasses the clear gate with a warning for non-Claude-Code environments or CI. The bead context primer is still emitted.
6. **Context budget estimation (R6)**: The bead context primer includes an estimated token count so agents and humans can gauge remaining context budget for implementation work.

## Scope

### In Scope

- `internal/state/state.go` — `needs_clear` field
- `internal/instruct/` — bead context primer builder
- `internal/next/` — clear gate check, `--emit-only` flag, `--force` flag
- `internal/complete/` — set `needs_clear` on successful close when next bead exists
- `.claude/settings.json` — PreToolUse hook for clear gate
- `cmd/mindspec/next.go` — new flags and primer output
- `internal/instruct/templates/` — bead primer template

### Out of Scope

- Automatic `/clear` invocation (Claude Code does not support programmatic `/clear`)
- Changes to beads CLI itself
- Context pack generation (existing `mindspec context` is a different, spec-level tool)
- Multi-agent orchestration logic (that's the team lead's responsibility)

## Non-Goals

- **Automatic context compression**: We don't try to summarize prior bead work — we enforce a clean break
- **Partial context reuse**: We don't try to identify which parts of prior context are still relevant — a full clear is simpler and more reliable
- **Agent memory persistence**: Cross-bead learnings should go into memory files (MEMORY.md), not LLM context

## Acceptance Criteria

- [ ] `mindspec complete` sets `needs_clear: true` in state when another bead is ready
- [ ] `mindspec next` refuses to proceed when `needs_clear` is set (exits with error and instruction to `/clear`)
- [ ] `mindspec next --force` bypasses the clear gate with a warning
- [ ] After `/clear` + SessionStart hook, `needs_clear` is reset to false
- [ ] `mindspec next` emits a bead-specific context primer (bead description, spec slice, plan slice, file paths, ADR citations)
- [ ] `mindspec next --emit-only` outputs the primer without claiming the bead
- [ ] PreToolUse hook blocks `mindspec next` when `needs_clear` is set
- [ ] Bead context primer includes estimated token count
- [ ] All existing tests pass (`make test`)
- [ ] New unit tests cover clear gate logic and primer generation

## Validation Proofs

- `make test`: All tests pass
- `mindspec complete` on a bead with a successor → `state.json` shows `needs_clear: true`
- `mindspec next` with `needs_clear: true` → exits with error message
- `mindspec next --force` with `needs_clear: true` → proceeds with warning
- `mindspec next` after clear gate reset → emits bead context primer and claims bead
- `mindspec next --emit-only` → outputs primer JSON to stdout, bead remains unclaimed

## Open Questions

- [x] Should the clear gate apply to all mode transitions or only implement→implement (sequential beads)? **Decision: Only implement→implement. Other transitions (e.g., spec→plan) naturally involve different work and the instruct system already handles them.**
- [x] Should the bead context primer be markdown or JSON? **Decision: Markdown for human/agent readability (matches instruct convention), with `--json` flag for programmatic use.**
- [x] Should `mindspec next --emit-only` require the bead to be specified, or auto-select the next ready bead? **Decision: Auto-select by default, but accept an explicit bead ID if passed.**
- [x] How should the primer handle beads that span multiple files/packages — should it include file content snippets or just paths? **Decision: Paths with line numbers only. The agent reads files as needed — don't burn context budget on pre-loaded content.**

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-26
- **Notes**: Approved via mindspec approve spec