---
approved_at: "2026-02-27T00:09:42Z"
approved_by: user
bead_ids:
    - mindspec-mol-7j2rs.1
    - mindspec-mol-7j2rs.2
    - mindspec-mol-7j2rs.3
last_updated: 2026-02-27T00:00:00Z
spec_id: 050-lifecycle-autocommit
status: Approved
version: 1
---

# Plan: 050-lifecycle-autocommit

## ADR Fitness

- **ADR-0006 (Zero-on-main)**: Still sound. This spec reinforces the worktree-first flow by ensuring files written to worktrees are committed before downstream branches are created. No divergence needed.

## Testing Strategy

- Unit tests for the new `CommitAll` helper in `internal/gitops/`
- Existing tests for `specinit`, `approve/spec`, `approve/plan` should still pass (auto-commits are best-effort warnings)
- Manual validation: `spec-init` should produce a commit visible in `git log` of the worktree

## Bead 1: Add CommitAll helper to gitops

**Steps**
1. Add `CommitAll(workdir, message string) error` to `internal/gitops/gitops.go`
2. Check `git status --porcelain` first — no-op if clean
3. Run `git -C workdir add -A` then `git -C workdir commit -m message`
4. Add unit test in `internal/gitops/gitops_test.go`

**Verification**
- [ ] `go test ./internal/gitops/...` passes
- [ ] New test covers clean-tree no-op and dirty-tree commit paths

**Depends on**
None

## Bead 2: Auto-commit in specinit after Phase 3

**Steps**
1. Import `gitops` in `internal/specinit/specinit.go`
2. After Phase 3 (molecule frontmatter written), call `gitops.CommitAll(wtPath, "chore: initialize spec <id>")`
3. Treat as best-effort warning (don't fail the whole init)
4. Add/update test to verify commit happens

**Verification**
- [ ] `go test ./internal/specinit/...` passes
- [ ] `make build` succeeds

**Depends on**
Bead 1

## Bead 3: Auto-commit in ApproveSpec and ApprovePlan

**Steps**
1. In `internal/approve/spec.go`, after `updateSpecApproval()`, call `gitops.CommitAll` using `state.Read(root).ActiveWorktree`
2. In `internal/approve/plan.go`, after `writeBeadIDsToFrontmatter()`, call `gitops.CommitAll` using `state.Read(root).ActiveWorktree`
3. Both are best-effort warnings
4. Add imports for `gitops` and `state` as needed

**Verification**
- [ ] `go test ./internal/approve/...` passes
- [ ] `make build` succeeds
- [ ] `make test` passes (full suite)

**Depends on**
Bead 1

## Provenance

| Acceptance Criterion | Bead | Verification |
|---|---|---|
| `make build` succeeds | Bead 2, 3 | `make build` |
| `make test` passes | All | `make test` |
| After spec-init, worktree has commit with spec.md | Bead 2 | `git log` in worktree |
| After spec-approve, approval changes committed | Bead 3 | `git log` in worktree |
| After plan-approve, plan + bead_ids committed | Bead 3 | `git log` in worktree |
