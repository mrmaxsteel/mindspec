---
status: Approved
spec_id: 004-instruct
version: "1.0"
last_updated: 2026-02-12
approved_at: 2026-02-12
approved_by: user
bead_ids: []
adr_citations:
  - id: ADR-0003
    sections: ["CLI Contract", "Instruction Sources", "Bootstrap and Fallback"]
  - id: ADR-0002
    sections: ["Beads as passive substrate"]
  - id: ADR-0005
    sections: ["State File Schema", "Write Surface", "Commit Ordering", "Cross-Validation"]
---

# Plan: Spec 004 — Mode-Aware Guidance Emission (`mindspec instruct`)

**Spec**: [spec.md](spec.md)

---

## Design Notes

### State File

`.mindspec/state.json` is committed to git (project-level workflow state). Schema:

```json
{
  "mode": "idle",
  "activeSpec": "",
  "activeBead": "",
  "lastUpdated": "2026-02-12T10:00:00Z"
}
```

Valid modes: `idle`, `spec`, `plan`, `implement`.

### State Write Surface

A `mindspec state set` CLI subcommand handles state writes. This is called by the skill hooks (spec-init, spec-approve, plan-approve) at each transition. Keeping writes in the CLI (rather than having Claude write JSON directly) ensures validation and consistency.

### Commit Ordering

State writes must happen **before** the milestone commit at each transition, so `.mindspec/state.json` is always co-committed with the transition artifacts. The sequence at every transition is:

1. Update artifacts (spec approval status, plan frontmatter, etc.)
2. Run `mindspec state set --mode=X ...`
3. `git add` both the artifacts and `.mindspec/state.json`
4. Milestone commit

This ensures `state.json` is always consistent with the committed artifact state.

### Cross-Validation

`instruct` reads `state.json` first, then spot-checks against artifact state:
- **spec mode**: checks that `docs/specs/<activeSpec>/spec.md` exists and has `Status: DRAFT` or is being worked on
- **plan mode**: checks that the spec is `APPROVED` and `plan.md` exists
- **implement mode**: checks that `plan.md` has `status: Approved` in frontmatter and active bead is in-progress via `bd show`
- **worktree**: in implement mode, checks current worktree name matches `worktree-<activeBead>`

Drift produces a warning in output, not a hard failure.

### Guidance Templates

Stored as embedded `.md` files under `internal/instruct/templates/`. Each template uses Go `text/template` syntax with a context struct providing `ActiveSpec`, `ActiveBead`, `SpecGoal`, etc.

### Beads Query

Shell out to `bd show <id>` for bead status checks. Failure is non-fatal (emits warning).

---

## Bead 004-A: State management package + CLI

**Scope**: `internal/state/` package, workspace path helpers, `mindspec state` subcommand, `.gitignore` update

**Steps**:
1. Add `workspace.MindspecDir()` and `workspace.StatePath()` to `internal/workspace/workspace.go`
2. Create `internal/state/state.go`: `State` struct, `Read(root)`, `Write(root, state)`, `SetMode(root, mode, spec, bead)` functions with JSON serialization
3. Create `internal/state/validate.go`: `CrossValidate(root, state)` — checks state against artifact state (spec files, plan frontmatter, beads). Returns `[]Warning`
4. Create `cmd/mindspec/state.go`: `mindspec state set --mode=X --spec=Y [--bead=Z]` and `mindspec state show` subcommands. Register in `root.go`
5. Remove `.mindspec/` from `.gitignore`; commit an initial `state.json` with `mode: "plan"`, `activeSpec: "004-instruct"`
6. Write tests for state read/write, validation, and CLI subcommands

**Verification**:
- [ ] `./bin/mindspec state set --mode=spec --spec=004-instruct` creates/updates `.mindspec/state.json`
- [ ] `./bin/mindspec state show` prints current state as JSON
- [ ] `./bin/mindspec state set --mode=invalid` returns an error
- [ ] `make test` passes with state package tests covering read, write, cross-validation
- [ ] `.mindspec/state.json` is tracked by git (not gitignored)

