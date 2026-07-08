# Workflow Domain — Interfaces

## Provided Interfaces

### Phase Derivation (ADR-0023)

```go
package phase

// DiscoverActiveSpecs queries beads for all open epics and derives phase for each.
func DiscoverActiveSpecs() ([]ActiveSpec, error)

// FindEpicBySpecID finds the epic ID for a given spec ID by querying metadata.
func FindEpicBySpecID(specID string) (string, error)

// DerivePhase determines the lifecycle phase from an epic's children statuses.
func DerivePhase(epicID string) (string, error)

// ResolveContext determines the current spec, bead, phase from working directory.
func ResolveContext(root string) (*Context, error)
```

### Target Resolution (Spec 079)

```go
package resolve

// ResolveTarget determines which spec to operate on via --spec flag, CWD, or auto-select.
func ResolveTarget(root, specFlag string) (string, error)

// ResolveSpecPrefix resolves a numeric prefix (e.g. "079") to a full spec ID.
func ResolveSpecPrefix(prefix string) (string, error)
```

### Guidance Emission (Spec 004)

```go
package instruct

// BuildContext creates a rendering context from state and project root.
func BuildContext(root string) *Context

// Render produces markdown guidance for the given context.
func Render(ctx *Context) (string, error)
```

### Beads Adapter (`internal/bead/`)

```go
package bead

func RunBD(args ...string) ([]byte, error)   // Execute bd commands
func ListJSON(args ...string) ([]byte, error) // List with JSON output
func Close(ids ...string) error               // Close beads
func WorktreeList() ([]WorktreeListEntry, error)
```

### Panel Artifact Schema (Spec 110 Bead 1, ADR-0040 portability contract)

The review-panel lifecycle's on-disk shapes are the agent-neutral
surface a non-Claude-Code runner targets (ADR-0040's portability
principle: agents integrate at the artifact + CLI contract level,
never at the prompt-format level). `internal/panel` is the single home
of these shapes; this section must not name a pattern the constants
below reject.

**Registration.** Every panel directory (`review/<slug>`, or the
co-located `<spec-dir>/reviews/<slug>` on a flat tree, ADR-0039) holds
exactly one `panel.json` — the literal filename `panel.FileName`. The
steady-state intent, from Bead 4 onward, is that `mindspec panel
create` (`panel.Create`, this bead) is the sole writer, in full; before
Bead 4 lands that CLI verb, `/ms-panel-run` step 0 hand-authors it
directly. The one SANCTIONED hand-edit path, in either era, is
`/ms-panel-tally`'s Abandon procedure, which sets `abandoned` /
`abandon_reason` by hand — every other field is machine-written once
Bead 4 lands.

`panel.Create` writes `panel.json` as a full-struct overwrite: it never
reads the existing file first, unlike its read-before-splice handling
of `BRIEF.md`'s machine-managed header (Maintenance Notes below). A
re-panel of an abandoned panel therefore deliberately clears
`abandoned`/`abandon_reason` and revives it into an active round — this
is the KNOWN, intentional behavior of calling `Create` again, not an
oversight (`TestCreate_RepanelOfAbandonedPanelRevivesIt`).

**Verdict files.** Each reviewer writes one `<slot>-round-<N>.json`
file beside the registration, with `N` starting at 1 (`panel.verdictFileRE`).
A conforming example is `R1-round-1.json`; `codex-correctness-round-2.json`
is another. The filename `R1-round-0.json` (nonconforming — rejected)
never appears — round numbering is 1-based, and a writer that emits
round 0 is a bug, not a valid first round.

**Consolidated change list.** `mindspec panel tally`'s aggregated
`concrete_changes_required`, once deduped and ranked, is authored to
`consolidated-round-1.md` for round 1 (the general shape is
consolidated-round-N.md, `panel.ConsolidatedName(N)`).

**Verdict JSON payload.** Every verdict file's top-level shape:

