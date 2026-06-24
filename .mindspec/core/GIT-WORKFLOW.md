# Git Workflow — Zero-on-Main

MindSpec enforces a zero-changes-on-main invariant: all spec-driven work happens on branches in worktrees, never directly on main.

## Branch Topology

```
main
 └── spec/NNN-slug          (created by spec-init, shared by spec + plan)
      ├── bead/<bead-id>     (created by next, one per impl bead)
      ├── bead/<bead-id>
      └── ...
```

- **main** — protected. No direct commits while mindspec is active.
- **spec/NNN-slug** — long-lived spec branch. Spec authoring, plan authoring, and final integration happen here.
- **bead/<bead-id>** — short-lived bead branches. One per implementation chunk. Merged back to spec branch on `mindspec complete`.

## Lifecycle

1. **`mindspec spec-init 010-feature`** — creates `spec/010-feature` branch + worktree under `.worktrees/`.
2. **Spec & Plan authoring** — happens in the spec worktree on the spec branch.
3. **`mindspec next`** — claims an impl bead, creates `bead/<id>` branch from the spec branch, creates worktree.
4. **Implementation** — happens in the bead worktree.
5. **`mindspec complete`** — merges `bead/<id>` → `spec/NNN-slug`, removes bead worktree.
6. **`/impl-approve`** — merges `spec/NNN-slug` → `main` (via PR or direct merge), cleans up.

## Worktree Location

All worktrees live under `.worktrees/` at the repo root (configurable via `worktree_root` in `.mindspec/config.yaml`). This directory is gitignored.

```
.worktrees/
  worktree-spec-010-feature/     (spec worktree)
  worktree-bead-abc123/          (bead worktree)
```

## Three-Layer Enforcement (ADR-0019)

| Layer | Mechanism | What it blocks |
|-------|-----------|----------------|
| 1 | Git pre-commit hook | Direct commits on protected branches |
| 2 | CLI guards | `mindspec complete` etc. from wrong CWD |
| 3 | Agent PreToolUse hooks | File writes and bash outside worktree |
| Soft | Instruct redirect | Emits "cd to worktree" instead of guidance |

## Escape Hatch

```bash
MINDSPEC_ALLOW_MAIN=1 git commit -m "emergency fix"
```

## Configuration

`.mindspec/config.yaml`:

```yaml
protected_branches:
  - main
  - master

merge_strategy: auto    # auto | pr | direct

worktree_root: .worktrees

enforcement:
  pre_commit_hook: true
  cli_guards: true
  agent_hooks: true
```

## State File

`.mindspec/state.json` is **gitignored** — it is a local runtime cursor, not version-controlled state. It contains machine-local fields (absolute worktree paths) and is derivable from molecules via `mindspec instruct`. See ADR-0015 for the demotion from "primary source of truth" to convenience cursor.

## Related ADRs

- **ADR-0006** — Branching model (zero-on-main, one PR per spec lifecycle)
- **ADR-0019** — Three-layer deterministic enforcement
