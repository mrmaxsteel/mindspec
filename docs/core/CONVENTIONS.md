# MindSpec Conventions

This document outlines the file organization, naming, and structural conventions for MindSpec-managed projects.

## File Organization

| Path | Purpose |
|:-----|:--------|
| `docs/core/` | Permanent architectural and convention documents |
| `docs/domains/<domain>/` | Domain-scoped documentation (overview, architecture, interfaces, runbook, ADRs) |
| `docs/specs/` | Historical and active specifications |
| `docs/context-map.md` | Bounded context relationships and integration contracts |
| `docs/adr/` | Cross-cutting architecture decision records |
| `architecture/` | Machine-readable policies |
| `GLOSSARY.md` | Concept-to-doc-section mapping for context injection |
| `docs/templates/` | Templates for specs, ADRs, domain docs |
| `AGENTS.md` | Agent behavioral instructions |
| `CLAUDE.md` | Claude Code project instructions |
| `mindspec.md` | Product specification (source of truth) |
| `.mindspec/state.json` | Workflow state: current mode, active spec/bead (ADR-0005) |

## Domain Doc Structure

Each domain lives at `/docs/domains/<domain>/` with:

| File | Purpose |
|:-----|:--------|
| `overview.md` | What the domain owns, its boundaries |
| `architecture.md` | Key patterns, invariants |
| `interfaces.md` | APIs, events, contracts (published language) |
| `runbook.md` | Ops/dev workflows |
| `adr/ADR-xxxx.md` | Domain-scoped architecture decision records |

## Spec Folder Layout

All artifacts for a feature are co-located in a single spec folder:

```
docs/specs/NNN-slug/
  spec.md                  # canonical specification
  plan.md                  # plan (live draft → approved)
  context-pack.md          # generated context pack
  proofs/                  # optional: proof runner outputs
    2026-02-11_1800.txt
```

If multiple plan iterations are needed, use a subfolder:

```
docs/specs/NNN-slug/
  spec.md
  plan/
    plan-v1.md
    plan-v2.md
  context/
    pack-v1.md
```

### `plan.md` Lifecycle

`plan.md` is a **first-class versioned artifact**, created as soon as Plan Mode starts:

1. **Plan Mode starts** — Create `plan.md` with YAML frontmatter:
   ```yaml
   status: Draft
   spec_id: NNN-slug
   version: "0.1"
   last_updated: YYYY-MM-DD
   ```
2. **During Plan Mode** — Iteratively edit `plan.md`. It is always readable on disk.
3. **Approval** — Update frontmatter to record the state change:
   ```yaml
   status: Approved
   approved_at: YYYY-MM-DDTHH:MM:SSZ
   approved_by: <human>
   bead_ids: [beads-xxx, beads-yyy]
   adr_citations: [ADR-NNNN]
   ```
4. **On approval** — Create/confirm implementation beads in Beads and write bead IDs back into `plan.md`.

Frontmatter is the **single source of truth** for plan status. No separate approval section at the bottom.

## Specification Naming

Specs follow the pattern `NNN-slug-name`:
- `001-skeleton`
- `002-glossary`
- `003-context-pack`

## ADR Naming

ADRs follow the pattern `ADR-NNNN.md`:
- Cross-cutting: `docs/adr/ADR-NNNN.md`
- Domain-scoped: `docs/domains/<domain>/adr/ADR-NNNN.md`

ADR metadata must include: domain(s), status (proposed/accepted/superseded), supersedes/superseded-by links, decision + rationale + consequences.

## Beads Conventions

- Spec beads contain a **concise summary** and **link to the canonical spec file**. No long-form content.
- Implementation beads contain: scope, micro-plan (3-7 steps), verification steps, dependencies.
- Keep the active workset intentionally small. Regularly clean up completed beads.
- Rely on git history + documentation for historical traceability, not Beads as archive.

### Bead Title Conventions

Bead titles use bracketed prefixes for idempotent lookup and convention enforcement:

- **Spec beads**: `[SPEC <spec-id>] <title>` — e.g., `[SPEC 006-validate] Workflow Validation`
- **Impl beads**: `[IMPL <spec-id>.<chunk-id>] <chunk-title>` — e.g., `[IMPL 007-beads-tooling.1] bdcli wrapper`

`mindspec bead spec` and `mindspec bead plan` create beads using these conventions. The bracket prefix enables reliable search-based idempotency.

### Gate Title Conventions

Human gates use the `[GATE ...]` prefix for idempotent lookup:

- **Spec approval gate**: `[GATE spec-approve <spec-id>]` — child of the spec bead. Resolved by `mindspec approve spec`.
- **Plan approval gate**: `[GATE plan-approve <spec-id>]` — child of the molecule parent. Resolved by `mindspec approve plan`.

Gates control `bd ready` visibility: implementation beads depend on the plan gate, which depends on the spec gate. Until both gates are resolved, `bd ready` will not show impl beads.

### Structured Descriptions

- **Spec bead descriptions** (≤400 chars): `Summary: <goal>\nSpec: docs/specs/<id>/spec.md\nDomains: <list>`
- **Impl bead descriptions** (≤800 chars): `Scope: <scope>\nVerify:\n- <step>\nPlan: docs/specs/<id>/plan.md`

### Plan `work_chunks` Format

Plans must include a `work_chunks` block in YAML frontmatter for machine-readable decomposition:

```yaml
work_chunks:
  - id: 1
    title: "Short chunk title"
    scope: "internal/pkg/file.go"
    verify:
      - "Specific verification step"
    depends_on: []
  - id: 2
    title: "Second chunk"
    scope: "internal/pkg/other.go"
    verify:
      - "Verification step"
    depends_on: [1]
```

Each chunk has a stable `id` (integer), `title`, `scope`, `verify` (list), and `depends_on` (list of chunk IDs).

### Molecule-Based Plan Decomposition

Plans are decomposed into **Beads molecules**. `mindspec bead plan` creates:

1. A **molecule parent** (epic type) with title `[PLAN <spec-id>] Plan decomposition`
2. **Task children** per work chunk with title `[IMPL <spec-id>.<chunk-id>] <title>`
3. **Dependencies** wired between children based on `depends_on`

The molecule parent ID and per-chunk bead IDs are written back to the plan frontmatter under `generated`:

```yaml
generated:
  mol_parent_id: beads-xxx
  bead_ids:
    "1": beads-abc
    "2": beads-def
```

`mindspec next` queries ready children within the molecule via `bd ready --parent <mol-parent-id>`. `mindspec complete` uses the molecule parent to determine state advancement (next ready child, blocked children, or all done).

### Worktree Lifecycle

Worktrees are managed entirely by Beads (`bd worktree`) — MindSpec orchestrates but does not implement git worktree operations directly.

**Creation**: `mindspec next` creates a worktree automatically when claiming a bead, via `bd worktree create`.

**Removal**: `mindspec complete` removes the worktree after closing the bead, via `bd worktree remove`.

**Naming**: Worktrees are named `worktree-<bead-id>` with branch `bead/<bead-id>`.

**State advancement** after `mindspec complete`:
- If ready children remain in the molecule → mode stays `implement`, next bead is set
- If children exist but are blocked → mode transitions to `plan`
- If all children are complete → mode transitions to `idle`

## Git Workflow Conventions

### Clean Tree Rule

A **clean working tree is a hard precondition** for:

- Starting new work (picking up a bead)
- Switching modes (Spec → Plan → Implement → Done)
- Running `mindspec next`, `mindspec pickup`, or any mode transition

If the tree is dirty: **commit or revert**. Do not auto-stash (it hides state and breaks determinism).

### Milestone Commits

Mode transitions are marked with explicit commits:

| Transition | What to commit |
|:-----------|:---------------|
| **Spec → Plan** | Spec artifact + bead update recording "spec approved" |
| **Plan → Implement** | Plan artifacts, spawned child beads, bead updates |
| **Implement → Done** | Code, tests, docs, bead closure notes |

