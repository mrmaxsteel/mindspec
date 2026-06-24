---
adr_citations:
    - id: ADR-0015
      sections:
        - Architecture
        - Bead 1
        - Bead 2
        - Bead 4
        - Bead 5
        - Bead 6
    - id: ADR-0005
      sections:
        - Architecture
        - Bead 6
    - id: ADR-0012
      sections:
        - Architecture
        - Bead 2
        - Bead 3
approved_at: "2026-02-27T21:22:02Z"
approved_by: user
bead_ids:
    - mindspec-mol-07lst.1
    - mindspec-mol-07lst.2
    - mindspec-mol-07lst.3
    - mindspec-mol-07lst.4
    - mindspec-mol-07lst.5
    - mindspec-mol-07lst.6
last_updated: "2026-02-27"
spec_id: 053-drop-state-json
status: Approved
version: 1
---

# Plan: 053-drop-state-json

## ADR Fitness

- **ADR-0015 (Per-Spec Molecule-Derived Lifecycle State)**: Sound and directly enabling. This spec completes the migration it mandated. No divergence needed.
- **ADR-0005 (Explicit State Tracking via State File)**: Already superseded by ADR-0015. This plan removes the last remnants.
- **ADR-0012 (Compose with External CLIs)**: Sound. We derive state from `bd` queries rather than maintaining a parallel state file — fully aligned.

## Architecture

### Three replacements for state.json

| Old (state.json) | New | Purpose |
|---|---|---|
| `Mode`, `ActiveSpec`, `ActiveMolecule`, `StepMapping` | `resolve.*` queries | Derived from molecule on demand |
| `ActiveBead` | `resolve.ResolveActiveBead()` | Query `bd list --status=in_progress` |
| `ActiveWorktree`, `SpecBranch` | Convention functions in `internal/state/` | Deterministic from spec/bead ID |
| `SessionSource`, `SessionStartedAt`, `BeadClaimedAt` | `.mindspec/session.json` | Transient per-session metadata |
| All of the above (cached) | `.mindspec/mode-cache` | Write-through cache for hook latency |

### Convention functions

```go
// SpecBranch returns the canonical branch name for a spec.
func SpecBranch(specID string) string { return "spec/" + specID }

// SpecWorktreePath returns the canonical worktree path for a spec.
func SpecWorktreePath(root, specID string) string {
    return filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
}

// BeadWorktreePath returns the canonical worktree path for a bead.
func BeadWorktreePath(root, beadID string) string {
    return filepath.Join(root, ".worktrees", "worktree-bead-"+beadID) // nested under spec worktree in practice
}
```

These already exist as inline string concatenations throughout the codebase. Centralizing them makes them testable and removes the need to store paths in state.

### Mode-cache format

```json
{
  "mode": "implement",
  "activeSpec": "053-drop-state-json",
  "activeBead": "beads-abc",
  "activeWorktree": "/path/to/.worktrees/worktree-bead-abc",
  "specBranch": "spec/053-drop-state-json",
  "timestamp": "2026-02-27T12:00:00Z"
}
```

Written by lifecycle commands (`next`, `complete`, `approve *`, `spec-init`, `explore`). Read by hooks. Stale cache is acceptable — hooks use it as a fast hint and fall back to molecule resolution if missing.

### session.json format

```json
{
  "sessionSource": "startup",
  "sessionStartedAt": "2026-02-27T12:00:00Z",
  "beadClaimedAt": "2026-02-27T12:01:00Z"
}
```

Written by `WriteSession()` (called by SessionStart hook) and bead claim in `next`. Read by session freshness gate in hooks and `next.go`.

## Testing Strategy

- **Unit tests**: Each bead includes targeted tests for the changed functions. Test the convention functions, session read/write, mode-cache read/write, resolver additions.
- **Integration test**: Bead 6 includes a full lifecycle integration test that exercises `spec-init` → `approve spec` → `approve plan` → `next` → `complete` → `approve impl` without any state.json file existing. This test uses the existing `internal/resolve/integration_test.go` pattern.
- **Regression**: `make test` must pass at every bead boundary. Existing tests are updated, not deleted — they verify the same behaviors against the new data sources.

## Dependency Graph

```
Bead 1 (state package rewrite)
  ├── Bead 2 (resolver additions) ──┐
  │                                  ├── Bead 4 (next.go + complete.go)
  │                                  ├── Bead 5 (approve + spec-init + explore)
  ├── Bead 3 (hook/guard migration) ─┘
  └── Bead 6 (cleanup + integration test)
         depends on: Bead 4, Bead 5
```

---

## Bead 1: Rewrite state package — Session, ModeCache, conventions

