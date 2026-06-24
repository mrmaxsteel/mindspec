# Spec 010: `mindspec spec-init` CLI Command

## Goal

Move spec-initialization logic from the fat `.claude/commands/spec-init.md` skill file into a `mindspec spec-init <id>` CLI command, so the skill becomes a thin shim that calls the CLI. This follows the CLI-first convention established by the approve commands.

## Background

The current `/spec-init` skill is 77 lines of step-by-step instructions that the agent interprets at runtime: create directory, copy template, set state, inform user. Every other workflow operation (approve spec/plan/impl, complete, next) already lives in Go and is invoked via a thin skill shim. `spec-init` is the last holdout.

The command is named `spec-init` (not `init`) because `mindspec init` is reserved for future agent/IDE initialization.

## Impacted Domains

- cli: New `spec-init` subcommand on the root command
- state: Reuses existing `state.SetMode()` — no changes needed
- instruct: Emits instruct-tail after init (existing convention)

## ADR Touchpoints

None — follows existing patterns, no architectural divergence.

## Requirements

1. `mindspec spec-init <id>` creates `docs/specs/<id>/spec.md` from the template, filling in ID and title placeholders
2. Sets state to spec mode with the new spec as active (`mode=spec, activeSpec=<id>`)
3. Emits instruct-tail so the agent receives spec-mode guidance
4. Fails gracefully if the spec directory already exists (no clobbering)
5. Derives title from the slug portion of the ID (e.g. `010-spec-init-cmd` → "Spec Init Cmd") unless a `--title` flag is provided
6. After migration, `.claude/commands/spec-init.md` becomes a thin shim that calls `mindspec spec-init <id>`

## Scope

### In Scope
- `cmd/mindspec/spec_init.go` — new subcommand
- `internal/specinit/` — init logic (template copy, placeholder fill, state set)
- `.claude/commands/spec-init.md` — rewrite to thin shim
- `docs/templates/spec.md` — unchanged, read as-is

### Out of Scope
- `mindspec init` (agent/IDE initialization — future spec)
- Context-pack generation at init time (stays as-is: generated later)
- Changes to the spec template itself
- `/spec-status` slimming (separate effort)

## Non-Goals

- Validating spec content at init time (that's `mindspec validate spec`)
- Auto-generating context packs (happens at `/spec-approve`)
- Interactive prompting within the CLI (the skill handles user interaction)

## Acceptance Criteria

- [ ] `mindspec spec-init 010-foo` creates `docs/specs/010-foo/spec.md` with ID and title filled in
- [ ] State is set to `mode=spec, activeSpec=010-foo` after successful init
- [ ] CLI output includes instruct-tail (spec-mode guidance)
- [ ] Running against an existing spec directory produces an error, not a clobber
- [ ] `--title "Custom Title"` overrides the slug-derived title
- [ ] `.claude/commands/spec-init.md` is reduced to a thin shim (<15 lines)
- [ ] `make test` passes with new tests covering the init logic

## Validation Proofs

- `mindspec spec-init 999-test-spec`: Creates directory and spec.md, prints spec-mode guidance
- `mindspec spec-init 999-test-spec` (second run): Fails with "already exists" error
- `mindspec state show`: Shows `mode=spec, activeSpec=999-test-spec`
- `make test`: All tests pass

## Open Questions

None.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via mindspec approve spec