- `verdict` (required): a non-empty string. A reviewer MUST write one
  of `APPROVE`, `REQUEST_CHANGES`, `REJECT` — the three values the gate
  acts on. `panel.Tally`'s parser (`internal/panel/tally.go`) does not
  itself enforce this enum: any OTHER non-empty string parses as
  present (it counts toward `expected_reviewers` completeness) but is
  neither an APPROVE nor a REJECT, so it can never help reach the
  approve threshold and never triggers a REJECT halt. An empty or
  whitespace-only `verdict`, or invalid JSON, is malformed and counts
  as MISSING, not present.
- `hard_block` (optional boolean): a top-level sibling of `verdict`.

`panel.Tally` parses only these two top-level keys from each verdict
file; `panel.PanelGateDecision` then acts on the resulting tally —
REJECT/`hard_block` halts the gate, otherwise the APPROVE count is
checked against the N-1 threshold. A reviewer additionally writes
`reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`,
and `findings` as context for `mindspec panel tally`'s
presentation-only aggregation — none of those five feed the gate
decision. `hard_block` is read only at the top level as a sibling of
`verdict`, never nested under any other reviewer-authored field.

Degraded modes (a slow reviewer, a runner that cannot render Markdown)
are the runner's concern; the schema itself makes no allowance for a
partial or alternate shape.

## Consumed Interfaces

- **core**: `workspace.FindRoot()`, `workspace.DetectWorktreeContext()` for locating state and worktrees
- **execution**: `executor.Executor` for all git/worktree operations
- **context-system**: `contextpack.RenderBeadContext()` for bead primers

## CLI Commands

| Command | Purpose |
|:--------|:--------|
| `mindspec state set` | Set current mode and active work |
| `mindspec state show` | Display current state |
| `mindspec instruct` | Emit mode-appropriate operating guidance |
| `mindspec next` | Discover, claim, and start next bead |
| `mindspec complete` | Close bead, remove worktree, advance state |
| `mindspec approve spec\|plan\|impl` | Transition between lifecycle phases |
| `mindspec cleanup` | Remove stale worktrees and branches |
| `mindspec config show` | Print the effective config (panel/models/loop/runner + pre-existing keys), read-only (spec 109 R9) |

## Agent Skills

| Skill | Purpose |
|:------|:--------|
| `/ms-explore` | Enter, promote, or dismiss an Explore Mode session |
| `/ms-spec-create` | Create a new specification |
| `/ms-spec-approve` | Request Spec -> Plan transition |
| `/ms-plan-approve` | Request Plan -> Implementation transition |
| `/ms-impl-approve` | Request Implementation -> Done transition |

## Maintenance Notes

- **2026-07-08 (spec 110 Bead 1, panel-verbs-parser-parity — R1/R4/R7b):**
  `internal/panel` gains a leaf-safe registration writer,
  `Create(dir string, in CreateInput) error` (`internal/panel/create.go`).
  It takes plain values only (`BeadID *string`, `Spec`, `Target`,
  `Round`, `ExpectedReviewers`, `ApproveThresholdExpr`,
  `ReviewedHeadSHA`) and imports no `internal/config` and no git — the
  caller (`cmd/mindspec`, Bead 4) resolves the 109 config resolvers and
  the target SHA and passes them in, keeping `internal/panel` an
  import-clean, config-free leaf. `Create` writes `panel.json` and
  rewrites `BRIEF.md`'s machine-managed header region (delimited by
  `<!-- mindspec:panel-header -->` / `<!-- /mindspec:panel-header -->`)
  in one operation: `round` and `reviewed_head_sha` land in the same
  `Panel` value, so no code path can bump one without the other, in
  either file. A re-panel replaces only the delimited region,
  preserving everything else in `BRIEF.md` — including CRLF line
  endings — byte-for-byte, and never touches any `<slot>-round-<N>.json`
  verdict file. A `BRIEF.md` with no markers (legacy) gets a fresh
  region prepended with the existing body kept byte-identical below
  it; an ambiguous or corrupt marker state (no closing marker, or more
  than one opening/closing pair) makes `Create` return an error and
  write NEITHER `panel.json` NOR `BRIEF.md` — the marker validation
  runs before either file is touched. The header's fixed "## Your job"
  block is the one place the verdict-JSON contract (§ Panel Artifact
  Schema above) is written into a BRIEF; the skill (Bead 5) no longer
  re-authors a second copy. `TestPanelSchemaDoc_MatchesConstants`
  (`internal/panel/create_test.go`) extracts this section's own
  backtick-quoted examples and asserts they agree with
  `panel.FileName` / `panel.verdictFileRE` / `panel.ConsolidatedName`,
  so this doc cannot silently drift from the code.
