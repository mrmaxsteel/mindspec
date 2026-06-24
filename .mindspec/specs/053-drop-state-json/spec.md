---
approved_at: "2026-02-27T08:16:23Z"
approved_by: user
molecule_id: mindspec-mol-jpkyf
status: Approved
step_mapping:
    implement: mindspec-mol-07lst
    plan: mindspec-mol-jchd7
    plan-approve: mindspec-mol-mw2ag
    review: mindspec-mol-0du51
    spec: mindspec-mol-zytod
    spec-approve: mindspec-mol-469r9
    spec-lifecycle: mindspec-mol-jpkyf
---





# Spec 053-drop-state-json: Eliminate state.json in Favor of Molecule-Derived State

## Goal

Complete the migration begun by ADR-0015: remove `.mindspec/state.json` as a stateful artifact and derive all lifecycle state from Beads molecule queries. Replace with a minimal `session.json` for transient per-session metadata and a write-through `mode-cache` for hook performance.

## Background

ADR-0015 established that lifecycle mode is derived from the spec's molecule, demoting `state.json` to a "non-canonical convenience cursor." In practice, `state.json` is still written by every lifecycle command and read by hooks, guards, instruct, and complete. The `resolve` package already implements molecule-derived mode resolution and is wired into `instruct` and `state show`, but the rest of the codebase still depends on `state.json` fields.

The single-cursor `ActiveBead` field in `state.json` is the primary blocker for future multi-agent parallel bead execution. Two agents cannot share one `activeBead` string. Deriving bead state from Beads queries (`bd list --status=in_progress`) makes each agent's view independent.

### Current state.json fields (11)

| Field | Category |
|-------|----------|
| `Mode` | Molecule-derivable (already done in `resolve.ResolveMode`) |
| `ActiveSpec` | Molecule-derivable (already done in `resolve.ResolveTarget`) |
| `ActiveMolecule` | Molecule-derivable (read from `spec.md` frontmatter) |
| `StepMapping` | Molecule-derivable (`bd mol show` returns it) |
| `ActiveBead` | Beads-queryable (`bd list --status=in_progress --parent <implStep>`) |
| `ActiveWorktree` | Convention-derivable (bead ID → worktree path) |
| `SpecBranch` | Convention-derivable (spec ID → `spec/<id>` branch) |
| `SessionSource` | Session-transient (no molecule equivalent) |
| `SessionStartedAt` | Session-transient (no molecule equivalent) |
| `BeadClaimedAt` | Session-transient (no molecule equivalent) |
| `LastUpdated` | Bookkeeping (drops naturally) |

## Impacted Domains

- **workflow**: Mode derivation, lifecycle transitions, session freshness gate
- **agent-interface**: Hooks, guards, instruct — all state consumers
- **core**: State package rewrite, resolve package promoted to primary

## ADR Touchpoints

- [ADR-0015](../../adr/ADR-0015.md): This spec completes the decision made there — molecule state becomes the sole authority, not just the preferred one
- [ADR-0005](../../adr/ADR-0005.md): Superseded by ADR-0015; this spec removes the last remnants of ADR-0005's state file model
- [ADR-0012](../../adr/ADR-0012.md): Composing with Beads CLI for state queries rather than maintaining a parallel state file

## Requirements

1. **No `state.json` reads or writes** — Remove `state.Read()`, `state.Write()`, `state.SetMode()`, `state.SetModeWithMetadata()`, and the dual-write propagation logic
2. **Mode derived from molecule** — All consumers that need the current mode call `resolve.ResolveMode()` (or read the mode-cache for latency-sensitive paths)
3. **ActiveSpec derived from molecule** — `resolve.ResolveTarget()` is the sole mechanism for determining which spec is active
4. **ActiveBead derived from Beads queries** — `bd list --status=in_progress --parent <implStep>` replaces the cached cursor. When no bead argument is provided, commands query Beads for the in-progress bead
5. **ActiveWorktree derived from convention** — Bead worktrees follow the pattern `.worktrees/worktree-<beadID>`. Spec worktrees follow `.worktrees/worktree-spec-<specID>`. No need to store paths
6. **SpecBranch derived from convention** — Branch name is `spec/<specID>`, deterministic from the spec ID
7. **Session-transient fields move to `session.json`** — A new minimal file at `.mindspec/session.json` holds only: `sessionSource`, `sessionStartedAt`, `beadClaimedAt`. Written by `WriteSession()` and the session freshness gate. Not lifecycle state
8. **Write-through mode-cache for hooks** — Lifecycle commands (`next`, `complete`, `approve *`) write a `.mindspec/mode-cache` file after mutating molecule state. Hooks read this cache instead of calling `bd mol show` on every `PreToolUse`. Cache contains: `mode`, `activeSpec`, `activeWorktree`, `timestamp`. If cache is missing, hooks fall back to molecule resolution
9. **Full behavioral parity** — Every existing workflow (spec-init, approve spec/plan/impl, next, complete, instruct, hooks, guards) produces identical outcomes

