# AGENTS.md — MindSpec

MindSpec is a spec-driven development framework. See [USAGE.md](../../.mindspec/core/USAGE.md) for the development workflow, or [Codex quick start](guides/codex.md) for the Codex quick start guide.

## Guidance

Run `mindspec instruct` for mode-appropriate operating guidance.

This step and the `bd` steps are for the top-level session only. A subagent that works from a dispatch brief does not run `mindspec instruct` or `bd`: its brief and the bead context it gets replace them.

## Build & Test

```bash
make build    # Build binary to ./bin/mindspec
make test     # Run all tests
```
