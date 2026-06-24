---
approved_at: "2026-03-03T20:46:45Z"
approved_by: user
spec_id: 060-eliminate-focus-lifecycle
status: Approved
version: "1"
bead_ids:
  - mindspec-3zab
  - mindspec-3076
  - mindspec-u1l1
  - mindspec-et59
  - mindspec-9iz2
  - mindspec-oqwm
---
# Plan: 060-eliminate-focus-lifecycle

## ADR Fitness

- **ADR-0023** (accepted): This plan implements all decisions in ADR-0023. Every bead maps to a rollout step.
- **ADR-0022**: Worktree path conventions unchanged; invariant 5 superseded per ADR-0023.
- **ADR-0020**: Per-spec lifecycle file eliminated per ADR-0023.

## Testing Strategy

- **Unit tests**: Each new function (`DerivePhase`, `ResolveContext`, `DiscoverActiveSpecs`, `CheckSpecNumberCollision`) gets table-driven unit tests covering all phase derivation edge cases.
- **Integration tests**: Existing `approve`, `complete`, `next`, `specinit` test suites updated to verify no focus/lifecycle files are created.
- **LLM test harness**: All 17 scenario setups and assertions migrated from focus-based to beads-based (Bead 6).
- **Regression gate**: `grep -r "ReadFocus\|WriteFocus\|ReadLifecycle\|WriteLifecycle" internal/` in CI must return zero matches in production code.

## Bead 1: Core derivation functions

Add the new beads-query layer that all other beads depend on.

**Steps**
1. Create `internal/lifecycle/` package (or extend `internal/resolve/`) with:
   - `DerivePhase(epicID string) → Phase` — query `bd list --parent <epicID> --json`, count statuses, return phase per ADR-0023 §3 table
   - `DiscoverActiveSpecs() → []ActiveSpec` — query `bd list --type=epic --status=open --json`, parse `metadata.spec_num`/`spec_title`, derive phase for each
   - `ResolveContext(root string) → Context` — combine worktree path convention parsing with beads query to return `(specID, beadID, phase, worktreePath)`
   - `CheckSpecNumberCollision(specNum int) → error` — `bd dolt pull`, query all epics, check `metadata.spec_num`
   - `SpecIDFromMetadata(specNum int, specTitle string) → string`
2. Write table-driven unit tests for `DerivePhase` covering all 8 rows of the phase table plus edge cases (mixed open/closed, multiple in_progress, empty epic)
3. Write unit tests for `ResolveContext` (main worktree, spec worktree, bead worktree paths)
4. Write unit test for `CheckSpecNumberCollision` (no collision, collision found)

**Verification**
- [ ] `go test ./internal/lifecycle/...` passes (or whichever package)
- [ ] All 8 phase derivation cases tested
- [ ] `ResolveContext` resolves correctly from main, spec worktree, and bead worktree

**Depends on**
None

## Bead 2: Migrate spec approve to epic-creation gate

Move epic creation from `specinit` to `spec approve`. Add collision prevention.

**Steps**
1. Update `internal/approve/spec.go`:
   - Remove `state.ReadLifecycle()` / `state.WriteLifecycle()` calls (lines 43-50)
   - Remove `state.WriteFocus()` call (lines 69-82)
   - Add: `bd dolt pull`, `CheckSpecNumberCollision(specNum)`, create epic with `--metadata='{"spec_num":N,"spec_title":"slug"}'`, `bd dolt push`
   - Parse spec_num and spec_title from the specID argument
2. Update `internal/specinit/specinit.go`:
   - Remove epic creation (Phase 3, ~lines 115-130)
   - Remove `state.WriteLifecycle()` call (lines 135-138)
   - Remove `state.WriteFocus()` call (line 156)
3. Update `internal/approve/plan.go`:
   - Remove `state.WriteFocus()` call
   - Change epic lookup: instead of reading `lifecycle.yaml` for `epic_id`, query beads for epic with matching `metadata.spec_num`
   - Remove `state.WriteLifecycle()` call
4. Update existing unit tests for all three files

**Verification**
- [ ] `go test ./internal/approve/... ./internal/specinit/...` passes
- [ ] `spec approve` creates epic with correct metadata
- [ ] `spec approve` rejects on spec_num collision
- [ ] `spec create` no longer writes lifecycle.yaml or focus
- [ ] `plan approve` finds epic via beads metadata query (not lifecycle.yaml)

**Depends on**
Bead 1

## Bead 3: Migrate impl approve and cleanup

Update `impl approve` and `cleanup` to derive state from beads.

**Steps**
1. Update `internal/approve/impl.go`:
   - Replace `readApproveImplFocus()` with `ResolveContext()` — verify phase is review
   - Remove `state.ReadLifecycle()` / `state.WriteLifecycle()` calls (lines 99-114)
   - Remove `state.WriteFocus()` calls (lines 167-174)
   - Epic close: query beads for epic by `metadata.spec_num` instead of reading lifecycle.yaml
2. Update `internal/cleanup/cleanup.go`:
   - Replace `state.ReadFocus()` with `ResolveContext()` for active-spec check
   - Remove `state.WriteFocus()` idle reset (beads state is authoritative)
