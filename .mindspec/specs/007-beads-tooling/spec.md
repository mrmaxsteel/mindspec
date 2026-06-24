# Spec 007: Beads Integration Conventions + Tooling

## Goal

Provide CLI commands that codify MindSpec's Beads conventions (ADR-0002) into repeatable, enforceable workflows. Agents and humans should be able to create correctly-shaped spec beads and implementation beads, manage bead-to-worktree mappings, and run active-workset hygiene — all through `mindspec bead` subcommands rather than ad-hoc `bd` invocations.

## Background

Beads is MindSpec's execution-tracking substrate (ADR-0002). Current Beads usage relies on raw `bd` CLI calls with conventions enforced only by documentation (AGENTS.md, CONVENTIONS.md). This creates several gaps:

1. **Spec bead creation** is manual — agents must remember to write a concise summary + spec link, not embed long-form content.
2. **Implementation bead creation** from an approved plan is manual — agents must parse `plan.md`, create beads per work chunk, wire dependencies, and write bead IDs back into plan frontmatter.
3. **Worktree association** is convention-only — no tooling validates or creates the `worktree-<bead-id>` mapping.
4. **Workset hygiene** relies on agents running cleanup voluntarily — no structured command exists to audit stale beads or enforce the bounded-workset discipline.

Specs 000 (hygiene checks in doctor) and 005 (`mindspec next` claiming) established the foundation. This spec builds the remaining Beads lifecycle tooling.

### Status field convention note

Spec approval status currently lives in a markdown section (`Status: APPROVED` in spec.md) while plan status uses YAML frontmatter (`status: Approved`). This spec's tooling will handle **both formats** gracefully rather than force a migration. A future convention-alignment effort may standardize on YAML frontmatter for all artifacts.

## Impacted Domains

- workflow: new `mindspec bead` command tree integrates with mode transitions and state management
- tracking: formalizes the interface between MindSpec and Beads CLI

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Defines Beads as passive tracking substrate; spec beads must be concise index entries, not canon. This spec implements those conventions as enforceable tooling.
- [ADR-0004](../../adr/ADR-0004.md): Go as implementation language — all new commands are Go.

## Requirements

1. **`mindspec bead spec <spec-id>`** — Create a spec bead from an approved specification. Reads `docs/specs/<id>/spec.md`, validates approval status (handles both `Status: APPROVED` markdown and `status: Approved` frontmatter formats). Extracts title, goal summary (first sentence or ≤120 chars), and impacted domains. Creates a Beads issue via `bd create` with a structured description (`Summary:` / `Spec:` / `Domains:` lines). Idempotent: searches for an existing bead by spec path before creating; if found, prints the existing bead ID and exits successfully. Outputs the bead ID (created or existing).
2. **`mindspec bead plan <spec-id>`** — Create implementation beads from an approved plan. Reads `docs/specs/<id>/plan.md`, validates `status: Approved` in frontmatter. Parses the `work_chunks` YAML block in plan frontmatter (see Plan Decomposition Convention below). Creates one Beads issue per work chunk with dependencies per `depends_on` fields. Sets up parent-child relationship to spec bead if it exists. Writes bead IDs into `plan.md` frontmatter under `generated.bead_ids` (generated metadata, does not invalidate approval). Idempotent: matches existing beads by `spec_id` + `chunk_id`; creates only missing beads on re-run.
3. **`mindspec bead worktree <bead-id>`** — Create or show the worktree mapping for a bead. `--create` creates a git worktree at `../worktree-<bead-id>` with branch `bead/<bead-id>`. Without flags, parses `git worktree list` and matches by naming convention. Validates bead is `in_progress` and working tree is clean before creating. Does not auto-set mode — `mindspec state set` remains a separate step.
4. **`mindspec bead hygiene`** — Audit the active workset. Flags stale beads (no update in >7 days, configurable via `--stale-days=N`), orphaned beads (no parent/spec link), and oversized descriptions. Reports total open count vs recommended max (default 15). `--fix` is dry-run by default; requires `--yes` to execute. Conservative: only auto-closes beads explicitly marked with a `done` label/state, never infers done-ness.
5. **Convention enforcement** — All bead creation commands enforce ADR-0002 rules: spec bead descriptions use the structured format and are capped at ≤400 chars; implementation bead descriptions are capped at ≤800 chars. Long-form content is never embedded; only links to canonical docs.
6. **Preflight checks** — All subcommands validate prerequisites: git repo, `.beads/` initialized, `bd` on PATH. Clear error messages with remediation steps on failure.
7. **Error strategy** — Deterministic exit codes: 0 = success, 1 = validation failure, 2 = Beads CLI error. All errors to stderr with actionable messages.

### Plan Decomposition Convention

Plans must include a `work_chunks` block in YAML frontmatter for machine-readable decomposition:

