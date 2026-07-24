# Core Domain — Interfaces

## Provided Interfaces

### Workspace

```go
package workspace

// FindRoot walks up from startDir looking for .mindspec/ or .git.
func FindRoot(startDir string) (string, error)

// DocsDir returns the canonical-or-legacy docs root (no flat tier). Retained
// for consumers not yet migrated to the per-artifact accessors.
func DocsDir(root string) string

// Per-artifact, three-tier flat-first resolvers (spec 106 Req 1):
// flat (.mindspec/<artifact>) → canonical (.mindspec/docs/<artifact>) →
// legacy (docs/<artifact>), first-exists-wins.
func SpecDir(root, specID string) (string, error)
func ADRDir(root string) string
func CoreDir(root string) string
func DomainDir(root, domain string) (string, error)
func ContextMapPath(root string) string
func RecordingDir(root, specID string) (string, error)

// Flat-aware ENUMERATION roots: the parent dirs SpecDir/DomainDir resolve
// an <id>/<domain> under (same three-tier flat-first precedence). For
// filesystem enumerators that list all specs/domains without re-deriving
// the layout. Byte-identical to filepath.Join(DocsDir(root), "specs"|
// "domains") on canonical/legacy/greenfield trees.
func SpecsDir(root string) string
func DomainsDir(root string) string

// TreeRootForSpecDir resolves the checkout tree root from a spec dir in any
// of the flat / canonical / legacy shapes (preserves mindspec-ew79).
func TreeRootForSpecDir(specDir string) string

// ADR file resolution (spec 123 R5). ADRFilePath is the exact-join
// WRITE-target resolver (path a NEW file is written to); ResolveADRFile
// is the shared READ resolver every existing-file caller uses (show,
// --supersedes, Supersede, CopyDomains): accepts canonical "ADR-0001" or
// a full slugged stem, resolves canonical-number driven to the bare OR
// slugged on-disk file, and errors (a `recovery:`-prefixed prose
// diagnostic — not ADR-0035's copy-pastable command form — with
// termsafe-escaped filenames) when more than one file carries the number.
func ADRFilePath(root, adrID string) (string, error)
func ResolveADRFile(root, id string) (string, error)

// Whole-tree layout classification (spec 106 Req 2).
type Layout string // flat | canonical | legacy | greenfield | mixed

// DetectLayout classifies the tree; mixed is a hard error (ErrMixedLayout)
// except under an IN-PROGRESS (non-terminal) .mindspec/migrations/<run-id>/
// run — a completed/"applied" record does not tolerate a mixed tree.
func DetectLayout(root string) (Layout, error)

// ClassifyLayout is the pure layout-signature classifier shared by
// DetectLayout (filesystem) and the cross-layout merge guard (git refs).
type LayoutMarkers struct{ Flat, Canonical, Legacy bool }
func ClassifyLayout(m LayoutMarkers) Layout
func LayoutMarkersFromMindspecChildren(children []string) LayoutMarkers
```

Used by context-system (for glossary location) and workflow (for spec/bead resolution).

### Health Check Report

```go
package doctor

type Status int // OK, Missing, Error, Warn

type Check struct {
    Name    string
    Status  Status
    Message string
}

type Report struct {
    Checks []Check
}

func (r *Report) HasFailures() bool  // true if any Error or Missing
func Run(root string) *Report        // execute all checks
```

### CLI Command Registration

Other domains register subcommands via cobra in `cmd/mindspec/`. Core owns the top-level `mindspec` command group.

