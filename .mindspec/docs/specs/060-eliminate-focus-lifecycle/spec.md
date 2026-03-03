---
id: "060-eliminate-focus-lifecycle"
title: "Eliminate Focus and Lifecycle Files — Beads as Single State Authority"
status: Draft
adr_citations:
  - ADR-0023
---

# Spec 060: Eliminate Focus and Lifecycle Files

## Problem Statement

MindSpec maintains two denormalized state files — `.mindspec/focus` (JSON) and per-spec `lifecycle.yaml` — that cache information already stored authoritatively in beads (Dolt). These caches create consistency drift, parallel-agent contention, and fragile filesystem-based discovery. ADR-0023 (accepted) mandates their elimination.

## Goals

1. Remove all reads and writes of `.mindspec/focus` from production code
2. Remove all reads and writes of `lifecycle.yaml` from production code
3. Replace focus/lifecycle queries with beads-derived equivalents (epic metadata, bead status derivation, path conventions)
4. Update the LLM test harness assertions to use beads-based state checks instead of focus-file assertions
5. Clean up dead code: `state.Focus`, `state.Lifecycle`, `ReadFocus()`, `WriteFocus()`, `ReadLifecycle()`, `WriteLifecycle()` structs and functions

## Non-Goals

- Changing the beads (Dolt) backend or schema
- Modifying worktree path conventions (already correct per ADR-0022)
- Removing spec artifacts (spec.md, plan.md) from the filesystem — these are documents, not state

## Design

### Phase derivation from beads

The lifecycle phase for any spec is derived entirely from beads state:

| Condition | Derived phase |
|:----------|:-------------|
| No epic with matching `metadata.spec_id` | **spec** (draft, still being written/discussed) |
| Epic exists, no children | **plan** (spec approved, plan being drafted) |
| Epic exists, all children open (none claimed) | **plan** (plan approved, beads ready to claim) |
| Epic exists, some closed, some open, none in_progress | **plan** (next bead ready) |
| Epic exists, any child in_progress | **implement** |
| Epic exists, all children closed, epic open | **review** |
| Epic closed | **done** |
| No open epics at all | **idle** |

The key gate: **epic creation = spec approval**. `spec approve` creates the epic with structured metadata, making the epic's existence the durable approval record.

### Epic metadata schema

Each spec epic stores two metadata fields:

```json
{
  "spec_num": 60,
  "spec_title": "eliminate-focus-lifecycle"
}
```

All other identifiers and paths are derived from these two fields:

| Derived value | Formula |
|:-------------|:--------|
| `spec_id` | `fmt.Sprintf("%03d-%s", spec_num, spec_title)` |
| Spec branch | `spec/<spec_id>` |
| Spec worktree path | `.worktrees/worktree-spec-<spec_id>` |
| Spec directory | `.mindspec/docs/specs/<spec_id>/` |
| Epic title | `[SPEC <spec_id>] <Human-Readable Title>` |

The epic's own standard fields (`created_at`, `created_by`, `status`) provide the audit trail for when and by whom the spec was approved — no need to duplicate these into metadata.

### Spec number collision prevention

`spec approve` must prevent two agents from independently claiming the same spec number:

1. `bd dolt pull` — fetch latest epics from Dolt remote (all agents/machines)
2. Query `bd list --type=epic --json`, check if any epic has `metadata.spec_num` matching the candidate number
3. If collision → reject: "Spec number 060 is already in use by epic \<id\>. Increment to 061."
4. If clear → create epic with `--metadata='{"spec_num":60,"spec_title":"eliminate-focus-lifecycle"}'`
5. `bd dolt push` — publish the new epic so other agents see it immediately

This provides optimistic-locking semantics: pull-before-create ensures visibility of all previously published epics across the distributed Dolt database.

## Scope

### Production code changes

- `internal/state/` — remove `Focus`, `Lifecycle`, `ReadFocus`, `WriteFocus`, `ReadLifecycle`, `WriteLifecycle` and related helpers
- `internal/approve/spec.go` — remove focus/lifecycle writes; add `bd dolt pull`, `spec_num` collision check, epic creation with `spec_num`/`spec_title` metadata, `bd dolt push`
- `internal/approve/plan.go` — remove focus write; epic already exists (created at spec approve), so plan approve only creates child beads
- `internal/approve/impl.go` — remove focus reads/writes, lifecycle reads/writes; replace with beads queries
- `internal/specinit/specinit.go` — remove focus write
- `internal/complete/complete.go` — remove focus write
- `internal/next/` — remove focus reads; derive context from beads + path conventions
- `internal/instruct/` — replace `ReadFocus()` with beads-derived context resolution
- `cmd/mindspec/state.go` — update `state show` to derive from beads; remove `state set` or repoint it

### New functions

- `ResolveContext(root) → (specID, beadID, phase, worktreePath)` — combines beads query with path conventions
- `DiscoverActiveSpecs() → []ActiveSpec` — queries `bd list --type=epic --status=open --json`, derives phase from bead statuses
- `DerivePhase(epicID) → Phase` — implements the status-to-phase mapping table above
- `CheckSpecNumberCollision(specNum int) → error` — pulls from Dolt remote, checks for existing epics with matching `metadata.spec_num`
- `SpecIDFromMetadata(specNum int, specTitle string) → string` — `fmt.Sprintf("%03d-%s", specNum, specTitle)`

### Test harness changes

- Replace `assertFocusMode` / `assertFocusFields` with beads-based assertions
- Remove `sandbox.WriteFocus()` / `sandbox.WriteLifecycle()` from test setups; replace with `sandbox.CreateBead()` / epic metadata setup
- Update all 17 LLM scenario setups and assertions

### Cleanup

- Delete `state.Focus` and `state.Lifecycle` structs
- Delete `ReadFocus()`, `WriteFocus()`, `ReadLifecycle()`, `WriteLifecycle()`
- Remove `.mindspec/focus` from `.gitignore` (no longer needed)
- `mindspec doctor` — detect and remove stale focus/lifecycle.yaml files

## Acceptance Criteria

1. `make test` passes with zero references to `ReadFocus`/`WriteFocus`/`ReadLifecycle`/`WriteLifecycle` in production code
2. `mindspec instruct` correctly derives phase from beads without any focus file
3. `mindspec spec approve` creates an epic with `metadata.spec_num` and `metadata.spec_title` (epic existence = approval gate)
4. `mindspec spec approve` performs `bd dolt pull` before epic creation and rejects on `spec_num` collision
5. `mindspec plan approve` creates child beads under the existing epic (no new epic creation)
6. `mindspec approve impl` completes without reading/writing focus or lifecycle.yaml
7. `mindspec next` and `mindspec complete` work without reading/writing focus
8. All LLM test assertions use beads-based state checks
9. `grep -r "ReadFocus\|WriteFocus\|ReadLifecycle\|WriteLifecycle" internal/` returns only test cleanup code (if any)
10. No `.mindspec/focus` or `lifecycle.yaml` files are created during a full spec lifecycle run
11. Phase derivation matches the design table for all edge cases (tested)

## Dependencies

- ADR-0023 (accepted)
- Beads `--metadata` flag support (verified available)
- Epic metadata convention: `spec_num` (int) + `spec_title` (kebab-case string) per this spec's design section

## Risks

- **Beads daemon availability** — if Dolt server is down, no state queries work. Mitigated by: auto-start on first `bd` call, `mindspec doctor` health check.
- **Migration** — existing repos have stale focus/lifecycle.yaml files. Mitigated by: `mindspec doctor` cleanup command.
