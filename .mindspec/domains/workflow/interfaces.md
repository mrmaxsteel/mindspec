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

### Panel CLI Verb Tree (Spec 110 Bead 4, ADR-0040 portability contract)

`mindspec panel create | verify | tally <slug>` is the CLI half of the
ADR-0040 portability contract (the Panel Artifact Schema above is the
artifact half): every subcommand is a thin adapter over `panel.Create` /
`panel.ResolveGateFacts` / `panel.PanelGateDecision` — never a second
decision implementation (R7a).

**Shared slug validation.** All three subcommands validate their
`<slug>` positional argument through one shared `validatePanelSlug`
BEFORE any `filepath.Join`: an empty slug, `.`, `..`, any `/` or `\`, an
absolute path, or any control character (including `\n`/`\r`/NUL) is
rejected with a `guard.NewFailure` naming the offending value. `panel
create` additionally rejects a `--bead`/`--target` value containing a
control character through the same control-byte check before it is
ever written to `panel.json` or interpolated into a rendered message or
recovery line — closing the spec-109-final-review G2 terminal-injection
class (a slug/flag value bearing a control byte can never forge an
extra display line or a fake `recovery:` line).

**`mindspec panel create <slug> --spec <id> --target <ref> [--bead
<id>] [--round N]`** stamps `expected_reviewers`/`approve_threshold`
from the config resolvers (`config.PanelExpectedReviewers()` /
`config.PanelApproveThresholdExpr()`, spec 109) and `reviewed_head_sha`
from `--target`'s live commit (resolved via the executor at write
time), in the SAME `panel.Create` write that co-bumps `panel.json` and
the BRIEF machine-managed header (Bead 1) — leaving prior verdict files
and the skill-authored BRIEF body untouched on a re-panel. The panel
directory is resolved layout-aware, reusing
`internal/complete.panelGateRoots`' own logic: the co-located
`<spec-dir>/reviews/<slug>` on a flat tree, otherwise the repo-root
`review/<slug>` convention. A `--bead <id>` panel expects `--target
bead/<id>` — the same ref `mindspec complete`'s gate rev-parses for
staleness; a divergent `--target` can only fail SAFE (a stale-SHA Block
at gate time), never a false-PASS.

**`mindspec panel verify <slug>`** is a READ-ONLY completeness/
staleness report: verdicts present vs `expected_reviewers`, per-slot
parse status (malformed files named), `reviewed_head_sha` vs the live
target tip, and a PASS/BLOCK preview computed by
`panel.PanelGateDecision` over facts gathered by
`panel.ResolveGateFacts` — the IDENTICAL decision `mindspec complete`'s
gate enforces. It writes no file and always exits `0` (a read-only
report is not itself a gate).

**`mindspec panel tally <slug>`** prints the per-slot verdict table
(slot, verdict, `hard_block`), the aggregate APPROVE/REQUEST_CHANGES/
REJECT counts + the resolved threshold, the `panel.PanelGateDecision`
decision, and the aggregated `concrete_changes_required` — read
presentation-only from each REQUEST_CHANGES/REJECT verdict file (a
re-parse failure, an absent key, or a non-array-of-strings type
attributes zero items to that slot with an advisory note, never fatal;
this second-pass read never feeds the decision). The exit code is
derived from the decision's `Action` ALONE — never re-derived from raw
verdict counts (`res.Approves` etc.): `Allow` -> `0`; `Warn` -> `0` with
the advisory printed (non-blocking, parity with `internal/complete`'s
Warn handling); `Block` -> non-zero, carrying `PanelGateDecision`'s
raw-`git merge` fence plus a genuine ADR-0035 `recovery:` line
(`guard.HasFinalRecoveryLine`).

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
| `mindspec panel create <slug>` | Register (or re-panel) a review panel — stamps the config resolvers + `reviewed_head_sha` (spec 110 R1) |
| `mindspec panel verify <slug>` | Read-only completeness/staleness report; decision-identical to the gate, writes nothing (spec 110 R2) |
| `mindspec panel tally <slug>` | Per-slot verdicts + aggregate + decision; exit code tracks the decision alone (spec 110 R3) |

## Agent Skills

| Skill | Purpose |
|:------|:--------|
| `/ms-explore` | Enter, promote, or dismiss an Explore Mode session |
| `/ms-spec-create` | Create a new specification |
| `/ms-spec-approve` | Request Spec -> Plan transition |
| `/ms-plan-approve` | Request Plan -> Implementation transition |
| `/ms-impl-approve` | Request Implementation -> Done transition |

## Maintenance Notes

- **2026-07-09 (spec 110 Bead 4, panel-verbs-parser-parity — R1/R2/R3/R7a):**
  `cmd/mindspec/panel.go` adds the `panel create | verify | tally <slug>`
  verb tree (§ Panel CLI Verb Tree above). `create` is the sole caller
  that resolves `internal/config`'s 109 resolvers and the target SHA and
  passes them to `panel.Create` as plain values (`internal/panel` stays
  an import-clean, config-free leaf — `go list -deps ./internal/panel`
  carries no `internal/config`, R7b). `verify` and `tally` both call
  `panel.ResolveGateFacts` + `panel.PanelGateDecision` and render its
  `Action` verbatim — `TestPanelVerbs_DecisionIsPanelGateDecision`
  (`cmd/mindspec/panel_test.go`) is the R7a contract pin: a
  branch-complete table of `panel.GateFacts` rows spanning every
  `gate.go` branch asserts `renderPanelVerify` and `renderPanelTally`
  render the IDENTICAL `Action`, so relocating any decision branch into
  a CLI adapter breaks the test. `panel tally`'s exit code
  (`tallyExitAction`) is derived from the decision's `Action` alone,
  never from raw verdict counts, closing the "passes every planned gate
  yet exits 0 on a stale-SHA or `hard_block` Block" regression class.
  Adding the `panel` command also required registering it in
  `internal/redact`'s `CommandTokens`/`SubcommandTokens` closed-set
  enums (`tally`/`verify`; `create` already existed) — the drift guard
  (`cmd/mindspec/redact_enum_drift_test.go`) fails closed on any
  cobra-tree addition the redaction allowlist doesn't mirror.
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
