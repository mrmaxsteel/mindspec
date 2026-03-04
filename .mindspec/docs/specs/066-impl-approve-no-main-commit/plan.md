---
adr_citations: []
approved_at: "2026-03-04T00:05:15Z"
approved_by: user
bead_ids:
    - mindspec-scal.1
last_updated: "2026-03-04"
spec_id: 066-impl-approve-no-main-commit
status: Approved
version: "1"
---
# Plan: 066 — Stop impl-approve from causing main merge conflicts

## ADR Fitness

No ADRs are directly relevant. This is a bug fix to skill instructions and minor hardening of existing auto-commit in `impl.go`. No architectural decisions are being changed.

## Testing Strategy

- Manual verification: inspect updated skill file for absence of commit-on-main instructions
- Build verification: `make build` ensures no compile errors
- Existing `impl_test.go` covers auto-commit behavior

## Bead 1: Fix impl-approve skill and verify auto-commit coverage

**Steps**
1. Update `.claude/skills/ms-impl-approve/SKILL.md` — remove the session close protocol that instructs `git add`/`git commit`/`git push` on main. Replace with `bd dolt push` for beads sync only.
2. Verify `internal/approve/impl.go` already auto-commits artifacts on the spec branch before worktree removal (line 131-134).
3. If any bead worktrees also need auto-commit before cleanup, add it in `cleanupBeadBranchesAndWorktrees`.
4. Run `make build` to verify.

**Verification**
- [ ] `cat .claude/skills/ms-impl-approve/SKILL.md` shows no `git add`/`git commit`/`git push` on main
- [ ] `grep -n commitAll internal/approve/impl.go` confirms auto-commit before worktree removal
- [ ] `make build` succeeds
- [ ] `make test` passes

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---|---|
| Skill does not instruct git add/commit/push on main | Bead 1: `cat` skill file |
| impl approve auto-commits on spec branch before cleanup | Bead 1: `grep commitAll` |
| After impl approve, main has no uncommitted changes | Bead 1: follows from removing commit-on-main |
| Skill instructs bd dolt push | Bead 1: `cat` skill file |
