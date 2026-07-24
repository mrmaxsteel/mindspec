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

### Readiness Engine (`internal/validate/readiness/`, spec 124)

```go
package readiness

// Signal IDs, stable across releases: "MF-1" (plan section concrete-by-
// structure), "MF-2" (claimed R/AC tokens resolve), "MF-3" (dependencies
// closed AND landed-merged), "MF-4" (no genuine blocking marker).
const (
    SignalPlanSection  = "MF-1"
    SignalTokens       = "MF-2"
    SignalDependencies = "MF-3"
    SignalBlocking     = "MF-4"
)

// EvaluateReadiness evaluates the four mechanical-floor signals for a
// bead. Pure read: no bd write, no git write, no file write. Owning-spec
// resolution is lineage-authoritative (bead -> epic -> spec), never cwd.
func EvaluateReadiness(root, beadID string) (*Report, error)

// Report always carries exactly four signals, ordered MF-1..MF-4.
func (r *Report) AllPass() bool
func (r *Report) FailingSignals() []Signal
func (r *Report) RecoveryCommands() []string // one recovery line per FAIL

// Render is the SOLE renderer of the per-signal report — shared by
// `bead ready-check` and `next`'s gate refusal (ADR-0040 no-restate).
// Agent-influenced detail text passes through termsafe.Escape here.
func Render(r *Report) string
```

The gate-before-mutate wiring lives in `internal/next/ready_gate.go`:

```go
package next

// GateReadiness runs after bead selection and BEFORE any claim/branch/
// worktree mutation (ADR-0041 fourth-verb preflight leg). On a failing
// floor: allowNotReady=false returns a guard refusal (zero mutation);
// allowNotReady=true returns the failing signal IDs for the caller's
// stderr warning + override marker.
func GateReadiness(root, beadID string, allowNotReady bool) (*GateResult, error)

// RecordReadinessOverride writes the durable --allow-not-ready marker
// (bead metadata key "mindspec_readiness_override": bypassed signal IDs
// + UTC timestamp). Called BEFORE ClaimBead, fail-closed — a marker-write
// failure refuses the whole `next` invocation with nothing claimed.
func RecordReadinessOverride(beadID string, signals []string) error
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
absolute path, or any control character — via `unicode.IsControl`, the
full C0/DEL/C1 range (including `\n`/`\r`/NUL and the C1 CSI `U+009B`
terminal-injection vector, mirroring `report.go`'s `stripControl`) — is
rejected with a `guard.NewFailure` naming the offending value. `panel
create` additionally rejects a `--bead`/`--target` value containing a
control character through the same control-byte check before it is
ever written to `panel.json` or interpolated into a rendered message or
recovery line — closing the spec-109-final-review G2 terminal-injection
class (a slug/flag value bearing a control byte can never forge an
extra display line or a fake `recovery:` line).

**`mindspec panel create <slug> --spec <id> --target <ref> [--bead
<id>] [--round N] [--gate <name>]`** stamps `expected_reviewers`/`approve_threshold`
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
at gate time), never a false-PASS. On success, stdout carries two
lines: `panel <slug> registered: round N, K expected reviewer(s),
reviewed_head_sha <sha>` followed by `panel directory: <dir>` — the
SAME resolved `dir` `panel.Create` wrote to. This second line is the
stable, greppable contract a caller (e.g. the `ms-panel-run` skill)
reads to learn the panel directory without re-deriving the layout
logic above.

**`--gate <name>`** (spec 112 R9's deferred writer, completed by spec
113 R3) is optional and validated against the closed enum
`config.PanelGateKeys` (`spec_approve`, `plan_approve`, `bead`,
`final_review`, `adhoc` — the single declaration,
`internal/config/config.go:101`; the CLI never keeps a second copy) —
same control-byte discipline as `--bead`/`--target`, checked before the
enum-membership check. A value outside the enum is rejected with a
`guard.NewFailure` whose recovery line names all five keys, BEFORE any
filesystem write. When given, `panel.json`'s decision-inert `gate` field
(`json:"gate,omitempty"`) is stamped with the value, and
`expected_reviewers`/`approve_threshold` are resolved through spec 112
R3's gate-scoped resolvers (`cfg.PanelGateExpectedReviewers(gate)` /
`cfg.PanelGateApproveThresholdExpr(gate)`) instead of the global ones.
When omitted, behavior is byte-identical to pre-spec-113 output: the
global resolvers are used and no `gate` key is written. The field stays
decision-inert end to end — neither `PanelGateDecision` nor
`ApproveThreshold()` reads it — but it IS what `config
show`/`mindspec complete`'s `PanelGateAdvisoryDefault` selection keys on
for the reviewer-count advisory note, so a CLI-created `final_review`
panel now selects that gate's per-gate default instead of being
advisory-skipped.

**`mindspec panel verify <slug>`** is READ-ONLY: it writes no file and
ALWAYS exits `0` — it is a report, never a gate. It prints verdicts
present vs `expected_reviewers`, per-slot parse status (malformed files
named), `reviewed_head_sha` vs the live target tip, and a PASS/BLOCK
preview computed by `panel.PanelGateDecision` over facts gathered by
`panel.ResolveGateFacts` — the IDENTICAL decision `mindspec complete`'s
gate enforces.

**`mindspec panel tally <slug>`** is ADVISORY-BUT-BLOCK-CAPABLE: its
exit code tracks the `panel.PanelGateDecision` decision alone, and is
non-zero on `Block`. Do not describe `verify`/`tally` as both
"read-only/advisory" — `verify` never blocks; `tally` can. `tally`
prints the per-slot verdict table (slot, verdict, `hard_block`), the
aggregate APPROVE/REQUEST_CHANGES/REJECT counts + the resolved
threshold, the `panel.PanelGateDecision` decision, and the aggregated
`concrete_changes_required` — read presentation-only from each
REQUEST_CHANGES/REJECT verdict file (a re-parse failure, an absent key,
or a non-array-of-strings type attributes zero items to that slot with
an advisory note, never fatal; this second-pass read never feeds the
decision). The exit code is derived from the decision's `Action` ALONE
— never re-derived from raw verdict counts (`res.Approves` etc.):
`Allow` -> `0`; `Warn` -> `0` with the advisory printed (non-blocking,
parity with `internal/complete`'s Warn handling); `Block` -> non-zero,
carrying `PanelGateDecision`'s raw-`git merge` fence plus a genuine
ADR-0035 `recovery:` line (`guard.HasFinalRecoveryLine`).

**Non-bead staleness (spec 113 R1).** For a non-bead panel (`bead_id`
null), `panel verify`/`panel tally` resolve staleness from the panel's
RECORDED `panel.json.target` — rev-parsed in `cmd/mindspec` via the
same `revParseForPanelFn` seam `panel create` uses to capture
`reviewed_head_sha` at write time — fed through the SAME
`panel.PanelGateDecision` legs a bead panel uses (internal/panel takes
a zero-byte diff). Previously the CLI left `beadID` empty for a
non-bead panel and `panel.ResolveGateFacts` rev-parsed the literal,
always-absent ref `bead/`, so every non-bead panel short-circuited at
the missing-ref leg with a malformed `references branch bead/` message
and reported PASS even after its target advanced past
`reviewed_head_sha`, or with a REJECT verdict on file. The fix un-shadows
staleness and the incomplete/REJECT/threshold legs for non-bead panels;
a `sanitizeNonBeadDecision` CLI-layer rewrite (never touching
`Decision.Action`) replaces the empty-bead-interpolation fragments and
the empty `git merge bead/` fence with target-naming text, and never
emits a `mindspec complete <bead>` recovery line for a panel that is not
complete-gated.

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
| `mindspec next` | Discover, claim, and start next bead — evaluates the mechanical readiness floor gate-before-mutate: a NOT-READY refusal claims nothing, creates no branch/worktree (spec 124 R3, ADR-0041 fourth-verb preflight leg) |
| `mindspec next --allow-not-ready` | Proceed past a failing readiness floor deliberately — warns naming every failing signal and records the durable `mindspec_readiness_override` marker BEFORE claiming, fail-closed (spec 124 R3/AC-4); orthogonal to `--force` (session-freshness only) |
| `mindspec bead ready-check <id>` | Read-only per-signal readiness report (MF-1..MF-4); exit 0 when all pass, else one `recovery:` line per failing signal (spec 124 R1) |
| `mindspec bead clarify <id> --file <record.json>` | Write the append-only readiness-attempt record for a NOT-READY bead — original ordinal-keyed report + span-grounded clarifications; refuses a second write per bead, the categorical R8 cap (spec 124 R8) |
| `mindspec complete` | Close bead, remove worktree, advance state |
| `mindspec approve spec\|plan\|impl` | Transition between lifecycle phases |
| `mindspec cleanup` | Remove stale worktrees and branches |
| `mindspec config show` | Print the effective config (panel/models/loop/runner + pre-existing keys), read-only (spec 109 R9) |
| `mindspec panel create <slug>` | Register (or re-panel) a review panel — stamps the config resolvers + `reviewed_head_sha` (spec 110 R1); optional `--gate <name>` stamps the decision-inert `gate` field + that gate's creation-time defaults (spec 113 R3) |
| `mindspec panel verify <slug>` | Read-only completeness/staleness report; decision-identical to the gate, writes nothing, always exits 0 (spec 110 R2; spec 113 R1 non-bead staleness from recorded target) |
| `mindspec panel tally <slug>` | Per-slot verdicts + aggregate + decision; advisory-but-block-capable — exit code tracks the decision alone, non-zero on Block (spec 110 R3; spec 113 R1 non-bead staleness from recorded target) |
| `mindspec config show --gate <name> [--json]` | Print one panel gate's resolved creation-time defaults — expanded slots, expected reviewer count, raw `approve_threshold` expression, effective substitution policy — as text or JSON; read-only (spec 112 R8/R9) |
| `mindspec models populate` | Print the ZFC agent prompt for declaring the per-phase `models:` protocol in `.mindspec/config.yaml`; writes nothing (spec 123 R6, mirrors `mindspec source populate`) |
| `mindspec commands populate` | Print the ZFC agent prompt for declaring the consumer's build/test guidance under `commands:`; writes nothing — once populated, `init`/`setup` render the entries as the managed AGENTS.md "Build & Test" section (spec 123 R7) |
| `mindspec adr create "<title>" [--slug <kebab>]` | Create an ADR with a slugged filename `ADR-NNNN-<slug>.md` derived from the title (or `--slug` override); every surface reports/accepts the canonical `ADR-NNNN` ID (spec 123 R5) |
| `mindspec panel create <slug> --gate adhoc --target <ref>` | Create an ad-hoc panel WITHOUT `--spec` at `.mindspec/reviews/<slug>/` (flat layout) — talliable via `panel tally`, never scanned by `mindspec complete`'s gate (spec 123 R8, ADR-0037 scope) |
| `mindspec domain add <name>` | Scaffold a domain AND converge from any partial state: backfills missing standard files and the context-map entry; refuses "already exists" only when fully scaffolded and mapped (spec 123 R2) |

## Agent Skills

| Skill | Purpose |
|:------|:--------|
| `/ms-explore` | Enter, promote, or dismiss an Explore Mode session |
| `/ms-spec-create` | Create a new specification |
| `/ms-spec-approve` | Request Spec -> Plan transition |
| `/ms-plan-approve` | Request Plan -> Implementation transition |
| `/ms-impl-approve` | Request Implementation -> Done transition |

## Maintenance Notes

- **2026-07-09 (spec 111 Bead 2, the `/ms-panel` workflow adapter — R1-R5,
  R3b, R8):** `plugins/mindspec/workflows/ms-panel.js` (embedded and
  installed byte-identical to `.claude/workflows/ms-panel.js`) is the first
  Claude Code **dynamic workflow** MindSpec ships — a `.js` orchestration
  script invocable as `/ms-panel` that coordinates agents and performs no
  shell/file I/O itself (every CLI call and file write below is an `agent()`
  step).
  **Args contract:** `{slug, spec, target, bead_id?, round, sha?, lenses[],
  mix, claude_sub_on_quota?}`. `mix` and `claude_sub_on_quota` are resolved
  by the invoking skill (`ms-panel-run`, spec 111 Bead 3) from config
  `panel:` / `panel.substitution.claude_sub_on_quota` (spec 109) — this
  workflow never reads `.mindspec/config.yaml` itself (no fs access, and
  `mindspec config show` is deliberately outside `ALLOWED_CLI`).
  `claude_sub_on_quota` is not part of spec 111 R1's illustrative args tuple;
  it is the field this bead adds and documents here so Bead 3 can rely on it.
  `sha` is advisory-only display text — the workflow never reads it to set
  the recorded SHA.
  **Input hardening** runs before any command or path is built: `slug`,
  `spec`, `bead_id` (optional), and every `mix[].family` are validated
  against the clean-single-path-element contract (reject empty, `.`, `..`,
  `/`, `\`, control bytes); `target` against a branch-name-safe grammar
  (reject empty, control bytes, a leading `-`, `git check-ref-format`
  disallowed constructs, a trailing `/` or `.lock`, whitespace); `target` and
  `bead_id` additionally reject shell metacharacters; `round` must be a
  positive integer. Any failure throws before registration runs.
  **`buildCommand(verb, ...values)`** is the sole command-construction
  chokepoint: `verb` must be one of the four identifiers destructured from
  the static `ALLOWED_CLI` array (`mindspec panel create`, the sandboxed
  `codex exec --sandbox read-only --skip-git-repo-check`, `mindspec panel
  verify`, `mindspec panel tally`), and every value is rejected if it starts
  with `-` (argument-injection guard). The lifecycle merge-terminal command
  appears nowhere in the file.
  **Fan-out (R3/R3b):** `mix` flattens into `{slotId, family, lens}`
  descriptors (`slotId` from a fixed `R1, R2, …` enumeration, never from
  args), fanned out via `pipeline()`. A `claude` slot is a single `agent()`
  step that writes its verdict to `<panel-dir>/<slot>-round-<N>.json`. A
  `codex` slot is a **wrapper agent**: it execs the sandboxed codex command,
  persists codex's raw stdout verbatim to `<slot>-round-<N>.codex.log`, and
  writes the verdict itself — codex never writes a file, sandbox-enforced by
  the `--sandbox read-only` pin. A rendered-but-malformed verdict is
  re-prompted to the **same** reviewer exactly once (a re-serialize, never a
  fresh review); still unparseable after that single retry fails CLOSED to a
  MISSING slot — never substituted.
  **Substitution (R4):** reserved exclusively for a quota wall with **no**
  verdict ever rendered. When `claude_sub_on_quota === true`, a `claude`
  agent substitutes for that slot keeping the slot id, with `reviewer_id:
  "<slot> claude-sub"`; when false (the fail-closed default when the field is
  absent), the slot is left missing.
  **Return (R5):** the workflow's single structured result carries `mindspec
  panel verify` + `mindspec panel tally` stdout verbatim (unmodified) plus
  the per-slot outcomes; it never runs the lifecycle merge-terminal command,
  never consolidates, never authors a `consolidated-round-<N>.md`.
  **Distribution (R8):** `plugins/mindspec/embed.go`'s `WorkflowFiles()`
  (`//go:embed workflows/*`) mirrors `SkillFiles()`; `internal/setup/claude.go`'s
  `installWorkflows` mirrors `installSkills`' create/skip/notice disposition
  and installs to `.claude/workflows/` on the Claude target only —
  `RunCodex`/`RunCopilot` (which install only to `.agents/skills/`) never
  receive it.
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
