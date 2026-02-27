---
approved_at: "2026-02-27T00:09:20Z"
approved_by: user
molecule_id: mindspec-mol-rlb4c
status: Approved
step_mapping:
    implement: mindspec-mol-7j2rs
    plan: mindspec-mol-w85bm
    plan-approve: mindspec-mol-gzy1q
    review: mindspec-mol-gvtid
    spec: mindspec-mol-6agkc
    spec-approve: mindspec-mol-iavcq
    spec-lifecycle: mindspec-mol-rlb4c
---



# Spec 050-lifecycle-autocommit: Auto-commit spec artifacts at lifecycle boundaries

## Goal

Ensure spec artifacts (spec.md, plan.md, frontmatter) are committed to the spec branch at lifecycle boundaries so that downstream worktrees (implementation bead branches) contain those artifacts when they branch from the spec branch.

## Background

`specinit.Run()` creates a `spec/NNN-slug` branch + worktree and writes `spec.md` to the worktree filesystem, but never commits. When `plan-approve` creates implementation beads and `mindspec next` creates worktrees for those beads, it branches from `s.SpecBranch`. Since nothing was committed to that branch, the new bead worktrees start without spec.md or plan.md.

The same gap exists in `ApproveSpec()` and `ApprovePlan()` — they modify files in the worktree without committing, so downstream branches miss approved spec/plan content.

## ADR Touchpoints

- [ADR-0006](../../adr/ADR-0006.md): Zero-on-main worktree-first flow — this spec ensures artifacts written to worktrees are committed before branching

## Impacted Domains

- specinit: needs auto-commit after writing spec.md + molecule frontmatter
- approve: spec and plan approval need auto-commits before state transitions
- gitops: needs a `CommitAll(workdir, message)` helper

## Requirements

1. Add `gitops.CommitAll(workdir, message)` that stages all changes and commits (no-op if clean)
2. `specinit.Run()` auto-commits after Phase 3 (spec.md + molecule frontmatter written)
3. `ApproveSpec()` auto-commits after updating spec approval frontmatter
4. `ApprovePlan()` auto-commits after writing plan approval + bead_ids
5. All auto-commits are best-effort (warnings, not hard errors) to avoid blocking the workflow

## Scope

### In Scope
- `internal/gitops/gitops.go` — new `CommitAll` function
- `internal/specinit/specinit.go` — auto-commit after Phase 3
- `internal/approve/spec.go` — auto-commit after spec approval
- `internal/approve/plan.go` — auto-commit after plan approval + bead_ids

### Out of Scope
- Changing the worktree branching strategy in `next`
- Auto-push to remotes

## Non-Goals

- Replacing manual commits by the agent during implementation
- Changing git branch topology

## Acceptance Criteria

- [ ] `make build` succeeds
- [ ] `make test` passes
- [ ] After `spec-init`, the spec worktree has a commit containing spec.md
- [ ] After `spec-approve`, the approval changes are committed to the spec branch
- [ ] After `plan-approve`, the plan approval + bead_ids are committed before implementation worktrees branch

## Validation Proofs

- `make build`: clean build
- `make test`: all tests pass
- `git -C <spec-worktree> log --oneline -1`: shows auto-commit after spec-init

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-27
- **Notes**: Approved via mindspec approve spec