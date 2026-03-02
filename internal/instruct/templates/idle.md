# MindSpec — No Active Work

You are not currently working on any spec or bead.
{{- if .BranchProtection}}

## Branch Protection

**main is protected.** You MUST NOT edit files or commit while on main. All changes require a branch.

## How to Make Changes

**All new features and non-trivial changes MUST go through `mindspec spec-init`.** The only exception is single-file bugfixes or typo corrections — for those, use the direct worktree path below.

### Default: spec-driven (features, multi-file changes, new commands)

Run `mindspec spec-init` — it creates the branch + worktree automatically.

### Exception: trivial fixes only (typos, single-file bugfixes)

1. **FIRST**: `git worktree add .worktrees/fix-<description> -b fix/<description>` then `cd .worktrees/fix-<description>`
2. Make your changes in the worktree
3. `git add <files>` + `git commit -m "<message>"`
4. `git push -u origin <branch-name>`
5. `gh pr create` — open a pull request

Work is NOT complete until the PR is created. Always finish all 5 steps.
{{- end}}

## Available Actions

- `mindspec explore "idea"` — evaluate whether an idea is worth pursuing
- `mindspec spec-init` — start a new specification (creates branch + worktree)
- `mindspec state set --mode=spec --spec=<id>` — resume work on an existing spec
- `mindspec doctor` — check project health

## Available Specs

{{- if .AvailableSpecs}}
{{range .AvailableSpecs}}
- `{{.}}`
{{- end}}
{{- else}}
No specs found in `.mindspec/docs/specs/`.
{{- end}}

## Next Action

If the user already gave a concrete task, execute it immediately.
- Do NOT greet or ask what they'd like to work on first.
- Do NOT claim success unless commands actually ran and exited 0.

If the user did NOT give a concrete task, do this in your first message:

1. Greet the user
2. Suggest these options directly:
   - `mindspec explore "idea"` to explore whether an idea is worth pursuing
   - `mindspec spec-init` to draft a new specification (if they already know what to build)
   - Resuming an existing spec (if any are listed above)
   - `mindspec doctor` to check project health
3. Ask what they'd like to work on
