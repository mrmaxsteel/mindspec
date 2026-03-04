# MindSpec — Review Mode

**Active Spec**: `{{.ActiveSpec}}`
{{- if .SpecGoal}}
**Goal**: {{.SpecGoal}}
{{- end}}

## MindSpec Lifecycle

```
idle ── spec ── plan ── implement ──── >>> review ── idle
```

| Phase | Command | What happens |
|-------|---------|--------------|
| idle → spec | `mindspec spec create <slug>` | Creates branch + worktree + spec template |
| spec → plan | `mindspec spec approve <id>` | Validates spec, auto-commits |
| plan → impl | `mindspec plan approve <id>` | Validates plan, auto-creates beads, auto-commits |
| per bead | `mindspec next` | Claims bead, creates bead worktree |
| bead done | `mindspec complete "msg"` | Auto-commits, closes bead, merges bead→spec, removes worktree |
| review → idle | `mindspec impl approve <id>` | Merges spec→main, removes all worktrees + branches |

### Git rules
- You should not need any raw git commands — all git operations are handled by mindspec
- Raw git is available for repair/recovery but the happy path never requires it

## Objective

All implementation beads are complete. Present the work for human review before closing out.

## Permitted Actions

- Reading code, tests, and documentation to verify completeness
- Running tests and quality gates
- Minor fixes discovered during review (typos, formatting)
- Updating documentation if gaps are found

## Forbidden Actions

- New feature work (create a new spec instead)
- Significant refactoring beyond the spec's scope
- Closing out without human approval
- Manually closing the lifecycle epic via `bd close` — `mindspec impl approve` handles this automatically

## Review Checklist

1. **Acceptance criteria**: Read the spec at `.mindspec/docs/specs/{{.ActiveSpec}}/spec.md` and verify each acceptance criterion is met
2. **Tests**: Run `make test` and confirm all tests pass
3. **Build**: Run `make build` and confirm clean build
4. **Doc-sync**: Verify documentation matches the implementation
5. **Summary**: Present a brief summary of what was built and how each acceptance criterion was satisfied

## Human Gate

- **Implementation approval**: Run `mindspec impl approve <id>` when the human accepts the implementation

## Next Action

Read the spec's acceptance criteria, verify each one, and present the review summary to the human. When they approve, run `mindspec impl approve {{.ActiveSpec}}`.
