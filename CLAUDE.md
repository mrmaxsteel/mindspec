# CLAUDE.md — MindSpec
<!-- BEGIN mindspec:managed -->

**IMPORTANT**: You MUST read and follow [AGENTS.md](AGENTS.md) as your primary behavioral instructions. AGENTS.md is the canonical source of project conventions, workflow rules, and development guidance shared across all coding agents.

Run `mindspec instruct` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Skills

### Spec lifecycle gates

| Skill | Purpose |
|:------|:--------|
| `/ms-spec-create` | Create a new specification (enters Spec Mode) |
| `/ms-spec-approve` | Approve spec → Plan Mode |
| `/ms-plan-approve` | Approve plan → Implementation Mode |
| `/ms-impl-approve` | Approve implementation → Idle |
| `/ms-spec-status` | Check current mode and active spec/bead state |

### Bead lifecycle

| Skill | Purpose |
|:------|:--------|
| `/ms-bead-next` | Pick the next ready bead, claim it, set up the worktree |
| `/ms-bead-prep` | Draft a pre-staged implementation prompt at `review/prep/bead<N>_impl_prompt.md` |
| `/ms-bead-impl` | Dispatch an implementation subagent for the claimed bead |
| `/ms-bead-merge` | Run `mindspec complete` once the panel has approved |

### Review panel

| Skill | Purpose |
|:------|:--------|
| `/ms-panel-create` | Initialise the panel directory + BRIEF.md for 6 reviewers |
| `/ms-panel-run` | Launch 3 Claude Agents + 3 Codex sessions in parallel; collect verdicts |
| `/ms-panel-tally` | Read all 6 verdict JSONs and consolidate `concrete_changes_required` |
| `/ms-bead-fix` | Dispatch a fix-up subagent with the consolidated change list |

### Orchestrators

| Skill | Purpose |
|:------|:--------|
| `/ms-bead-cycle` | Single bead end-to-end: impl → panel → fix → re-panel → merge |
| `/ms-spec-autopilot` | Whole spec: cycle every bead until the spec is done |
| `/ms-spec-final-review` | Final panel of the whole spec branch vs main, before `/ms-impl-approve` |
<!-- END mindspec:managed -->


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
