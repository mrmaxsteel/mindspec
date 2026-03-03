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

## Scope

### Production code changes

- `internal/state/` — remove `Focus`, `Lifecycle`, `ReadFocus`, `WriteFocus`, `ReadLifecycle`, `WriteLifecycle` and related helpers
- `internal/approve/spec.go` — remove focus write (lines 69-82) and lifecycle write (lines 43-50)
- `internal/approve/plan.go` — remove focus write
- `internal/approve/impl.go` — remove focus reads/writes, lifecycle reads/writes; replace with beads queries
- `internal/specinit/specinit.go` — remove focus write
- `internal/complete/complete.go` — remove focus write
- `internal/next/` — remove focus reads; derive context from beads + path conventions
- `internal/instruct/` — replace `ReadFocus()` with beads-derived context resolution
- `cmd/mindspec/state.go` — update `state show` to derive from beads; remove `state set` or repoint it

### New functions

- `ResolveContext(root) → (specID, beadID, phase, worktreePath)` — combines beads query with path conventions
- `DiscoverActiveSpecs() → []ActiveSpec` — queries `bd list --type=epic --status=open --json`, derives phase from bead statuses
- `DerivePhase(epicID) → Phase` — implements the status-to-phase mapping from ADR-0023 §3

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
3. `mindspec approve spec/plan/impl` complete without writing focus or lifecycle.yaml
4. `mindspec next` and `mindspec complete` work without reading/writing focus
5. All LLM test assertions use beads-based state checks
6. `grep -r "ReadFocus\|WriteFocus\|ReadLifecycle\|WriteLifecycle" internal/` returns only test cleanup code (if any)
7. No `.mindspec/focus` or `lifecycle.yaml` files are created during a full spec lifecycle run

## Dependencies

- ADR-0023 (accepted)
- Beads `--metadata` flag support (verified available)
- Epic `metadata.spec_id` convention (defined in ADR-0023 §2)

## Risks

- **Beads daemon availability** — if Dolt server is down, no state queries work. Mitigated by: auto-start on first `bd` call, `mindspec doctor` health check.
- **Migration** — existing repos have stale focus/lifecycle.yaml files. Mitigated by: `mindspec doctor` cleanup command.
