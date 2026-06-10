---
name: ms-bead-impl
description: Dispatch an implementation subagent for a claimed mindspec bead, with optional pre-staged prompt
---

# Bead Implementation Dispatch

Spawn a `general-purpose` subagent to land one PR-sized implementation for a single bead. You orchestrate; the subagent codes.

## Inputs

- `bead-id` (required) — the `bd` id, e.g. `lola-8gbp.3`.
- `prompt-path` (optional) — pre-staged implementation prompt at `<repo>/review/prep/bead<N>_impl_prompt.md`. If absent, draft one in-conversation before dispatching.

## Steps

1. **Verify bead state.**
   ```bash
   bd show <bead-id>
   ```
   - Status must be `open` or `in_progress`.
   - Note the spec branch from the parent epic.

2. **Confirm the bead worktree.**
   ```bash
   mindspec next --spec <spec-slug> --pick <N> --force
   ```
   The worktree lands at `<spec-worktree>/.worktrees/worktree-<bead-id>/` on branch `bead/<bead-id>`. Fallback if mindspec flakes:
   ```bash
   bd update <bead-id> --claim --status in_progress
   cd <spec-worktree>
   git worktree add .worktrees/worktree-<bead-id> -b bead/<bead-id> <spec-branch>
   ```

3. **Load or draft the prompt.** If `prompt-path` is supplied, read it. If absent, call `/ms-bead-prep <bead-id>` first to stage one at `<repo>/review/prep/bead<N>_impl_prompt.md`, then read it. Only fall back to in-conversation drafting if `/ms-bead-prep` is unavailable (e.g. plugin not installed). Drafted prompts should include:
   - Bead id + branch + worktree path
   - "Read plan.md §<bead> + spec.md"
   - Files that *must* be touched (from the plan)
   - Shared helpers / models from prior beads (with import paths)
   - Tests to add (from plan ACs)
   - **What NOT to do**: no `mindspec complete`, no `git push`, no scope creep, no unrelated cleanup
   - "Report back: commit SHA, test pass/fail/skip counts, deviations from the prompt"

4. **Dispatch.** Spawn a `general-purpose` agent with the prompt as `prompt`. Run in background if you have other parallel work; otherwise foreground.
   - The subagent should make exactly ONE commit on the bead branch.
   - It should NOT merge, push, or complete the bead.

5. **On return, capture:**
   - Commit SHA → record in conversation
   - Test summary → record
   - Any flagged deviations → add to the BRIEF for the panel review (these become "fix-author deviation (A/B/...)" sections)

## Then

Hand off to `/ms-panel-create` to set up the round-1 review panel.

## Anti-patterns

- Don't ask the subagent to "iterate" or "fix issues as they come up" — that's the panel's job, not the impl agent's.
- Don't pass the spec or plan in-line; reference paths so the subagent reads fresh.
- Don't let the subagent run `mindspec complete` — separation of concerns; the cycle controls merge.