**Steps**
1. In `internal/state/state.go`: replace the `State` struct with two new structs: `Session` (3 fields: `SessionSource`, `SessionStartedAt`, `BeadClaimedAt`) and `ModeCache` (5 fields: `Mode`, `ActiveSpec`, `ActiveBead`, `ActiveWorktree`, `SpecBranch`, `Timestamp`)
2. Add `ReadSession(root)` / `WriteSession(root, s)` functions that read/write `.mindspec/session.json`
3. Add `ReadModeCache(root)` / `WriteModeCache(root, c)` functions that read/write `.mindspec/mode-cache`
4. Add convention functions: `SpecBranch(specID)`, `SpecWorktreePath(root, specID)`, `BeadWorktreePath(specWorktree, beadID)`
5. Keep the mode constants (`ModeIdle`, `ModeSpec`, etc.) — these are used everywhere as string constants
6. Deprecate but do not yet delete `State`, `Read()`, `Write()`, `SetMode()`, `SetModeWithMetadata()` — mark with `// Deprecated: will be removed in Bead 6`. This allows other beads to compile against both old and new APIs during migration
7. Add `.mindspec/mode-cache` to `.gitignore`

**Verification**
- [ ] `go test ./internal/state/...` passes with new Session/ModeCache tests
- [ ] `go build ./...` compiles (old API still present, just deprecated)
- [ ] Convention functions tested: `SpecBranch("053-foo")` → `"spec/053-foo"`, etc.

**Depends on**
None

---

## Bead 2: Resolver additions — ResolveActiveBead and spec metadata helpers

**Steps**
1. Add `ResolveActiveBead(root, specID string) (string, error)` to `internal/resolve/` — queries `bd list --status=in_progress --parent <implStepID> --json` using the spec's molecule binding from `specmeta.ReadForSpec()`. Returns the single in-progress bead ID, or `""` if none, or error if multiple (ambiguous)
2. Add `ResolveSpecBranch(root, specID string) string` — returns `state.SpecBranch(specID)` (thin wrapper for discoverability)
3. Add `ResolveWorktree(root, specID string) string` — returns `state.SpecWorktreePath(root, specID)`
4. Remove the `fallbackToCursor` in `resolve/target.go` that reads `state.Read(root).ActiveSpec` — replace with: if no active specs found via molecule scan and no `--spec` flag, return a clear error ("no active specs; use --spec flag")
5. Update `resolve` tests for the new functions and removed fallback

**Verification**
- [ ] `go test ./internal/resolve/...` passes
- [ ] `ResolveActiveBead` returns correct bead when one is in_progress, empty when none, error when multiple
- [ ] `ResolveTarget` no longer reads state.json (grep confirms no `state.Read` in resolve package)

**Depends on**
Bead 1

---

## Bead 3: Hook and guard migration — read mode-cache + session.json

**Steps**
1. In `internal/hook/dispatch.go`: replace `ReadState()` callers with a new `readHookState(root)` that constructs the needed fields from `state.ReadModeCache(root)` for Mode/ActiveWorktree/ActiveSpec and `state.ReadSession(root)` for session freshness fields. Return a lightweight struct (not the old `State`)
2. Define a `HookState` struct in `internal/hook/` with only the fields hooks actually use: `Mode`, `ActiveSpec`, `ActiveWorktree`, `SessionStartedAt`, `SessionSource`, `BeadClaimedAt`
3. Update `PlanGateExit`, `PlanGateEnter`, `WorkflowGuard` to use `HookState.Mode`
4. Update `WorktreeFile`, `WorktreeBash` to use `HookState.ActiveWorktree`, `HookState.ActiveSpec`
5. Update `SessionFreshnessGate` to use `HookState.SessionStartedAt`, `HookState.SessionSource`, `HookState.BeadClaimedAt`
6. Add fallback: if mode-cache is missing, call `resolve.ResolveMode()` for Mode (accept the latency on first access)
7. Update `internal/guard/guard.go` to accept `ActiveWorktree` and `ActiveSpec` from mode-cache via the same mechanism
8. Update all hook and guard tests

**Verification**
- [ ] `go test ./internal/hook/...` passes
- [ ] `go test ./internal/guard/...` passes
- [ ] No `state.Read` calls remain in `internal/hook/` or `internal/guard/` (grep confirms)

**Depends on**
Bead 1

---

## Bead 4: Migrate next.go and complete.go

**Steps**
1. **next.go — session freshness gate**: Replace `state.Read(root)` with `state.ReadSession(root)` for the freshness gate (lines 53-70). Read `SessionStartedAt`, `SessionSource`, `BeadClaimedAt` from session.json
2. **next.go — bead claim timestamp**: Replace `state.Read` + patch `BeadClaimedAt` + `state.Write` with `state.ReadSession` + patch + `state.WriteSession`
3. **next.go — mode transition**: Replace `state.SetModeWithMetadata()` / `state.SetMode()` with `state.WriteModeCache()` writing: mode from resolver, activeSpec, activeBead from selected bead, activeWorktree from convention function, specBranch from convention function
4. **next.go — worktree path**: Remove the second `state.Read` + patch `ActiveWorktree` + `state.Write` — this is now part of the single `WriteModeCache` call
5. **complete.go — Run()**: Replace `readStateFn(root)` with: `activeSpec` from `resolve.ResolveTarget()`, `activeBead` from argument or `resolve.ResolveActiveBead()`, `specBranch` from `state.SpecBranch(specID)`, worktree paths from convention functions
6. **complete.go — advanceState()**: Replace `readStateFn(root)` + `s.StepMapping["implement"]` with `specmeta.ReadForSpec(root, specID)` to get the molecule ID and step mapping. Query `bd ready --parent <implStepID>` as before
7. **complete.go — state writes**: Replace `setModeFn` / `writeStateFn` with `state.WriteModeCache()`
8. Remove the `readStateFn`, `writeStateFn`, `setModeFn` function vars (testability hooks for old API)
9. Update tests in `cmd/mindspec/` and `internal/complete/` — replace state.json setup with session.json + mode-cache setup