- **2026-07-07 (spec 109 Bead 4, orchestration config substrate — R8/R9):**
  `cmd/mindspec/config.go` adds a read-only `config` command with a `show`
  subcommand: it loads the effective config via `config.Load`, renders it
  through the pure `renderConfig(*config.Config) (string, error)` (no fs, no
  panel scan — testable without a process), and prints it to stdout. The
  `models:`, `loop:`, and `runner:` blocks are annotated "declared, not yet
  enforced" in the rendered output; `panel:` is not, since it already drives
  a fresh panel.json's creation-time defaults today. `renderConfig` sorts
  `Loop.GateAuthority`'s map keys before rendering (a map iterated directly
  would make the command's output nondeterministic) and renders
  `PanelApproveThresholdExpr()` verbatim (no trim/normalize — the resolver's
  contract is "exactly as configured").
  Two caller-side surfaces render the leaf-safe `panel.ReviewerCountNote`
  advisory (Bead 3) when a registered panel's recorded `expected_reviewers`
  differs from `cfg.PanelExpectedReviewers()` — never altering any gate
  `Allow`/`Block`: (1) `config show`'s command handler
  (`reviewerCountNotesFor` in `cmd/mindspec/config.go`) scans every
  registered panel across the repo root and every spec's own directory
  (`configShowReviewRoots`, since `config show` has no bead/spec context
  to scope the scan the way the complete-gate does) and appends one note
  line per differing panel; (2) `internal/complete`'s authoritative panel
  gate (`panelGate`, `panel_advisory.go`) — reached from `complete.Run`
  step 2.25, AFTER the gate's own Allow/Block decision — prints the same
  note via the new `reviewerCountAdvisory` helper to the advisory writer
  when the matched panel's recorded count differs from the config default.
- **2026-07-02 (spec 107 wave 1):** `mindspec complete`'s children/epic bd
  fan-out was collapsed. The post-close state advance (`internal/complete`
  `advanceState`) now reads children through the new exported
  `phase.FetchChildren(epicID)` seam — a single uncached `bd list --parent`
  query — replacing the old per-status `queryAllChildren` loop (~5 subprocesses).
  `complete.Run` also resolves the immutable spec→epic mapping ONCE through a
  shared `phase.Cache` (threaded via `phase.EnsureMigratedWithCache` /
  `phase.FindEpicBySpecIDWithCache` / `phase.DerivePhaseWithCache`), so a run
  issues at most one `bd list --type=epic` while the post-close children read
  stays fresh. Gate-failure error/`recovery:` lines are unchanged (ADR-0035).
- **2026-07-02 (spec 108 wave 2, bead wpjv.3):** `internal/validate` grew two
  per-run caching seams that cut redundant reads without changing a single
  emitted diagnostic (ADR-0036/ADR-0032 contracts intact). (1) An in-memory
  `ownershipCache` (keyed by domain, routed through the `loadOwnershipForRefFn`
  seam) loads each candidate domain's `OWNERSHIP.yaml` at most once per
  `ValidateDivergence`, `checkInternalPackages`, and `normalizeImpactedDomains`
  run — replacing the old per-`(file × domain)` `attributeDomain` re-load, and
  with it the up-to-three `git show` subprocesses per domain in
  `LoadOwnershipAtRef`. `attributeDomain` is now a one-shot wrapper over the
  shared-cache `attributeDomainCached`. (2) A `memoStore` decorator (validate-
  local, `internal/adr` untouched) wraps the store from the `adrStoreForSpecFn`
  seam so `coverageOf` / `hasAcceptedCitation` / `checkADRCitations` read each
  distinct cited ADR from disk at most once per run instead of
  `O(domains × citations)` times. Both seams are countable, and a golden
  diagnostics fixture pins byte-identical `(code, message)` output pre/post
  caching.
