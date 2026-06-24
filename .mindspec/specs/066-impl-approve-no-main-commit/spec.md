---
approved_at: "2026-03-04T00:04:32Z"
approved_by: user
status: Approved
---
# Spec 066: Stop impl-approve from causing main merge conflicts

## Goal

Eliminate the merge conflicts that occur on main after `impl approve` by removing the session close protocol's commit-on-main step from the impl-approve skill and ensuring all artifacts are committed on the spec branch before cleanup.

## Background

The current `/ms-impl-approve` skill tells the agent to run a session close protocol after `mindspec approve impl` — this includes `git add`, `git commit`, and `git push` on main. This is problematic because:

1. `impl approve` pushes the spec branch and suggests creating a PR
2. After worktree cleanup, the agent is back on main
3. The skill's session close protocol commits beads state files on main
4. If the PR was already merged (or origin/main moved), `git push` fails
5. The agent attempts to rebase/merge, causing conflicts on main

Evidence: The main branch was found in a broken rebase state with diverged commits (`1 local, 11 remote`), caused by a beads backup commit on main conflicting with remote.

## Impacted Domains

- skill: `/ms-impl-approve` session close protocol removes commit-on-main
- approve: `internal/approve/impl.go` auto-commits all artifacts before cleanup

## ADR Touchpoints

None — this is a bug fix to existing behavior.

## Requirements

1. The `/ms-impl-approve` skill MUST NOT instruct the agent to commit on main after impl approve
2. `impl approve` MUST auto-commit all remaining artifacts (beads state, recordings) on the spec branch before removing worktrees
3. After impl approve completes, the agent should only need to `git push` on main if there are actual code changes (there shouldn't be any)
4. The skill should instruct the agent to `bd dolt push` for beads state sync (this is independent of git)

## Scope

### In Scope
- `.claude/skills/ms-impl-approve/SKILL.md` — remove commit-on-main from session close
- `internal/approve/impl.go` — ensure auto-commit covers all artifacts before cleanup

### Out of Scope
- Changes to the git merge/PR flow itself
- Changes to other skills or lifecycle commands

## Non-Goals

- Changing how PRs are created or merged
- Adding local merge-to-main as an alternative to PRs

## Acceptance Criteria

- [ ] `/ms-impl-approve` skill does not instruct agent to `git add`/`git commit`/`git push` on main
- [ ] `impl approve` auto-commits all artifacts on spec branch before worktree removal
- [ ] After impl approve, main has no uncommitted changes
- [ ] Skill instructs agent to run `bd dolt push` for beads sync

## Validation Proofs

- `cat .claude/skills/ms-impl-approve/SKILL.md`: No `git add`/`git commit`/`git push` on main in session close
- `grep -n commitAll internal/approve/impl.go`: Auto-commit happens before worktree cleanup

## Open Questions

None.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-04
- **Notes**: Approved via mindspec approve spec