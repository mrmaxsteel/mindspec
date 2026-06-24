---
approved_at: "2026-03-08T08:10:55Z"
approved_by: user
status: Approved
---
# Spec 074-self-contained-beads: Self-Contained Beads — Populate Bead Fields at Plan Approval

## Goal

Make implementation beads fully self-contained work units. An agent with only `bd show <id>` receives everything it needs — work chunk, requirements, acceptance criteria, and relevant architectural decisions — without reading spec.md, plan.md, or ADR files from disk.

This eliminates the runtime bead primer and decouples agent context from filesystem layout.

## Background

### Current state

`createImplementationBeads()` (`internal/approve/plan.go:191-264`) creates beads with only `--title` and `--type=task`. No description, design, or acceptance criteria. The bead is an empty shell.

Context is assembled at runtime by `BuildBeadPrimer()` (`internal/contextpack/primer.go:44-123`), which reads spec.md, plan.md, ADR files, and domain docs from disk. This fails when:
- The bead is dispatched to an agent without the filesystem (CI, remote worker, Gastown sling)
- Files are moved, renamed, or the worktree is missing
- The agent loses context after compaction and can't re-derive paths

### Gastown pattern

In Gastown, beads carry their own context. When `gt sling` dispatches a bead, the agent gets everything from `gt prime` — no filesystem reads. The bead's description, acceptance_criteria, and metadata fields contain the full feature context.

### Design principle (ADR-0023)

ADR-0023 established beads as the single state authority. This spec extends that to context authority: beads should be the single source of both lifecycle state *and* implementation context.

### Relationship to Spec 075

This spec snapshots ADR Decision sections as text into the bead's `design` field at plan approval time. A subsequent Spec 075 will migrate mindspec's file-based ADR system to beads `decision` issues, at which point the snapshot approach here will be replaced by `related` dependency links between implementation beads and decision issues. The `design` field format is designed to make that transition clean.

## Impacted Domains

- approve: Plan approval populates bead fields with spec/plan/ADR content
- contextpack: `BuildBeadPrimer()` and `RenderBeadPrimer()` deleted
- instruct: Implementation mode renders bead context from `bd show` output
- next: Bead claiming emits context from `bd show` output

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): Extends "beads as single state store" to "beads as single context store"

## Requirements

1. At plan approval, `createImplementationBeads()` must populate each bead's `description` with the matching `## Bead N:` work chunk from `plan.md`
2. At plan approval, each bead's `acceptance_criteria` must be populated with the spec-level acceptance criteria from `spec.md`
3. At plan approval, each bead's `design` field must be populated with the spec Requirements section and relevant ADR Decision sections (snapshotted as text)
4. At plan approval, each bead's `metadata` JSON must include `spec_id` and `file_paths` (extracted from the plan work chunk)
5. Delete `BuildBeadPrimer()` and `RenderBeadPrimer()` — no runtime primer
6. `mindspec instruct` in implementation mode emits bead context by calling `bd show <bead-id> --json` and rendering the fields directly
7. `mindspec next` emits bead context the same way after claiming
8. `mindspec context bead` renders from `bd show` output
9. An agent calling `bd show <id> --json` must receive enough structured content to implement the bead without accessing spec.md or plan.md
10. Plan re-approval must close existing beads (reason: `"superseded by plan v<N>"`) and create fresh ones. If any bead is `in_progress` or `closed`, re-approval must error.

## Scope

### In Scope
- `internal/approve/plan.go` — `createImplementationBeads()`: populate `--description`, `--design`, `--acceptance-criteria`, `--metadata`
- `internal/contextpack/primer.go` — delete `BuildBeadPrimer()`, `RenderBeadPrimer()`, `extractBeadSection()`, and related helpers
- `internal/contextpack/builder_test.go`, `internal/contextpack/primer_test.go` — delete or rewrite primer tests
- `internal/instruct/instruct.go` — replace primer assembly with `bd show` rendering
- `cmd/mindspec/next.go` — replace primer emission with `bd show` rendering
- `cmd/mindspec/context.go` — replace primer path with `bd show` rendering

### Out of Scope
- ADR migration to beads decisions (Spec 075)
- Changes to spec.md or plan.md file formats
- Spec approval flow — epic creation is unchanged
- Bead worktree creation — `mindspec next` still creates worktrees
- Domain docs — agents read them directly from the codebase when needed

## Non-Goals

- Eliminating spec.md and plan.md files — humans still author in markdown; beads capture a distilled snapshot at the approval gate
- Real-time sync between spec.md edits and bead content — the bead captures the approved state
- Migrating the ADR system — that's Spec 075; this spec snapshots ADR content as text

## Acceptance Criteria

- [ ] `bd show <bead-id> --json` returns a populated `description` containing the plan work chunk
- [ ] `bd show <bead-id> --json` returns a populated `acceptance_criteria` from the spec
- [ ] `bd show <bead-id> --json` returns a populated `design` with spec requirements and ADR decisions
- [ ] `bd show <bead-id> --json` returns `metadata` with `spec_id` and `file_paths`
- [ ] `BuildBeadPrimer()` and `RenderBeadPrimer()` are deleted
- [ ] `mindspec instruct` in implementation mode renders bead context from `bd show`
- [ ] `mindspec next` renders bead context from `bd show` after claiming
- [ ] Existing LLM harness tests pass with bead-sourced context
- [ ] `mindspec approve plan` creates beads with populated fields (integration test)

## Validation Proofs

- `bd show <bead-id> --json | jq '.description'`: non-empty work chunk
- `bd show <bead-id> --json | jq '.acceptance_criteria'`: spec-level AC
- `bd show <bead-id> --json | jq '.design'`: spec requirements + ADR decisions
- `go test ./internal/approve/ -run TestCreateImplementationBeads`: beads with populated fields
- `go test ./internal/instruct/ -v`: instruct renders from `bd show`
- `grep -r "BuildBeadPrimer\|RenderBeadPrimer" internal/`: no matches

## Open Questions

- [x] What happens when a plan is re-approved (plan iteration)? **Resolved**: Close existing beads with reason `"superseded by plan v<N>"` and create fresh ones. If any bead is `in_progress` or `closed`, re-approval must error — don't silently discard started work.
- [x] Size constraints — TEXT fields in Dolt are ~64KB. **Resolved**: Not a concern. Typical spec+plan+ADR snapshots total under 20KB. Dolt's natural limit serves as a backstop.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-08
- **Notes**: Approved via mindspec approve spec