**Depends on**: nothing

---

## Bead 004-B: Guidance templates + `instruct` command

**Scope**: Embedded guidance templates, `internal/instruct/` package, `mindspec instruct` command replacing the stub

**Steps**:
1. Create `internal/instruct/templates/` with four embedded `.md` files: `idle.md`, `spec.md`, `plan.md`, `implement.md`. Each contains the mode-specific rules, permitted/forbidden actions, gates, and `{{.ActiveSpec}}` / `{{.ActiveBead}}` template variables
2. Create `internal/instruct/instruct.go`: `Context` struct (mode, active spec/bead, goal, warnings), `Render(ctx)` function using `text/template` + embedded templates, `RenderJSON(ctx)` for JSON output
3. Create `internal/instruct/worktree.go`: `CheckWorktree(activeBead)` — runs `git worktree list` and checks current directory against expected `worktree-<bead-id>`. Returns warning if mismatched
4. Replace `cmd/mindspec/stubs.go` instruct stub with `cmd/mindspec/instruct.go`: reads state via `state.Read()`, runs `CrossValidate()`, checks worktree (if implement mode), renders guidance. Supports `--format=json` flag
5. Write tests: template rendering for each mode, JSON output structure, worktree check logic, graceful fallback when state.json missing

**Verification**:
- [ ] `./bin/mindspec instruct` reads `.mindspec/state.json` and emits mode-appropriate markdown guidance
- [ ] `./bin/mindspec instruct --format=json | jq .mode` returns one of `idle`, `spec`, `plan`, `implement`
- [ ] `./bin/mindspec instruct --format=json | jq .warnings` shows array (empty or with drift/worktree warnings)
- [ ] With `state.json` deleted, `./bin/mindspec instruct` falls back with a warning (not a crash)
- [ ] Guidance output includes: permitted actions, forbidden actions, gates, next expected action
- [ ] `make test` passes with instruct package tests

**Depends on**: 004-A

---

## Bead 004-C: Hook integration + doc-sync

**Scope**: Update skill hooks to write state at transitions, update documentation

**Steps**:
1. Update `.claude/commands/spec-init.md`: add step to run `mindspec state set --mode=spec --spec=<id>` after creating the spec directory, **before** the milestone commit (so `state.json` is included in the commit)
2. Update `.claude/commands/spec-approve.md`: add step to run `mindspec state set --mode=plan --spec=<id>` on approval, **before** the milestone commit
3. Update `.claude/commands/plan-approve.md`: add step to run `mindspec state set --mode=implement --spec=<id> --bead=<bead-id>` on approval, **before** the milestone commit
4. Update `.claude/commands/spec-status.md`: add step to run `mindspec state show` and `mindspec instruct` as primary state source
5. Update `docs/domains/workflow/interfaces.md`: add State Management interface (Go, not Python pseudocode)
6. Update `docs/core/CONVENTIONS.md`: add `.mindspec/state.json` convention
7. Add `State File` entry to `GLOSSARY.md`

**Verification**:
- [ ] `/spec-init` instructions include `mindspec state set` call
- [ ] `/spec-approve` instructions include state transition to plan mode
- [ ] `/plan-approve` instructions include state transition to implement mode
- [ ] `/spec-status` instructions reference `mindspec state show`
- [ ] `docs/domains/workflow/interfaces.md` documents State Management interface in Go
- [ ] `GLOSSARY.md` includes `State File` entry
- [ ] `mindspec doctor` still passes (no broken glossary links)

**Depends on**: 004-A

---

## Dependency Graph

```
004-A (state package + CLI)
  ├── 004-B (guidance templates + instruct command)
  └── 004-C (hook integration + doc-sync)
```

004-B and 004-C can be implemented in parallel after 004-A.
