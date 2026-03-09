# MindSpec — Implementation Mode

**Active Spec**: `{{.ActiveSpec}}`
**Active Bead**: `{{.ActiveBead}}`
{{- if .SpecGoal}}
**Goal**: {{.SpecGoal}}
{{- end}}

## MindSpec Lifecycle

```
idle ── spec ── plan ──── >>> implement ── review ── idle
```

| Phase | Command | What happens |
|-------|---------|--------------|
| idle → spec | `mindspec spec create <slug>` | Creates branch + worktree + spec template |
| spec → plan | `mindspec spec approve <id>` | Validates spec, auto-commits |
| plan → impl | `mindspec plan approve <id>` | Validates plan, auto-creates beads, auto-claims first bead |
| per bead | `mindspec next` | Claims next bead, creates bead worktree |
| bead done | `mindspec complete "msg"` | Auto-commits, closes bead, merges bead→spec, removes worktree |
| review → idle | `mindspec impl approve <id>` | Merges spec→main, removes all worktrees + branches |

### Git rules
- You should not need any raw git commands — all git operations are handled by mindspec
- Raw git is available for repair/recovery but the happy path never requires it

## Objective

Execute the active bead in an isolated worktree. Stay within scope.

## Worktree Bootstrap — MANDATORY
{{- if .ActiveWorktree}}

**Active Worktree**: `{{.ActiveWorktree}}`
{{- if .InWorktree}}

You are in the bead worktree. All code changes go here, not the main repo.
{{- else}}

Run `cd {{.ActiveWorktree}}` to enter the bead worktree. All code changes go there, not the main repo.
{{- end}}
{{- else}}

**No active worktree.** Run `mindspec next` NOW — before any other action. Do NOT create files, edit code, or run `git commit` until `mindspec next` has succeeded.
{{- end}}

Do NOT create manual workflow branches/worktrees in implement mode.
After `mindspec complete` succeeds, STOP. Do NOT automatically continue to the next bead — let the user decide when to proceed.
If the user asks for an interrupt fix (urgent bug + continue feature), do both:
1. Apply and commit the urgent fix.
2. Resume bead scope and produce the requested feature artifact(s).
Do not stop after step 1.
Never report completion unless required files exist and `mindspec complete` succeeds.

## Permitted Actions

- Code changes within the bead's defined scope
- Test creation for the bead's scope
- Documentation updates (doc-sync is mandatory)
- Capturing proof/evidence (command outputs, test results)

## Forbidden Actions

- Widening scope beyond the bead definition (discovered work becomes new beads)
- Ignoring ADR divergence
- Completing a bead without proof and doc-sync
- Making changes outside the assigned worktree
- Creating worktrees via raw tooling (`bd worktree create`, `git worktree add`) instead of `mindspec next`
- Closing beads directly with `bd close` — use `mindspec complete` instead
- Manually closing the lifecycle epic — `mindspec impl approve` handles epic closure automatically

## Obligations

1. **Scope discipline**: Changes must stay within the bead's scope
2. **Doc sync**: Every code change must update corresponding documentation
3. **Proof of done**: Bead can be completed only when verification steps pass with captured evidence
4. **Worktree isolation**: Work in the bead-specific worktree
5. **ADR compliance**: Follow cited ADRs; divergence triggers the divergence protocol

## Human Gates

- **ADR divergence**: If implementation requires deviation from a cited ADR, stop immediately and inform the user
- **Scope expansion**: Discovered work must be filed as new beads, not absorbed into this bead

## Completion

When the bead is done:

1. Run verification steps and capture evidence
2. Update documentation (doc-sync)
3. Run `mindspec complete "describe what you did"` — auto-commits all changes, closes the bead, merges bead→spec, removes the worktree, and advances state
4. **STOP and report completion** — do NOT automatically continue to the next bead. The user will run `mindspec next` when ready

**Do NOT use `bd close` to finish a bead.** It skips merge topology, worktree cleanup, and state transitions. Always use `mindspec complete`.

## Next Action
{{- if .ActiveBead}}

1. Implement the bead's scope
2. Run `mindspec complete "describe what you did"` to finish
{{- else}}

1. Run `mindspec next` to claim a bead and enter its worktree
2. Implement the bead's scope
3. Run `mindspec complete "describe what you did"` to finish
{{- end}}
