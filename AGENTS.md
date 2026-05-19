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

### Beads: The Substrate

[Beads](https://github.com/steveyegge/beads) is the interface between the two layers. Each bead is a self-contained work packet — requirements, context, dependencies, acceptance criteria — that a fresh agent can pick up without session history. The planning layer writes beads; the execution engine reads them. This is what makes the `Executor` interface pluggable: any orchestrator that can read a bead can dispatch work.

### Execution Engine (the "how")

The execution engine reads beads and implements them — it never decides *what* should happen:

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