**Verification**
- [ ] `go test ./internal/complete/...` passes
- [ ] `go test ./cmd/mindspec/...` passes (next and complete tests)
- [ ] No `state.Read` or `state.Write` or `state.SetMode` calls remain in `next.go` or `complete.go`

**Depends on**
Bead 1, Bead 2

---

## Bead 5: Migrate approve, spec-init, explore, instruct

**Steps**
1. **approve/spec.go, approve/plan.go**: Replace `state.SetMode()` / `state.SetModeWithMetadata()` calls with `state.WriteModeCache()`. ActiveWorktree from convention function
2. **approve/impl.go**: Replace state reads (`ActiveSpec`, `SpecBranch`, `ActiveWorktree`) with resolver + convention functions. Replace `state.SetMode(root, ModeIdle, "", "")` with `state.WriteModeCache()` for idle transition
3. **cmd/mindspec/approve.go**: Remove any `state.Read()` calls; use resolver for targeting
4. **internal/specinit/specinit.go**: Replace `state.SetModeWithMetadata()` with `state.WriteModeCache()`
5. **internal/explore/explore.go**: Replace `state.SetMode()` calls in `Enter()`, `Dismiss()`, `Promote()` with `state.WriteModeCache()`
6. **cmd/mindspec/instruct.go + internal/instruct/instruct.go**: Remove state.json fallback path; use resolver exclusively for Mode/ActiveSpec. For ActiveBead in implement mode, call `resolve.ResolveActiveBead()`. Remove `state.Read()` calls
7. **cmd/mindspec/state.go**: Remove `stateSetCmd` entirely. Update `stateShowCmd` to use resolver only (no state.json fallback). Keep `stateWriteSessionCmd` writing to session.json
8. Update all affected tests

**Verification**
- [ ] `go test ./internal/approve/...` passes
- [ ] `go test ./internal/specinit/...` passes
- [ ] `go test ./internal/explore/...` passes
- [ ] `go test ./internal/instruct/...` passes
- [ ] `go test ./cmd/mindspec/...` passes
- [ ] `mindspec state set` subcommand no longer exists (`mindspec state set spec foo` returns unknown command)

**Depends on**
Bead 1, Bead 2

---

## Bead 6: Cleanup — delete old API, integration test, gitignore

**Steps**
1. In `internal/state/state.go`: delete deprecated `State` struct, `Read()`, `Write()`, `SetMode()`, `SetModeWithMetadata()`, `mainWorktreeRoot()` (dual-write logic), `copyStepMapping()`, `ErrNoState`
2. Update `workspace.go`: rename `StatePath()` to return session.json path (or remove if unused). Add `ModeCachePath()` if not already present
3. `grep -r 'state\.Read\|state\.Write\|state\.SetMode\|state\.State\b\|StatePath\|state\.json' internal/ cmd/` — fix any remaining references (excluding test fixture comments)
4. Verify `.mindspec/mode-cache` is in `.gitignore`
5. Add integration test: full lifecycle without state.json — create a temp repo, run through spec-init → approve spec → approve plan → next → complete → approve impl, assert no `.mindspec/state.json` exists at any point, assert mode-cache and session.json are written correctly at each step
6. `make test` — full suite
7. `make build` — verify binary builds clean

**Verification**
- [ ] `grep -r 'state\.json' internal/ cmd/` returns zero matches (excluding comments noting the migration)
- [ ] `make test` passes
- [ ] `make build` succeeds
- [ ] Integration test passes end-to-end lifecycle
- [ ] `.mindspec/mode-cache` is in `.gitignore`

**Depends on**
Bead 4, Bead 5

---

## Provenance

| Acceptance Criterion | Satisfied By |
|---|---|
| state.json never read or written | Bead 6 step 3 (grep verification) |
| state.json absent from fresh spec-init | Bead 5 (specinit migration), Bead 6 (integration test) |
| `state show` derives mode from molecule | Bead 5 step 7 |
| `next` claims bead with session.json + mode-cache | Bead 4 steps 1-4 |
| `complete` derives ActiveBead from Beads query | Bead 4 steps 5-6, Bead 2 step 1 |
| `approve *` transitions via molecule + mode-cache | Bead 5 steps 1-4 |
| `instruct` emits correct guidance without state.json | Bead 5 step 6 |
| Hooks function with mode-cache + session.json | Bead 3 |
| `make test` passes with no state.json refs | Bead 6 step 6 |
| Full lifecycle integration test | Bead 6 step 5 |
