---
approved_at: "2026-02-27T00:24:43Z"
approved_by: user
bead_ids:
    - mindspec-mol-a4bf4.1
    - mindspec-mol-a4bf4.2
    - mindspec-mol-a4bf4.3
last_updated: 2026-02-27T00:00:00Z
spec_id: 051-worktree-state-propagation
status: Approved
version: 1
---

# Plan: 051-worktree-state-propagation

## ADR Fitness

- **ADR-0006 (Zero-on-main)**: Still sound. This spec extends the worktree isolation model so bead worktrees are truly self-contained. No divergence needed.

## Testing Strategy

- Unit tests for each changed function (state propagation, guard relaxation, hook update)
- Existing test suites must continue passing (`make test`)
- CI must pass (the CI failure pattern from 050 — missing git config in tests — will be watched for)

## Bead 1: Propagate state into bead worktrees

`EnsureWorktree()` in `internal/next/beads.go` creates bead worktrees but doesn't write state into them. `specinit.Run()` already has the pattern (lines 166-173): write state to both main root and worktree root.

**Steps**
1. In `EnsureWorktree()`, after the worktree is created (line 245), read the current state from root
2. Clone the state, update `ActiveWorktree` to point to the new bead worktree path, and set `ActiveBead` to the bead ID
3. Write the cloned state into the bead worktree root via `state.Write(wtPath, beadState)`
4. This is best-effort (warning on failure, not hard error)
5. Add a test in `internal/next/beads_test.go` that verifies state is written into the bead worktree

**Verification**
- [ ] `go test ./internal/next/...` passes
- [ ] `make build` succeeds

**Depends on**
None

## Bead 2: Relax CWD guard for spec worktree

`guard.CheckCWD()` blocks any CWD that isn't inside `ActiveWorktree`. When `ActiveWorktree` points to a bead worktree, the spec worktree (its parent) is blocked. The fix: also allow CWD inside the spec worktree (identifiable via `s.SpecBranch` → worktree-spec-<specID>).

**Steps**
1. In `guard.CheckCWD()`, after checking `cwdAbs` against `wtAbs`, also check if CWD is inside the spec worktree
2. Derive the spec worktree path from root + config.WorktreeRoot + "worktree-spec-" + s.ActiveSpec
3. If CWD is under the spec worktree path, return nil (allow)
4. Apply the same logic in `WorktreeBash()` in `internal/hook/dispatch.go`: also pass if CWD is inside the spec worktree
5. Add/update tests in `internal/guard/guard_test.go` and `internal/hook/dispatch_test.go`

**Verification**
- [ ] `go test ./internal/guard/...` passes
- [ ] `go test ./internal/hook/...` passes
- [ ] `make build` succeeds

**Depends on**
None

## Bead 3: Add /mindspec to .gitignore

Running `go build` without `-o` in a worktree produces a `mindspec` binary at repo root. This gets swept into `git add -A` and committed accidentally.

**Steps**
1. Add `/mindspec` to the root `.gitignore` (the leading `/` anchors to repo root only)
2. Verify it doesn't affect `./bin/mindspec` or `cmd/mindspec/` paths
3. Add a test in `internal/gitops/gitops_test.go` that verifies `EnsureGitignoreEntry` handles root-anchored patterns

**Verification**
- [ ] `go test ./internal/gitops/...` passes
- [ ] `make build` still works (produces `./bin/mindspec`)
- [ ] `make test` passes

**Depends on**
None

## Provenance

| Acceptance Criterion | Bead | Verification |
|---|---|---|
| `mindspec next` creates bead worktree with state.json | Bead 1 | Unit test + `cat` state.json |
| `mindspec complete` succeeds from spec worktree | Bead 2 | guard test |
| `impl-approve` works from spec worktree without bypass | Bead 2 | hook dispatch test |
| `go build` doesn't leave committable binary | Bead 3 | gitignore check |
| `make build` succeeds | All | `make build` |
| `make test` passes | All | `make test` |
| CI passes | All | `gh pr checks` |
