# MindSpec Workflow State Machine

This is the canonical reference for MindSpec's lifecycle: what states exist, what each command does, and what artifacts it creates or destroys.

## Happy Path

```
idle ──── spec ──── plan ──── implement ──── review ──── idle
  │         │         │          │    ↺        │
  │         │         │          │  (per bead) │
  ▼         ▼         ▼          ▼             ▼
spec      spec      plan       next          impl
create    approve   approve    + complete    approve
```

```bash
mindspec spec create 123-my-spec         # idle → spec (creates branch + worktree)
# write spec.md in the spec worktree
mindspec spec approve 123-my-spec        # spec → plan (validates, auto-commits)
# write plan.md in the spec worktree
mindspec plan approve 123-my-spec        # plan → implement (validates, creates beads)
mindspec next                            # claims bead, creates bead worktree
cd <bead-worktree>                       # switch to bead worktree
# write code
mindspec complete "what I did"           # auto-commits, closes bead, merges bead→spec
# repeat next/complete for each bead
mindspec impl approve 123-my-spec        # review → idle (merges spec→main, cleans everything)
```

No raw git commands are needed. All git operations (commit, branch, merge, worktree create/remove) happen inside mindspec commands.

---

## State Layers

MindSpec tracks three things:

| Layer | File | Purpose |
|:------|:-----|:--------|
| **Lifecycle phase** | `docs/specs/<id>/lifecycle.yaml` | Per-spec phase: `spec → plan → implement → review → done` |
| **Focus cursor** | `.mindspec/focus` (per-worktree) | Current working context: mode, activeSpec, activeBead, activeWorktree, specBranch |
| **Work graph** | Beads epic + child beads | Execution status of each implementation unit |

`idle` is a focus mode only (no lifecycle phase). `done` is a lifecycle phase only (no focus mode — focus returns to `idle`).

---

## What Each Command Does

### `mindspec spec create <slug>`

**Transition**: idle → spec

| Category | What happens |
|:---------|:-------------|
| **Git** | Creates branch `spec/<slug>` from HEAD; creates worktree `.worktrees/worktree-spec-<slug>`; auto-commits initial files |
| **Files created** | `docs/specs/<slug>/spec.md` (template), `docs/specs/<slug>/lifecycle.yaml` |
| **Beads** | Creates lifecycle epic: `[SPEC <slug>] <title>` |
| **Focus** | `mode=spec`, `activeSpec=<slug>`, `specBranch=spec/<slug>`, `activeWorktree=<path>` |
| **Lifecycle** | `phase: spec` |
| **CWD after** | Spec worktree (`.worktrees/worktree-spec-<slug>`) |

### `mindspec spec approve <id>`

**Transition**: spec → plan

| Category | What happens |
|:---------|:-------------|
| **Guard** | `validate spec` must pass |
| **Git** | Auto-commits spec.md + lifecycle.yaml changes to spec branch |
| **Files modified** | `spec.md` frontmatter: `status: Approved`, `approved_at`, `approved_by` |
| **Beads** | None |
| **Focus** | `mode=plan` |
| **Lifecycle** | `phase: plan` |
| **CWD after** | Spec worktree (unchanged) |

### `mindspec plan approve <id>`

**Transition**: plan → implement (lifecycle), focus stays `plan` until `next`

| Category | What happens |
|:---------|:-------------|
| **Guard** | `validate plan` must pass |
| **Git** | Auto-commits plan.md changes to spec branch |
| **Files modified** | `plan.md` frontmatter: `status: Approved`, `bead_ids: [...]` |
| **Beads** | Creates one task bead per `## Bead N` section in plan.md, parented to the spec epic; wires dependencies from `Depends on` fields |
| **Focus** | `mode=plan` (unchanged — `next` advances to `implement`) |
| **Lifecycle** | `phase: implement` |
| **CWD after** | Spec worktree (unchanged) |

### `mindspec next`

**Transition**: plan/implement → implement (claims a bead, creates its worktree)

| Category | What happens |
|:---------|:-------------|
| **Guard** | Must run from **spec worktree** (hard error from main or bead worktree); clean tree required; session freshness gate |
| **Git** | Creates branch `bead/<beadID>` from spec branch; creates worktree `.worktrees/worktree-<beadID>` under the spec worktree |
| **Files created** | Bead worktree directory with `.mindspec/focus` |
| **Beads** | Queries `bd ready` for the spec's epic; claims the selected bead (`status=in_progress`) |
| **Focus** | `mode=implement`, `activeBead=<beadID>`, `activeWorktree=<bead-wt-path>` |
| **Lifecycle** | Unchanged (already `implement`) |
| **CWD after** | Agent must `cd` into the bead worktree to work |

### `mindspec complete "message"`

**Transition**: implement → implement (more beads) / plan (blocked beads) / review (all done)