3. Update unit tests for both files

**Verification**
- [ ] `go test ./internal/approve/... ./internal/cleanup/...` passes
- [ ] `impl approve` works without reading/writing focus or lifecycle.yaml
- [ ] `cleanup` works without focus file

**Depends on**
Bead 1

## Bead 4: Migrate complete and next

Update bead completion and work selection to use beads-derived context.

**Steps**
1. Update `internal/complete/complete.go`:
   - Replace `state.ReadFocus()` with `ResolveContext()` to get active bead/spec
   - Replace `state.ReadLifecycle()` with beads epic query for next-bead determination
   - Remove `state.WriteFocus()` call (line 208)
2. Update `internal/next/beads.go`:
   - Replace `readFocusFn` / `writeFocusFn` with `ResolveContext()`
   - Replace `state.ReadLifecycle()` with beads epic query for epic_id
   - Remove focus propagation into bead worktree
3. Update `cmd/mindspec/next.go`:
   - Replace `state.ReadLifecycle()` with beads query
   - Remove `state.WriteFocus()` calls
4. Update `cmd/mindspec/complete.go`:
   - Replace `state.ReadFocus()` worktree redirect with `ResolveContext()`
5. Update unit tests

**Verification**
- [ ] `go test ./internal/complete/... ./internal/next/... ./cmd/mindspec/...` passes
- [ ] `mindspec next` claims bead without reading/writing focus
- [ ] `mindspec complete` closes bead without reading/writing focus

**Depends on**
Bead 1

## Bead 5: Migrate instruct, state commands, and resolve

Update context-reading commands and discovery.

**Steps**
1. Update `cmd/mindspec/instruct.go`:
   - Replace `state.ReadFocus()` with `ResolveContext()`
   - Derive mode from beads phase
2. Update `cmd/mindspec/instruct_tail.go`:
   - Replace `state.ReadFocus()` with `ResolveContext()`
3. Update `cmd/mindspec/state.go`:
   - `state show`: derive from `ResolveContext()` instead of `ReadFocus()`
   - `state set`: either remove entirely (beads is authoritative) or repoint to update beads metadata
4. Update `internal/resolve/resolve.go`:
   - Replace `state.ReadLifecycle()` scan with `DiscoverActiveSpecs()` beads query

**Verification**
- [ ] `go test ./cmd/mindspec/... ./internal/resolve/...` passes
- [ ] `mindspec instruct` emits correct guidance from beads-derived phase
- [ ] `mindspec state show` displays correct state without focus file

**Depends on**
Bead 1

## Bead 6: Dead code removal and cleanup

Remove all focus/lifecycle code and update harness.

**Steps**
1. Delete from `internal/state/state.go`:
   - `Focus` struct, `ReadFocus()`, `WriteFocus()`
   - `Lifecycle` struct, `ReadLifecycle()`, `WriteLifecycle()`
   - Keep mode constants (`ModeIdle`, `ModeSpec`, etc.) and path helpers (`SpecBranch()`, `SpecWorktreePath()`)
2. Remove `sandbox.WriteFocus()` / `sandbox.WriteLifecycle()` from `internal/harness/sandbox.go`
3. Replace `assertFocusMode` / `assertFocusFields` in `internal/harness/scenario.go` with beads-based assertions (use existing `assertBeadsState`)
4. Update all 17 LLM scenario setups: replace `WriteFocus`/`WriteLifecycle` with `CreateBead` + epic metadata
5. Remove `.mindspec/focus` from `.gitignore` entries
6. Add `mindspec doctor` check: detect and warn about stale focus/lifecycle.yaml files
7. Run `grep -r "ReadFocus\|WriteFocus\|ReadLifecycle\|WriteLifecycle" internal/` — must return zero matches
8. `make test` — full suite passes

**Verification**
- [ ] `grep -r "ReadFocus\|WriteFocus\|ReadLifecycle\|WriteLifecycle" internal/` returns nothing
- [ ] `make test` passes
- [ ] All 17 LLM scenarios have beads-based assertions
- [ ] `mindspec doctor` detects stale focus/lifecycle.yaml files

**Depends on**
Bead 2, Bead 3, Bead 4, Bead 5

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| No ReadFocus/WriteFocus/ReadLifecycle/WriteLifecycle in production code | Bead 6 grep check |
| `mindspec instruct` derives phase from beads | Bead 5 verification |
| `spec approve` creates epic with spec_num/spec_title | Bead 2 verification |
| `spec approve` rejects on spec_num collision | Bead 2 verification |
| `plan approve` creates child beads (no epic creation) | Bead 2 verification |
| `impl approve` without focus/lifecycle | Bead 3 verification |
| `next` and `complete` without focus | Bead 4 verification |
| All LLM test assertions beads-based | Bead 6 verification |
| grep returns zero matches | Bead 6 verification |
| No focus/lifecycle.yaml created during lifecycle | Bead 6 full test suite |
| Phase derivation matches design table | Bead 1 unit tests |
