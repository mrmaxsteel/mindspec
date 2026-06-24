---
adr_citations:
    - id: ADR-0002
      sections:
        - Beads as passive tracking substrate
    - id: ADR-0003
      sections:
        - Centralized instruction emission
        - Minimal bootstrap in repo-facing files
approved_at: "2026-02-13T13:49:47Z"
approved_by: user
last_updated: 2026-02-13T00:00:00Z
spec_id: 009-workflow-gaps
status: Approved
version: "0.1"
work_chunks:
    - depends_on: []
      id: 1
      scope: internal/next/mode.go, internal/instruct/worktree.go, internal/complete/complete.go
      title: Small surgical fixes (R2, R3, R6)
      verify:
        - '`parseSpecID("[IMPL 009-feature.1] Chunk title")` returns `"009-feature"`'
        - '`parseSpecID("005-next: Old style title")` returns `"005-next"` (backward compat)'
        - '`parseSpecID("[SPEC 008b-gates] Feature")` returns `"008b-gates"`'
        - Worktree check does not emit mismatch warning when worktree exists but CWD differs
        - complete.go error message says "commit before completing" with no mention of stash
        - '`make test` passes'
    - depends_on: []
      id: 2
      scope: internal/approve/spec.go, internal/approve/plan.go, cmd/mindspec/approve.go, cmd/mindspec/bead.go
      title: Approval command enhancements (R1, R4, R5)
      verify:
        - '`approve spec` calls `CreateSpecBead()` before resolving gate — spec bead + gate exist after approval'
        - '`approve plan` calls `CreatePlanBeads()` + `WriteGeneratedBeadIDs()` before resolving gate — molecule + impl beads exist after approval'
        - Both approve commands succeed with warnings (not errors) when `bd` is unavailable
        - '`approve spec` generates `context-pack.md` in the spec directory (best-effort)'
        - '`approve spec --approved-by=max` records `Approved By: max` in spec frontmatter'
        - '`approve plan --approved-by=max` records `approved_by: max` in plan frontmatter'
        - '`mindspec bead spec` and `mindspec bead plan` show no deprecation warnings'
        - '`make test` passes'
    - depends_on: []
      id: 3
      scope: internal/instruct/templates/idle.md, docs/core/CONVENTIONS.md
      title: Idle template directive + docs (R7, R8)
      verify:
        - '`./bin/mindspec instruct` in idle state includes `## Next Action` directive'
        - Directive mentions `/spec-init`, resuming a spec, and `mindspec doctor`
        - CONVENTIONS.md documents milestone commits as agent convention, not CLI-enforced
        - '`make build` succeeds'
    - depends_on:
        - 1
        - 2
        - 3
      id: 4
      scope: CLAUDE.md, AGENTS.md, .claude/rules/mindspec-modes.md, .claude/commands/spec-approve.md, .claude/commands/plan-approve.md, internal/instruct/templates/*.md
      title: Strip static instruction files to minimal bootstraps (R9)
      verify:
        - CLAUDE.md is under 30 lines — contains project identity, build/test commands, pointer to `mindspec instruct`
        - AGENTS.md is marked as human reference, does not duplicate instruct output
        - '`.claude/rules/mindspec-modes.md` is deleted'
        - '`.claude/commands/spec-approve.md` is under 15 lines'
        - '`.claude/commands/plan-approve.md` is under 15 lines'
        - Session-close protocol is emitted by `mindspec instruct` or `mindspec complete`
        - '`make build` succeeds'
        - Starting a fresh session with `mindspec instruct` still provides complete agent guidance
---

# Plan: Spec 009 — Workflow Happy-Path Gap Fixes

**Spec**: [spec.md](spec.md)

## Bead 1: Small surgical fixes (R2, R3, R6)

**Scope**: `internal/next/mode.go`, `internal/instruct/worktree.go`, `internal/complete/complete.go`

**Steps**

1. In `internal/next/mode.go`, rewrite `parseSpecID()` to first try bracket-prefix parsing: find `[` and `]`, extract the tag (IMPL/SPEC) and spec ID (portion after tag, before dot or closing bracket)
2. Keep the existing colon-based parsing as a fallback when no bracket prefix is found
3. In `internal/instruct/worktree.go`, change `CheckWorktree()` to check whether the named worktree exists (via `bead.WorktreeList()`) rather than requiring CWD to match it
4. If the worktree exists but CWD differs, emit an informational message ("switch to worktree-X to begin work") instead of a warning implying it's missing
5. In `internal/complete/complete.go`, change "commit or stash before completing" to "commit before completing"
6. Run `make test` to verify all changes

**Verification**

- [ ] `parseSpecID("[IMPL 009-feature.1] Chunk title")` returns `"009-feature"`
- [ ] `parseSpecID("005-next: Old style title")` returns `"005-next"` (backward compat)
- [ ] `parseSpecID("[SPEC 008b-gates] Feature")` returns `"008b-gates"`
- [ ] Worktree check does not emit mismatch warning when worktree exists but CWD differs
- [ ] complete.go error message says "commit before completing" with no mention of stash
- [ ] `make test` passes

**Depends on**: None

## Bead 2: Approval command enhancements (R1, R4, R5)

**Scope**: `internal/approve/spec.go`, `internal/approve/plan.go`, `cmd/mindspec/approve.go`, `cmd/mindspec/bead.go`

**Steps**

1. In `internal/approve/spec.go` `ApproveSpec()`, add a call to `bead.CreateSpecBead(root, specID)` before the gate resolution step — best-effort, warn on failure
2. In `internal/approve/spec.go` `ApproveSpec()`, after updating frontmatter, call `contextpack.Build()` and `WriteToFile()` — best-effort, warn on failure
3. Update `ApproveSpec()` signature to accept `approvedBy string` parameter, pass it through to `updateSpecApproval()`
4. In `internal/approve/plan.go` `ApprovePlan()`, add calls to `bead.CreatePlanBeads()` and `bead.WriteGeneratedBeadIDs()` before gate resolution — best-effort, warn on failure
5. Update `ApprovePlan()` signature to accept `approvedBy string`, pass it through to `updatePlanApproval()`
6. In `cmd/mindspec/approve.go`, add `--approved-by` string flag (default `"user"`) to both commands, pass the value to the approve functions
7. In `cmd/mindspec/bead.go`, remove `Deprecated` field from `beadPlanCmd` (keep worktree deprecation)

**Verification**

- [ ] `approve spec` calls `CreateSpecBead()` — spec bead + gate exist after approval
- [ ] `approve plan` calls `CreatePlanBeads()` + `WriteGeneratedBeadIDs()` — molecule + impl beads exist after approval
- [ ] Both approve commands succeed with warnings (not errors) when `bd` is unavailable
- [ ] `approve spec` generates `context-pack.md` in the spec directory (best-effort)
- [ ] `approve spec --approved-by=max` records `Approved By: max` in spec frontmatter
- [ ] `approve plan --approved-by=max` records `approved_by: max` in plan frontmatter
- [ ] `mindspec bead plan` shows no deprecation warning
- [ ] `make test` passes

**Depends on**: None

## Bead 3: Idle template directive + docs (R7, R8)

**Scope**: `internal/instruct/templates/idle.md`, `docs/core/CONVENTIONS.md`

**Steps**

1. Add a `## Next Action` section at the end of `internal/instruct/templates/idle.md`
2. Write the directive: instruct the agent to greet the user and suggest `/spec-init`, resuming an existing spec, or `mindspec doctor`
3. In `docs/core/CONVENTIONS.md`, add a note under the milestone commit section clarifying these are agent conventions enforced by training, not by CLI tooling
4. Run `make build` and verify `./bin/mindspec instruct` in idle state includes the directive

**Verification**

- [ ] `./bin/mindspec instruct` in idle state includes `## Next Action` directive
- [ ] Directive mentions `/spec-init`, resuming a spec, and `mindspec doctor`
- [ ] CONVENTIONS.md documents milestone commits as agent convention, not CLI-enforced
- [ ] `make build` succeeds

**Depends on**: None

## Bead 4: Strip static instruction files (R9)

**Scope**: `CLAUDE.md`, `AGENTS.md`, `.claude/rules/mindspec-modes.md`, `.claude/commands/spec-approve.md`, `.claude/commands/plan-approve.md`, `internal/instruct/templates/*.md`

**Steps**

1. Rewrite `CLAUDE.md` to ~25 lines: project identity, build/test commands (`make build`, `make test`), pointer to `mindspec instruct`, custom commands table — remove behavioral rules, key files, project layout, CLI reference
2. Rewrite `AGENTS.md` with a header: "This file is a human-readable reference. Agent guidance is emitted dynamically by `mindspec instruct`." — keep content as human reference, not agent-consumed
3. Move the "Landing the Plane" session-close protocol from `AGENTS.md` into the instruct templates (append to active-mode templates or add a dedicated section in the instruct rendering)
4. Delete `.claude/rules/mindspec-modes.md`
5. Reduce `.claude/commands/spec-approve.md` to a thin shim: trigger, confirm with user, run `mindspec approve spec <id>`, handle rejection
6. Reduce `.claude/commands/plan-approve.md` to the same thin pattern
7. Run `make build` and verify a fresh `mindspec instruct` invocation provides complete agent guidance without relying on static files

**Verification**

- [ ] CLAUDE.md is under 30 lines — contains project identity, build/test commands, pointer to `mindspec instruct`
- [ ] AGENTS.md is marked as human reference, does not duplicate instruct output
- [ ] `.claude/rules/mindspec-modes.md` is deleted
- [ ] `.claude/commands/spec-approve.md` is under 15 lines
- [ ] `.claude/commands/plan-approve.md` is under 15 lines
- [ ] Session-close protocol is emitted by `mindspec instruct` or `mindspec complete`
- [ ] `make build` succeeds
- [ ] Starting a fresh session with `mindspec instruct` still provides complete agent guidance

**Depends on**: Bead 1, Bead 2, Bead 3
