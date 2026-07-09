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
| `mindspec config show --gate <name> [--json]` | Print one panel gate's resolved creation-time defaults — expanded slots, expected reviewer count, raw `approve_threshold` expression, effective substitution policy — as text or JSON; read-only (spec 112 R8/R9) |

## Agent Skills

| Skill | Purpose |
|:------|:--------|
| `/ms-explore` | Enter, promote, or dismiss an Explore Mode session |
| `/ms-spec-create` | Create a new specification |
| `/ms-spec-approve` | Request Spec -> Plan transition |
| `/ms-plan-approve` | Request Plan -> Implementation transition |
| `/ms-impl-approve` | Request Implementation -> Done transition |

## Maintenance Notes

- **2026-07-09 (spec 112 Bead 3, per-gate panel config — gate-aware
  advisory + `config show` gates/substitutes/`--gate` — R7/R8/R9):** Both
  `ReviewerCountNote` callers (`internal/complete`'s `complete.Run` step
  2.25 and `cmd/mindspec/config.go`'s `reviewerCountNotesFor`) now compare a
  recorded panel's `expected_reviewers` against the GATE-APPROPRIATE
  config default through the single shared selection rule,
  `(*config.Config).PanelGateAdvisoryDefault(recordedGate string, isBead
  bool) (int, bool)` (homed in `internal/config`, spec 112 Bead 1): a
  known recorded gate uses that gate's resolved default; a gate-less bead
  panel falls back to the `bead` gate; a gate-less non-bead panel or an
  unrecognized recorded gate value SKIPS the note (`ok == false`) once
  `gates:` is configured; with `gates:` absent every panel still compares
  against the flat global default, byte-identical to spec 109. The
  `internal/complete` call site is guarded on `panelReg != nil` —
  `panelGate` returns a nil registration on its fail-open paths (empty bead
  ID, no registered panel), and `PanelGateAdvisoryDefault`'s arguments
  deref `panelReg.Panel`, so the guard sits ahead of that call even though
  `reviewerCountAdvisory` itself also nil-checks its `reg` parameter. No
  `Allow`/`Block` decision is touched by any of this — the gate's outcome
  is fully computed before the advisory call site runs.

  `renderConfig` (`cmd/mindspec/config.go`) now also echoes a set
  `panel.note` verbatim (escaped), renders `panel.gates` — only configured
  gates, in `config.PanelGateKeys` enum declaration order, never map
  iteration order — each with its as-configured reviewer entries, its
  resolved reviewer sum (`PanelGateExpectedReviewers`), and its raw
  `approve_threshold` expression (`PanelGateApproveThresholdExpr`); renders
  `panel.substitution.substitutes` in sorted-key order beside the
  slot-id-preservation convention line (a substituted reviewer writes
  `reviewer_id "<slot> <substitute-model>-sub"`, keeping the slot id); and
  annotates any model id (from the global reviewers, any gate's reviewers,
  or either side of `substitutes`) absent from `config.KnownModels()` with
  an advisory warning that never affects the exit code. The `gates:` and
  `substitutes:` keys are never omitted — an unconfigured/empty map still
  renders `gates: {}` / `substitutes: {}`. Every config-controlled string
  this bead adds to a text-render path passes through the existing
  `escapeConfigValue`.

  `mindspec config show` gained `--gate <name> [--json]` (R8/R9): two pure
  functions, `renderGateResolved` (text) and `gateResolvedJSON` (a typed
  struct marshaled with `encoding/json`, never string concatenation), both
  delegating to a shared `buildGateResolvedDoc` that calls ONLY the R3
  config resolvers (`PanelGateReviewerSlots`/`PanelGateExpectedReviewers`/
  `PanelGateApproveThresholdExpr`) — so `--gate` output cannot disagree
  with them. The JSON document's five members —`gate`, `slots` (`{slot,
  model, lens}` in R3 expansion order), `expected_reviewers`,
  `approve_threshold` (the raw expression string), and `substitution`
  (`substitutes` map, the legacy `claude_sub_on_quota` bool, and
  `in_force`: `"substitutes"` when the map is non-empty, else
  `"claude_sub_on_quota"`, per R5's supersession rule) — are a STABLE,
  ADDITIVE-ONLY CONTRACT (spec 112 R9): the surface the spec-110
  `panel.json` writer and the spec-111 orchestration runner build on.
  Renaming, retyping, or removing a documented member is a breaking change
  no follow-up may make silently — same stability guarantee as the
  recorded `gate` field on `panel.json` (Bead 2). An unknown `--gate` value
  propagates the R3 resolver's own ADR-0035 error (already carrying a
  `recovery:` line enumerating the five valid gate keys); `--json` without
  `--gate` is refused with its own recovery line, since the resolved view
  is inherently per-gate. The command stays read-only on every path — no
  writer- or runner-side behavior is added by this bead (out of scope for
  spec 110/111).
- **2026-07-08 (spec 112 Bead 1, per-gate panel config — the pointerization
  ride-along):** `internal/config.Reviewer.Count` became a pointer
  (`*int`, spec 112 R1) so an absent `count` is distinguishable from an
  explicit `count: 0`. `cmd/mindspec/config.go`'s `renderConfig` (the sole
  out-of-package `Reviewer.Count` reader) now renders reviewer counts
  through the exported `(Reviewer).CountValue()` accessor instead of the
  raw field — an absent `count` renders as its default, `1`. No other
  workflow-domain behavior changes in this bead.
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
