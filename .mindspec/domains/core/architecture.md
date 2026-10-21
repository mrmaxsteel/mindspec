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

## The `mindspec_phase` write lock, backward-move comparator, and sanctioned sync (spec 128 R3.3, Bead 1)

Every writer of an epic's `mindspec_phase` metadata key — `spec approve`'s
existing-epic backfill (`internal/approve/spec.go`'s
`backfillExistingEpicPhase`), `plan approve` (`plan.go`'s step-4b
`WithPhaseWriteLock` write), `next`'s claim-time advance
(`cmd/mindspec/next.go`'s `advancePhaseOnClaim` — the bead-2 writer this
spec itself added, classified forward-by-intent in lifecycle-modes.md's
writer taxonomy), `complete`'s step-6.5 sync (`internal/complete/complete.go`),
`release`'s step-6.5 sync (`cmd/mindspec/release.go`), `impl approve`'s
reconcile and done writes (`internal/approve/impl.go`), `impl adopt`'s done
write (`internal/approve/adopt.go`), `mindspec repair phase`
(`cmd/mindspec/repair.go`), `phase.EnsureMigrated`'s ADR-0034 backfill
(`migrate.go`), and the one-shot v2 phase-semantics translation
(`semantics_v2.go`'s `EnsurePhaseSemanticsV2`) — now executes inside
`phase.WithPhaseWriteLock(root, epicID,
mergeMetadata, fn)`: a repo-scoped, KERNEL-HELD lock at `.mindspec/phase.lock`
(`PhaseLockRelPath`) that reads the stored phase, hands it to `fn`, performs
`fn`'s returned conditional `MergeMetadata` write, and releases — so a
concurrent writer can never land a stale value over a fresher one (AC-5(v)).
Kernel-held on both platforms means a crashed holder's lock releases with its
process: `flock(2) LOCK_EX` on unix (`phaselock_unix.go`), and on Windows
(`phaselock_windows.go`) `LockFileEx` (an exclusive byte-range lock) on a
RETAINED handle opened WITHOUT `FILE_SHARE_DELETE`, held for the whole
critical section — the FILE is never the lock, so an empty or partial
leftover lockfile never blocks acquisition. Acquisition is a bounded,
poll-with-deadline wait (10s default); on timeout it returns
`ErrPhaseLockTimeout`, and `DescribeLockFailure` renders the ADR-0035
recovery text naming the lock path and a plain re-run — never a
delete-the-lock instruction, since a live holder means the write is
genuinely contended and a crashed holder has already released. A
grep/seam-anchored anti-drift test (`writer_inventory_test.go`) asserts every
`MergeMetadata` call site writing `mindspec_phase` executes inside the
lock's critical section, so a new unlocked writer is red (AC-5(vi)); a
companion legacy-migration contention test drives `EnsureMigrated`'s
absent-check re-run against a fresher locked writer and asserts the locked
value survives (AC-5(vii)).

`order.go`'s `phaseRank`/`IsBackwardMove(from, to)` is the lifecycle total
order (`spec < plan < implement < review < done`); a value outside the five
named phases — including the empty string — is UNKNOWN and treated as
INCOMPARABLE, which `IsBackwardMove` reports as backward. `mindspec repair
phase` (`cmd/mindspec/repair.go`) is the one OPERATOR-facing consumer: it
refuses a write that would move an epic's stored phase backward under this
comparator unless `--force-backward "<reason>"` is passed, and a forced
write's success line names the recorded reason.

`sanctioned_sync.go`'s `SanctionedDerivedSync(root, epicID)` is the single
SANCTIONED internal backward-write surface — distinct from the operator
guard above: it re-derives an epic's child-derived phase and writes it
INSIDE the lock's own critical section, backward permitted, no force flag,
no recorded reason, because the caller is re-deriving from the epic's own
FRESH children rather than asking a human to override a guard against
possibly-stale evidence. Its production reachability is fenced by a
call-site enumeration test pinning `cmd/mindspec/release.go`'s
`releaseSyncPhase` as the ONLY direct caller (the `impl adopt` house
pattern); a failed children read propagates as an error rather than
deriving `plan` from an empty result — the exact "discovery failure
rendered as a successful answer" class this spec exists to close,
reproduced inside its own sanctioned-backward-write fix.

`semantics_v2.go` performs a related but DISTINCT one-shot: `plan approve`
now writes `plan` (not `implement`) as OQ-3's v2 semantics (`plan.go`'s
sibling change). An epic whose stored phase was written `implement` under
the OLD (pre-v2) semantics — at least one lifecycle child, every one still
`open` — carries a value `LegacyPhaseSignature(stored, children)` detects as
wrong under v2. `EnsurePhaseSemanticsV2(root, epicID)` rewrites it exactly
once per epic, gated by the durable `mindspec_phase_semantics: "v2"` marker
(`PhaseSemanticsV2MarkerKey`/`PhaseSemanticsV2Value`) so it can never re-fire
— a TRANSLATION of an unchanged underlying fact into the new representation,
not an ongoing backward move, and therefore does not route through
`SanctionedDerivedSync`. `derive.go`'s `DerivePhaseWithStatus(WithCache)` —
the status-aware surface every AC-2 agreement-table row's derivation leg
uses — checks for this legacy signature before its general stored-vs-derived
mismatch warning: an unmigrated legacy epic gets a "legacy phase semantics
detected" notice naming `mindspec repair phase` (which itself runs
`EnsurePhaseSemanticsV2` at its own entry, before the backward guard
evaluates) instead of the general warning. That general warning is now
DIRECTION-CONDITIONAL (spec 128 Bead 2): a FORWARD disagreement (derived
phase ahead of stored) keeps the `mindspec repair phase <spec-id>` recovery
line; a BACKWARD disagreement (derived is behind stored) never suggests that
command — writing it would move the epic backward — and instead ends with
an inspection-only `bd show <epic-id>` line, belt-and-suspenders alongside
the R3.3 guard's own refusal of an unforced backward write.

