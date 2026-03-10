# AGENTS.md — MindSpec

MindSpec is a spec-driven development framework. See [USAGE.md](.mindspec/docs/core/USAGE.md) for the development workflow, or [.mindspec/docs/guides/codex.md](.mindspec/docs/guides/codex.md) for the Codex quick start guide.

## Guidance

Run `mindspec instruct` for mode-appropriate operating guidance.

## Build & Test

```bash
make build    # Build binary to ./bin/mindspec
make test     # Run all tests
```

<!-- mindspec:managed -->

## MindSpec

This project uses [MindSpec](https://github.com/mindspec/mindspec), a spec-driven development framework.

Run `mindspec instruct` for mode-appropriate operating guidance.

### Build & Test

```bash
make build    # Build binary
make test     # Run all tests
```

### Modes

This project follows a strict spec-driven workflow with human gates:

1. **Explore** — evaluate whether an idea is worth pursuing
2. **Spec** — define the problem and acceptance criteria (no code)
3. **Plan** — break the spec into implementation beads (no code)
4. **Implement** — write code against the approved plan
5. **Review** — verify implementation meets acceptance criteria

Transition between modes using `mindspec approve spec|plan` and `mindspec complete`.

### Conventions

- Every functional change must reference a spec in `.mindspec/docs/specs/`
- In Spec and Plan modes, only documentation may be created or modified — no code changes
- Working tree must be clean before switching modes
- Run `mindspec doctor` to verify project structure health


<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Auto-Sync

bd automatically syncs via Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/QUICKSTART.md.

<!-- END BEADS INTEGRATION -->

## Architecture: Workflow/Execution Boundary

MindSpec has a two-layer architecture separating *what* from *how*:

### Workflow Layer (the "what")

The workflow layer owns the spec-driven development lifecycle — deciding which operations should happen and enforcing quality at every gate:

- **Spec creation** — `internal/spec/` creates spec branches, worktrees, and template files
- **Plan decomposition** — breaks specs into bitesize beads with clear acceptance criteria. Well-decomposed plans are critical for AI agent success (see [arXiv:2512.08296](https://arxiv.org/abs/2512.08296) on task decomposition quality)
- **Validation** — `internal/validate/` checks ADR compliance, doc-sync, and structural requirements
- **Quality gates** — `internal/approve/` enforces human-in-the-loop approval at spec, plan, and impl transitions
- **Phase enforcement** — `internal/phase/` derives lifecycle phase from beads epic/child statuses (ADR-0023)
- **Work selection** — `internal/next/` selects ready beads, `internal/complete/` orchestrates bead close-out
- **Cleanup** — `internal/cleanup/` handles post-lifecycle worktree/branch removal

Key packages: `internal/approve/`, `internal/complete/`, `internal/next/`, `internal/spec/`, `internal/cleanup/`, `internal/phase/`, `internal/validate/`, `internal/bead/`

### Execution Engine (the "how")

The execution engine implements operations delegated by the workflow layer — it never decides *what* should happen:

- **`MindspecExecutor`** (`internal/executor/`) — dispatches beads to worktrees, merges completed bead branches, finalizes specs via PR or direct merge
- **`MockExecutor`** (`internal/executor/`) — test double for enforcement testing without git side effects
- **`internal/gitutil/`** — low-level git helpers (branch, merge, PR, diffstat) used only by `MindspecExecutor`

DI wiring: `cmd/mindspec/root.go` has `newExecutor(root)` factory.

### Import Rule

Workflow packages call `executor.Executor` methods. They MUST NOT import `internal/gitutil/` directly. This keeps enforcement logic testable with `MockExecutor` and decouples workflow decisions from git mechanics.

See `.mindspec/docs/domains/execution/` and `.mindspec/docs/domains/workflow/` for full documentation.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
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
