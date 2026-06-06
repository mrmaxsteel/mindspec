---
name: ms-bead-merge
description: Run `mindspec complete` on a panel-approved bead — auto-commits, merges to spec branch, removes the worktree
---

# Bead Merge

The panel approved (≥5/6 APPROVE). Close the bead in `bd`, merge `bead/<id>` into the spec branch, and remove the bead worktree. This is `mindspec complete` plus a few sanity checks around it.

## Inputs

- `bead-id` (required).
- `summary` (required) — 1-2 sentence summary of what landed, for the merge commit message.

## Steps

1. **Sanity-check the bead state.**
   ```bash
   bd show <bead-id>          # status: in_progress
   git -C <bead-worktree> status  # working tree clean
   git -C <bead-worktree> log --oneline <spec-branch>..HEAD  # 1-2 commits visible (impl + optional fix-up)
   ```
   If anything is uncommitted, abort and ask the user.

2. **Run the merge.**
   ```bash
   mindspec complete <bead-id> "<summary>"
   ```
   This auto-commits any stray staged files, closes the bead in `bd`, merges `bead/<id>` into the spec branch with a `Merge bead/<id>` commit, and removes the bead worktree.

3. **Handle ADR-divergence errors.** `mindspec complete` runs `mindspec validate` against modified files. If you see:
   ```
   error: adr-divergence: [adr-divergence-unowned] file X is not claimed by any OWNERSHIP.yaml
   ```
   - Verify the modified file actually belongs to the bead's domain.
   - If yes, add it to the relevant `.mindspec/docs/domains/<name>/OWNERSHIP.yaml`.
   - If the file was modified accidentally (e.g. a stray `.gitignore` edit picked up by auto-stage), revert that change and re-run.
   - Only use `--override-adr "<reason>"` after these checks.

4. **Verify the merge.**
   ```bash
   git -C <spec-worktree> log --oneline -3  # should show `Merge bead/<bead-id>` at top
   git worktree list | grep <bead-id>        # should be absent
   bd show <bead-id>                         # status: closed
   ```

5. **Do NOT push.** The autopilot's user-controlled gate is at the end of the spec, not per-bead. `mindspec complete` leaves the spec branch with one extra merge commit; pushing happens later via `git push` after the whole spec is approved.

## Anti-patterns

- Don't run `mindspec complete` until the panel has actually approved. The merge is destructive of the bead branch — re-doing it requires `git reset` on the spec branch.
- Don't `git push` after every bead merge. Single push at end-of-spec keeps CI runs proportional to logical units of work.
- Don't manually merge with `git merge bead/<id>` — bypasses `bd` closure and worktree cleanup. Always go through `mindspec complete`.
- Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first.

## Then

Hand off to `/ms-bead-next` (orchestrator continues) or `/ms-impl-approve` (no more beads).