## The `state.ModeIndeterminate` sentinel (spec 128 R2.3)

`internal/state`'s `ModeIndeterminate` (`"indeterminate"`) is a NEW mode
constant, deliberately excluded from `ValidModes`/`IsValidMode` (the same
way the pre-existing `"ambiguous"` literal is): it is never a real operating
mode and never persisted. It exists purely so a workflow-domain instruct-tail
caller that hits a genuine discovery failure (bd/Dolt unreachable) or an
unresolved ambiguity (multiple active specs, no explicit target) can select
`templates/indeterminate.md` by the same `ctx.Mode + ".md"` convention every
other mode uses, instead of fabricating `idle.md` guidance for a failure it
never actually observed. The workflow domain's `internal/instruct` and
`internal/resolve` packages are this sentinel's only producers/consumers;
core's role is limited to defining and excluding it from the valid-mode set.

### Lineage and spec binding

The phase package owns bead lineage and epic-to-spec resolution.
One parent-child dependency on an epic supplies lineage. Blocks edges and
bead titles do not supply lineage. The resolver returns edge provenance
with the epic and full spec ID. Callers can classify a row without a second
bead read.

Full spec IDs retain optional letters. Resolution uses `spec_id` first,
the legacy numeric/title pair second, and the full epic title ID third.
A stored full ID decides after validation. A bad full ID refuses. A legacy
pair that conflicts with the title ID requires repair. Zero numeric metadata
or an empty legacy title means that pair is absent.

The invocation cache checks every epic, including closed epics, before it
stores a scoped binding. Duplicate bindings block only their spec.
Listings warn and skip bad bindings. Epics with no ID source remain silent.

Spec binding refusals wrap `ErrSpecResolution`. This sentinel is distinct
from tracker read failures and missing bead lineage. The resolver escapes
external data and supplies the final recovery command. Callers preserve
that command as an unquoted line.
