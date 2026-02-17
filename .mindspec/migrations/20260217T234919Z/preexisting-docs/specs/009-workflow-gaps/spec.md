# Spec 009: Workflow Happy-Path Gap Fixes

## Goal

Close the gaps identified in the [happy-path review](../../happy-path.md) so that the end-to-end MindSpec development workflow — from `/spec-init` through `mindspec complete` — works without the agent needing undocumented tribal knowledge or manual workarounds. Then strip back the static instruction files (`CLAUDE.md`, `AGENTS.md`, `.claude/rules/*`, `.claude/commands/*`) to minimal bootstraps, delegating all mode-specific guidance to `mindspec instruct`.

## Background

A dogfooding review of the MindSpec workflow (documented in `docs/happy-path.md`) found 8 gaps between what the documentation/CLI promises and what actually happens. The most critical is that bead creation is never orchestrated automatically — the approve commands assume beads exist but nothing creates them. The remaining gaps range from broken parsing to misleading error messages.

Beyond the bugs, there's a structural problem: the static instruction files (`CLAUDE.md`, `AGENTS.md`, `.claude/rules/mindspec-modes.md`) contain ~250 lines of mode rules, Beads conventions, git workflow, and governance that `mindspec instruct` already emits dynamically per-mode. Every session loads all of it into context regardless of which mode is active — wasting tokens and creating two sources of truth that can drift apart. The design philosophy (Goal #8, ADR-0003) says `mindspec instruct` should be the single source of agent context. Once the CLI reliably handles everything (R1–R8), we can strip the static files to minimal bootstraps.

## Impacted Domains

- **workflow**: Approval commands (`approve spec`, `approve plan`) gain bead creation side-effects; `complete` error message fix; idle template directive; static instruction files stripped to bootstraps
- **beads**: Spec/plan bead creation becomes part of the approval flow rather than a standalone step
- **instruct**: Idle template gets a `## Next Action` section; worktree check suppressed for fresh claims; becomes the authoritative source for all agent guidance (replacing static files)

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Beads as passive tracking substrate — bead creation is moved into approval commands, keeping Beads as the execution layer
- [ADR-0003](../../adr/ADR-0003.md): Centralized Agent Instruction Emission — idle template fix ensures all modes meet the directive standard

## Requirements

1. Automate bead creation in approval commands (R1)
2. Fix spec ID parsing in `ResolveMode` (R2)
3. Suppress worktree warning on fresh claim (R3)
4. Generate context pack on spec approval (R4)
5. Accept `--approved-by` flag in approval commands (R5)
6. Fix "stash" error message (R6)
7. Add `## Next Action` directive to idle template (R7)
8. Document milestone commit convention as agent-training-only (R8)
9. Strip static instruction files to minimal bootstraps (R9)

### R1: Automate bead creation in approval commands (Gap #1)

`mindspec approve spec <id>` must call `CreateSpecBead()` (creating the spec bead and spec gate) before resolving the gate. `mindspec approve plan <id>` must call `CreatePlanBeads()` (creating molecule parent, plan gate, impl beads) and `WriteGeneratedBeadIDs()` before resolving the plan gate. Both are idempotent, so re-running is safe. Un-deprecate `mindspec bead spec` and `mindspec bead plan` as they remain useful for manual/debug use — just remove the deprecation notice and the false claim that approval calls them automatically.

### R2: Fix spec ID parsing in `ResolveMode` (Gap #2)

`parseSpecID()` in `internal/next/mode.go` looks for a colon, but the title convention is `[IMPL 009-feature.1] Chunk title` (no colon). Update the parser to extract the spec ID from the `[IMPL <specID>.<chunk>]` bracket prefix. Fall back to the existing colon-based parsing for backward compatibility with older bead titles.

### R3: Suppress worktree warning on fresh claim (Gap #3)

