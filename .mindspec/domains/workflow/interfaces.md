# Workflow Domain — Interfaces

## Provided Interfaces

### Doctor status and merge state

`doctor.Info` renders as `[INFO]`. `Report.HasFailures` counts only Error
and Missing. `Report.Fix` never runs an Info fix function.

Doctor reports one Info for a top-level `merge_strategy` key in
`.mindspec/config.yaml`: `merge_strategy is no longer used; remove it from .mindspec/config.yaml`.
A commented or indented key gives no finding. This Info does not change
the exit code. `doctor --fix` leaves the key in place.

| Lifecycle function | Contract |
|:-------------------|:---------|
| `FindNamingMerges(root, specBranch, beadID)` | Delegates to the shared `gitutil.FindNamingMerges` query used by finalization. Returns two-parent naming merges on the first-parent chain, newest first. It does not check content or the bead tip. |
| `CheckMergePresence(root, specBranch, specDir, merge)` | Returns changed paths, reverted paths and later commits that changed reverted paths. It runs independently of bead branch survival. |
| `FindValidMerge(root, specBranch, beadID)` | Requires one naming merge, an exact second-parent match to the current bead tip, and no reverted path. Nil with no error means no valid merge. |
| `MergeRestoreOutcome(specWorktree, specDir)` | Reads status. Tool artifacts do not count as user changes. A clean checkout returns `DestructionClean`; read errors return `DestructionEvidenceError`; user changes return `DestructionStaleDeletion`. |
| `RestoreMergeRecovery(specWorktree, beadID, epicID, merge, presence, outcome)` | Builds restore and follow-up guidance. Only `DestructionClean` permits the path-scoped checkout choice. The choice uses the destructive command constructor. |

`ScanIntegrityFindings` returns valid merges in `StaleOpen`, with
`ValidMerge: true`. Doctor renders these at Info with no `FixFunc`.
`RevertedMerges` holds open beads with reverted paths. Doctor renders these
at Error with no `FixFunc`. These beads do not also appear in `StaleOpen`.

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

### Conduct Rule Table (Spec 131)

`internal/rules` embeds the conduct rule table from `internal/rules/rules.yaml`.

- `Load` reads and validates the table.
- `Filter` selects rules by scope and gate in table order.
  Empty filters select all rules. A gate filter includes rules with no gate limit.
- `Lookup` finds a rule by its exact ID.
- `GateKeys` returns a copy of the accepted gate keys.

`cmd/mindspec/rules.go` provides `mindspec rules [--scope <s>] [--json]`.
The command lists all rules or selects rules by scope.
The `--json` flag selects JSON output.
`mindspec rules show <id>` prints one rule and its fields.
An unknown ID or scope returns an error with a `recovery:` line.

`internal/instruct` adds a `## Rules` section to mode guidance.
It lists orchestrator rules in every mode.
It also lists implementer rules in implement mode.

### Doctor Panel and Rule Checks

`mindspec doctor` shares one config load for source, model, command, and panel checks.
A load failure gives one `MindSpec config` Error with a `recovery:` line.
The command exits 1 if an Error or Missing result remains. Warn and Info do not fail it.

Doctor reports an Error for a reviewer family with no runner.
It reports a Warn for a reviewer entry that specifies only a model.
For each process runner used by a resolved gate roster, it checks `argv[0]` with `LookPath`.
A missing command gives an Error. Available review tools can produce Info suggestions.
A process runner's `probe` runs with `--probe`, regardless of its config source.
Without `--probe`, it runs only if the runner comes from untracked `.mindspec/config.local.yaml`.
If the tracking check fails, doctor requires `--probe`. A skipped probe gives Info.
Each probe runs at the workspace root with a 10-second limit. Failure gives an Error.

Doctor warns if the local config is tracked or if an existing local config is not ignored.
It also warns if the panel isolation root is not ignored.

Doctor checks staged-rule markers in `SKILL.md` files under `.claude/skills/` and `.agents/skills/`.
It warns for invalid markers, missing or unknown rule IDs, and past `remove-by` dates.
It compares dates in UTC. An Info result lists rules with non-advisory enforcement.

Doctor checks slot metadata under the configured panel isolation root.
It warns if the slot's panel is missing, abandoned, or at a later round.
The recovery line is `mindspec panel checkout <slug> --prune`.
For a spec panel, the line adds `--spec <spec-id>`. Doctor does not prune the slot.

`--ci` skips launcher checks, probes, tool suggestions, and stale checkout checks.
It retains config checks, reviewer resolution checks, and staged-marker checks.

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

// PlanMapsBead requires both the Nth positional "## Bead " heading
// (including headings inside fences) and a work_chunks id N.
func PlanMapsBead(planContent string, n int) (mapped bool, why string)

// Report carries lineage EpicID and SpecID from the resolver, never cwd.
// NonPlanFormError carries BeadID, EpicID and SpecID when lineage resolves
// but the bead has no usable dotted index; Error preserves the diagnostic.
type NonPlanFormError struct { BeadID, EpicID, SpecID string }

// Signal.Failures contains sorted, unique raw failure records; human
// Detail and terminal escaping do not participate in record identity.
func FailureRecordDigests(s Signal) []string // sorted lowercase SHA-256
func DependencyFailureRecord(depID, class, candidateSHA string) string
func IsIndeterminateRecord(rec string) bool

// Report always carries exactly four signals, ordered MF-1..MF-4.
func (r *Report) AllPass() bool
func (r *Report) FailingSignals() []Signal
func (r *Report) RecoveryCommands() []string // one recovery line per FAIL

// Render is the SOLE renderer of the per-signal report — shared by
// `bead ready-check` and `next`'s gate refusal (ADR-0040 no-restate).
// Agent-influenced detail text passes through termsafe.Escape here.
func Render(r *Report) string
```

Failure records (spec 129 R6.2) have these grammars:

| Signal | Raw record |
|:-------|:-----------|
| MF-1 | `<key>:<n>`, where key is `section-missing`, `ac-entry-missing`, `chunk-missing`, `key-file-paths-empty`, `key-file-paths-placeholder` or `key-file-paths-blank` |
| MF-2 | `dangling:<normalized-token>`; `AC9` normalizes to `AC-9`, parenthetical sub-letters to appended letters, Roman enumerators and dotted sub-numbers to the base |
| MF-3 | `<dep-id>:<class>[:<candidate-sha>]`; classes are `indeterminate`, `not-closed`, `no-candidate`, `off-first-parent`, `no-evidence`, `contradiction`, `reverted` |
| MF-4 | `<kind>:<source>:<section>:<k>:<sha256-of-normalized-line>` |

MF-1, MF-2 and MF-4 aggregate all failures. MF-4's kind is `token` or
`checklist`; source is `description` or `plan`. Section is the ATX heading
path joined with ` / ` (including heading markers), or `-` for descriptions
and unheaded text. Line normalization trims and collapses whitespace;
other raw bytes remain. The 1-based occurrence index `k` distinguishes
identical `(kind, source, section, hash)` tuples, including multiple tokens
on one line. Failure records contain no free-form errors or timestamps.
`FailureRecordDigests` is the shared writer/reader fingerprint function;
it hashes records before rendering. An MF-3 `indeterminate` record cannot
be covered by an override. The constructors expose the shared MF-3
grammar for the dependency evaluator.

The gate-before-mutate wiring lives in `internal/next/ready_gate.go`:

```go
package next

// GateReadiness checks plan-bead form, lineage/scope agreement and the
// lineage spec branch before any mutation. Scoped failures require fresh
// evidence; indeterminate reads refuse. GateResult.SpecID is the dispatch spec.
func GateReadiness(root, beadID string, opts GateOptions) (*GateResult, error)

