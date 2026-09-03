# Core Domain — Overview

## What This Domain Owns

The **core** domain owns the foundational infrastructure of MindSpec:

- **CLI entry point** (`mindspec`) and command routing via cobra
- **Project health validation** (`mindspec doctor`) — structure checks, broken-link detection, Beads hygiene
- **Policy framework** — loading and evaluating machine-readable policies from `architecture/policies.yml`
- **Workspace resolution** — finding the project root, locating standard directories

## Boundaries

Core does **not** own:
- Glossary parsing, context pack assembly, or provenance tracking (context-system)
- Mode enforcement logic, spec/plan lifecycle, or Beads/worktree integration (workflow)

Core provides the CLI shell and health infrastructure that other domains plug into.

## Key Files

| File | Purpose |
|:-----|:--------|
| `cmd/mindspec/main.go` | CLI entry point |
| `cmd/mindspec/root.go` | Root command + subcommand registration |
| `cmd/mindspec/doctor.go` | Doctor command wiring |
| `cmd/mindspec/stubs.go` | Stub commands (instruct, next, validate) |
| `internal/workspace/workspace.go` | Project root detection |
| `internal/doctor/` | Health check logic (docs, beads) |
| `architecture/policies.yml` | Machine-checkable policies |

## Current State

Go CLI skeleton implemented (Spec 001). Doctor command validates docs structure and Beads hygiene.

### Doctor Checks (Spec 000)

The `doctor` command validates:
- **Docs structure**: `docs/` directory, `GLOSSARY.md`, domain directories
- **Glossary links**: broken link detection for glossary targets
- **Beads hygiene**: `.beads/` exists, durable state present (`issues.jsonl`, `config.yaml`, `metadata.json`), no runtime artifacts (`bd.sock`, `*.db`, locks) tracked by git
  - Exits non-zero if runtime artifacts are git-tracked

## Maintenance Notes

- **2026-09-03 (spec 128, lifecycle-verb-state-honesty):** `internal/phase`
  gained a repo-scoped, kernel-held write lock (`phaselock.go`,
  `.mindspec/phase.lock`) that now serializes every `mindspec_phase`
  metadata writer across the codebase, a total-order backward-move
  comparator (`order.go`'s `IsBackwardMove`), the one sanctioned internal
  backward-write surface (`sanctioned_sync.go`'s `SanctionedDerivedSync`),
  and a one-shot legacy-to-v2 phase-semantics translation
  (`semantics_v2.go`). `internal/state` gained the `ModeIndeterminate`
  sentinel mode, excluded from `ValidModes`, that the workflow domain's
  instruct tail selects instead of fabricating idle guidance for an
  undiscovered failure. See `architecture.md`'s "The `mindspec_phase`
  write lock, backward-move comparator, and sanctioned sync" section and
  `interfaces.md` for the exported surface; the consuming verbs
  (`repair phase`, `plan approve`, `spec
  approve`, `complete`, `release`, `impl approve`/`adopt`) live in the
  workflow domain.
- **2026-07-02 (spec 107 wave 1):** `internal/phase` gained the exported
  `FetchChildren(epicID)` wrapper — the uncached single-call children query
  (delegating to the package-private `fetchChildren`) that `mindspec complete`
  uses for its fresh post-close read; the memoized `Cache.GetChildren` path is
  unchanged. `EnsureMigratedWithCache` was added so a caller resolving the same
  epic elsewhere shares one `bd list --type=epic` lookup. The dead non-cache
  `FindActiveBeadForEpic` wrapper was removed (`FindActiveBeadForEpicWithCache`
  remains the live path).