`CheckWorktree()` in `internal/instruct/worktree.go` fires a false alarm when called from the instruct-tail of `mindspec next`, because the CWD is still the main repo even though the worktree was just created. Either: (a) skip the worktree check when called from `next` (pass a flag/context), or (b) check whether the worktree *exists* (not whether CWD matches it) and adjust the message to say "switch to worktree-X" rather than implying it's missing.

### R4: Generate context pack on spec approval (Gap #4)

`mindspec approve spec <id>` should call `contextpack.Build()` and `WriteToFile()` after approving the spec, so the context pack is ready for Plan Mode. Best-effort — warn on failure, don't block approval.

### R5: Accept `--approved-by` flag in approval commands (Gap #5)

Add an `--approved-by` flag to both `approve spec` and `approve plan` commands. Default to `"user"` (preserving current behavior) but allow passing an identity string.

### R6: Fix "stash" error message (Gap #6)

Change the error message in `internal/complete/complete.go` line 149 from "commit or stash before completing" to "commit before completing" to match the project convention in AGENTS.md that forbids auto-stash.

### R7: Add `## Next Action` directive to idle template (Gap #7)

Add a `## Next Action` section to `internal/instruct/templates/idle.md` directing the agent to greet the user and suggest `/spec-init`, resuming an existing spec, or `mindspec doctor`. Matches the convention in spec/plan/implement templates.

### R8: Document milestone commit convention as agent-training-only (Gap #8)

No code change. Add a note to `docs/core/CONVENTIONS.md` clarifying that milestone commits are an agent convention, not enforced by CLI tooling. This makes the gap explicit rather than leaving it as an undocumented assumption.

### R9: Strip static instruction files to minimal bootstraps

Once R1–R8 make `mindspec instruct` fully self-sufficient, strip the static files down:

