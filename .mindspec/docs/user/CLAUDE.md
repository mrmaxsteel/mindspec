# CLAUDE.md — MindSpec

MindSpec is a spec-driven development framework (Claude Code-first). See [USAGE.md](docs/core/USAGE.md) for the development workflow, or [docs/guides/claude-code.md](docs/guides/claude-code.md) for the quick start guide.

## Guidance

Run `mindspec instruct` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Build & Test

```bash
make build    # Build binary to ./bin/mindspec
make test     # Run all tests
```

## Custom Commands

| Command | Purpose |
|:--------|:--------|
| `/spec-init` | Initialize a new specification (enters Spec Mode) |
| `/spec-approve` | Approve spec → Plan Mode |
| `/plan-approve` | Approve plan → Implementation Mode |
| `/impl-approve` | Approve implementation → Idle |
| `/spec-status` | Check current mode and active spec/bead state |
