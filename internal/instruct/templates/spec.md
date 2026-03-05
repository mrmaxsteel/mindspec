# MindSpec — Spec Mode

**Active Spec**: `{{.ActiveSpec}}`
{{- if .SpecGoal}}
**Goal**: {{.SpecGoal}}
{{- end}}

## MindSpec Lifecycle

```
idle ──── >>> spec ── plan ── implement ── review ── idle
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

Discuss user-facing value and define what "done" means. Spec Mode is intentionally implementation-light.

## Permitted Actions

- Create/edit `.mindspec/docs/specs/{{.ActiveSpec}}/spec.md`
- Create/edit domain docs (`.mindspec/docs/domains/`)
- Add glossary entries (`GLOSSARY.md`)
- Edit architecture docs (`.mindspec/docs/core/`)
- Draft ADRs (`.mindspec/docs/adr/`)

## Forbidden Actions

- Creating or modifying code (`cmd/`, `internal/`, or equivalent)
- Creating or modifying test code
- Changing build/config that affects runtime behavior

## Required Output

A spec containing:
- Problem statement and target user outcome
- Acceptance criteria (specific, measurable)
- Impacted domains and ADR touchpoints
- Non-goals / constraints
- All open questions resolved

## Human Gates

- **Spec approval**: You MUST run `mindspec spec approve {{.ActiveSpec}}` before starting any plan work. This gate resolves the spec-approve step in the lifecycle molecule. Skipping it causes mode resolution to remain stuck in spec mode.

## Next Action

Complete the spec at `.mindspec/docs/specs/{{.ActiveSpec}}/spec.md`, then run `mindspec spec approve {{.ActiveSpec}}`. Do NOT create `plan.md` or begin planning until this command succeeds.