// RecordReadinessOverride replaces the schema-v2 marker before ClaimBead,
// fail-closed: signals, complete sorted failure-record digests, UTC timestamp,
// evidence path/sha256/bytes (never text).
func RecordReadinessOverride(beadID string, result *GateResult) error
```

### Panel Artifact Schema

`internal/panel` owns the panel artifact shapes. Every new round uses verdict schema v2.
A reviewer submits through `mindspec panel submit`. A file alone does not count.
Process reviewers use the output channel in their slot prompt. Launch submits their verdicts.
The latest submission replaces the earlier submission for that slot and round.
The ledger retains every submission. See Panel Submission Core and Panel Gate v2 below.

`mindspec panel verify` reports one status for each roster slot:
`final`, `damaged`, `void`, `draft`, `running`, `quota_wall`, `launch_failed`, `malformed`, `unsubmitted` or `missing`.
It exits 0 when all slots are final.
It exits 1 for running or missing slots, harness drafts, or a quota handoff that is pending.
It exits 2 in every other case. A final status does not imply a passing gate.

**Tool artifacts and ledger commits.**
`panel.ToolArtifactPaths` is the single home for `.beads/issues.jsonl` and
`<specs>/*/reviews/*/dispositions.jsonl`, matched with stdlib `path.Match`.
The gate, next guard and complete materialization share it. The ledger is
artifact dirt; other panel files retain their ordinary classification. The gate,
next guard and impl approve share `panel.PorcelainPath` to decode git porcelain
C-style quoted paths and rename destinations before matching, including
non-ASCII spec and panel directory names and literal arrows in path names.
The next guard matches decoded filename bytes exactly, including whitespace.
Its proportional claim check exempts a rename or copy only when both endpoints
match artifact paths. It names both endpoints of user renames and copies.
Normal and merge-only completion read the artifact-sync snapshot delta with
rename detection off and NUL separators. Filename bytes stay exact.
On flat layouts complete checks each existing matched ledger with
`exec.CheckIgnore` in the spec worktree before any side effect. After the
bead merge, it commits all matched ledgers once with `exec.CommitPaths`
and subject `chore(panel): ledger`, in that same worktree. A legacy panel
without a ledger causes no commit or message. If the ledger commit fails,
complete prints its error and recovery once on stderr, clears the terminal marker,
writes override and panel audit metadata, and attempts to advance state. It syncs
the phase only when the next mode can be derived; otherwise it reports that the
next mode is indeterminate. It returns the ledger failure even if state advance
or phase sync also fails. The CLI prints the result summary and cd-back note
before exiting non-zero. The recovery states that the bead is
closed and merged and only the ledger commit remains. Impl approve first checks
ignore status, then refuses any existing spec ledger with uncommitted or
untracked content. The ignored-path recovery runs `mindspec init` in the
spec worktree; the uncommitted-path and commit-failure recoveries name
`git -C <spec-worktree> add <path>` and the ledger commit. Neither panel
hatch skips these checks or the ledger commit. Adhoc ledgers stay ignored.

**Registration.** Every panel directory (`review/<slug>`, or the
co-located `<spec-dir>/reviews/<slug>` on a flat tree) holds
exactly one `panel.json` — the literal filename `panel.FileName`. The
snapshot writer `panel.WriteSnapshot` exists (temp file and rename); `create`, `rebind`, `refute`, `defer` and `abandon` call it after their ledger events are appended.
`panel.AppendRecords` accepts the
exact panel directory in flat, non-flat and adhoc layouts, validates the full
batch and checks keys under one `dispositions.lock` hold before appending.
`panel.AppendRecord` retains the legacy spec-directory/panel-name adapter.
`panel.AppendRecordsWithCheck` runs a pre-append check under that same lock;
the disposition CLI uses it to check legacy markers before appending a manifest.

The optional registration fields at the end of `panel.Panel` are:

| Field | Shape |
|:------|:------|
| `subject_digest` | `sha256:` plus the subject-tree digest, recorded beside `reviewed_head_sha` |
| `roster` | ordered `[{slot, family, model, lens}]` on new rounds, with ids `R1..Rn` |
| `verdict_schema` | `2` on every new round; pinned to the round event |
| `deferrals` | `[{slot, round, finding, finding_sha256, finding_text, bead, by, created_at}]` |
| `rebinds` | `[{round, from_sha, to_sha, paths, reason, created_at}]` |
| `telemetry` | `"required"` on new registrations |
| `abandon_history` | `[{reason, abandoned_at, revived_at}]` |

Refutations have `slot`, `round`, `reason`, `evidence` and optional
`finding`, `finding_sha256`, `finding_text`, `ground`, `ac`, `by`, `created_at`.
Absent new fields are omitted on serialization, preserving legacy JSON shapes.

The panel directory's `dispositions.jsonl` stores these verb events in addition
to `disposition` rows and `panel` coverage manifests. Every event carries
`spec`, `panel` and `created_at`; adhoc events use `spec: "adhoc"`.

| Kind | Payload | Key within the panel directory |
|:-----|:--------|:-------------------------------|
| `round` | round, verdict_schema, reviewed_head_sha, subject_digest, target, gate, bead_id, roster, expected_reviewers, telemetry, carried | round, created_at |
| `refutation` | slot, round, finding, finding_sha256, finding_text, ground, ac?, reason, evidence, by | slot, round, finding, finding_sha256 |
| `deferral` | slot, round, finding, finding_sha256, finding_text, bead, by, source_verdict?, source_file_sha256? | slot, round, finding, finding_sha256; plus source_verdict, source_file_sha256 when bound |
| `rebind` | round, from_sha, to_sha, subject_digest, paths, reason | to_sha |
| `abandon` | reason, by | created_at |

A same-key retry differing only in `created_at` adds nothing; another payload
difference refuses the entire batch. For `round` and `abandon`, a different
timestamp is a different key. Coverage manifest keys are
`spec, panel, round, created_at` (timestamp absent on legacy manifests).
`panel disposition validate`, `check` and `query` accept all event kinds and
reject malformed records. A check that finds no coverage manifest (before the first tally) says to
run `mindspec panel tally <panel>` first. The completeness check is ledger-only: it reads only `dispositions.jsonl`. Source currency is checked by the panel gate, `complete`, and `impl approve`, which already read verdict files. `panel disposition check` takes the panel as an
optional positional (`check <panel>`, same as `--panel`; a conflicting pair
refuses) and infers `--spec` from that panel's registration when omitted.
`panel disposition append` completes a finding-only disposition row: it fills
`record`, `spec` and `gate` (panel registration; spec falls back to `--spec`),
`panel`, `created_at`, `convergent_with` and `backfilled` when absent, and a
rejected row prints the full required shape with an example.
`panel create` recovery lines fill in the spec and bead target the caller
already gave, shell-quoted, instead of `<id>` placeholders. Events contribute no telemetry rows or slot coverage.
`panel disposition append` refuses the verb kinds, and manifests on non-legacy
panels; only `tally` writes those manifests.

A legacy panel has neither a `telemetry` field nor a `round` ledger event.
Exactly one marker is a registration error. `panel.CheckHandEdits` is the snapshot-to-ledger
matcher: it returns the first snapshot field without a backing event,
using the latest round, subsequent rebind/abandon events, and payload equality
for entry arrays (ignoring `created_at`). The first round event's `carried`
object supplies baselines for legacy refutations, deferrals and rebinds.

`panel.Subject(gate, specDir)` derives roots: spec/plan approval reviews only
`<specs>/<spec>/`; bead, final review, adhoc and an empty gate review the whole
tree. `<specs>` is the workspace-resolved specs directory in either layout.
Every subject excludes exactly `<specs>/*/reviews/**`, `.mindspec/reviews/**`,
`review/**` and `.beads/issues.jsonl`; other `reviews` directories count.
The digest hashes the retained recursive `ls-tree` listing; read errors and an
empty filtered listing refuse creation and Block a mismatched-SHA gate.
Equal SHAs are fresh without a subject read. On a SHA mismatch a recorded
digest is compared to the live subject: equality is fresh, a difference stale.
A legacy panel without a digest retains the original stale message.
`complete`, `verify`, `tally` and instruct panel state use the same gate facts.
The snapshot-to-ledger matcher pins `subject_digest` to the later of the latest round and
rebind events, alongside `reviewed_head_sha`.

On historical file-based rounds, a Finding is `ccr[i]` from the latest-round verdict's
`concrete_changes_required`, with the sha256 of its raw text. Empty lists use
`ccr[0]` with the rationale; an empty rationale too is INVALID. Reviewer
`findings[]` is evidence only. Storage text replaces each `/tmp/` or `/Users/`
path token with `<path>` after hashing. Slot-level discharges use `finding:
"slot"` and hash the compact JSON encoding of the entire ordered list, or the
raw rationale for an empty list.

The gate checks snapshot/event equality before abandonment. An unbacked field
blocks with `panel.json changed outside the panel verbs`; legacy panels with
neither marker remain exempt. The check detects accidental desync, not forgery.

`panel.Create` reads and parses the existing `panel.json` first and merges forward (see the Create paragraph below). A re-panel of an abandoned panel revives it, clears `abandoned`, and appends `{reason, abandoned_at, revived_at}` to `abandon_history` (`TestCreate_RepanelOfAbandonedPanelRevivesIt`).

**Verdict files.** Submit writes `<slot>-round-<N>.json` beside the registration.
The first round is 1. `R1-round-1.json` is a valid filename.
The filename `R1-round-0.json` (nonconforming — rejected) is refused.
On v2 rounds, the gate checks the file hash against the latest submission.
Reviewers must not write this destination by hand.

**Consolidated change list.** `mindspec panel tally`'s aggregated
`concrete_changes_required`, once deduped and ranked, is authored to
`consolidated-round-1.md` for round 1 (the general shape is
consolidated-round-N.md, `panel.ConsolidatedName(N)`).

**Historical verdict JSON payload.** Rounds without a v2 round event use this reader. The top-level shape is:

- `verdict` (required): a non-empty string. A reviewer MUST write one
  of `APPROVE`, `REQUEST_CHANGES`, `REJECT` — the three values the gate
  acts on. A parseable file with any other verdict value is INVALID and blocks.
  This includes an empty, whitespace-only or missing verdict.
  On a v2 round, an unreadable verdict is INVALID in tally. Verify gives `damaged` only when the latest submission is final, is not VOID, and is closed. Otherwise the latest submission or attempt sets the status. With no such event, verify gives `unsubmitted`.
  On a pre-v2 round, an unreadable or unparseable file is malformed.
  It counts as missing on the roster and counts for nothing off the roster or on a legacy panel.
- `hard_block` (optional boolean): a top-level sibling of `verdict`.

`panel.Tally` parses verdict, hard_block, rationale and concrete_changes_required.
It checks the finding list's shape only for REQUEST_CHANGES. A list that is not
an array of strings makes REQUEST_CHANGES INVALID. It does not invalidate APPROVE
or REJECT.
The gate passes only when complete, without veto, fresh, discharged and intact.
Every roster slot must have a parseable latest-round verdict. Only roster slots
count toward completeness or APPROVE; unexpected files are named and count for
nothing, except REJECT, hard_block or INVALID blocks and names the file. Every
file still contributes to LatestRound. Legacy panels without a roster retain
count-based completeness. Registrations require at least two reviewers.

Every REQUEST_CHANGES finding must be discharged at the current round with an
unchanged `finding_sha256`. An item-level refutation clears one finding; a
slot-level refutation clears all until the list or fallback rationale changes.
Legacy unpinned refutations apply only to legacy panels. An effective deferral
discharges the finding it covers; the gate re-checks it. REJECT, hard_block,
INVALID, staleness and incompleteness
cannot be discharged. Zero APPROVE can pass: resolved slots are reported as
resolved, never approvals. `approve_threshold` remains parse-compatible and
prints an ignored legacy notice, with no decision effect. Tally, verify and
instruct render the same rule line, including INVALID and unresolved findings.
`hard_block` is read only at the top level. Reviewer `findings[]` is evidence,
not the blocking finding list.

Degraded modes (a slow reviewer, a runner that cannot render Markdown)
are the runner's concern; the schema itself makes no allowance for a
partial or alternate shape.

Coverage manifests are written only by binary `tally`, from roster slots that count in the latest round, when every pass-rule row except freshness holds. A stale non-bead panel can therefore record coverage. The append key is `spec, panel, round, created_at`; tally appends unless the last manifest of that round equals the derivation. Earlier manifests remain after abandonment. Every manifest reader, including `ComputeQ4`, uses only the last manifest per spec/panel. Core 3 validates all ledger records, requires the current round's manifest to equal the roster/verdict derivation, and requires covering disposition rows for REQUEST_CHANGES and REJECT slots.

### Panel Submission Core

`panel.VerdictV2` requires `schema: 2`, `slot`, `round`, `reviewed_sha`, `status`,
`verdict`, `model`, `rationale`, `concrete_changes_required` and `commands_run`.
Status is `draft` or `final`. Verdict is `APPROVE`, `REQUEST_CHANGES`, `REJECT` or `VOID`.
Change items have `text`, `basis` and an executed-only command index. Commands have `cmd`, `exit` and `tail`.
The parser rejects unknown fields, files over 1 MiB and command tails over 4096 UTF-8 bytes.
It requires change items only for REQUEST_CHANGES and `void_reason` only for VOID.
Other verdicts require a non-empty rationale. Finding projection uses item texts for `ccr[i]` hashes.

`panel.Submit` checks the panel, current v2 round, roster slot, effective model and verdict bytes.
It holds `dispositions.lock` through the checks, ledger append and verdict-file rename.
It refuses abandoned or landed rounds, target mismatches and direct submissions for process slots.
A new submission must match the reviewed head. Its sequence starts at 1 and increases per slot and round.
A retry with the latest submission's exact bytes repairs the file without another verdict record.
Direct attempt files must match the latest started or dispatched attempt. Older, unknown or failed attempts refuse.
An open dispatched attempt receives a `submitted` outcome after the verdict file is written.
The slot lock API is separate from this ledger transaction.

`panel.NewSlotLayout` uses `<main_root>/<isolation_root>/<spec-or-adhoc>/<slug>/r<N>/<slot>/`.
It supplies `slot.json`, `tree/` and `attempt-<k>/` paths. Lock paths use `.locks/<spec-or-adhoc>/<slug>/r<N>/<slot>.lock` under the isolation root.
It requires an absolute main root, a safe relative isolation root, safe identity parts and a positive round.
`AttemptFromPath` resolves directory symlinks and rejects attempt paths that cannot bind to the target slot.
`NextAttemptNumber` uses the selected round's slot events and that slot directory's attempt directories.

`TrySlotLock` does not wait. `AcquireSlotLock` retries for up to one second. Holders call `Release`.
Unix uses `flock` and retains the lock file. Windows uses exclusive creation and a PID file.
Windows stale-lock recovery renames and reads the file again before removal. Live PIDs or invalid lock contents keep the slot held.
`ProbeSlotLock` does not wait and does not create or remove a file.

The ledger validates `verdict`, `launch` and `substitution` records with `spec` and `panel` identities.
Their keys are `slot, round, seq`; `slot, round, attempt, status`; and `slot, round`, respectively.
`LatestSubmissions` selects the highest sequence per slot in one round. `SlotAttempts` returns that round's launch events in ledger order.
These readers take no lock. `SlotAttempts` marks events after the latest submission.

`mindspec panel schema` prints the v2 JSON Schema. Transport and relation checks remain in the parser.
`panel disposition append` refuses these three record kinds and names the owning verb in its recovery line.
Disposition validate, check and query accept these record kinds.
The CLI helper `findPanelRoot` returns the verified slot's main root or falls back to `findRoot` when no slot is found.
`landedBindingFor` checks the exact panel-round metadata value. It returns false for an empty bead identity and propagates read errors.

### Panel Gate v2 (verdict ledger)

Each v2 change item can include `artifact_path`, a repository-relative slash path.
The validator refuses absolute paths, parent segments and control bytes.
It names the field's JSON pointer. `panel schema` includes the field.
The gate checks each unresolved item's path in the round record's
`reviewed_head_sha` tree. A missing path adds the slot, item and path to
the refusal. A rebind does not change that tree.
A refutation or deferral discharges the item and its path block.
A new round checks its own tree. A present path does not discharge a finding.
An item without this field keeps the existing gate behavior.

`panel.Create` writes `verdict_schema: 2` in the snapshot and round record.
`RoundSchema` selects the schema from the latest round record.
`CheckHandEdits` requires the snapshot's pin to match that record.
A v2 round uses the submission ledger gate.
A historical round without the pin keeps the file-based gate rules.
A downgrade after a v2 round blocks the gate.
`Tally` judges only the latest verdict-file round.
A snapshot round that differs from the latest verdict-file round blocks the gate.
The snapshot round must also match the latest round record.

`panel.SubmissionRecord` stores `record: "verdict"` in the ledger.
It records `spec`, `panel`, `slot`, `round` and `seq` as submission identity.
It records `status`, `verdict`, `family` and `model`.
It records `runner_kind`, `runner_source`, `via`, `attempt` and `reason`.
It records `file_sha256` and `created_at`.
The optional boolean `hard_block` records a veto.
The writer omits `hard_block` when false.

The highest `seq` replaces the earlier submission for that slot and round.
The ledger retains the earlier records.
The v2 gate counts a roster file only when its hash matches that submission.
The submission must be final and must not have verdict VOID.
If a later attempt exists, it must end as `submitted` with the same submission sequence.
A finding refutation matches the slot, round, finding ID and raw-text hash.
It remains applicable to a replacement REQUEST_CHANGES with that same finding.
A hard block or invalid verdict prevents the refutation from applying.

`mindspec panel submit [slug] --file <path|->` accepts a file or standard input.
Outside a slot directory, it requires a slug, `--slot` and `--round`.
`--spec` selects the owning spec.
The command infers the target from the working directory or an absolute file path.
Slot discovery checks `slot.json`, `tree/` and the configured isolation path.
The command refuses different working-directory and file-path slots.
It refuses explicit targets that differ from the inferred slot identity.
It refuses slot directories that differ from the current panel round or spec.
It resolves the effective runner and model under the submission lock.
It refuses direct submissions for process slots and rounds that have landed.
It prints the slot status and rule line after submission.
It exits 0 on success and 1 on a returned error.

`panel.CheckCreate` checks the roster before prior-round scratch cleanup.
It refuses a carried slot without a family if its model has no matching runner.
It refuses an empty model or a model with whitespace or control characters.
It also refuses a model that starts with a dash.
These refusals preserve the prior round's scratch paths.

In text mode, `panel verify` prints a rule preview and BLOCK for an unreadable registration.
It exits 2 when it cannot read slot status.
`panel tally` prints the BLOCK decision for an unreadable registration.
It exits 1 on that decision's guard error.
`panel verify --json` prints slot records without a preview when status reading succeeds.
If status reading fails, it prints the text preview and exits 2 even with `--json`.
`panel tally` has no `--json` flag.

`paneltest.CreateV1` seeds a historical v1 round for tests.
It writes a round event without `verdict_schema` and a panel snapshot.

### Panel Checkout (`cmd/mindspec/panel_checkout.go`)

`mindspec panel checkout <slug> --slot S` creates or reuses a reviewer clone at the reviewed SHA.
`--all` selects every roster slot. `--spec` selects the owning spec when a slug is ambiguous.
`panel.NewSlotLayout` places each slot at `<main_root>/<isolation_root>/<spec-or-adhoc>/<slug>/r<N>/<slot>/`.
`slot.json` records the spec, slug, round, slot and main root. `tree/` holds the clone.
Locks use `<isolation_root>/.locks/<spec-or-adhoc>/<slug>/r<N>/<slot>.lock`.
The layout accepts one trailing slash on the isolation root and refuses unsafe path components.
`--fixer` creates only the fixer scratch directory, without metadata or a tree.
Plain checkout writes no ledger record, including after a final submission.

Checkout refuses paths outside the main root, symlink components, and an isolation root that is not ignored.
It refuses abandoned panels, registration edits without ledger records, slots outside the roster, and changed rounds or reviewed SHAs while it waits for a lock.
It refuses mismatched slot metadata and clones that fail `CheckSlotTree`.
Refusals include a `recovery:` line. Root ignore failures name `mindspec init`.
Metadata or clone failures name `mindspec panel checkout <slug> --prune --slot S`.
`--dispatch` requires `--slot` and a harness runner. Process slots name `mindspec panel launch` for recovery.
It refuses a final non-VOID submission without `--replace` and refuses a landed round.
It reserves `attempt-<k>/`, marks earlier open dispatched attempts `launch_failed`, and appends a `dispatched` event.
It checks the round and ledger again under the ledger lock before it appends.

`--prune` removes slot scratch, including dirty files. `--slot` limits removal to that slot across rounds.
Cleanup takes all affected slot locks in sorted order before the ledger lock and retains the slot locks through the caller's writes.
A held slot lock prevents removal and names `panel verify` plus the lock paths. Unix lock files remain.
`panel create --round N` removes scratch for earlier rounds before it writes the new round.
`panel abandon` removes scratch for all rounds before it records abandonment.
Panel registration lookup excludes the isolation root through `panel.ScanSkipping`.
Doctor's ownership walk and moved-tree link walk skip the configured isolation root and return isolation config errors.
The source Markdown and canonical-doc walks in `migrate` also skip that root.
Before config exists, these migration walks use the default isolation root.

### Panel BRIEF Regions

`mindspec panel create` renders managed acceptance, quoted criteria, conduct and verdict-contract regions.
The acceptance region uses the gate's configured question and `not_blocking` list.
The quoted region uses the spec's `## Acceptance Criteria` section at the reviewed commit.
It keeps printable text and line breaks, and escapes control characters and invalid UTF-8 bytes.
Its backtick fence is longer than each backtick run in the quotation.
Paths in the quotation are not checked.
An absent section prints `no acceptance criteria section found`; an ad-hoc panel prints `none (adhoc panel)`.
Conduct uses reviewer rules for the gate. The verdict contract names the schema and submission commands.

The prior-round region reads `consolidated-round-<N-1>.md`, or prints `none recorded` if absent.
It applies `termsafe.Escape` before insertion.
It adds `> ` if the trimmed result starts with three backticks or three tildes.
It also adds `> ` for a panel-header marker lookalike or an exact prior-round opening or closing marker.

The path check applies to authored guidance, including acceptance and conduct text.
A path is a code span with no whitespace and either `/` or a suffix that matches `\.[a-z][a-z0-9]*$`.
Opening and closing backtick runs must have the same length.
An unmatched opening run is literal text; the scanner still checks later runs.
Bare text and code spans that start with `mindspec ` are not paths. The check skips `panel.json`.
Create refuses a path that ends in `.md`, `.rst` or `.adoc`, or contains `/domains/`.
It refuses any other checked path absent at the reviewed commit, and reports path-read errors.
It also refuses internal references in authored guidance.

The first-create skeleton names no documentation path or internal record.
Its acceptance text says: "Use the quoted acceptance criteria in the managed region above."

### Panel Slot Prompt

`mindspec panel prompt <slug> --slot S [--spec <id>]` prints a prompt and writes no files.
It prints the BRIEF, slot id, effective family and model, lens, focus, slot directory and tree.
Current-round substitution records override the roster's family and model.
For a process runner, it selects the next attempt and creates a fresh nonce for the printed prompt.
For file output, it names that attempt's `verdict.json` path.
For stdout output, it names the nonce-bearing `MINDSPEC-VERDICT-BEGIN` and `MINDSPEC-VERDICT-END` lines.
For a harness runner, it prints `mindspec panel submit --file` for the latest dispatched attempt.
It encloses the verdict path in single quotes and replaces each embedded single quote with `'"'"'`.
For a non-printable character or invalid UTF-8 in that path, it prints this line instead of the command:

> Cannot render the submission command: the verdict path contains a non-printable character or invalid UTF-8.

With no dispatched attempt, it asks for `panel checkout` with `--slot` and `--dispatch`, then another prompt call.
From round 2, it prints only that slot's prior-round items. For v2, the file must match the latest submission digest.
It prints a warning if v2 evidence is unavailable; legacy rounds use the parsed verdict file.
It asks for ADDRESSED, PARTIAL, MISSED or NEW_ISSUE grades when items exist.
The scratch instruction limits writes to absolute paths in the slot directory.

### Panel CLI Verb Tree (Spec 110 Bead 4, ADR-0040 portability contract)

**`mindspec panel rebind <slug> --reason <text> [--spec <id>]`** refreshes
`reviewed_head_sha` and `subject_digest` to the registered target's live tip
without bumping the round. A non-empty reason records the operator's judgement.
The panel must parse, be intact, not abandoned, have matching rounds, and pass
the normal vote rule with staleness ignored. The reviewed SHA must be an
ancestor of the live tip. The diff disables rename detection. It checks and
records both the source and destination paths of a rename. Every diff path is
accepted except `.mindspec/config.yaml`. There is no path allowlist or path
configuration.
The entry records `{round, from_sha, to_sha, paths, reason, created_at}`,
with subject exclusions removed from `paths`. The `rebind` event also records
the new subject digest. The verb appends the event before one snapshot write.
An equal SHA prints `already fresh` and writes nothing. For a `spec_approve`
or `plan_approve` panel pinned to a commit, the live subject tracks its spec
branch. A hex-looking name is read as a commit, not as a branch or tag name.
Resolution uses exactly `refs/heads/<spec branch>` so a tag cannot shadow it.
Only confirmed branch absence permits fallback to the recorded target. The
exact lookup uses `git show-ref --exists`: exit 0 means present, exit 2 confirms
absence, and every other exit blocks. Git 2.43 or later is needed for a spec or
plan panel whose target is a commit. Older Git (exit 129) blocks and gives the
version requirement. Upgrade Git, or create the panel with the spec branch name
as its target. A ref that exists but has an unreadable
commit (including a dangling symbolic ref or a tree/blob tip) also blocks.
Ordinary branch or tag targets and bead panels
keep their existing target behavior. `verify`, `tally`, `rebind` and instruct
panel state share this rule, so an edit to the spec or plan after a pinned
review reads stale and a rebind records the real branch tip. Completion copies
entries into `panel_rebound_entries` through the fail-closed metadata stage
before close; only ledger-matched `from_sha` values join the landed reviewed-SHA
set, with exact equality. This verb shares slug validation and supports `--spec`
for ambiguous slugs.

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

**`mindspec panel refute <slug> --slot S --round N [--finding ccr[i]] --ground G [--ac AC-n] --reason T --evidence REF --by WHO`** validates a latest-round REQUEST_CHANGES finding without hard_block. APPROVE, REJECT, INVALID, absent slots, stale rounds, unknown findings and empty names refuse without writes. Grounds are `disproven`, `not-load-bearing` and `out_of_acceptance_scope`; the last requires an AC id in the owning spec. Evidence resolves to a commit, repo path or HTTPS URL. One batch lock hold appends the refutation event and an `audited-refuted` disposition row; an atomic rename updates the snapshot afterward. Retries converge by key. Finding text is path-redacted while hashes pin the raw text.

**`mindspec panel defer <slug> --slot S --round N [--finding ccr[i]] --bead B --by WHO`** appends a deferral event and a `confirmed-deferred` row in one locked batch. It then replaces the snapshot. Only the panel verbs write discharge records. Spec approval and adhoc panels cannot defer findings. Defer accepts a finding on a REQUEST_CHANGES verdict or an item in a schema-v2 APPROVE verdict's `findings`, addressed as `ccr[i]`. Legacy (v1) APPROVE items come from `concrete_changes_required`, with the same ids and hashes. Defer refuses REJECT, INVALID and `hard_block` verdicts. The refusal names defer. A deferral from an APPROVE slot writes severity `minor` (REQUEST_CHANGES writes `blocking`). Each new deferral records the source verdict kind and file hash. Both the event key and its coverage row ID include that source submission identity, so a resubmitted finding can be deferred again and exact retries remain idempotent. Unbound legacy events and rows retain their original keys. The gate and completion count only effective REQUEST_CHANGES deferrals. A new deferral is effective only while its source submission is current. An APPROVE-slot deferral is an audit record only, even if the slot later submits REQUEST_CHANGES with the same text. Legacy deferrals without a source binding count only for a current REQUEST_CHANGES verdict. When the follow-up bead misses the `discovered-from` edge and/or the finding text, one refusal lists every miss: the exact `bd dep add <follow-up> <epic> --type discovered-from` command and the finding text to add to the description, with the edge command as the recovery line.

An effective deferral applies to the latest round and unchanged finding text. A covering refutation supersedes it. Rebind uses the same resolution rule as the gate. Completion re-checks every effective deferral before materialization. It persists a pending marker from Allow decisions and settles the durable `panel_deferred` audit before close. Each entry holds panel, spec, slot, round, finding, finding text, finding hash, follow-up bead and author. The audit also stores effective finding text boundaries for slot-level entries.

Implementation approval checks registration integrity before it skips abandoned panels. It reads non-bead deferrals from the snapshot and ledger. It reads bead-panel deferrals from the union of settled and pending deferral entries in the completion audit. A fresh `complete` attempt replaces the entries of the panels it gated. A later panel refutation does not alter that audit. Approval checks each identified audit entry once. Completion and approval refuse a panel identity registered at more than one path. Approval skips an entry only when its exact spec and panel registration has a verb-backed abandonment. A missing or renamed registration does not skip the entry. Approval checks entries with no panel identity under each of the bead's registrations. It also checks a plan bead with no registration left. Both lifecycle checks require an existing open follow-up outside the owning epic, a `discovered-from` edge to the epic or reviewed bead, and each path-redacted finding in the path-redacted description. A failed read or changed fact voids that deferral; the finding blocks unless another discharge covers it. Both panel hatches skip these checks.

**`mindspec panel abandon <slug> --reason T --by WHO`** requires both fields, appends an `abandon` event and sets `abandoned` and `abandon_reason`. It writes no manifest and retains existing manifests. The discharge and abandon verbs print the shared pass-rule line and accept `--spec` to disambiguate a slug.

**`mindspec panel create <slug> --spec <id> --target <ref> [--bead
<id>] [--round N] [--gate <name>] [--reviewers N]`** resolves the target's
live commit and subject digest and the gate's roster through
`config.PanelGateReviewerSlots`. It appends a `round` ledger event before
rewriting `panel.json`, then rewrites the BRIEF machine-managed header.
Both snapshot and event contain `roster: [{slot, family, model, lens}]`,
`expected_reviewers: len(roster)` and `telemetry: "required"`. New writes
omit `approve_threshold`. The BRIEF lists each roster entry as `slot model lens`.

Gate defaults are 6 slots for `spec_approve`, `plan_approve`, `final_review`
and `adhoc`, and 4 for `bead`. An explicit reviewer list retains its configured
size. `--reviewers N` requires N >= 2 and distributes slots round-robin over
the resolved reviewer entries in list order, ignoring their configured counts.
Without an explicit list, slots follow the merged runner order.
The zero-config default uses the `harness` family for every slot. Slots use ids
`R1..Rn`. A re-panel copies the latest round event's roster unless
`--reviewers` is supplied (legacy panels copy a recorded roster if present).
Without `--gate`, create uses `adhoc` roster defaults, omits the snapshot
`gate` key and writes an empty `gate` in the matching round event.

The panel directory is `<spec-dir>/reviews/<slug>` on flat layouts,
otherwise `review/<slug>`. Explicit `--gate adhoc` requires `--spec` to be
omitted and uses `.mindspec/reviews/<slug>` on flat layouts. For `--gate bead`,
`--target` must be `bead/<id>`; `bead_id` is derived and validated with
`idvalidate.BeadID`. An explicit `--bead` must match and is refused on other
gates. `--gate` accepts the closed enum `config.PanelGateKeys`, with control
bytes refused before membership checks. Existing panel.json must parse, and
verdict filenames forbid recreating their round or an older round. These
refusals write nothing.

For existing bead panels with a divergent recorded target, the gate compares that target to the live `bead/<id>` branch. On a SHA mismatch, a changed subject tree blocks; an equal subject stays fresh. A legacy panel without a digest blocks on a SHA mismatch.

Create preserves each refutation, deferral and rebind that matches its ledger
event or the first round event's carried baseline, ignoring only `created_at`.
It names each unbacked entry it drops. Legacy panels retain every entry and
record them in the first round event's `carried` field. A legacy entry with a
`/tmp/` or `/Users/` token refuses before any write. The recovery names the entry.
Remove the absolute path from that entry in the legacy `panel.json`.
Run `create` again. Create reports dropped entries after the snapshot write,
before the BRIEF write. Revival clears
abandonment and appends `{reason, abandoned_at, revived_at}` to
`abandon_history`; legacy panels without an abandonment event have an empty
`abandoned_at`. A round bump archives the old BRIEF as `BRIEF-round-N.md`
without overwriting an existing archive. The managed `mindspec:prior-round`
region renders the preceding round's `consolidated-round-N.md` through
`termsafe.Escape`, or `none recorded`. It quotes marker and fence lines so
later header updates can read the region. Verdicts and the authored BRIEF body
are preserved.

On success, stdout carries `panel <slug> registered: round N, K expected
reviewer(s), reviewed_head_sha <sha>` and `panel directory: <dir>`.
Dropped-entry reports precede these lines. The reported K reflects the
written roster, including a carried roster after a config change.

**Reviewer-count advisory (spec 130 R2.1).** `config show` and
`mindspec complete` compare recorded `expected_reviewers` against
`cfg.PanelGateAdvisoryDefault(recordedGate, isBead)` when it returns
`ok == true`. A known recorded gate selects that gate's resolved count.
Thus a CLI-created `final_review` panel selects the `final_review` count.
A bead panel with no recorded gate selects the `bead` count.
A non-bead panel with no recorded gate and no configured gate overrides
selects the `adhoc` count through `cfg.PanelExpectedReviewers()`.
A non-bead panel with no recorded gate and configured gate overrides
skips the note. An unknown non-empty recorded gate always skips the note.
These rules apply when `gates:` is absent too. The advisory does not
change the gate decision.

**`mindspec panel verify <slug> [--spec <id>]`** is READ-ONLY: it writes no file and
exits `0` after resolving the registration — it is a report, never a gate. It prints verdicts
present vs `expected_reviewers`, per-slot parse status (malformed files
named), `reviewed_head_sha` vs the live target tip, and a PASS/BLOCK
preview computed by `panel.PanelGateDecision` over facts gathered by
`panel.ResolveGateFacts` — the IDENTICAL decision `mindspec complete`'s
gate enforces.

**`mindspec panel tally <slug> [--spec <id>]`** is ADVISORY-BUT-BLOCK-CAPABLE: its
exit code tracks the `panel.PanelGateDecision` decision alone, and is
non-zero on `Block`. Do not describe `verify`/`tally` as both
"read-only/advisory" — `verify` never blocks; `tally` can. `tally`
prints the per-slot verdict table (slot, verdict, `hard_block`), the
malformed-file list, the shared rule line (`panel.RuleLine`), the legacy
`approve_threshold` notice when the field is recorded, any unexpected slots, the `panel.PanelGateDecision` decision, and the aggregated
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
staleness and the incomplete/REJECT/unresolved-finding legs for non-bead panels;
a `sanitizeNonBeadDecision` CLI-layer rewrite (never touching
`Decision.Action`) replaces the empty-bead-interpolation fragments and
the empty `git merge bead/` fence with target-naming text, and never
emits a `mindspec complete <bead>` recovery line for a panel that is not
complete-gated.

**Create integrity (spec 130 R1).** An existing `panel.json` must parse.
`create --round N` refuses when N is at or below the filename-derived
LatestRound of verdict files, even when their contents are malformed.
The refusal leaves every file unchanged and names `--round <LatestRound+1>`
and `mindspec panel rebind`. A round with no verdict files may be recreated.

**Registration resolution.** Lookup collects every matching slug. `--spec`
selects the owning spec for `tally` and `verify`. Among copies belonging to
one spec, a single copy under the current checkout wins over copies under
`.worktrees/`. Any remaining ambiguity refuses and lists every candidate
spec and directory. Both commands print `panel directory: <dir>` and
`resolved round: N` before their report. `create` refuses a slug owned by
another spec. Adhoc creation refuses a slug registered at another destination;
spec creation refuses an adhoc slug. An adhoc registration at the create
destination may be re-created under the round rules; canonical paths identify
that destination across symlinks.

**Recovery and usage.** Non-bead tally recovery uses the recorded spec id
when known. Every adhoc create recovery omits `--spec`, including the
missing-ref advisory. A `--bead` refusal on an explicit non-bead gate recovers with that gate
and no `--bead`. Every panel subcommand, including disposition leaves, sets
`SilenceUsage: true`, so an error prints no Cobra usage text.

### Lifecycle verb state honesty (spec 128)

**Draft/discovery honesty (`internal/resolve`, R1):**

| Symbol | Purpose |
|:-------|:--------|
| `IsDraftSpecWithCache(c, root, specID)` | Reports whether specID is an on-disk pre-epic draft (OQ-2) — shared by `ResolveModeWithCache`, `ResolveSpecPrefixProven`, and `internal/next`'s R6.7 scope refusal. |
| `ResolveSpecPrefixProven(root, specID)` | `ResolveSpecPrefix`'s root-aware sibling: the full-hyphenated-ID passthrough branch now PROVES the ID names an epic-bearing or legitimate-draft spec (never a syntactically-valid-but-nonexistent one), consulting `IsDraftSpecWithCache` and `gitutil.FileExistsAtRef`. |

**Indeterminate-mode rendering (`internal/instruct`, R2.3):**

| Symbol | Purpose |
|:-------|:--------|
| `BuildIndeterminateContext(WithCache)(root, reason)` | Builds a rendering context with `Mode = state.ModeIndeterminate` and `IndeterminateReason = reason`. |
| `RenderIndeterminate(root, reason)` | Convenience wrapper external callers (`emitInstruct`, `completeTail`) use to render `templates/indeterminate.md` directly instead of falling back to `idle.md` for an undiscovered failure. |

**Gate-before-mutate reorder / prospective-snapshot content gates (`internal/complete`, `internal/validate`, R4):**

| Symbol | Purpose |
|:-------|:--------|
| `complete.runMaterialization` (`materialize.go`) | Orchestrates the M/R/S/G/T staged sequence over the execution domain's eight `Executor` seam methods — no `internal/gitutil` import (ADR-0030). |
| `validate.ValidateDocsSnapshot(root, base, treeOID, ownerRef, wtPath, exec)` | The doc-sync gate diffs `base..T` in `wtPath` and reads ownership at the separate landing-tree `ownerRef`; `ValidateDocsRange` still serves reconcile mode and `impl approve`'s whole-branch backstop. Both readers list both rename endpoints and preserve exact filename bytes. |
| `validate.CheckADRDivergenceSnapshot` / `ValidateDivergenceSnapshot` | The ADR-divergence gate diffs candidate `treeOID` in `wtPath`. Both functions take a separate `ownerRef` for all ownership reads at the landing tree. |

**Proportional claim guard (`internal/next/guard.go`, R5):**

| Symbol | Purpose |
|:-------|:--------|
| `ComputeEntangleablePaths(repoRoot, cwd, specID)` | Derives the worktree-creating claim path's entanglement set as two CONTAINMENT-gated conditional classes — an entry-absent `.gitignore` at the claim anchor, and (when recording is enabled) the target spec's `events.ndjson`/`manifest.json` — never unconditionally empty; dirt on either class still blocks. |
| `EntangleableClaimPaths()` / `SetEntangleableClaimPathsForTest` | Test-injectable residual unioned on top of the two derived classes, keeping the block machinery exercised (AC-8(iii)) independent of which conditional classes are live in a given fixture. |
| `CheckDirtyTreeProportional(repoRoot, cwd, specID) (blockingDirt, ignoredDirt []string, err error)` | Narrows the dirty-tree guard to the derived rule for the claim path only; every other caller (`spec approve`, `release`) is unaffected. |

**Deterministic, honest bead selection (`internal/next`, R6):**

| Symbol | Purpose |
|:-------|:--------|
| `ReadySet{Filtered, Raw, SpecFlag, EpicID, NearMisses, PlanNotice, Cache}` + `ExcludedCount()`/`ExclusionNotice()` (`beads.go`) | One construction shared by claims and non-positional emit-only. `Filtered` preserves the selectable bd order; `Raw` retains global ready rows for positional resolution. `SpecFlag`/`EpicID` identify the scope, `NearMisses` explains exclusions, `PlanNotice` discloses source divergence, and the per-invocation `Cache` shares phase reads. Positional emit-only fetches by ID without constructing a set and prints `emit-only: bead fetched by ID; readiness not checked.` |
| `NearMiss{Bead, Class, Message, Recoveries, Blocking, Lineage, NoLineage}` (`nearmiss.go`) | A linked non-member with its N1–N4 classification, true reason and recovery. `String()` renders the diagnostic; lineage fields govern positional N4 handling. |
| `EmptyAfterFilterError` (`beads.go`) | Typed blocking emptiness: the returned set is still valid for printing derived lifecycle state before refusing. |
| `ReadySet.HasBlockingNearMisses()` (`beads.go`) | Reports whether the near-misses make an empty scoped result non-zero; informational near-misses alone do not. |
| `BuildReadySet(root, specFlag string) (ReadySet, error)` | Builds a `ReadySet` — global `bd ready` when `specFlag == ""`, filtered by the plan-bead predicate otherwise — and REFUSES (never silently widens to global) when an explicit `specFlag`'s epic can't be resolved (AC-15). |
| `SelectWork(items, pick)` (`select.go`) | Total pick semantics: `pick >= 1` always indexes the displayed list, even with exactly one item; `0` means unset; negative errors. |
| `SelectNamedFromReadySet(rs, name)` | Resolves scoped selectors before empty-set handling: near-misses name their reason, unlinked ready rows name their parent, no-lineage N4 refuses before readiness, and lineage-E N4 reaches MF-1. Empty positional and pick refusals end with `recovery: mindspec next --spec <S>`. |
| `RenderBDError(err)` (`beads.go`) | Wraps a bd/lock-derived error so callers render it via `%v`/`%s` with every line escaped through `termsafe.Escape` (ADR-0042) — the rendering half of AC-11 (the execution domain's `wrapBDError` supplies the content half). |

**Total verb classification (`cmd/mindspec`, R2.1/AC-14):**

| Symbol | Purpose |
|:-------|:--------|
| `newExecutorFn` (`root.go`) | Package-level `Executor`-constructing seam every verb's `RunE` composes with, letting `verb_output_test.go` drive real `RunE` functions against `executor.MockExecutor`. |
| `mutatingVerbRows` / `mutatingVerbAliases` / `nonMutatingAllowlist` (`verb_output_test.go`) | The total-classification tables the cobra-tree walk checks every runnable command against — an unclassified new command, or one misclassified as mutating without a row, is red. |

Core 3 runs in `complete` before materialization for every required matched registration whose returned panel gate decision is Allow. Warn registrations (abandoned, missing ref, git error) skip coverage with one advisory line; legacy registrations pass with one advisory. `impl approve` checks coverage for every required spec registration. Its final-review gate also runs the full gate decision. It checks legacy markers and snapshot/ledger integrity before skipping abandoned or legacy registrations; a hand-set abandoned flag blocks. Directories without `panel.json` are not registrations. Both Core 3 legs honour `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false`; the orphan/obligation refusal retains its no-hatch rule.

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
| `mindspec next --allow-not-ready=<ids> --evidence <path>` | Proceed only when every failing signal is named and determinate, with fresh evidence — warns naming each failure and replaces the schema-v2 failure-digest `mindspec_readiness_override` marker BEFORE claiming, fail-closed (spec 124 R3/AC-4); orthogonal to `--force` (session-freshness only) |
| `mindspec next <bead-id>` (session freshness) | A stale session (resumed, compacted, or a bead already claimed in it) refuses an unselected claim unless `--force` is set. A claim that names the bead only warns (`claiming the named bead anyway`) and proceeds, so lane claims need no `--force`. The decision lives in `sessionFreshnessGate`. |
| `mindspec bead ready-check <id>` | Read-only MF-1..MF-4 report with validated schema-v2 override coverage (spec 129 R6.4). Exit 0 iff all signals pass or all current failures are covered; indeterminate records never cover. Reports OVERRIDDEN, no-longer-needed, legacy or malformed markers. Otherwise exits non-zero naming uncovered signals and release/re-claim recovery with ALL currently failing IDs. Metadata read errors refuse. |
| `mindspec bead clarify <id> --file <record.json>` | Write the append-only readiness-attempt record for a NOT-READY bead — original ordinal-keyed report + span-grounded clarifications; refuses a second write per bead, the categorical R8 cap (spec 124 R8) |
| `mindspec complete` | Close bead, remove worktree, advance state |
| `mindspec approve spec\|plan\|impl` | Transition between lifecycle phases |
| `mindspec cleanup` | Remove stale worktrees and branches |
| `mindspec config show` | Print the effective config (panel/models/loop/runner + retained keys), with no `merge_strategy` row; read-only |
| `mindspec panel create <slug>` | Register a panel with the live commit, subject digest, roster and round ledger event. `--gate` selects the subject and roster defaults. `--reviewers N` sets at least 2 slots. A round bump keeps the latest roster unless this flag is supplied. New writes omit `approve_threshold`. |
| `mindspec panel verify <slug>` | Read-only completeness/staleness report; decision-identical to the gate, writes nothing, always exits 0 (spec 110 R2; spec 113 R1 non-bead staleness from recorded target) |
| `mindspec panel tally <slug>` | Per-slot verdicts + rule line + decision; advisory-but-block-capable — exit code tracks the decision alone, non-zero on Block (spec 110 R3; spec 113 R1 non-bead staleness from recorded target) |
| `mindspec config show --gate <name> [--json]` | Print one panel gate's resolved defaults: slots, expected reviewer count, and raw `approve_threshold`. Output is text or JSON. The `substitution` member is removed. This command is read-only. |
| `mindspec models populate` | Print the ZFC agent prompt for declaring the per-phase `models:` protocol in `.mindspec/config.yaml`; writes nothing (spec 123 R6, mirrors `mindspec source populate`) |
| `mindspec commands populate` | Print the ZFC agent prompt for declaring the consumer's build/test guidance under `commands:`; writes nothing — once populated, `init`/`setup` render the entries as the managed AGENTS.md "Build & Test" section (spec 123 R7) |
| `mindspec adr create "<title>" [--slug <kebab>]` | Create an ADR with a slugged filename `ADR-NNNN-<slug>.md` derived from the title (or `--slug` override); every surface reports/accepts the canonical `ADR-NNNN` ID (spec 123 R5) |
| `mindspec panel create <slug> --gate adhoc --target <ref>` | Create an ad-hoc panel WITHOUT `--spec` at `.mindspec/reviews/<slug>/` (flat layout) — talliable via `panel tally`, never scanned by `mindspec complete`'s gate (spec 123 R8, ADR-0037 scope) |
| `mindspec domain add <name>` | Scaffold a domain AND converge from any partial state: backfills missing standard files and the context-map entry; refuses "already exists" only when fully scaffolded and mapped (spec 123 R2) |
| `mindspec reattest <bead-id> [--spec-branch spec/<id>]` | Explicit audited landed-binding recovery. Derives identity from first-parent merges, then a source-named reachable fallback only on zero primary candidates. Fallback requires F1/F3/F4/F5; registrations are advisory. Writes binding and valid audited tuple atomically. On the fallback, no-op only when valid audit, binding and candidate agree. On the primary path, no-op when the binding equals the newest owned merge. Non-matching panel sets never veto; contradictory bindings are repaired with prior values audited. No bypass, SHA assertion or `--all`. `--spec-branch` scopes only cleanly absent lineage (ADR-0041 amendments, specs 125/129). |

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
  `panelGate` returns each matched registration with its Allow or Warn
  decision in scan order, or an empty collection on its fail-open paths
  (empty bead ID, no registered panel). `panelReg` is the first registration,
  or nil for an empty collection, and `PanelGateAdvisoryDefault`'s arguments
  deref `panelReg.Panel`, so the guard sits ahead of that call even though
  `reviewerCountAdvisory` itself also nil-checks its `reg` parameter. No
  `Allow`/`Block` decision is touched by any of this — the gate's outcome
  is fully computed before the advisory call site runs.

  `renderConfig` (`cmd/mindspec/config.go`) shows a set `panel.note` and
  configured `panel.gates` in `config.PanelGateKeys` order. Each gate shows
  its configured reviewers, resolved reviewer sum
  (`PanelGateExpectedReviewers`), and raw `approve_threshold` expression
  (`PanelGateApproveThresholdExpr`). An empty gate map renders `gates: {}`.
  The output has no substitution map or known-model warning. Config strings
  in text output pass through `escapeConfigValue`.

  `mindspec config show` gained `--gate <name> [--json]` (R8/R9): two pure
  functions, `renderGateResolved` (text) and `gateResolvedJSON` (a typed
  struct marshaled with `encoding/json`, never string concatenation), both
  delegating to a shared `buildGateResolvedDoc` that calls ONLY the R3
  config resolvers (`PanelGateReviewerSlots`/`PanelGateExpectedReviewers`/
  `PanelGateApproveThresholdExpr`) — so `--gate` output cannot disagree
  with them. The JSON document has `gate`, `slots` (`{slot, model, lens}`
  in expansion order), `expected_reviewers`, and `approve_threshold`
  (the raw expression string). These members are a stable, additive-only
  contract. The `substitution` member is removed.
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

### Landed review identity (`complete`, Spec 130 R12)

On a normal run, before `exec.CompleteBead`, `complete.Run` calls the optional
`executor.LandedReviewSetter.SetLandedReview(sha, panel)` once for the first
Allow returned by `panelGate`. The values are that registration's
`reviewed_head_sha` and `<slug>@r<N>`. A Warn (including abandoned panels,
missing refs and the environment hatch), `enforcement.panel_gate: false`, or
no panel supplies no identity. A config-disabled matched panel returns Warn
without evaluating its facts. The executor writes `mindspec_landed_reviewed_sha`
and `mindspec_landed_review_panel` together with the landed binding;
`FinalizeEpic` leaves them unchanged. Before publishing each write-ahead stage
marker, `complete` records the first Allow's identity in the same metadata write
as the marker attestation. An authenticated materialization resume skips the
panel gate decision. It still re-checks the recorded deferrals against live
tracker facts. It keeps each stored entry's identity. An intact abandoned panel
removes its deferrals from the pending marker and audit. An applicable slot
refutation removes each covered item whose round and raw-text hash still match.
It replays the recorded landed identity through `SetLandedReview` before
`CompleteBead`. A resume with no recorded identity supplies none. Each new marker
attestation overwrites both identity fields, including when there was no Allow.

### Lineage health and spec IDs

Spec approval writes the full spec ID to epic metadata `spec_id`.
Legacy `spec_num` and `spec_title` stay available for numeric queries.
Plan approval, lifecycle scans and scoped next selection use the phase
package's full-ID resolver. A lettered ID resolves to its own directory.
Scoped lookup refuses duplicate epic bindings, including closed duplicates.

`mindspec spec create` accepts a bare slug or a numeric full ID. It refuses
a new lettered full ID. Existing lettered spec directories remain valid.

Doctor runs `checkLineage` after the lifecycle integrity check. It reports
each open bead that only the old title rule would resolve, at Warn, with
`bd update <bead> --parent <epic>` recovery. It also reports duplicate epic
bindings at Warn. It continues to the remaining checks.

Scoped next selection uses `phase.FindEpicLineageWithCache`. Its returned
provenance replaces the second bead read. A foreign parent-child epic stays
inspection-only. A mapped title-only row has no lineage and is N1.
Readiness accepts a positional bead index only for the exact `<epic>.<N>` ID.

## Merge-only completion

| Surface | Contract |
|:--|:--|
| `mindspec complete <bead> --merge-only` | Run all gates and merge for verification. Print the full merge SHA and both parents. Keep the bead open. |
| `--merge-only --resolve-merge` | Resume the preserved merge. Check ownership from the resolved index. Keep the bead open. |
| Plain `complete` after a naming merge | Check the bead tip, user changes, panel, and M^1..M content. Publish the close before cleanup. |
| `mindspec_merge_only=pending` | Retain merge-only mode across interruptions, including a plain re-entry. |
| `mindspec_merge_only_sha` | Name the recorded merge. A foreign or non-naming marker refuses. Git state decides validity. |

The closing call reads bead user changes again after publication and before
removal. The executor repeats this check at removal. Tool artifacts stay exempt.
The exemption preserves decoded filename whitespace. A lookalike path blocks the close.
Status reads disable rename and copy detection.
New user changes or a status or stat read error refuse with a completion retry
command. The pending file, worktree, and branch remain.

The closing call clears both merge-only keys after publication. No key
replaces the panel gate. A closed resume checks the same merge and tip,
skips presence, and finishes the binding, override, export, marker, and
cleanup steps. Step 1–4 recovery repeats the supplied doc-skew and ADR
override flags with their reasons. A repeated merge-only call records the
existing valid merge and makes no second merge.

A closed child of the spec epic without a naming merge runs the merge
recovery without a second close. It publishes the closed export and panel ledger before cleanup.
An epic, phase, or child-status read failure refuses before close, merge,
or cleanup. Retry completion when the read succeeds. A closed-unmerged
conflict keeps its line boundaries and ends with one `--resolve-merge`
recovery. The executor omits `--merge-only` from every re-entry hint on this path.
Other merge guard failures keep their recovery.

## Implementation approval finalization

The temporary finalization worktree has its own removal check. It runs after
commit and push hooks, including failed publication attempts, and before removal
of a leftover checkout on retry. The check first confirms that the path is a
registered worktree of this repository. An unregistered directory stays in place.
Move that directory to a safe path, then retry approval.

Status reads disable rename and copy detection before artifact checks.
Tool artifacts, including the regenerated `.beads/issues.jsonl` tracker export,
are exempt in this temporary checkout. Other pending changes or a status or stat
read error stop removal. A registration read error also stops removal. The pending
file, worktree, and carrier branch remain. The error gives an approval retry
command. Preserve the changes at a safe path. Leave the worktree clean and
readable, then retry approval.


Implementation approval checks pending beads for an existing naming merge before
its first write. A moved tip refuses with the same follow-up recovery as
`complete`. Finalization repeats the check before each live merge. An export or
artifact-commit failure stops publication and cleanup. The error gives the epic
state, the failed stage, and the approval retry command. An absent spec or pending
bead worktree has no artifact commit to run. A pending bead branch can still
merge through the spec worktree.

After publication and its hooks, finalization checks all scoped bead worktrees
and the spec worktree before forced removal of those worktrees. The branch-missing resume uses
the same check before its first removal. Pending changes or a read error return
a cleanup-stage failure with the epic state and recovery commands. No worktree
or branch is removed.


## Spec findings deferred to the plan

`panel defer <slug> --slot S --round N --to-plan --by <actor>` works only on a
`spec_approve` panel. It refuses `--bead`. Add `--finding I` to cover one finding.
Without that flag, the verb covers each finding in the slot.

The verb appends one `deferred_to_plan` record and one `confirmed-deferred` row
per finding in one ledger write. The evidence is `plan:<deferral id>`. Each
record has a stable id, finding identity, raw text, hash, actor and time. It has
no `bead` field. The snapshot stores the entries in `plan_deferrals` and leaves
`deferrals` unchanged. A retry repairs a missing snapshot entry without a second
ledger record.

The integrity check requires a matching ledger record for each entry. A new
round carries backed entries. The spec approval gate discharges only findings
whose slot, round, id and hash match. Other gates do not use these entries to
discharge findings. The implementation approval bead-deferral check reads only
bead deferrals.

## Bead panel presence

`complete` applies `enforcement.panel_presence` when its bead gate finds no
registration. The default `warn` policy prints one warning with the gate name
and panel creation command. A successful completion records `panel_absent`,
`panel_absent_gate` and `panel_absent_at` on the bead. The `enforce` policy
refuses before close or merge. A failed registration refuses under both policies.

A closed bead refusal keeps its status and the spec tip. Its recovery runs a
panel at the bead tip, then retries completion. The non-flat cleanup guard
still refuses when the bead worktree and registration are absent. Branchless
reconciliation and busy-worktree refusals add no presence warning or audit.
The skip and disabled hatches retain their rules. Refutation obligations still
settle before close. A closed, unmerged bead shares its presence outcome with
the merged completion handler, so the command prints only one absence warning.

## Lifecycle panel gate core

`panel.DocumentSubjectDigest` computes the semantic subject for document and
final-review panels. Create, rebind and verify use the same reader. Document
subjects exclude approval-owned fields, the standard approval section and
the exact initial plan scaffold. Other frontmatter values remain in scope.
Final review excludes the exact generated placeholder ADR. Each lifecycle
panel target must be `spec/<id>`.

`runLifecycleGate(root, specID, gate, worktree, out)` returns an `Outcome`
and an error. It makes no lifecycle write. It checks each live registration,
the recorded target tip, semantic subject dirt and durable coverage. A failed
leg refuses under both presence policies. A missing panel warns or refuses
as the policy directs. The skip, disabled and abandonment hatches retain
their audit rules.

`recordLifecycleGateAudit(epicID, outcome, deferralIDs)` writes the due audit
through a metadata merge. It records skip, abandonment and absence with their
times. Absence includes `panel_absent_gate`. It writes the accepted IDs to
`plan_deferral_ids`. A nil list writes no deferral key. Call it after epic
resolution. A failed write returns an error.

### Implementation final-review gate

`impl approve` runs `final_review` before the placeholder write, phase reconcile,
epic audit, close and finalization. The gate applies with any plan bead count.
It reads the subject at `spec/<id>`. The shared lifecycle gate records absence,
skip and abandonment on the epic. The ledger-persistence preflight stays active
under each policy and hatch.

Before these writes, approval checks each plan-scoped bead worktree from the
tracker worktree list. Any tracked or untracked change refuses. This check has
no subject exclusions and no hatch. It names the bead and each path. Export or
panel-ledger dirt directs the operator to `mindspec complete <bead>`. Other dirt
requires a commit or discard before completion. A moved tip requires a follow-up
bead and a new final-review round. Clean merged worktrees pass.

An exact generated placeholder is an own write. A retry accepts it without a
second create. It must be a regular file, not a symlink. Its permissions do not
affect this check. Any other placeholder content stays in the subject. With no spec
branch, the existing publication checks decide replay. With a closed epic, done
phase and no spec worktree, approval compares the current subject to the final
publication tip. A changed subject requires a spec worktree and a new panel.
In all other cases with a spec branch and no spec worktree, approval refuses
under both presence policies. Re-create the spec worktree and run a final-review
panel. The subject dirt check runs only in the spec worktree.

## Approval guidance

Spec and Plan instruct output names the document panel gate and its spec-branch
target. Approval skills use the same target for document and final-review panels.
The bead cycle runs the content check after each implementation commit and before
the bead panel. Setup preserves old skill bodies as refresh snapshots and keeps
user edits.

Doctor reports `enforcement.panel_presence: warn` at Info. It reports each
in-flight spec in Plan or Implement mode with `panel_absent` metadata at Warn,
under both presence policies. These checks do not change approval state.

### Content check before completion

`mindspec complete <bead> --check` reports doc-sync, ADR and work-destruction
failures on the same candidate and landing tree as completion. It skips panel
review and coverage. It leaves git, worktrees and bead records unchanged. The
bead must be open, have a branch and have no pending merge-only run. Check refuses
`--merge-only`, `--resolve-merge`, `--override-adr`, `--supersede-adr` and
`--allow-net-deletion`. Completion also reports all failing content gates.
A file that no domain owns gives a warning and retains its structured finding.
An undeclared owner, an attribution failure or missing ADR coverage still refuses.

### Domain doc declarations and owners

`complete --no-doc-impact "<domain>: <reason>"` declares no doc impact for
one touched domain. The flag can repeat. Completion records each domain,
reason, actor and time on the bead. Other domain errors still refuse.

`complete --allow-doc-skew "<reason>" --doc-owner <bead>` defers missing
domain docs to another open bead in the same epic. Both beads carry the
record. An owner must land the domain doc updates or transfer the record
to a new owner. Recovery commands retain the owner and each declaration.
The content check validates the same flags, names each covered domain and
writes no record.

Implementation approval checks every source-changing step on the spec
branch. A domain with no doc update passes only when each source change
came through a landed declaring bead merge. A direct source commit on the
spec branch has no declaration exemption. Approval lists each open doc
deferral. Its doc-skew override records the open deferral ids and reason
on the epic. Old ownerless skew records remain subject to branch doc-sync.

## Document approval gates

`ApproveSpec` checks `spec_approve` before its first write on every run.
`ApprovePlan` checks `plan_approve` before migration, audit, supersede,
frontmatter, bead creation and commits. A replay runs the same gate.
Existing approvals remain valid for other lifecycle operations.

Both verbs persist the shared gate audit after epic resolution. Spec approval
also records accepted to-plan deferral ids. If epic creation fails with an
audit due, it commits the approval artifacts, warns with the due records and
refuses with a same-environment retry. A retry can persist those records.

Plan approval requires all ids from the epic and tracked spec approval
ledgers in the plan's `## Resolutions` section. It reads abandoned, replaced
and ledger-only registrations. Missing ids refuse before any lifecycle write.
A refusal does not supersede or create beads. The Approval section is optional
in a draft spec because spec approval creates it.
