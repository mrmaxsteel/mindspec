---
name: ms-bead-next
description: Pick the next ready mindspec bead, claim it, and set up the worktree — the entry point for one bead cycle
---

# Pick + Claim Next Bead

Find the next bead the autopilot should work on, claim it in `bd`, and have a worktree ready.

## Inputs

- `spec-slug` (optional) — restrict to a given spec; otherwise honour the project's currently-active spec from `mindspec state show`.

## Steps

1. **Resolve the active spec.**
   ```bash
   mindspec state show
   ```
   The output names the active spec id, branch, and worktree. If multiple specs are active, ask the user which to focus on.

2. **List ready beads scoped to the spec.**
   ```bash
   bd ready 2>&1 | grep "<spec-prefix>"
   ```
   `bd ready` already filters to beads with no blocking deps. Cross-check against the plan's `**Depends on:**` lines for each bead — `bd` and the plan can disagree (see lola spec-050 where `bd` showed Bead 5 ready while the plan said it depended on Beads 1-4). When they disagree, **trust the plan**.

3. **Pick deterministically.** Among the plan-eligible ready beads, pick the lowest-numbered one (`.3` before `.4` before `.5`). If two beads of the same depth are eligible, pick the one with the smaller `bd id`.

4. **Claim.**
   ```bash
   mindspec next --spec <spec-slug> --pick 1 --force
   ```
   This claims the bead, creates `bead/<id>` branch off the spec branch, and creates a worktree at `<spec-worktree>/.worktrees/worktree-<bead-id>/`.

   Fallback if `mindspec next` flakes (seen on lola when the bead description exceeds the `events.old_value` column):
   ```bash
   bd update <bead-id> --claim --status in_progress
   cd <spec-worktree>
   git worktree add .worktrees/worktree-<bead-id> -b bead/<bead-id> <spec-branch>
   ```

5. **Verify state.**
   - `bd show <bead-id>` shows `in_progress`.
   - The worktree dir exists.
   - `git -C <worktree> branch --show-current` reports `bead/<bead-id>`.

## Output

Return to the caller (`/ms-bead-cycle` typically):
- `bead-id`
- `bead-worktree` (absolute path)
- `bead-branch`
- `plan-section-path` — `<spec-dir>/plan.md` with anchor `## Bead <N>:` for the impl agent to read.

## Anti-patterns

- Don't claim multiple beads at once. The autopilot is serial per bead by design (parallelism happens *inside* each cycle, across reviewers + impl agent).
- Don't pick a bead `bd` says is ready if the plan dep graph disagrees — investigate the discrepancy and fix `bd` (`bd update <bead> --add-dep <blocker>`) before proceeding.

## Then

Hand off to `/ms-bead-impl` with the four output fields above.
