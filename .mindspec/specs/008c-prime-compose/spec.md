# Spec 008c: Compose `bd prime` into `mindspec instruct`

## Goal

Unify the two agent context sources — `mindspec instruct` (spec-driven mode guidance, ~1k tokens) and `bd prime` (Beads workflow context, ~3k tokens) — into a single emission from `mindspec instruct`. Agents get one coherent context block instead of two disjoint ones, and MindSpec can curate which Beads context is relevant per mode.

## Background

### Current state

Agents receive context from two separate SessionStart hooks:

1. **`mindspec instruct`** (MindSpec hook) — emits mode-appropriate guidance: permitted/forbidden actions, obligations, human gates, commit conventions, next action. Tailored to current state (idle/spec/plan/implement). ~1k tokens.

2. **`bd prime`** (Beads hook) — emits generic Beads workflow context: session close protocol, command reference, common workflows. Always the same content regardless of mode. ~3k tokens.

This dual-injection creates several issues:

- **Redundancy**: `mindspec instruct` already mentions Beads commands (`bd update`, `bd close`, `mindspec complete`) but `bd prime` provides a full command reference. The agent sees overlapping instructions.
- **Missed curation**: In Spec Mode, the agent doesn't need `bd create` or `bd dep add` references. In Implementation Mode, it doesn't need the "Creating dependent work" workflow. `bd prime` emits everything regardless.
- **Two hooks, one concern**: Both hooks serve the same purpose (prime the agent's operating context). Having two is an artifact of MindSpec and Beads being separate tools — but MindSpec is the orchestrator (ADR-0003), so it should own the combined emission.
- **Instruct-tail gap**: When `mindspec approve/next/complete` emit instruct-tail output, they only emit MindSpec guidance — the Beads context is missing from those transitions. The agent loses Beads awareness mid-session after a mode change.

### Design principle

Per Goal #8 (CLI-first, minimal IDE glue) and ADR-0003 (MindSpec owns instruction emission), `mindspec instruct` should be the single source of agent context. It already owns mode guidance — extending it to include curated Beads context is a natural next step.

## Impacted Domains

- **workflow**: `mindspec instruct` output changes shape — now includes Beads section
- **agent-interface**: agents receive unified context from one source instead of two

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Beads as passive substrate. MindSpec curates what Beads context to surface — Beads doesn't change.
- [ADR-0003](../../adr/ADR-0003.md): Centralized instruction emission. This spec extends the "single source of truth" principle to include Beads workflow context.

## Requirements

1. **`mindspec instruct` includes Beads context** — The output of `mindspec instruct` includes a "Beads Workflow Context" section alongside the existing mode guidance. This section provides the essential Beads commands and conventions the agent needs.

2. **`bd prime` captured at runtime** — `mindspec instruct` shells out to `bd prime` to capture the Beads context. This ensures the content stays in sync with the installed Beads version and any project-specific PRIME.md overrides.

3. **Graceful degradation** — If `bd prime` is unavailable (Beads not installed, not initialized), `mindspec instruct` emits a brief warning but continues with MindSpec-only guidance. Beads context is additive, not required.

4. **Session close protocol preserved** — The Beads session close protocol (`bd sync --flush-only`) must remain visible in the composed output. This is critical for data durability.

5. **Instruct-tail includes Beads context** — Since `emitInstruct()` is the shared helper for approve/next/complete, composing Beads context into `instruct` automatically gives it to all state-changing commands via the instruct-tail convention.

6. **Single SessionStart hook** — After this change, only the `mindspec instruct` hook is needed. The separate `bd prime` hook should be documented as removable (MindSpec subsumes it).

7. **Token budget awareness** — The combined output should stay under ~4k tokens. `bd prime` is already compact (~3k), and `mindspec instruct` is ~1k. If both are simply concatenated, that's ~4k — acceptable. No truncation logic is needed in v1, but the spec acknowledges this as a future concern if either grows.

## Scope

### In Scope
- `internal/instruct/instruct.go` — add `bd prime` capture and compose into output
- `internal/instruct/prime.go` — new file for `bd prime` execution and output capture
- `cmd/mindspec/instruct.go` — no changes expected (composition happens in the library)
- `cmd/mindspec/instruct_tail.go` — no changes expected (uses `emitInstruct()` which calls `Render()`)
- `.claude/settings.json` — document that `bd prime` hook can be removed
- Doc-sync: `CLAUDE.md`, `docs/core/CONVENTIONS.md`

### Out of Scope
- Mode-aware filtering of `bd prime` content (e.g., hiding `bd create` in Spec Mode) — future enhancement
- Modifying `bd prime` output format — Beads is upstream
- Changing the `--format=json` output structure — keep backward-compatible

## Non-Goals

- Replacing `bd prime` as a standalone command — it remains useful outside MindSpec
- Making MindSpec depend on specific `bd prime` output structure — treat it as opaque text
- Token-budget truncation logic — not needed at current sizes

## Acceptance Criteria

- [ ] `mindspec instruct` output includes a Beads workflow section when `bd prime` is available
- [ ] `mindspec instruct` works without error when `bd prime` is unavailable (graceful degradation)
- [ ] Session close protocol (`bd sync --flush-only`) appears in the composed output
- [ ] `mindspec instruct --format=json` still returns valid JSON (Beads context in a new field or appended to guidance)
- [ ] Instruct-tail commands (`approve`, `next`, `complete`) emit Beads context as part of their output
- [ ] Combined output stays under ~5k tokens for a typical project
- [ ] `make test` passes
- [ ] Doc-sync: CLAUDE.md and CONVENTIONS.md updated

## Validation Proofs

- `./bin/mindspec instruct`: Output contains both mode guidance and Beads workflow context sections
- `./bin/mindspec instruct` (with Beads not initialized): Output contains mode guidance only, with a warning about missing Beads context
- `./bin/mindspec instruct --format=json`: Valid JSON with Beads context included
- `make test`: All tests pass

## Open Questions

None.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via mindspec approve spec