- **`CLAUDE.md`**: Keep only project identity ("this is MindSpec"), build/test commands (`make build`, `make test`), and a pointer to `mindspec instruct`. Remove the behavioral rules summary, key files table, project layout tree, and full CLI reference — all of which `mindspec instruct` or `mindspec --help` already provide.
- **`AGENTS.md`**: Reduce to a short human-readable reference (not agent-consumed). Move the "Landing the Plane" session-close protocol into an instruct template or `mindspec complete` output. The mode rules, Beads conventions, git workflow, ADR governance, and doc-sync rules are all emitted by `mindspec instruct` per-mode — they don't need to be in a static file that loads every session.
- **`.claude/rules/mindspec-modes.md`**: Delete. The SessionStart hook already runs `mindspec instruct`. The "offline fallback" note is unnecessary — if `mindspec instruct` fails, the agent can run it manually.
- **`.claude/commands/spec-approve.md`**: Reduce to a thin shim — confirm with user, run `mindspec approve spec <id>`. Remove step-by-step procedural detail (the CLI now handles everything).
- **`.claude/commands/plan-approve.md`**: Same — thin shim.
- **`.claude/commands/spec-init.md`**: Keep for now (scaffolding logic isn't in the CLI yet), but note it as a candidate for a future `mindspec init spec` command.
- **`.claude/commands/spec-status.md`**: Already thin. Keep as-is.

## Scope

### In Scope

- `internal/approve/spec.go` — call `CreateSpecBead()`, call `contextpack.Build()`, accept `--approved-by`
- `internal/approve/plan.go` — call `CreatePlanBeads()` + `WriteGeneratedBeadIDs()`, accept `--approved-by`
- `cmd/mindspec/approve.go` — wire `--approved-by` flag, pass to approve functions
- `cmd/mindspec/bead.go` — remove `Deprecated` notices from `bead spec` and `bead plan`
- `internal/next/mode.go` — fix `parseSpecID()` for bracket-prefix titles
- `internal/instruct/worktree.go` — fix false alarm on fresh worktree claim
- `internal/instruct/templates/idle.md` — add `## Next Action` directive
- `internal/complete/complete.go` — fix "stash" error message
- `docs/core/CONVENTIONS.md` — document milestone commit convention status
- `CLAUDE.md` — strip to minimal bootstrap (project identity + build commands + pointer to `mindspec instruct`)
- `AGENTS.md` — reduce to short human-readable reference; move session-close protocol to instruct/CLI
- `.claude/rules/mindspec-modes.md` — delete
- `.claude/commands/spec-approve.md` — reduce to thin shim
- `.claude/commands/plan-approve.md` — reduce to thin shim

### Out of Scope

- Automating milestone commits in CLI commands (deliberate — training-only)
- Changes to plan/spec/implement templates beyond idle.md (the existing spec/plan/implement templates already contain the guidance that static files duplicate)
- New CLI commands or subcommands
- `.claude/commands/spec-init.md` — stays as-is (scaffolding not yet in CLI)
- `.claude/commands/spec-status.md` — already thin, stays as-is

## Non-Goals

- Full identity/auth system for `--approved-by` — it's a plain string, not verified
- Making bead creation mandatory (approve commands should still succeed if `bd` is unavailable — best-effort with warnings)
- Rewriting the worktree check architecture — just fix the false alarm
- Deleting `AGENTS.md` entirely — it remains as a human-readable reference, just not agent-consumed
- Moving `spec-init` scaffolding into the CLI — that's a separate spec (candidate for 010+)

## Acceptance Criteria

- [ ] `mindspec approve spec <id>` creates the spec bead + gate if they don't exist, then resolves the gate
- [ ] `mindspec approve plan <id>` creates molecule parent + plan gate + impl beads if they don't exist, writes bead IDs to plan frontmatter, then resolves the gate
- [ ] Both approve commands succeed with warnings (not errors) when `bd` is unavailable
- [ ] `mindspec bead spec` and `mindspec bead plan` no longer show deprecation warnings
- [ ] `parseSpecID("[IMPL 009-feature.1] Chunk title")` returns `"009-feature"`
- [ ] `parseSpecID("005-next: Old style title")` still returns `"005-next"` (backward compat)
- [ ] `mindspec next` followed by instruct-tail does not emit a worktree mismatch warning for the just-created worktree
- [ ] `mindspec approve spec <id>` generates `context-pack.md` in the spec directory
- [ ] `mindspec approve spec <id> --approved-by=max` records `Approved By: max` in spec frontmatter
- [ ] `mindspec approve plan <id> --approved-by=max` records `approved_by: max` in plan frontmatter
- [ ] `mindspec complete` with uncommitted changes says "commit before completing" (no mention of stash)
- [ ] `./bin/mindspec instruct` in idle state includes a `## Next Action` directive
- [ ] `docs/core/CONVENTIONS.md` documents that milestone commits are agent convention, not CLI-enforced
- [ ] `CLAUDE.md` is under 30 lines and contains no mode rules, no project layout tree, no CLI reference beyond build/test
- [ ] `AGENTS.md` is clearly marked as human reference (not agent-consumed) and does not duplicate `mindspec instruct` output
- [ ] `.claude/rules/mindspec-modes.md` is deleted
- [ ] `.claude/commands/spec-approve.md` is under 15 lines (confirm + run command)
- [ ] `.claude/commands/plan-approve.md` is under 15 lines (confirm + run command)
- [ ] Session-close protocol ("Landing the Plane") is emitted by `mindspec instruct` or `mindspec complete`, not only in `AGENTS.md`

## Validation Proofs

- `make build`: Succeeds
- `make test`: All tests pass
- `./bin/mindspec approve spec <test-id>`: Creates bead + gate, generates context pack, approves spec
- `./bin/mindspec approve plan <test-id>`: Creates impl beads, writes IDs to plan, approves plan
- `./bin/mindspec instruct` (idle state): Output includes `## Next Action` directive
- `wc -l CLAUDE.md`: Under 30 lines
- `cat .claude/rules/mindspec-modes.md`: File not found (deleted)

## Open Questions

(none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via mindspec approve spec