Normal commits during a mode are expected and encouraged (especially in Implementation Mode — tests first, refactor, docs, etc.). The milestone commit marks the boundary cleanly.

### Commit Message Conventions

Use conventional-commit style scoped to the bead ID:

```
spec(<bead-id>): <summary>
plan(<bead-id>): <summary>
impl(<bead-id>): <summary>
chore(<bead-id>): <summary>
```

- `spec` — spec artifacts and related documentation
- `plan` — plan artifacts, bead creation, dependency mapping
- `impl` — implementation code, tests, doc-sync
- `chore` — cleanup, formatting, dependency bumps, tooling

### Co-committing `.beads/` and `.mindspec/`

Always commit `.beads/` and `.mindspec/state.json` changes alongside the relevant work in the same commit. State writes (`mindspec state set`) must happen **before** the milestone commit so state is co-committed with transition artifacts (ADR-0005).

### Preflight (before starting any forward-progress work)

1. Confirm you are on the correct worktree/branch for the active bead
2. Confirm working tree is clean (`git status` shows no changes). If not: commit with an appropriate message, or revert/discard the changes.
3. Confirm the active bead exists and is in the expected state
4. Only then proceed

## Worktree Conventions

- Worktrees are named with the bead ID: `worktree-<bead-id>`
- One worktree per implementation bead
- Changes are isolated per bead
- Closing a bead requires clean state sync from worktree

## Glossary Conventions

- **Pathing**: Always use **relative paths** from the project root for glossary targets (e.g., `docs/core/ARCHITECTURE.md#section-id`). Do not use absolute paths.
- **Format**: Use the standard table format: `| **Term** | [label](relative/path#anchor) |`.
- **Coverage**: Every new concept introduced in a spec or domain doc should have a glossary entry.

## Documentation Anchors

Use stable Markdown header anchors for deterministic section retrieval:
`## Component X {#component-x}`

## State File Convention {#state-file}

`.mindspec/state.json` is the **primary source of truth** for current mode and active work (ADR-0005).

- **Committed to git** — project-level workflow state, not personal
- **Written via CLI only** — `mindspec state set --mode=X --spec=Y [--bead=Z]`
- **Co-committed with transitions** — state writes happen before milestone commits
- **Cross-validated** — `mindspec instruct` checks state against artifact state and warns on drift

Schema:
```json
{
  "mode": "idle|spec|plan|implement",
  "activeSpec": "004-instruct",
  "activeBead": "beads-xxx",
  "lastUpdated": "2026-02-12T10:00:00Z"
}
```

## Tooling Interface

The primary interface is the Go CLI binary. Key commands:

- `mindspec doctor`: Project structure health check
- `mindspec glossary list|match|show`: Glossary-based context injection
- `mindspec context pack <spec-id>`: Generate context for an agent session
- `mindspec state set|show`: Manage workflow state (ADR-0005)
- `mindspec instruct`: Emit mode-appropriate operating guidance (ADR-0003)
- `mindspec bead spec|plan|worktree|hygiene`: Beads lifecycle tooling (Spec 007)
- `mindspec validate spec|plan|docs`: Check artifact quality (Spec 006)
- `mindspec approve spec|plan <id>`: Validate, update frontmatter, resolve gate, set state, emit instruct (Spec 008b)
- `mindspec next`: Select and claim next ready work (Spec 005)
- `mindspec complete`: Close bead, remove worktree, advance state (Spec 008)

## Instruct-Tail Convention {#instruct-tail}

Every state-changing command emits `mindspec instruct` output as its tail. This ensures the agent always receives fresh, mode-appropriate guidance after a transition:

- **`mindspec approve spec <id>`** — emits plan mode guidance after spec approval
- **`mindspec approve plan <id>`** — emits guidance after plan approval
- **`mindspec next`** — emits implement mode guidance after claiming a bead
- **`mindspec complete`** — emits guidance for the new mode (next bead, plan, or idle)

The session-start hook (`mindspec instruct`) covers cold-start. The instruct-tail covers all subsequent transitions. Together, the agent never lacks context about its operating mode.
