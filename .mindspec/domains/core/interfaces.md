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
silently at runtime (spec 109 Bead 4 hit this adding `config`).

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
}

type Panel struct {
    Reviewers        []Reviewer   `yaml:"reviewers"`         // default [{claude,3},{codex,3}]
    ApproveThreshold string       `yaml:"approve_threshold"` // RAW "n-1" or integer string; default "n-1"; never resolved here
    Substitution     Substitution `yaml:"substitution"`
}
type Reviewer struct {
    Family string `yaml:"family"`
    Count  int    `yaml:"count"`
}
type Substitution struct {
    ClaudeSubOnQuota bool `yaml:"claude_sub_on_quota"` // default true
}

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
```

`Load` refuses (guard-style error with a `recovery: <command>` line, ADR-0035), and never panics, on: a `panel_skip` key anywhere under `loop.gate_authority` (panel-skip is permanently human, `MINDSPEC_SKIP_PANEL` is env-only, ADR-0037 §7); a `loop.halt.on_reject` other than `halt`; a `gate_authority` value outside `{panel, human}`; a `controller_handoff` outside `{per-spec, at-usage-threshold}`; an unknown `runner`; a `panel.approve_threshold` that is neither `"n-1"` nor an integer in `[1, sum(reviewers.count)]`; a `reviewers[].count < 1`; or a `sum(reviewers.count) < 2` (both close the threshold-skip loophole from the reviewer side, since a single-reviewer panel makes the default `"n-1"` resolve to an always-pass `0`). An absent/empty `panel:`/`models:`/`loop:`/`runner:` block round-trips to its documented default.

Used by workflow: `internal/panel` stays a config-free leaf and never imports this package — its callers resolve `PanelExpectedReviewers()`/`PanelApproveThresholdExpr()` here and pass plain values in. `cmd/mindspec config show` and `internal/complete`'s panel-advisory path render those resolved values alongside `panel.ReviewerCountNote`.

## Consumed Interfaces

- **context-system**: Glossary parsing (for broken-link validation in doctor)
- **workflow**: None currently

## Events

None defined yet. Future: health check completion events for observability.
