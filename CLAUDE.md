# CLAUDE.md — MindSpec
<!-- BEGIN mindspec:managed -->

MindSpec is a spec-driven development framework (Claude Code-first). See [USAGE.md](.mindspec/docs/core/USAGE.md) for the development workflow, or [.mindspec/docs/guides/claude-code.md](.mindspec/docs/guides/claude-code.md) for the quick start guide.

## Guidance

Run `mindspec instruct` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Build & Test

```bash
make build    # Build binary to ./bin/mindspec
make test     # Run all tests
```

## Lifecycle Commands

All git operations are handled internally by mindspec — no raw git needed for the normal workflow.

```bash
mindspec spec create <slug>      # Create spec (idle → spec)
mindspec spec approve <id>       # Approve spec (spec → plan)
mindspec plan approve <id>       # Approve plan (plan → implement)
mindspec next                    # Claim bead, create worktree
mindspec complete "message"      # Auto-commit, close bead, merge, cleanup
mindspec impl approve <id>       # Approve impl (review → idle)
```

## Skills

| Skill | Purpose |
|:------|:--------|
| `/ms-spec-create` | Create a new specification (enters Spec Mode) |
| `/ms-spec-approve` | Approve spec → Plan Mode |
| `/ms-plan-approve` | Approve plan → Implementation Mode |
| `/ms-impl-approve` | Approve implementation → Idle |
| `/ms-spec-status` | Check current mode and active spec/bead state |
| `/llm-test` | Enter the iterative LLM test harness loop (test/observe/fix/retest) |
<!-- END mindspec:managed -->