## Scope

### In Scope

- `internal/state/state.go` — Rewrite: remove `State` struct, `Read()`, `Write()`, `SetMode()`, `SetModeWithMetadata()`, dual-write. Replace with `Session` struct and `ReadSession()`/`WriteSession()` for session-transient fields, plus `WriteModeCache()`/`ReadModeCache()` for the hook cache
- `internal/resolve/` — Promote to primary: add `ResolveActiveBead(root, specID)` that queries `bd list --status=in_progress`
- `cmd/mindspec/next.go` — Remove state.json writes; write session.json for `BeadClaimedAt`; write mode-cache after bead claim
- `cmd/mindspec/complete.go` + `internal/complete/complete.go` — Remove state.json reads/writes; derive ActiveBead from Beads query; derive SpecBranch/ActiveWorktree from convention; write mode-cache after completion
- `cmd/mindspec/approve.go` + `internal/approve/` — Remove state.json writes; write mode-cache after approval
- `cmd/mindspec/instruct.go` + `internal/instruct/instruct.go` — Remove state.json fallback; use resolver exclusively
- `cmd/mindspec/state.go` — Remove `state set` command entirely (`bd update` is the override mechanism now). `state show` becomes a pure resolver view. `state write-session` writes to session.json
- `internal/hook/dispatch.go` — Read mode-cache instead of state.json for Mode/ActiveWorktree. Read session.json for freshness gate
- `internal/guard/` — Derive ActiveWorktree from convention or mode-cache
- `cmd/mindspec/spec_init.go` + `internal/specinit/` — Remove state.json writes; write mode-cache
- `internal/explore/` — Remove state.json writes; write mode-cache
- All tests touching state.json

### Out of Scope

- Multi-agent orchestration (Part 2 — agent teams, fan-out, claim serialization)
- Changes to Beads CLI itself
- Session freshness gate redesign (kept as-is, just relocated to session.json)
- `mindspec setup` / hook installation changes (hooks read different file, but hook scripts are regenerated by setup)

## Non-Goals

- Introducing agent identity or multi-cursor state — that's Part 2
- Changing the molecule formula or lifecycle steps
- Dropping the session freshness gate (may happen in Part 2, but not here)
- Performance optimization of `bd` queries beyond the mode-cache

## Acceptance Criteria

- [ ] `.mindspec/state.json` is never read or written by any MindSpec command
- [ ] `state.json` is absent from a fresh spec-init worktree
- [ ] `mindspec state show` derives mode from molecule and shows correct state for all 6 lifecycle phases
- [ ] `mindspec next` claims a bead, writes session.json + mode-cache, and the session freshness gate still prevents double-claiming
- [ ] `mindspec complete` derives ActiveBead from Beads query when no argument given, merges correctly, advances state via molecule
- [ ] `mindspec approve spec/plan/impl` transitions molecule steps and writes mode-cache
- [ ] `mindspec instruct` emits correct mode-appropriate guidance in all phases without state.json
- [ ] PreToolUse hooks (plan gate, worktree guard, workflow guard, session freshness gate) function correctly reading mode-cache and session.json
- [ ] `make test` passes with no state.json references in non-test code
- [ ] Full lifecycle integration: `spec-init` → `approve spec` → `approve plan` → `next` → `complete` → `approve impl` works end-to-end without state.json

## Validation Proofs

- `grep -r 'state\.json' internal/ cmd/` returns zero matches (excluding test fixtures and migration notes)
- `make test` passes
- Manual lifecycle walkthrough on a fresh spec confirms parity

## Decisions

- **`state set` removed entirely** — `bd update` is the override mechanism for molecule state. A mode-cache override would diverge from the source of truth
- **mode-cache is `.gitignore`d** — it's a local write-through cache, not shared state. Each worktree gets its own mode-cache when lifecycle commands run in it. Hooks fall back to molecule resolution when cache is missing (e.g., fresh worktree before first lifecycle command)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-27
- **Notes**: Approved via mindspec approve spec