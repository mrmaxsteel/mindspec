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

## `LifecycleChildIDsForEpic` — the FinalizeEpic scoping allow-set source (spec 119)

`internal/phase.LifecycleChildIDsForEpic(epicID)` resolves an epic's
LIFECYCLE (task / empty-type) children via the shared cache — the same
`bd list --parent` query `OpenNonLifecycleChildrenForEpic` already issues
— and returns their bead IDs. Unlike its advisory sibling
(`OpenNonLifecycleChildren(ForEpic)`, which swallows a query failure to an
empty hint because nothing downstream of it ever blocks), this function is
FAIL-CLOSED: a bd query failure returns an error rather than silently
reading as "no lifecycle children". It feeds
`internal/approve.ApproveImpl`'s `FinalizeEpic` lifecycle-allow-set
resolution (execution domain, spec 119 R6) — under-scoping that allow-set
by misreading a query failure as "everything is out of scope" would
either strand real lifecycle beads unmerged or (worse) merge nothing
silently, so this call fails the whole `impl approve` invocation
pre-mutation instead. bd-only, no git I/O (ADR-0030 boundary unaffected).

## ADR resolution: write-target vs read-resolution (spec 123 R5)

Spec 123's slugged-ADR-filename convention (workflow domain: `adr create`
now emits `ADR-NNNN-<slug>.md`) split the core ADR path surface into two
deliberately distinct resolvers in `internal/workspace/workspace.go`:

- **`ADRFilePath(root, id)`** stays the exact-join WRITE-target resolver:
  it composes the path a NEW file should be written to (and the path
  `CreateWithID`'s own existence probe checks), and never resolves an
  existing on-disk file that may be slugged.
- **`ResolveADRFile(root, id)`** is the new shared READ-resolution
  helper every caller that must find an EXISTING, possibly-slugged file
  uses (`show`, `--supersedes`, `Supersede`, `CopyDomains`). It accepts a
  canonical `ADR-NNNN` or a full slugged stem, validates via
  `idvalidate.ADRID` BEFORE any `filepath.Glob`/`Join` (no unvalidated id
  reaches the filesystem), derives the canonical number prefix
  (`idvalidate.ADRCanonicalPrefix`), and enumerates every file carrying
  that number: the bare `<canonical>.md` plus every `<canonical>-*.md`
  glob match. Exactly one candidate resolves; zero is "not found"; MORE
  than one is a COLLISION error naming both files, ending in a
  `recovery:`-prefixed prose diagnostic (rename or remove the redundant
  file so exactly one carries the number — guidance, not the
  copy-pastable `recovery: <command>` form ADR-0035 defines for guard
  failures) — never a silent short-circuit to the bare file. Resolution is canonical-number driven
  for EVERY input shape, so naming the full slugged stem while a genuine
  collision exists still surfaces the ambiguity. On-disk filenames
  rendered into the collision error are routed through `termsafe.Escape`
  (spec 116/ADR-0042: attacker-influenceable names never reach the
  terminal raw).

Canonical `ADR-NNNN` remains the reference currency everywhere — slugs
are filename ergonomics only; ADR-citation gates and `--supersedes`
chains match on the canonical ID.

## Declared `commands:` build/test guidance (spec 123 R7, ADR-0040 consumer-identity)

`Config` gains `Commands map[string]string` (`yaml:"commands"`) beside
`Models`: a free-form task→shell-command map with the documented (not
enforced) vocabulary keys `build` and `test`. UNLIKE `Models`/`Loop`/
`Runner`, this key is NOT inert: `mindspec init` and every `mindspec
setup <agent>` verb render its populated entries as the managed AGENTS.md
"Build & Test" section, so mindspec never plants its OWN build commands
into a consuming repo. The rendering surface is single-homed in config:

- **`hasNonBlankEntry` / `HasDeclaredModels()` / `HasDeclaredCommands()`**
  — the empty≠declared predicate: an entry counts only when key AND value
  are non-blank after trimming, so `commands:\n  build: ""` does NOT
  count as declared (doctor's `missing-commands`/`missing-models` Warns
  still fire, and no runnable-command-less section is ever rendered).
- **`CommandLines()`** — renders `"<command>   # <task>"` lines in the
  stable order build, test, then remaining keys sorted (the single
  ordering rule, so bootstrap's starter AGENTS.md and setup's managed
  block can never disagree); blank entries are skipped; every value is
  routed through `termsafe.Escape` (operator/agent-declared free text
  reaching a generated document).
- **`RenderBuildTestSection(level)`** — the ONE renderer every
  managed-content call site uses (level 2 for top-level managed blocks,
  3 for bootstrap's nested append block); returns `""` when `Commands`
  is unset, so the section is OMITTED entirely rather than rendered as a
  placeholder.

The framework never guesses these values (ADR-0036 ZFC): unset means
omitted plus the workflow-domain doctor nudge and the `mindspec commands
populate` prompt emitter — never an inferred `npm test`.
