# Core Domain — Architecture

## Key Patterns

### Workspace Resolution

The `Workspace` package finds the project root by walking up from the current directory looking for `.mindspec/` or `.git`. All path resolution is relative to this root.

#### Per-artifact three-tier resolvers (spec 106)

Each docs accessor (`SpecDir`, `ADRDir`, `DomainDir`, `ContextMapPath`,
`CoreDir`, `RecordingDir`) resolves its artifact independently with a
three-tier, **flat-first** read precedence, first-exists-wins:

1. **flat** — `.mindspec/<artifact>` (e.g. `.mindspec/adr`, `.mindspec/specs/<id>`, `.mindspec/context-map.md`)
2. **canonical** — `.mindspec/docs/<artifact>`
3. **legacy** — root `docs/<artifact>`

"Flat FIRST" is read precedence, not delivery order. When no flat tier exists
on disk the resolvers fall back to the historical `DocsDir` canonical-or-legacy
join, so a canonical, legacy, or greenfield tree with no flat tree present
resolves byte-for-byte as before. The single `DocsDir` join-point no longer
funnels the per-artifact accessors — each owns its flat tier (so they can be
flattened independently). `SpecDir` additionally probes both the flat and the
canonical worktree shapes, and `TreeRootForSpecDir` recognizes the flat spec
shape (`<tree>/.mindspec/specs/<id>`) so the cross-worktree ADR-visibility fix
(mindspec-ew79) survives a flattened worktree.

#### Whole-tree layout classification (`DetectLayout`)

`DetectLayout(root) → {flat | canonical | legacy | greenfield | mixed}`
classifies the whole tree. A flat lifecycle tree coexisting with any
canonical/legacy tree is **mixed** — a hard error (`ErrMixedLayout`) except
inside a recorded `.mindspec/migrations/<run-id>/` recovery. The
classification drives the write-default: a bootstrapped flat tree is born flat;
existing canonical/legacy projects keep writing their existing form. New
(greenfield) projects are bootstrapped born-flat (`.mindspec/{specs,domains}`).

The pure, I/O-free classifier `ClassifyLayout(LayoutMarkers)` (with
`LayoutMarkersFromMindspecChildren`, fed from a git tree listing) is the single
source of truth that both `DetectLayout` (filesystem) and the cross-layout
merge guard (git refs) reuse, so the two fingerprints never drift.

`MigrationRecoveryActive(root)` exposes the SAME in-flight-run-id scoping the
`DetectLayout` mixed-tree exception uses — a recorded, non-terminal
`.mindspec/migrations/<run-id>/` run — for cross-package reuse: the execution
domain's directional merge guard (Spec 106) calls it to EXEMPT a transient
cross-layout merge during a live migration recovery, rather than reimplementing
the run-state read. A stale/completed run record never activates it.

### Health Checks

`mindspec doctor` validates project structure. Checks are categorized:

- **Errors**: Missing critical files (e.g., `GLOSSARY.md`, `.mindspec/core/`)
- **Warnings**: Missing optional structure (e.g., `.mindspec/domains/`, `.mindspec/context-map.md`)

The distinction allows fresh projects to pass basic checks while still surfacing incomplete scaffolding.

### Policy Framework

Policies in `architecture/policies.yml` are declarative rules with:
- `id`, `description`, `severity` (error/warning)
- Optional `scope` (file glob) and `mode` (spec/plan/implementation)
- `reference` pointing to the authoritative doc section

## Invariants

1. Workspace resolution must be deterministic — same directory always resolves to same root.
2. Health checks must never hard-fail on optional structure in a fresh project.
3. Policy evaluation is read-only — policies describe constraints, they don't enforce them at runtime (yet).

## Phase detail derivation and guard context (spec 092)

`internal/phase` exposes the stored-vs-derived phase split behind the
spec-092 gate hardening:

- `PhaseDetail{EpicID, Stored, Derived}` — the metadata-cached
  `mindspec_phase` alongside the child-derived ground truth
  (ADR-0023 §3/§5, ADR-0034).
- `DerivePhaseDetail(epicID)` / `DerivePhaseDetailWithCache(c, epicID)`
  — read-only derivation. Callers (`mindspec impl approve`,
  `mindspec repair phase`) decide whether to reconcile the cache
  forward; derivation itself never writes.

`internal/workspace.ContextLine(dir, checkedPath)` renders the
fixed-format worktree-context line that guard failures emit
immediately before their final `recovery:` line (spec 092 Req 8).

## Dead-code sweep — spec 107 wave 1 (2026-07-02)

Bead `mindspec-oexu.1` removed a confirmed-dead core-domain symbol:

- `internal/recording/codex_bootstrap.go`: `DefaultCodexConfigPath`
  (no live callers).
