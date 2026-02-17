# Spec 004: Mode-Aware Guidance Emission (`mindspec instruct`)

## Goal

Replace static agent instruction files (AGENTS.md, CLAUDE.md) with a dynamic CLI command that emits authoritative, mode-appropriate operating guidance based on explicit MindSpec state. Agents run `mindspec instruct` at session start and receive exactly the rules, gates, and context relevant to their current mode and active work — no more, no less.

## Background

Today, agent guidance lives in static markdown files (AGENTS.md, CLAUDE.md, `.claude/rules/`). These files contain the full rule set for all three modes, creating several problems identified in ADR-0003:

- **Drift**: multiple instruction sources diverge over time
- **Mode ambiguity**: static files can't reflect "you are currently in Plan Mode working on spec 003"
- **Token waste**: agents load rules for all modes when they only need one
- **Tool coupling**: different agent runtimes need different file formats

ADR-0003 decided that MindSpec will emit guidance dynamically via `mindspec instruct`, with static files reduced to a minimal bootstrap.

A prototype `.mindspec/current-spec.json` already exists with `mode`, `activeSpec`, and `lastUpdated` fields. This spec promotes that into a governed state file that the CLI reads and mode transitions write.

## Impacted Domains

- **workflow**: Mode state tracking, mode detection logic, mode-specific rule emission
- **context-system**: Guidance assembly is a form of context delivery (parallels context pack assembly)
- **core**: CLAUDE.md and AGENTS.md will be reduced to bootstrap stubs

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): Centralized Agent Instruction Emission — this spec implements the `instruct` command defined there
- [ADR-0002](../../adr/ADR-0002.md): Beads Integration Strategy — `instruct` reads Beads state to determine active work
- [ADR-0001](../../adr/ADR-0001.md): DDD Enablement — guidance may reference impacted domains

## Requirements

### State Tracking

1. **Explicit state file**: `.mindspec/state.json` is the primary source of truth for current mode and active work. Schema:
   ```json
   {
     "mode": "idle|spec|plan|implement",
     "activeSpec": "004-instruct",
     "activeBead": "beads-xxx",
     "lastUpdated": "2026-02-12T10:00:00Z"
   }
   ```
2. **State writes at transitions**: Mode transitions (`/spec-init`, `/spec-approve`, `/plan-approve`, bead completion) update `state.json`
3. **Cross-validation**: `instruct` reads `state.json` as primary signal, then cross-validates against artifact state (spec approval status, plan frontmatter, Beads in-progress). If they disagree, emit a warning with recovery guidance rather than silently trusting stale state
4. **Worktree check**: In Implementation Mode, verify the current git worktree matches the active bead's expected worktree (per naming convention `worktree-<bead-id>`). If mismatched, emit a prominent warning with the correct worktree path before any guidance

### Guidance Emission

4. **Mode-specific guidance**: Output only the rules, permitted actions, forbidden actions, and gates relevant to the detected mode
5. **Active work context**: Include the specific spec/bead being worked on, its status, and next expected actions
6. **Hard gates**: Clearly emit the human-in-the-loop gates that apply in the current mode
7. **Read-only**: `instruct` itself is read-only — it reads state but never modifies it. State writes happen in the transition commands.
8. **Machine-readable option**: Support `--format=json` for structured output alongside the default markdown
9. **Graceful degradation**: If `state.json` is missing or Beads is unavailable, infer mode from artifact state with a warning that state file should be initialized

## Scope

### In Scope

- `internal/state/` package: read/write `.mindspec/state.json`, cross-validation logic
- `internal/instruct/` package: guidance assembly, rendering
- `cmd/mindspec/instruct.go`: replace the stub in `cmd/mindspec/stubs.go`
- State file writes integrated into existing transition points (spec-init, spec-approve, plan-approve skill hooks)
- Guidance templates stored in-repo (Go embedded files or string constants)
- Markdown and JSON output formats
- Tests covering state read/write, cross-validation, and each mode's guidance output

### Out of Scope

- Modifying CLAUDE.md/AGENTS.md to bootstrap stubs (deferred to a follow-up once `instruct` is proven)
- `mindspec next` (Spec 005) and `mindspec validate` (Spec 006)
- Token budgeting or deduplication (that's context packs, not guidance)
- Worktree creation or management
- Migration of the existing `.mindspec/current-spec.json` (it will be superseded by `state.json`)

## Non-Goals

- Replacing context packs — `instruct` emits operational rules, not domain/architectural context
- Auto-claiming or modifying Beads state (that's `mindspec next`)
- Supporting multiple concurrent active work items (v1 assumes one active bead at a time)

## Acceptance Criteria

### State Tracking
- [ ] `.mindspec/state.json` is created/updated by `/spec-init` with `mode: "spec"` and the active spec ID
- [ ] `/spec-approve` updates `state.json` to `mode: "plan"`
- [ ] `/plan-approve` updates `state.json` to `mode: "implement"` with the active bead ID
- [ ] `state.json` is committed to git (project-level workflow state, not personal)

### Guidance Emission
- [ ] `mindspec instruct` with `state.json` missing falls back to artifact-based inference with a warning
- [ ] `mindspec instruct` in idle state (no `state.json` or `mode: "idle"`) emits Idle guidance listing available specs
- [ ] `mindspec instruct` in spec mode emits Spec Mode guidance naming the active spec
- [ ] `mindspec instruct` in plan mode emits Plan Mode guidance naming the spec and plan status
- [ ] `mindspec instruct` in implement mode emits Implementation Mode guidance naming the active bead and its verification steps
- [ ] Guidance for each mode includes: permitted actions, forbidden actions, applicable gates, and next expected action
- [ ] When `state.json` disagrees with artifact state, output includes a drift warning with suggested fix
- [ ] In implement mode, if the current worktree doesn't match the active bead, output includes a worktree mismatch warning with the correct worktree path

### Format and Quality
- [ ] `mindspec instruct --format=json` produces valid JSON with fields: `mode`, `active_spec`, `active_bead`, `guidance`, `gates`, `warnings`
- [ ] `make test` passes with tests covering state read/write, cross-validation, and all four mode guidance paths
- [ ] Existing `mindspec doctor`, `glossary`, and `context` commands are unaffected

## Validation Proofs

- `make build && ./bin/mindspec instruct`: Emits mode-appropriate guidance based on `state.json`
- `./bin/mindspec instruct --format=json | jq .mode`: Returns one of `idle`, `spec`, `plan`, `implement`
- `rm .mindspec/state.json && ./bin/mindspec instruct`: Falls back gracefully with a warning
- `make test`: All tests pass including state and instruct tests

## Open Questions

None — all resolved.

## Design Decisions (resolved during spec)

- **Guidance templates**: Go `embed` files (`.md` files in-repo). Easier to edit and review as plain markdown.
- **Beads query**: Shell out to `bd` CLI. Simpler, avoids coupling to Beads internals, and keeps Beads as an opaque substrate per ADR-0002.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-12
- **Notes**: Approved via /spec-approve workflow
