# MindSpec — Implementation Mode

**Active Spec**: `{{.ActiveSpec}}`
**Active Bead**: `{{.ActiveBead}}`
{{- if .SpecGoal}}
**Goal**: {{.SpecGoal}}
{{- end}}

## Objective

Execute the active bead in an isolated worktree. Stay within scope.

## Permitted Actions

- Code changes within the bead's defined scope
- Test creation for the bead's scope
- Documentation updates (doc-sync is mandatory)
- Capturing proof/evidence (command outputs, test results)
- Updating bead status in Beads (`bd update`, `bd close`)

## Forbidden Actions

- Widening scope beyond the bead definition (discovered work becomes new beads)
- Ignoring ADR divergence
- Completing a bead without proof and doc-sync
- Making changes outside the assigned worktree

## Obligations

1. **Scope discipline**: Changes must stay within the bead's scope
2. **Doc sync**: Every code change must update corresponding documentation
3. **Proof of done**: Bead closes only when verification steps pass with captured evidence
4. **Worktree isolation**: Work in the bead-specific worktree
5. **ADR compliance**: Follow cited ADRs; divergence triggers the divergence protocol

## Human Gates

- **ADR divergence**: If implementation requires deviation from a cited ADR, stop immediately and inform the user
- **Scope expansion**: Discovered work must be filed as new beads, not absorbed into this bead

## Commit Convention

Use `impl({{.ActiveBead}}): <summary>` for implementation commits.

## Completion Checklist

When the bead is done:

1. Run verification steps and capture evidence
2. Update documentation (doc-sync)
3. Commit all changes (`git add` + `git commit`) — you MUST commit before completing
4. Run `mindspec complete` — closes the bead, removes the worktree, and advances state automatically

## Next Action

Implement the bead's scope, then follow the completion checklist above.

## Session Close

Before ending a session: commit all changes, run quality gates (tests, build), update bead status, and push to remote (if configured). Work is not complete until changes are committed and pushed.
