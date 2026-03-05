# MindSpec — Multiple Active Specs

There are multiple specs in progress. Ask the user which one to work on.

## Active Specs

| Spec | Phase |
|------|-------|
{{- range .ActiveSpecList}}
| `{{.SpecID}}` | {{.Mode}} |
{{- end}}

## MindSpec Lifecycle

```
idle ── spec ── plan ── implement ── review ── idle
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

## Next Action

1. Ask the user which spec to work on
2. Run `mindspec instruct --spec <id>` to get mode-appropriate guidance for that spec
