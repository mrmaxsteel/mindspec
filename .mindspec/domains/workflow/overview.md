# Workflow Domain — Overview

## What This Domain Owns

The **workflow** domain owns the spec-driven development lifecycle — the "what" layer that decides which operations should happen:

- **Mode system** — Spec/Plan/Implement/Review mode enforcement and transitions
- **Spec lifecycle** — spec creation, approval gates, status tracking
- **Plan lifecycle** — plan decomposition, bead creation, plan approval gates
- **Beads integration** — adapter layer between MindSpec and the Beads work graph
- **Phase derivation** — determining lifecycle phase from beads epic/child statuses (ADR-0023)
- **Validation gates** — human-in-the-loop approval, ADR compliance checks, doc-sync enforcement

## Boundaries

Workflow does **not** own:
- Git operations, worktree lifecycle, or filesystem operations (execution domain)
- CLI infrastructure or project health checks (core)
- Glossary parsing, context pack assembly, or provenance tracking (context-system)

Workflow **delegates** all git and worktree operations to the `Executor` interface (execution domain). Workflow packages MUST NOT import `internal/gitutil/` directly.

Workflow **uses** context packs (from context-system) to provide mode-appropriate context during planning and implementation.

## Key Packages

| Package | Purpose |
|:--------|:--------|
| `internal/approve/` | Spec, plan, and impl approval enforcement |
| `internal/complete/` | Bead close-out orchestration |
| `internal/next/` | Work selection, claiming, worktree dispatch |
| `internal/spec/` | Spec creation (worktree-first flow) |
| `internal/cleanup/` | Post-lifecycle worktree/branch cleanup |
| `internal/phase/` | Phase derivation from beads (ADR-0023) |
| `internal/resolve/` | Target spec resolution and prefix matching |
| `internal/state/` | Mode definitions, worktree path conventions |
| `internal/bead/` | Beads CLI adapter (bd commands) |
| `internal/validate/` | Spec/plan validation gates |

## Current State

Mode system is implemented. Beads is the single state store (ADR-0023) — no filesystem state files. Phase is derived from epic/child statuses. All git operations go through the Executor interface (Spec 077).

## JSONL as Artifact

`.beads/issues.jsonl` is a **build artifact**, not user authorship (ADR-0025). It is a deterministic projection of Dolt, rewritten by `bd export` and by bd's pre-commit hook after every mutation. Workflow guards must not treat its diff as user work:

- **`mindspec next`** classifies dirty paths. If the only dirty path is `.beads/issues.jsonl`, the guard runs `bd export` from the main repo root to normalize the diff against stale throttled exports, then re-checks. User-authored dirt still blocks — the guard's purpose is to protect user code, not to enforce hygiene on derived files (`internal/next/guard.go`, citing ADR-0025).
- **Executor commits** (`approve spec`, `approve plan`, `approve impl`, `complete`) refresh the JSONL via `bd export` before `git add -A`, so every mindspec-driven commit carries current beads state. In projects without a Dolt remote, this makes `git push` the off-machine durability guarantee.

Adding a future artifact (e.g. `.beads/events.jsonl`) is a one-line change to the classifier's path list; the broader artifact policy (ADR-0025) does not change.