```yaml
work_chunks:
  - id: 1
    title: "Add bdcli wrapper"
    scope: "internal/bead/bdcli.go — thin wrapper around bd CLI"
    verify:
      - "Unit tests pass for bdcli create/search/update"
    depends_on: []
  - id: 2
    title: "Implement bead spec command"
    scope: "internal/bead/spec.go, cmd/mindspec/bead.go"
    verify:
      - "mindspec bead spec 006-validate creates a bead or returns existing"
      - "Rejects unapproved specs with exit code 1"
    depends_on: [1]
```

Each chunk has a stable `id` (integer, unique within the plan), `title`, `scope`, `verify` (list of verification steps), and `depends_on` (list of chunk IDs). This format is deterministic and machine-parseable.

## Scope

### In Scope
- `cmd/mindspec/bead.go` — cobra command tree (`bead spec`, `bead plan`, `bead worktree`, `bead hygiene`)
- `internal/bead/spec.go` — spec bead creation logic (with idempotent lookup)
- `internal/bead/plan.go` — plan-to-beads decomposition logic (with idempotent lookup)
- `internal/bead/worktree.go` — worktree creation and lookup via `git worktree list`
- `internal/bead/hygiene.go` — workset audit and cleanup
- `internal/bead/bdcli.go` — wrapper around `bd` CLI invocations (insulates from flag changes)
- Updates to `cmd/mindspec/root.go` to register `bead` command
- Updates to `docs/templates/plan.md` to include `work_chunks` block
- Tests for all new packages

### Out of Scope
- Worktree lifecycle management beyond create/show (cleanup, sync, registry — that's Spec 008)
- Domain scaffolding (Spec 009)
- ADR lifecycle tooling (Spec 010)
- Proof runner (Spec 011)
- Changes to Beads CLI itself
- Multi-repo targeting
- Standardizing status field format across spec.md / plan.md (future convention work)

## Non-Goals

- Replacing `bd` as the general-purpose issue tracker — `mindspec bead` is a convention-enforcing layer on top
- Implementing bead templates or interactive forms
- Syncing beads to remote repositories (handled by `bd sync`)
- Modifying the `mindspec next` or `mindspec doctor` commands (they already work)
- Full worktree registry (deferred to Spec 008)

## Acceptance Criteria

- [ ] `mindspec bead spec 006-validate` creates a Beads issue with structured description (Summary/Spec/Domains lines), description ≤400 chars, and outputs the bead ID
- [ ] Re-running `mindspec bead spec 006-validate` returns the existing bead ID without creating a duplicate
- [ ] `mindspec bead spec` rejects specs without approved status with exit code 1 and a clear error message
- [ ] `mindspec bead plan 006-validate` reads an approved plan with `work_chunks`, creates one bead per chunk, wires dependencies, and writes bead IDs into `plan.md` frontmatter under `generated.bead_ids`
- [ ] Re-running `mindspec bead plan 006-validate` does not create duplicates; prints existing mapping
- [ ] `mindspec bead plan` rejects plans without `status: Approved` in frontmatter with exit code 1
- [ ] `mindspec bead plan` rejects plans missing the `work_chunks` block with a clear error
- [ ] `mindspec bead worktree <bead-id> --create` creates a git worktree at `../worktree-<bead-id>` with branch `bead/<bead-id>`
- [ ] `mindspec bead worktree <bead-id> --create` refuses if working tree is dirty or bead is not `in_progress`
- [ ] `mindspec bead worktree <bead-id>` (no flags) shows the worktree path or reports "no worktree found"
- [ ] `mindspec bead hygiene` reports stale beads, orphaned beads, oversized descriptions, and total open count
- [ ] `mindspec bead hygiene --fix` defaults to dry-run; requires `--yes` to execute changes
- [ ] All subcommands check prerequisites (git repo, `.beads/`, `bd` on PATH) and fail with actionable errors
- [ ] All bead creation commands enforce description length caps per ADR-0002
- [ ] `mindspec bead --help` shows all subcommands with usage
- [ ] All new code has unit tests; `make test` passes
- [ ] Doc-sync: CLAUDE.md, CONVENTIONS.md, plan template updated

## Validation Proofs

- `./bin/mindspec bead --help`: Shows `spec`, `plan`, `worktree`, `hygiene` subcommands
- `./bin/mindspec bead spec 006-validate`: Creates a spec bead (or returns existing ID if already created)
- `./bin/mindspec bead spec 006-validate` (second run): Returns same bead ID, no duplicate
- `./bin/mindspec bead hygiene`: Produces an audit report of the active workset
- `./bin/mindspec bead hygiene --fix`: Shows dry-run output (no changes without `--yes`)
- `make test`: All tests pass including new `internal/bead/` tests

## Open Questions

None — all resolved.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-12
- **Notes**: Approved via /spec-approve workflow
