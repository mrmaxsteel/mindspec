# CLAUDE.md — MindSpec

MindSpec is a spec-driven development framework (Claude Code-first). See [USAGE.md](.mindspec/docs/core/USAGE.md) for the development workflow, or [.mindspec/docs/guides/claude-code.md](.mindspec/docs/guides/claude-code.md) for the quick start guide.

## Guidance

Run `mindspec instruct` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Build & Test

```bash
make build    # Build binary to ./bin/mindspec
make test     # Run all tests
```

## Skills

| Skill | Purpose |
|:------|:--------|
| `/ms-explore` | Enter, promote, or dismiss an Explore Mode session |
| `/ms-spec-create` | Create a new specification (enters Spec Mode) |
| `/ms-spec-approve` | Approve spec → Plan Mode |
| `/ms-plan-approve` | Approve plan → Implementation Mode |
| `/ms-impl-approve` | Approve implementation → Idle |
| `/ms-spec-status` | Check current mode and active spec/bead state |
