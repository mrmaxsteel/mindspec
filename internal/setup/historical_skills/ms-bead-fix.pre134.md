---
name: ms-bead-fix
description: Dispatch one fixer with the consolidated panel findings and fixer rules
---

# Fix Subagent Dispatch

## Inputs

Use the bead id, target worktree, panel slug, round and consolidated findings path.
For final review, use the spec worktree as the target worktree.

## Steps

1. Run `mindspec panel verify <panel-slug>`. Proceed only on exit 0 (`panel.fix-after-final`).
2. Run `mindspec panel checkout <panel-slug> --fixer`. Capture the absolute fixer directory.
3. Run `mindspec rules --scope fixer`. Capture its complete stdout as `fixer_rules`.
4. Compose the fixer prompt with this structure. Insert the captured output, not the command name, in the rules block.

```text
Target worktree: <absolute target worktree>
Scratch directory: <absolute fixer directory>
Bead, branch and current HEAD: <identity>
Files in scope and shared helpers: <plan paths and imports>
Consolidated findings: <grouped asks and evidence>
Fixer rules (verbatim stdout):
<fixer_rules>
Repository guardrails: read AGENTS.md, Bead-loop guardrails.
Return one commit SHA, test results and deviations.
If an item cannot be fixed in scope, report its reason under Deviations:.
```

5. Dispatch one agent in the target's own worktree (`panel.one-fixer`). Never dispatch the fixer in a slot tree.
Apply `panel.scratch-in-slot`: use the absolute fixer directory, never a relative path.
Apply `panel.artifact-gate`: keep a missing artifact finding open until the artifact exists.
Apply `review.derive` and `lifecycle.complete-by-orchestrator` from the captured rules.
6. Capture the SHA, test results and deviations for the next BRIEF.

## Commit

Make one follow-up commit on the target branch. Keep the original implementation commit.
End the message with `Deviations: <list or none>`.
For final-review fixes on the spec branch, use the commit escape hatch in `/ms-spec-final-review`.

## Then

Return to `/ms-bead-cycle` for the next `/ms-panel-run` round.