Registering a NEW top-level command also requires adding its name to
`internal/redact.CommandTokens` (`internal/redact/redact.go`), the
closed-set enum `RedactEvent` checks before letting a friction/success
journal event carry a `Command` value — an unregistered command's events
are silently DROPPED, not merely unredacted. `TestRedactEnum_NoCobraDrift`
(`cmd/mindspec/redact_enum_drift_test.go`) walks the real cobra tree and
fails on drift, catching a missed registration at test time rather than
silently at runtime (spec 109 Bead 4 hit this adding `config`; spec 125
followed the same rule adding `reattest` — a one-token, core-owned
registration for the workflow domain's new verb, no other core change).

### Config

```go
package config

// Config represents .mindspec/config.yaml settings.
type Config struct {
    // ... pre-existing fields (ProtectedBranches, MergeStrategy,
    // WorktreeRoot, AutoFinalize, Enforcement, Recording, Decomposition,
    // SourceGlobs) — unchanged by spec 109.

    // Orchestration substrate (spec 109, ADR-0040). Panel supplies the
    // creation-time defaults for a fresh panel.json (the spec-110 writer
    // consumes the resolvers below); it never overrides an already-recorded
    // panel's decision inputs — internal/panel stays a config-free leaf.
    // Models, Loop, and Runner are declared, defaulted, validated, and
    // surfaced by `mindspec config show` (Bead 4), but every key in them is
    // INERT in this spec: nothing reads them to change behavior until a
    // later spec wires enforcement.
    Panel  Panel             `yaml:"panel"`
    Models map[string]string `yaml:"models"` // phase -> model id; free-form (runner-specific); default {}
    Loop   Loop              `yaml:"loop"`
    Runner string            `yaml:"runner"` // claude-code-skills | claude-code-workflow | external; default claude-code-skills

    // Commands is the CONSUMER's declared build/test guidance (spec 123
    // R7, ADR-0040 consumer-identity clause): a free-form task -> shell
    // command map (documented vocabulary keys: "build", "test"). NOT
    // inert, unlike Models/Loop/Runner: `mindspec init` and every
    // `mindspec setup <agent>` verb render populated entries as the
    // managed AGENTS.md "Build & Test" section via
    // RenderBuildTestSection; unset means the section is OMITTED (ZFC —
    // the framework never guesses a consumer's build system).
    Commands map[string]string `yaml:"commands"`
}

// Declared-key predicates + the single Build & Test renderer (spec 123).
// An all-blank map is NOT declared (empty≠declared): doctor's
// missing-models/missing-commands Warns key off these, never bare len().
func (c *Config) HasDeclaredModels() bool
func (c *Config) HasDeclaredCommands() bool
func (c *Config) CommandLines() []string              // "<command>   # <task>" lines; stable order build, test, then sorted; termsafe-escaped
func (c *Config) RenderBuildTestSection(level int) string // "" when Commands unset — section omitted, never a placeholder

type Panel struct {
    Reviewers        []Reviewer   `yaml:"reviewers"`         // ALL-GATES default; default [{claude,3},{codex,3}]
    ApproveThreshold string       `yaml:"approve_threshold"` // RAW "n-1" or integer string; default "n-1"; never resolved here
    Substitution     Substitution `yaml:"substitution"`
    // Gates is the optional per-gate override map (spec 112 R1), keyed by
    // one of PanelGateKeys. Absent and present-but-empty (`gates: {}`) are
    // EQUIVALENT everywhere — every "gates is configured" predicate keys
    // off len(Gates) > 0, never key presence. Default: empty (the standing
    // per-gate protocol below is the documented EXAMPLE, never the
    // default — DefaultConfig's panel stays 109's 3+3/"n-1").
    Gates map[string]GatePanel `yaml:"gates"`
    // Note is optional free-text advisory metadata (spec 112 R1): parsed
    // and echoed verbatim by `config show`; never read by any validation
    // or resolver.
    Note string `yaml:"note"`
}

// GatePanel is one panel.gates entry: a per-gate override of the reviewer
// mix and/or approve_threshold. A configured gate must set at least one of
// the two (Load refuses an entry that sets neither).
type GatePanel struct {
    Reviewers        []Reviewer `yaml:"reviewers"`
    ApproveThreshold string     `yaml:"approve_threshold"` // RAW expression, same grammar as Panel's
}

// PanelGateKeys is the closed, ordered enum of panel.gates keys (spec 112
// R1, ADR-0034): {"spec_approve", "plan_approve", "bead", "final_review",
// "adhoc"}. These name REVIEW EVENTS — deliberately distinct from
// loop.gate_authority's vocabulary, which names APPROVAL ACTS
// (bead_merge/impl_approve). Exported: the single source for validation,
// recovery lines, and config-show's enum-order rendering.
var PanelGateKeys = []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc"}

// Reviewer is one entry of a reviewer mix (global or per-gate). Model and
// Lens are open-vocabulary strings — NO name-membership validation exists
// anywhere in Load (spec 112 R1, ADR-0040): model ids are runner-specific
// and ship faster than any enum could track (e.g. claude-fable-5). Family
// is the legacy 109 field, still parseable; Model wins for slot expansion
// when an entry sets both. Count is a POINTER so an absent count (nil) is
// distinguishable from an explicit `count: 0`/negative: nil resolves to 1
// via CountValue() (spec 112 R2's one deliberate monotone relaxation over
// 109, which refused count < 1 wholesale); non-nil non-positive is refused.
type Reviewer struct {
    Family string `yaml:"family"`
    Model  string `yaml:"model"`
    Lens   string `yaml:"lens"`
    Count  *int   `yaml:"count"` // nil -> defaults to 1; explicit <= 0 refused
}

// CountValue returns the resolved count (nil -> 1). EXPORTED because
// cmd/mindspec's config-show renderer is an out-of-package consumer (Go
// forbids a Count() method beside the Count field); every consumer
// (validation, resolvers, slot expansion, that renderer) goes through this
// one accessor.
func (r Reviewer) CountValue() int

type Substitution struct {
    ClaudeSubOnQuota bool `yaml:"claude_sub_on_quota"` // default true; INERT while Substitutes is non-empty
    // Substitutes is the model-level, one-step substitution map (spec 112
    // R5): unavailable-model -> substitute-model, both non-empty and
    // key != value (chains are not followed; a mutual A<->B pair is
    // legal). Global, not per-gate. Non-empty Substitutes IS the
    // substitution policy and supersedes ClaudeSubOnQuota; empty means
    // ClaudeSubOnQuota keeps its 109 meaning.
    Substitutes map[string]string `yaml:"substitutes"`
}

// KnownModels returns a copy of the curated, deliberately non-exhaustive
// advisory model-id list (spec 112 R8) — seeded with claude-fable-5,
// claude-opus-4-8, claude-sonnet-5, gpt-5.5, claude, codex. Consumed ONLY
// by `config show`'s warning annotation; NEVER by Load or
// validateOrchestration — an unseeded id must never fail to load.
func KnownModels() []string

// Loop is the orchestration-loop governance skeleton (research §3.2 L3 /
// §3.3). Every key is parsed, defaulted, validated, and surfaced — none is
// enforced in this spec.
type Loop struct {
    Enabled       bool              `yaml:"enabled"`        // default false (loops are opt-in)
    GateAuthority map[string]string `yaml:"gate_authority"` // exactly spec_approve/plan_approve/bead_merge/impl_approve -> panel|human, default human
    Halt          Halt              `yaml:"halt"`
    Budget        Budget            `yaml:"budget"`
    Context       LoopContext       `yaml:"context"`
    HandoffLog    string            `yaml:"handoff_log"` // default "AUTOPILOT-LOG.md"
}
type Halt struct {
    MaxRoundsPerBead           int    `yaml:"max_rounds_per_bead"`            // default 3
    PanelDeadlockRounds        int    `yaml:"panel_deadlock_rounds"`          // default 2
    MaxConsecutiveImplFailures int    `yaml:"max_consecutive_impl_failures"`  // default 2
    OnReject                   string `yaml:"on_reject"`                      // must be "halt"; Load refuses any other value
}
type Budget struct {
    MaxBeadsPerWake int `yaml:"max_beads_per_wake"` // default 0 (unlimited)
    TokenBudget     int `yaml:"token_budget"`       // default 0 (unlimited)
}
type LoopContext struct {
    ControllerHandoff string `yaml:"controller_handoff"` // per-spec | at-usage-threshold; default per-spec
}

// PanelExpectedReviewers is the sum of panel.reviewers[].count (default 6).
// PanelApproveThresholdExpr is the RAW panel.approve_threshold expression —
// it does NOT resolve to an int; resolution is single-homed in
// internal/panel.Panel.ApproveThreshold (workflow domain). These are the
// values the spec-110 panel.json writer stamps as a fresh panel's defaults.
func (c *Config) PanelExpectedReviewers() int
func (c *Config) PanelApproveThresholdExpr() string

// ReviewerSlot is one expanded reviewer position (spec 112 R3):
// deterministic, declaration-ordered, ids "R1".."Rn".
type ReviewerSlot struct {
    Slot  string
    Model string
    Lens  string
}

// Gate-scoped resolvers (spec 112 R3) — the creation-time-default surface
// the spec-110 panel.json writer and ms-panel-run step 0 consume; NOTHING
// reads these at gate-decision time (internal/panel stays the sole
// decision authority, ADR-0037). Each returns an error for a gate name
// outside PanelGateKeys (fail loud on a caller typo, never silently fall
// back to defaults). Resolution walks a PER-FIELD chain — reviewers and
// approve_threshold resolve independently: the gate's own configured value
// -> (for "adhoc" only) "bead"'s resolved value -> the global
// reviewers/approve_threshold -> the built-in 3+3/"n-1" default.
// PanelGateApproveThresholdExpr returns the RAW expression, NEVER a
// resolved integer — resolution to an int stays single-homed in
// internal/panel.Panel.ApproveThreshold (ADR-0037 §3).
func (c *Config) PanelGateExpectedReviewers(gate string) (int, error)
func (c *Config) PanelGateApproveThresholdExpr(gate string) (string, error)
func (c *Config) PanelGateReviewerSlots(gate string) ([]ReviewerSlot, error)

// PanelGateAdvisoryDefault is the single-home selection rule (spec 112 R7)
// both cmd/mindspec's `config show` and internal/complete's panel advisory
// resolve through, so the two callers cannot drift. recordedGate is a
// panel.json Panel.Gate value (possibly empty or outside PanelGateKeys);
// isBead is Panel.IsBead(). Returns (0, false) when the advisory must be
// SKIPPED. Selection: len(Gates) == 0 -> (PanelExpectedReviewers(), true)
// always; a known recordedGate -> that gate's PanelGateExpectedReviewers;
// an empty recordedGate with isBead -> the "bead" gate's; anything else
// (non-bead with no recorded gate, or an unknown recorded value) ->
// (0, false). Never calls a gate-scoped resolver with a value outside
// PanelGateKeys, so no resolver error can surface through this helper.
func (c *Config) PanelGateAdvisoryDefault(recordedGate string, isBead bool) (int, bool)
```

**Deterministic slot expansion** (spec 112 R3): `PanelGateReviewerSlots` flattens a gate's resolved reviewer list into slots `"R1".."Rn"` in declaration order, expanding each entry's `CountValue()`. An entry's explicit `lens` applies to every slot it produces and does NOT advance the default-lens cursor. A lens-less slot takes `defaultLenses[cursor % 6]` from the interleaved ordering `author-of-record, empirical-prober, codebase-pin, adversarial, contract-stability, integration` (structural/sharp alternating); **one global cursor per expansion, starting at index 0**, advances only over lens-less slots. Normative worked example — a 9-reviewer, all-lens-less, 3-entries-of-`count: 3` panel expands to exactly: `R1` author-of-record, `R2` empirical-prober, `R3` codebase-pin, `R4` adversarial, `R5` contract-stability, `R6` integration, `R7` author-of-record, `R8` empirical-prober, `R9` codebase-pin.

`Load` refuses (guard-style error with a `recovery: <command>` line, ADR-0035), and never panics, on: a `panel_skip` key anywhere under `loop.gate_authority` (panel-skip is permanently human, `MINDSPEC_SKIP_PANEL` is env-only, ADR-0037 §7); a `loop.halt.on_reject` other than `halt`; a `gate_authority` value outside `{panel, human}`; a `controller_handoff` outside `{per-spec, at-usage-threshold}`; an unknown `runner`; a `panel.approve_threshold` that is neither `"n-1"` nor an integer in `[1, sum(reviewers.count)]`; a `reviewers[].count < 1`; or a `sum(reviewers.count) < 2` (both close the threshold-skip loophole from the reviewer side, since a single-reviewer panel makes the default `"n-1"` resolve to an always-pass `0`). An absent/empty `panel:`/`models:`/`loop:`/`runner:` block round-trips to its documented default.

**Per-gate refusals (spec 112 R4)**, same guard-style/never-panic contract: a `panel.gates` key outside `PanelGateKeys` (the recovery line enumerates all five valid keys and disambiguates from `loop.gate_authority`'s different vocabulary); a configured gate entry setting neither `reviewers` nor `approve_threshold`; a reviewer entry (global or per-gate) with neither `model` nor `family`; an explicit non-positive `count` (`Count != nil && *Count < 1`; an ABSENT count is fine, defaulting to 1); a per-gate `reviewers` list whose expanded sum is `< 2`; a per-gate or INHERITED `approve_threshold` that is neither `"n-1"` nor an integer in `[1, that gate's resolved reviewer sum]` — checked at every link of the `adhoc` -> `bead` -> global inheritance chain, so a global integer valid for the global sum but out of range for a smaller inheriting gate is refused, and likewise a `bead` integer threshold inherited through `adhoc` by a smaller reviewers-only `adhoc` gate; and a `substitutes` entry with an empty key or value, or key == value. **No model or lens name-membership check exists on any `Load` path** — an unknown model id alone never errors; the curated `KnownModels()` list drives only a `config show` warning (a later bead).

Used by workflow: `internal/panel` stays a config-free leaf and never imports this package — its callers resolve `PanelExpectedReviewers()`/`PanelApproveThresholdExpr()`/the gate-scoped resolvers here and pass plain values in. `cmd/mindspec config show` and `internal/complete`'s panel-advisory path render those resolved values alongside `panel.ReviewerCountNote`.

### Per-gate panel config example (spec 112)

The operator's standing review protocol (rev 2026-07-07) is the DOCUMENTED EXAMPLE for `panel.gates` — never the shipped default (`DefaultConfig` stays 109's 3+3/`"n-1"` with `gates`/`substitutes` empty). An operator commits this shape to their own `.mindspec/config.yaml` to enable it:

```yaml
panel:
  note: "fable-window 2026-07, codex-enabled"   # optional free-text; echoed by `config show`, never consumed
  reviewers:                       # 109 global list — the default for any gate not configured below
    - {family: claude, count: 3}
    - {family: codex, count: 3}
  approve_threshold: "n-1"
  substitution:
    claude_sub_on_quota: true      # legacy family-level knob; superseded while substitutes is non-empty
    substitutes:
      gpt-5.5: claude-sonnet-5     # quota wall -> Sonnet, slot id kept: reviewer_id "R7 claude-sonnet-5-sub"
  gates:
    spec_approve:                  # 9 reviewers, pass >= 8 ("n-1")
      reviewers:
        - {model: claude-fable-5, count: 3}
        - {model: claude-opus-4-8, count: 3}
        - {model: gpt-5.5, count: 3}
    plan_approve:                  # same 9-reviewer mix as spec_approve
      reviewers:
        - {model: claude-fable-5, count: 3}
        - {model: claude-opus-4-8, count: 3}
        - {model: gpt-5.5, count: 3}
    bead:                          # 6 reviewers, pass >= 5; exploded form with explicit lenses
      reviewers:
        - {model: claude-opus-4-8, lens: author-of-record}
        - {model: claude-opus-4-8, lens: codebase-pin}
        - {model: claude-opus-4-8, lens: contract-stability}
        - {model: claude-sonnet-5, lens: empirical-prober}
        - {model: claude-sonnet-5, lens: adversarial}
        - {model: claude-sonnet-5, lens: integration}
    final_review:                  # 12 reviewers, pass >= 11
      reviewers:
        - {model: claude-fable-5, count: 3}
        - {model: claude-opus-4-8, count: 3}
        - {model: gpt-5.5, count: 3}
        - {model: claude-sonnet-5, count: 3}
      approve_threshold: "11"      # equals "n-1" for N=12; explicit for illustration
    # adhoc: absent -> resolves to bead's mix
```

## Consumed Interfaces

- **context-system**: Glossary parsing (for broken-link validation in doctor)
- **workflow**: None currently

## Events

None defined yet. Future: health check completion events for observability.