| Category | What happens |
|:---------|:-------------|
| **Guard** | Must run from **bead worktree** (hard error from main or spec worktree; auto-redirects from focus if possible); if no message and dirty tree → error with hint |
| **Git** | If message provided: `git add -A && git commit` with `impl(<beadID>): <message>`; merges `bead/<beadID>` → `spec/<specID>`; removes bead worktree; deletes `bead/<beadID>` branch |
| **Files removed** | Bead worktree directory |
| **Beads** | Closes the active bead; queries remaining work to determine next state |
| **Focus** | Next mode based on remaining beads: `implement` (ready beads exist), `plan` (only blocked beads), `review` (all closed) |
| **Lifecycle** | Unchanged |
| **CWD after** | Returns to spec worktree |

**Next-state logic after closing a bead:**

| Remaining beads | Next mode | What to do |
|:----------------|:----------|:-----------|
| Ready beads exist | `implement` | Run `mindspec next` to claim the next one |
| Only blocked beads | `plan` | Resolve blockers or adjust the plan |
| All beads closed | `review` | Run `mindspec impl approve` |

### `mindspec impl approve <id>`

**Transition**: review → done (lifecycle), focus → idle

| Category | What happens |
|:---------|:-------------|
| **Guard** | Focus must be `mode=review` with `activeSpec` matching the target |
| **Git (no remote)** | Merges `spec/<id>` → `main`; deletes all `bead/*` branches; removes all bead worktrees; removes spec worktree; deletes `spec/<id>` branch |
| **Git (with remote)** | Pushes spec branch; creates PR via `gh`; optionally waits for CI + merges (`--wait`); then same cleanup |
| **Beads** | Closes the lifecycle epic |
| **Focus** | `mode=idle`, all fields cleared |
| **Lifecycle** | `phase: done` |
| **CWD after** | Main repo root |

---

## Worktree Topology

```
repo/                                    # main checkout (idle)
├── .worktrees/
│   └── worktree-spec-123-my-spec/       # spec worktree (spec/plan/review)
│       ├── .worktrees/
│       │   └── worktree-beads-xxx.1/    # bead worktree (implement)
│       ├── docs/specs/123-my-spec/
│       │   ├── spec.md
│       │   ├── plan.md
│       │   └── lifecycle.yaml
│       └── .mindspec/focus
└── .mindspec/focus
```

Each worktree has its own `.mindspec/focus` file. Bead worktrees nest under spec worktrees.

**Worktree scoping rules (hard errors, no bypass):**

| Command | Must run from | Error if run from |
|:--------|:-------------|:------------------|
| `mindspec next` | Spec worktree | Main or bead worktree |
| `mindspec complete` | Bead worktree | Main or spec worktree |

`complete` will auto-redirect to the active bead worktree (from focus) if not already in one, then re-check.

---

## Git Branch Topology

```
main
├── spec/123-my-spec          # created by spec create, merged to main by impl approve
│   ├── bead/beads-xxx.1      # created by next, merged to spec by complete
│   ├── bead/beads-xxx.2
│   └── bead/beads-xxx.3
```

All merges flow upward: bead → spec → main. The agent never runs raw git merge/commit/branch commands.

---

## Transition Matrix

| From | Allowed next states | Trigger |
|:-----|:-------------------|:--------|
| `idle` | `spec` | `spec create` |
| `spec` | `plan` | `spec approve` |
| `plan` | `implement` | `plan approve` + `next` |
| `implement` | `implement`, `plan`, `review` | `complete` (depends on remaining beads) |
| `review` | `done` → `idle` | `impl approve` |

Transitions not in this table are disallowed. You cannot skip phases (e.g., spec → implement) or go backward (e.g., review → implement on the same spec).

---

## Git Policy

The happy path requires zero raw git commands. Every git operation is internal to a mindspec command:

| Git operation | Handled by |
|:-------------|:-----------|
| Branch creation | `spec create`, `next` |
| Worktree creation | `spec create`, `next` |
| Commit | `spec create`, `spec approve`, `plan approve`, `complete` |
| Merge (bead → spec) | `complete` |
| Merge (spec → main) | `impl approve` |
| Branch deletion | `complete`, `impl approve` |
| Worktree removal | `complete`, `impl approve` |

Raw git is not blocked — it remains available for repair and recovery. But the normal workflow never needs it.

---

## Recovery

`mindspec state set --mode=...` can force focus to arbitrary values. This is a recovery tool, not a normal workflow mechanism. Use only to repair stale state after interruption.

---

## Related Docs

- [MODES.md](MODES.md)
- [USAGE.md](USAGE.md)
- [CONVENTIONS.md](CONVENTIONS.md)
- [GIT-WORKFLOW.md](GIT-WORKFLOW.md)
- [ADR-0006](../adr/ADR-0006.md) — worktree-first spec creation
- [ADR-0022](../adr/ADR-0022.md) — worktree-aware path resolution
