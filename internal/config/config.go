package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
	"gopkg.in/yaml.v3"
)

const (
	runnerSkillsAdapter   = "claude-code-skills"
	runnerWorkflowAdapter = "claude-code-workflow"
	runnerExternalAdapter = "external"
)

// Config represents .mindspec/config.yaml settings.
type Config struct {
	ProtectedBranches []string `yaml:"protected_branches"`
	MergeStrategy     string   `yaml:"merge_strategy"`
	WorktreeRoot      string   `yaml:"worktree_root"`
	AutoFinalize      bool     `yaml:"auto_finalize"`
	// AutoOpenFinalizePR and AutoMergeFinalizePR govern the spec 121
	// finalize-PR automation (R1/R2, ADR-0041 §4): when `impl approve`'s
	// finalize lands the epic-close export on a `chore/finalize-<specID>`
	// carrier, AutoOpenFinalizePR (default true) auto-opens/adopts a PR
	// from it into main; AutoMergeFinalizePR (default false — merging to
	// main without a human is opt-in policy, never default) additionally
	// auto-merges it on affirmative green checks with a true merge
	// commit. Both absent-key defaults are backfilled by loadUncached's
	// unmarshal-over-defaults pattern, mirroring Enforcement.PanelGate
	// and AutoFinalize above. See cmd/mindspec/finalize_pr.go.
	AutoOpenFinalizePR  bool          `yaml:"auto_open_finalize_pr"`
	AutoMergeFinalizePR bool          `yaml:"auto_merge_finalize_pr"`
	Enforcement         Enforcement   `yaml:"enforcement"`
	Recording           Recording     `yaml:"recording"`
	Decomposition       Decomposition `yaml:"decomposition"`

	// SourceGlobs declares which path globs count as "source" for the
	// doc-sync gate (spec 091 Req 11). OVERRIDE semantics: a non-empty
	// list FULLY REPLACES mindspec's built-in classifier (never a
	// union with it); while the list is empty or the field/file is
	// absent, the built-in classifier (.go files under cmd/ or
	// internal/, excluding _test.go) stays active as the disclosed
	// default. The default is EMPTY — the framework never guesses
	// repo-specific globs; `mindspec doctor --fix` scaffolds the
	// commented config block and `mindspec source populate` emits the
	// agent prompt to populate it. Note: an absent `source_globs:` key
	// and `source_globs: []` are indistinguishable through this typed
	// struct — raw-YAML inspection is required to tell them apart
	// (the doctor --fix scaffolder's concern, not Load's).
	SourceGlobs []string `yaml:"source_globs"`

	// Orchestration substrate (spec 109, ADR-0040). Panel supplies the
	// creation-time defaults for a fresh panel.json (the spec-110 writer
	// consumes PanelExpectedReviewers/PanelApproveThresholdExpr below); it
	// never overrides an already-recorded panel's decision inputs —
	// internal/panel stays a config-free leaf, and PanelGateDecision takes
	// no *Config. Models, Loop, and Runner are declared, defaulted,
	// validated, and surfaced by `mindspec config show`, but every key in
	// them is INERT in this spec: nothing reads them to change behavior
	// until a later spec wires enforcement.
	Panel  Panel             `yaml:"panel"`
	Models map[string]string `yaml:"models"`
	Loop   Loop              `yaml:"loop"`
	Runner string            `yaml:"runner"`

	// Commands declares the CONSUMER's build/test guidance (spec 123
	// R7b, ADR-0040's consumer-identity clause): a free-form task ->
	// shell-command map beside Models, with the documented (not
	// enforced) vocabulary keys "build", "test", and "ci" — the same
	// advisory posture as Models' phase keys. "ci" (spec 126 R8a) names
	// the project's declared CI invocation, reproduced by
	// ms-spec-final-review's CI-parity check; when "ci" is undeclared,
	// commands.test is the documented fallback. UNLIKE Models/Loop/Runner, this key
	// is NOT inert: `mindspec init` and every `mindspec setup <agent>`
	// verb render its populated entries as the managed AGENTS.md "Build
	// & Test" section (see CommandLines/RenderBuildTestSection below) —
	// so a populated commands: key changes generated content today. The
	// framework never guesses these values (ADR-0036 ZFC): unset means
	// the section is omitted entirely, plus a `missing-commands` doctor
	// nudge (internal/doctor/config.go), never an inferred command.
	Commands map[string]string `yaml:"commands"`
	sources  map[string]string
}

// Panel declares reviewer execution and creation-time defaults.
// Gates can override the reviewer mix, threshold, lenses and acceptance question.
// A recorded panel remains authoritative for its own gate decision.
type Panel struct {
	Reviewers []Reviewer `yaml:"reviewers"`
	// ApproveThreshold is the RAW expression ("n-1" or an integer string),
	// never resolved here. Expression resolution is single-homed in
	// internal/panel.Panel.ApproveThreshold; see PanelApproveThresholdExpr.
	ApproveThreshold string            `yaml:"approve_threshold"`
	Runners          []Runner          `yaml:"runners"`
	Lenses           map[string]string `yaml:"lenses"`
	IsolationRoot    string            `yaml:"isolation_root"`
	// Gates is the optional per-gate override map (spec 112 R1), keyed by
	// one of PanelGateKeys. Absent or present-but-empty are EQUIVALENT
	// everywhere in this package and its callers: every "gates is
	// configured" predicate keys off len(Gates) > 0, never key presence —
	// `gates: {}` behaves identically to an absent `gates:` key. Left empty
	// by DefaultConfig; loadUncached backfills it with nothing (absent means
	// "resolve the global list or the per-gate default size").
	Gates map[string]GatePanel `yaml:"gates"`
	// Note is optional free-text advisory metadata (spec 112 R1) — e.g.
	// "review window" — so a point-in-time reviewer
	// mix can self-document. Parsed and echoed verbatim by `config show`;
	// never read by any validation or resolver in this package.
	Note string `yaml:"note"`
}

// GatePanel supplies at least one reviewer, threshold, lens or acceptance override.
type GatePanel struct {
	Reviewers []Reviewer `yaml:"reviewers"`
	// ApproveThreshold is the RAW expression, exactly like Panel's; never
	// resolved here.
	ApproveThreshold string      `yaml:"approve_threshold"`
	Lenses           []string    `yaml:"lenses"`
	Acceptance       *Acceptance `yaml:"acceptance"`
}

// PanelGateKeys is the closed, ordered enum of panel.gates keys (spec 112
// R1, ADR-0034). These name REVIEW EVENTS and deliberately do not copy
// loop.gate_authority's vocabulary, which names APPROVAL ACTS
// (bead_merge/impl_approve). Exported because Bead 3's consumers
// (cmd/mindspec, internal/complete) cannot reference a package-private
// slice; declaration order is the single source for validation, recovery
// lines, and config-show's enum-order rendering. This is the one place the
// enum is declared — never duplicate it.
var PanelGateKeys = []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc"}

// Reviewer declares a family, a model, or both. A missing family uses the harness.
// Model and lens names are open vocabulary. An absent Count defaults to one.
type Reviewer struct {
	Family string `yaml:"family"`
	Model  string `yaml:"model"`
	Lens   string `yaml:"lens"`
	Count  *int   `yaml:"count"`
}

// CountValue returns the reviewer's resolved expanded count: an absent
// Count (nil) defaults to 1. Exported because cmd/mindspec's config-show
// renderer is an out-of-package consumer of this value (Go forbids a
// same-named Count() method beside the Count field) — every consumer
// (validation, resolvers, slot expansion, and that renderer) goes through
// this one accessor so the nil-vs-explicit-zero distinction is decided in
// exactly one place.
func (r Reviewer) CountValue() int {
	if r.Count == nil {
		return 1
	}
	return *r.Count
}

// model supplies an identity for structural validation. Load also checks the
// raw fields, so an explicit empty model cannot be mistaken for an absent one.
func (r Reviewer) model() string {
	if r.Model != "" {
		return r.Model
	}
	return r.Family
}

// intp returns a pointer to n — a tiny helper so DefaultConfig's reviewer
// literals can populate the pointer-typed Count field inline.
func intp(n int) *int {
	return &n
}

// ReviewerSlot is one expanded reviewer position from
// (*Config).PanelGateReviewerSlots (spec 112 R3): deterministic,
// declaration-ordered, ids "R1".."Rn".
type ReviewerSlot struct {
	Slot   string
	Family string
	Model  string
	Lens   string
}

// commandOrder is the stable rendering order CommandLines/
// RenderBuildTestSection apply: the documented vocabulary keys (build,
// test, ci) first when present, then every other declared key sorted — the
// single ordering rule so two independent renderers (bootstrap's starter
// AGENTS.md, setup's managed block) can never disagree about order (spec
// 123 R7b; "ci" added by spec 126 R8a).
var commandOrder = []string{"build", "test", "ci"}

// hasNonBlankEntry reports whether m has at least one entry whose key AND
// value are both non-blank after trimming (spec 123 FX-2, empty≠declared):
// a `commands:\n  build: ""` (or `models:\n  authoring: ""`) has map-length
// 1 but declares NO runnable command / real model id, so it must NOT count
// as "declared". Callers (HasDeclaredModels/HasDeclaredCommands, doctor's
// missing-X checks, CommandLines) use this instead of a bare len() > 0.
func hasNonBlankEntry(m map[string]string) bool {
	for k, v := range m {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// HasDeclaredModels reports whether models: carries at least one
// non-blank phase→model-id mapping (spec 123 R6c/FX-2). An all-blank
// map (e.g. `models:\n  authoring: ""`) is NOT declared — doctor's
// missing-models Warn must still fire.
func (c *Config) HasDeclaredModels() bool { return hasNonBlankEntry(c.Models) }

// HasDeclaredCommands reports whether commands: carries at least one
// non-blank task→command mapping (spec 123 R7c/FX-2). An all-blank map
// is NOT declared — doctor's missing-commands Warn must still fire and
// the managed AGENTS.md Build & Test section must be omitted (never a
// runnable-command-less section).
func (c *Config) HasDeclaredCommands() bool { return hasNonBlankEntry(c.Commands) }

// CommandLines returns c.Commands rendered as "<command>   # <task>"
// lines in the stable commandOrder (build, test, then remaining keys
// sorted). Entries with a blank key OR a blank (trimmed-empty) command
// value are SKIPPED (spec 123 FX-2): a blank command is not runnable, so
// it must never reach a generated Build & Test section. Each command
// value is routed through termsafe.Escape before rendering: Commands is
// operator/agent-declared free text reaching a generated document, the
// same class of value `mindspec config show` already escapes for Models
// (spec 116 AC6's safe-set/quote rule). Returns nil when Commands is
// empty/unset OR every entry is blank.
func (c *Config) CommandLines() []string {
	if len(c.Commands) == 0 {
		return nil
	}
	// nonBlank reports whether a task key names a real, runnable command.
	nonBlank := func(k string) bool {
		return strings.TrimSpace(k) != "" && strings.TrimSpace(c.Commands[k]) != ""
	}
	seen := make(map[string]bool, len(commandOrder))
	ordered := make([]string, 0, len(c.Commands))
	for _, k := range commandOrder {
		if _, ok := c.Commands[k]; ok && nonBlank(k) {
			ordered = append(ordered, k)
			seen[k] = true
		}
	}
	rest := make([]string, 0, len(c.Commands))
	for k := range c.Commands {
		if !seen[k] && nonBlank(k) {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	ordered = append(ordered, rest...)

	if len(ordered) == 0 {
		return nil
	}
	lines := make([]string, 0, len(ordered))
	for _, k := range ordered {
		lines = append(lines, fmt.Sprintf("%s   # %s", termsafe.Escape(c.Commands[k]), termsafe.Escape(k)))
	}
	return lines
}

// RenderBuildTestSection renders the managed "Build & Test" markdown
// section at the given heading depth (2 for a top-level managed block —
// bootstrap's starterAgentsMD, setup's agentsMDManagedBlock — 3 for
// content nested under a parent heading — bootstrap's appendAgentsBlock)
// or "" when Commands is unset (spec 123 R7b/AC-13/AC-14): this is the
// ONE renderer every managed-content call site uses, so mindspec never
// hardcodes its OWN build/test commands (ADR-0040's consumer-identity
// clause) and an unset declaration OMITS the section rather than
// rendering a placeholder that reads as runnable. The returned text
// starts with a leading blank line and ends immediately after the
// closing code fence (no trailing blank line) — callers own their own
// surrounding spacing.
func (c *Config) RenderBuildTestSection(level int) string {
	lines := c.CommandLines()
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(strings.Repeat("#", level))
	b.WriteString(" Build & Test\n\n```bash\n")
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	b.WriteString("```\n")
	return b.String()
}

// Loop is the orchestration-loop governance skeleton (spec 109 R4, the
// 2026-07-02 loop-engineering research §3.2 L3 / §3.3). Every key here is
// INERT in this spec: parsed, defaulted, validated, and surfaced by
// `mindspec config show`, but nothing reads it to change behavior —
// in-binary enforcement (gate authority at the approve gates, halt-condition
// checking, budget ceilings, controller handoff) is a later spec's work.
type Loop struct {
	Enabled bool `yaml:"enabled"`
	// GateAuthority names, for each of the four single-bead-lifecycle gates
	// ADR-0034 collapsed (spec_approve/plan_approve/bead_merge/
	// impl_approve), who may pass it unattended: "panel" or "human",
	// default "human". "panel_skip" is deliberately NOT a valid key here —
	// panel-skip stays permanently human (MINDSPEC_SKIP_PANEL is env-only,
	// ADR-0037 §7) and Load refuses it.
	GateAuthority map[string]string `yaml:"gate_authority"`
	Halt          Halt              `yaml:"halt"`
	Budget        Budget            `yaml:"budget"`
	Context       LoopContext       `yaml:"context"`
	// HandoffLog is the research-§3.2-L3 morning-handoff path, default
	// "AUTOPILOT-LOG.md".
	HandoffLog string `yaml:"handoff_log"`
}

// Halt names the loop's halt conditions, matching ms-spec-autopilot
// practice and research §3.2. OnReject must be "halt" — Load refuses any
// other value (a REJECT halts the track; auto-fixing a rejection is
// verification debt).
type Halt struct {
	MaxRoundsPerBead           int    `yaml:"max_rounds_per_bead"`
	PanelDeadlockRounds        int    `yaml:"panel_deadlock_rounds"`
	MaxConsecutiveImplFailures int    `yaml:"max_consecutive_impl_failures"`
	OnReject                   string `yaml:"on_reject"`
}

// Budget names the loop's per-wake ceilings. Both fields default to 0,
// meaning unset/unlimited.
type Budget struct {
	MaxBeadsPerWake int `yaml:"max_beads_per_wake"`
	TokenBudget     int `yaml:"token_budget"`
}

// LoopContext names the controller's context-handoff policy.
type LoopContext struct {
	ControllerHandoff string `yaml:"controller_handoff"`
}

// Decomposition holds advisory thresholds for plan decomposition-quality
// warnings emitted by `mindspec validate plan`. All thresholds are
// non-gating: they only produce warnings. Defaults match the original
// hard-coded values (Spec 076). Thresholds use strict comparisons (`>` /
// `<`, not `>=` / `<=`), so a project that wants exactly N beads to be
// the cap should set `max_beads: N-1`.
type Decomposition struct {
	MaxBeads        int     `yaml:"max_beads"`         // default 6  (>) bead-count warn
	MaxScopeOverlap float64 `yaml:"max_scope_overlap"` // default 0.50 (>) high-overlap warn
	MinScopeOverlap float64 `yaml:"min_scope_overlap"` // default 0.15 (<) low-overlap warn
	MaxChainDepth   int     `yaml:"max_chain_depth"`   // default 3  (>) deep-serial warn
	MinParallelism  float64 `yaml:"min_parallelism"`   // default 0.25 (<) low-parallelism warn
}

// Recording controls whether spec recording is active.
type Recording struct {
	Enabled bool `yaml:"enabled"`
}

// Enforcement controls which enforcement layers are active.
type Enforcement struct {
	PreCommitHook bool `yaml:"pre_commit_hook"`
	CLIGuards     bool `yaml:"cli_guards"`
	AgentHooks    bool `yaml:"agent_hooks"`
	// PanelGate toggles the PreToolUse pre-complete panel gate (Spec 093
	// Req 13c, ADR-0037). Default true; `enforcement.panel_gate: false` is
	// the persistent opt-out, mirroring PreCommitHook. Like that field, an
	// absent key in config.yaml retains the DefaultConfig true (yaml.v3
	// leaves pre-populated struct fields untouched for absent keys); an
	// explicit `false` disables it.
	PanelGate bool `yaml:"panel_gate"`
}

// DefaultConfig returns a Config with all defaults applied.
func DefaultConfig() *Config {
	return &Config{
		ProtectedBranches:   []string{"main", "master"},
		MergeStrategy:       "auto",
		WorktreeRoot:        ".worktrees",
		AutoOpenFinalizePR:  true,
		AutoMergeFinalizePR: false,
		Enforcement: Enforcement{
			PreCommitHook: true,
			CLIGuards:     true,
			AgentHooks:    true,
			PanelGate:     true,
		},
		Decomposition: Decomposition{
			MaxBeads:        6,
			MaxScopeOverlap: 0.50,
			MinScopeOverlap: 0.15,
			MaxChainDepth:   3,
			MinParallelism:  0.25,
		},
		Panel: Panel{
			ApproveThreshold: "n-1",
			Runners:          []Runner{{Family: "harness", Kind: "harness"}},
			Lenses:           defaultLensData().Lenses,
			IsolationRoot:    ".mindspec-scratch",
			// Reviewers stays empty: each gate supplies its own default size
			// (spec 130 R2.1). Gates and Note are opt-in.
		},
		Models:   map[string]string{},
		Commands: map[string]string{},
		Loop: Loop{
			Enabled: false,
			GateAuthority: map[string]string{
				"spec_approve": "human",
				"plan_approve": "human",
				"bead_merge":   "human",
				"impl_approve": "human",
			},
			Halt: Halt{
				MaxRoundsPerBead:           3,
				PanelDeadlockRounds:        2,
				MaxConsecutiveImplFailures: 2,
				OnReject:                   "halt",
			},
			Budget: Budget{
				MaxBeadsPerWake: 0,
				TokenBudget:     0,
			},
			Context:    LoopContext{ControllerHandoff: "per-spec"},
			HandoffLog: "AUTOPILOT-LOG.md",
		},
		Runner: runnerSkillsAdapter,
	}
}

// cachedConfig holds a memoized Load result (config and/or error) for a given root.
type cachedConfig struct {
	cfg *Config
	err error
}

var (
	configCacheMu sync.Mutex
	configCache   = map[string]cachedConfig{}
)

// Load reads .mindspec/config.yaml under root and returns the parsed Config.
// Results are cached per absolute root path for the lifetime of the process so
// repeated calls within a single CLI invocation do not re-read or re-parse the
// file. Callers must not mutate the returned *Config — it is shared across all
// callers under the same root. Use ResetCache to invalidate (tests, future
// daemon use).
//
// Returns DefaultConfig if the file does not exist.
func Load(root string) (*Config, error) {
	key, absErr := filepath.Abs(root)
	if absErr != nil {
		// Fall back to uncached load if Abs fails — we cannot safely key the cache.
		return loadUncached(root)
	}

	configCacheMu.Lock()
	if entry, ok := configCache[key]; ok {
		configCacheMu.Unlock()
		return entry.cfg, entry.err
	}
	configCacheMu.Unlock()

	cfg, loadErr := loadUncached(root)

	configCacheMu.Lock()
	configCache[key] = cachedConfig{cfg: cfg, err: loadErr}
	configCacheMu.Unlock()

	return cfg, loadErr
}

// ResetCache clears the per-process Load cache. Intended for tests that mutate
// .mindspec/config.yaml between Load calls and for any future long-running
// daemon that needs to pick up on-disk edits.
func ResetCache() {
	configCacheMu.Lock()
	defer configCacheMu.Unlock()
	configCache = map[string]cachedConfig{}
}

// loadUncached is the actual reader/parser. Not cached; prefer Load.
func loadUncached(root string) (*Config, error) {
	path := filepath.Join(root, ".mindspec", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		data = nil
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := loadPanelLayers(root, data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for zero-value fields
	if len(cfg.ProtectedBranches) == 0 {
		cfg.ProtectedBranches = DefaultConfig().ProtectedBranches
	}
	if cfg.MergeStrategy == "" {
		cfg.MergeStrategy = "auto"
	}
	if cfg.WorktreeRoot == "" {
		cfg.WorktreeRoot = ".worktrees"
	}
	// Validate relative worktree paths before path composition.
	if err := containment.ValidateWorktreeRoot(cfg.WorktreeRoot); err != nil {
		return nil, termsafe.NewFailure(fmt.Sprintf("invalid worktree_root: %s", err.Error()), containment.RejectionLever)
	}

	// Decomposition thresholds: per-field zero-value backfill. Explicit `0`
	// in YAML is treated as "unset" (same convention as MergeStrategy /
	// WorktreeRoot). Projects that want effectively-disabled checks should
	// pick large/small sentinel values rather than 0.
	d := &cfg.Decomposition
	defD := DefaultConfig().Decomposition
	if d.MaxBeads == 0 {
		d.MaxBeads = defD.MaxBeads
	}
	if d.MaxScopeOverlap == 0 {
		d.MaxScopeOverlap = defD.MaxScopeOverlap
	}
	if d.MinScopeOverlap == 0 {
		d.MinScopeOverlap = defD.MinScopeOverlap
	}
	if d.MaxChainDepth == 0 {
		d.MaxChainDepth = defD.MaxChainDepth
	}
	if d.MinParallelism == 0 {
		d.MinParallelism = defD.MinParallelism
	}

	// Panel: absent/empty new block resolves to its default (round-trip).
	if cfg.Panel.ApproveThreshold == "" {
		cfg.Panel.ApproveThreshold = "n-1"
	}

	// Loop: per-field zero-value backfill (explicit 0/""/nil is "unset",
	// same convention as Decomposition above). Budget's own default IS 0,
	// so no backfill is needed there.
	if len(cfg.Loop.GateAuthority) == 0 {
		cfg.Loop.GateAuthority = DefaultConfig().Loop.GateAuthority
	}
	if cfg.Loop.Halt.MaxRoundsPerBead == 0 {
		cfg.Loop.Halt.MaxRoundsPerBead = 3
	}
	if cfg.Loop.Halt.PanelDeadlockRounds == 0 {
		cfg.Loop.Halt.PanelDeadlockRounds = 2
	}
	if cfg.Loop.Halt.MaxConsecutiveImplFailures == 0 {
		cfg.Loop.Halt.MaxConsecutiveImplFailures = 2
	}
	if cfg.Loop.Halt.OnReject == "" {
		cfg.Loop.Halt.OnReject = "halt"
	}
	if cfg.Loop.Context.ControllerHandoff == "" {
		cfg.Loop.Context.ControllerHandoff = "per-spec"
	}
	if cfg.Loop.HandoffLog == "" {
		cfg.Loop.HandoffLog = "AUTOPILOT-LOG.md"
	}
	if cfg.Runner == "" {
		cfg.Runner = runnerSkillsAdapter
	}

	if err := validateOrchestration(cfg); err != nil {
		return nil, err
	}

	if err := validatePanelConfig(cfg); err != nil {
		return nil, err
	}
	if isolationTracked != nil {
		tracked, err := isolationTracked(root, cfg.Panel.IsolationRoot)
		if err != nil {
			return nil, termsafe.NewWrappedFailure(err, fmt.Sprintf("checking panel.isolation_root: %v", err), "check the repository index")
		}
		if tracked {
			return nil, termsafe.NewFailure(fmt.Sprintf("panel.isolation_root %q is tracked", cfg.Panel.IsolationRoot), "set panel.isolation_root to an untracked directory")
		}
	}
	return cfg, nil
}

// validateOrchestration enforces the un-weakenable knobs, the
// threshold-skip / reviewer-floor guards (spec 109 R5), and the per-gate
// refusals (spec 112 R4). Every path returns a guard-style error carrying a
// `recovery: <command>` line (ADR-0035), never a panic.
func validateOrchestration(cfg *Config) error {
	if _, ok := cfg.Loop.GateAuthority["panel_skip"]; ok {
		return termsafe.NewFailure("loop.gate_authority may not declare \"panel_skip\": panel-skip is permanently human (MINDSPEC_SKIP_PANEL is env-only)", "remove the panel_skip key from loop.gate_authority in .mindspec/config.yaml")
	}
	if cfg.Loop.Halt.OnReject != "halt" {
		return termsafe.NewFailure(fmt.Sprintf("loop.halt.on_reject must be \"halt\" (got %q): a REJECT halts the track, auto-fixing a rejection is verification debt", cfg.Loop.Halt.OnReject), "set loop.halt.on_reject: halt in .mindspec/config.yaml")
	}
	for k, v := range cfg.Loop.GateAuthority {
		if v != "panel" && v != "human" {
			// R8/AC-20 (spec 120): k is an agent-writable YAML map key —
			// quote at BOTH interpolation points (the :592-area
			// panel.gates precedent), so control bytes can never reach
			// the message or recovery clause raw.
			return termsafe.NewFailure(fmt.Sprintf("loop.gate_authority.%s must be \"panel\" or \"human\" (got %q)", strconv.Quote(k), v), fmt.Sprintf("set loop.gate_authority.%s to panel or human in .mindspec/config.yaml", strconv.Quote(k)))
		}
	}
	if ch := cfg.Loop.Context.ControllerHandoff; ch != "per-spec" && ch != "at-usage-threshold" {
		return termsafe.NewFailure(fmt.Sprintf("loop.context.controller_handoff must be \"per-spec\" or \"at-usage-threshold\" (got %q)", ch), "set loop.context.controller_handoff to per-spec or at-usage-threshold in .mindspec/config.yaml")
	}
	switch cfg.Runner {
	case runnerSkillsAdapter, runnerWorkflowAdapter, runnerExternalAdapter:
	default:
		return termsafe.NewFailure(fmt.Sprintf("runner %q is not a recognized orchestration adapter", cfg.Runner), fmt.Sprintf("set runner to one of %s in .mindspec/config.yaml", strings.Join([]string{runnerSkillsAdapter, runnerWorkflowAdapter, runnerExternalAdapter}, ", ")))
	}

	if err := validateReviewerEntries(cfg.Panel.Reviewers, "panel.reviewers"); err != nil {
		return err
	}

	// Only an explicit global list constrains the legacy global threshold.
	if len(cfg.Panel.Reviewers) > 0 {
		total := 0
		for _, r := range cfg.Panel.Reviewers {
			total += r.CountValue()
		}
		if total < 2 {
			return termsafe.NewFailure(fmt.Sprintf("panel.reviewers must sum to at least 2 (got %d): a one-reviewer panel cannot pass", total), "add at least one more reviewer under panel.reviewers in .mindspec/config.yaml")
		}
		expr := strings.TrimSpace(cfg.Panel.ApproveThreshold)
		if !strings.EqualFold(expr, "n-1") {
			n, err := strconv.Atoi(expr)
			if err != nil || n < 1 || n > total {
				return termsafe.NewFailure(fmt.Sprintf("panel.approve_threshold %q must be \"n-1\" or an integer in [1, %d] (sum of reviewers.count)", cfg.Panel.ApproveThreshold, total), fmt.Sprintf("set panel.approve_threshold to \"n-1\" or an integer between 1 and %d in .mindspec/config.yaml", total))
			}
		}
	}

	return validateGates(cfg)
}

// validateReviewerEntries checks identity, model safety and counts for one list.
// label names the list in refusal messages.
func validateReviewerEntries(reviewers []Reviewer, label string) error {
	for i, r := range reviewers {
		if r.Model != "" {
			if err := ValidateModel(r.Model); err != nil {
				return fmt.Errorf("%s[%d]: %w", label, i, err)
			}
		}
		if r.model() == "" {
			return termsafe.NewFailure(fmt.Sprintf("%s[%d] sets neither \"model\" nor \"family\": every reviewer entry needs an open-vocabulary model identity", label, i), fmt.Sprintf("add a model or family value to %s[%d] in .mindspec/config.yaml", label, i))
		}
		if r.Count != nil && *r.Count < 1 {
			return termsafe.NewFailure(fmt.Sprintf("%s[%d].count must be >= 1 (got %d)", label, i, *r.Count), fmt.Sprintf("set %s[%d].count to at least 1, or remove the count key to default to 1, in .mindspec/config.yaml", label, i))
		}
	}
	return nil
}

// validateGates applies the spec 112 R4 refusals scoped to panel.gates: (a)
// an unknown gates key, (b) a configured gate entry setting neither
// reviewers, approve_threshold, lenses or acceptance, (e) a per-gate reviewers list whose own
// expanded sum is < 2. Legacy gate thresholds are not range-checked
// (spec 130 R2.1). (c)/(d) ride
// validateReviewerEntries for each gate's own reviewers list.
func validateGates(cfg *Config) error {
	unknown := make([]string, 0, len(cfg.Panel.Gates))
	for k := range cfg.Panel.Gates {
		if !isValidGateKey(k) {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		// unknown[0] is an attacker/typo-controlled YAML map key: it is
		// quoted via %q in the message clause, and MUST be quoted again
		// (not %s) in the recovery clause below — an unescaped repeat is
		// exactly the terminal-injection hole a control-byte-carrying key
		// (ESC/BEL/embedded newline) would otherwise punch through the
		// guard-recovery line (round-1 panel G1.1).
		return termsafe.NewFailure(fmt.Sprintf("panel.gates key %q is not one of the five valid gate keys (%s) — distinct from loop.gate_authority's bead_merge/impl_approve vocabulary, which names approval acts rather than review events", unknown[0], strings.Join(PanelGateKeys, ", ")), fmt.Sprintf("rename the panel.gates key %s to one of %s in .mindspec/config.yaml", strconv.Quote(unknown[0]), strings.Join(PanelGateKeys, ", ")))
	}

	for _, gate := range PanelGateKeys {
		gp, ok := cfg.Panel.Gates[gate]
		if !ok {
			continue
		}
		hasReviewers := len(gp.Reviewers) > 0
		hasThreshold := strings.TrimSpace(gp.ApproveThreshold) != ""
		if !hasReviewers && !hasThreshold && len(gp.Lenses) == 0 && gp.Acceptance == nil {
			return termsafe.NewFailure(fmt.Sprintf("panel.gates.%s sets none of reviewers, approve_threshold, lenses or acceptance", gate), fmt.Sprintf("set at least one field under panel.gates.%s in .mindspec/config.yaml, or remove the empty panel.gates.%s entry", gate, gate))
		}

		if hasReviewers {
			if err := validateReviewerEntries(gp.Reviewers, fmt.Sprintf("panel.gates.%s.reviewers", gate)); err != nil {
				return err
			}
			ownSum := 0
			for _, r := range gp.Reviewers {
				ownSum += r.CountValue()
			}
			if ownSum < 2 {
				return termsafe.NewFailure(fmt.Sprintf("panel.gates.%s.reviewers must sum to at least 2 (got %d): a one-reviewer panel cannot pass", gate, ownSum), fmt.Sprintf("add at least one more reviewer under panel.gates.%s.reviewers in .mindspec/config.yaml", gate))
			}
		}
	}

	return nil
}

// IsProtectedBranch returns true if the given branch name is in the protected list.
func (c *Config) IsProtectedBranch(branch string) bool {
	for _, b := range c.ProtectedBranches {
		if b == branch {
			return true
		}
	}
	return false
}

// WorktreePath returns the absolute path for a worktree with the given name
// under the configured worktree root.
func (c *Config) WorktreePath(root, name string) string {
	return filepath.Join(root, c.WorktreeRoot, name)
}

// PanelExpectedReviewers returns the no-gate creation-time reviewer count.
// A create without --gate uses adhoc resolution, including explicit bead
// inheritance, and defaults to 6 (spec 130 R2.1).
func (c *Config) PanelExpectedReviewers() int {
	n, _ := c.PanelGateExpectedReviewers("adhoc")
	return n
}

// PanelApproveThresholdExpr returns the RAW panel.approve_threshold
// expression ("n-1" or an integer string) exactly as configured, defaulting
// to "n-1" when unset. It does NOT resolve the expression to an int —
// resolution is single-homed in internal/panel.Panel.ApproveThreshold (spec
// 109 R7); a second interpreter here would risk drifting from that one.
func (c *Config) PanelApproveThresholdExpr() string {
	if strings.TrimSpace(c.Panel.ApproveThreshold) == "" {
		return "n-1"
	}
	return c.Panel.ApproveThreshold
}

// isValidGateKey reports whether gate is one of the five PanelGateKeys.
func isValidGateKey(gate string) bool {
	for _, k := range PanelGateKeys {
		if k == gate {
			return true
		}
	}
	return false
}

// gateKeyError is the shared ADR-0035 refusal for a gate name outside
// PanelGateKeys, used by every gate-scoped resolver (spec 112 R3): a caller
// typo (e.g. "final" for "final_review") must fail loud, never silently
// fall back to defaults.
func gateKeyError(gate string) error {
	return termsafe.NewFailure(fmt.Sprintf("gate %q is not one of the five valid panel gate keys (%s)", gate, strings.Join(PanelGateKeys, ", ")), fmt.Sprintf("pass one of %s to the gate-scoped config resolver, or fix the panel.gates key spelling in .mindspec/config.yaml", strings.Join(PanelGateKeys, ", ")))
}

// resolveGateReviewers selects an explicit gate list, an explicit bead list
// for adhoc, or the explicit global list. Without a list, the gate's own
// default size is filled round-robin over the configured runner order.
// gate is assumed already validated against PanelGateKeys by the caller.
func (c *Config) resolveGateReviewers(gate string) []Reviewer {
	if reviewers := c.explicitGateReviewers(gate); len(reviewers) > 0 {
		return reviewers
	}
	size := 6
	if gate == "bead" {
		size = 4
	}
	return c.runnerReviewers(size)
}

func (c *Config) explicitGateReviewers(gate string) []Reviewer {
	if gp, ok := c.Panel.Gates[gate]; ok && len(gp.Reviewers) > 0 {
		return gp.Reviewers
	}
	if gate == "adhoc" {
		if gp, ok := c.Panel.Gates["bead"]; ok && len(gp.Reviewers) > 0 {
			return gp.Reviewers
		}
	}
	if len(c.Panel.Reviewers) > 0 {
		return c.Panel.Reviewers
	}
	return nil
}

func (c *Config) runnerReviewers(size int) []Reviewer {
	runners := c.runners()
	families := make([]string, len(runners))
	for i, runner := range runners {
		families[i] = runner.Family
	}
	reviewers := make([]Reviewer, size)
	for i := range reviewers {
		reviewers[i] = Reviewer{Family: families[i%len(families)]}
	}
	return reviewers
}

// resolveGateThresholdExpr walks the R3 per-field resolution chain for a
// gate's RAW approve_threshold expression: the gate's own configured
// expression -> (adhoc only) bead's resolved expression -> the global
// expression (with its own built-in "n-1" fallback via
// PanelApproveThresholdExpr). Never resolves to an int — resolution to an
// int stays single-homed in internal/panel.Panel.ApproveThreshold
// (ADR-0037 §3). gate is assumed already validated against PanelGateKeys.
func (c *Config) resolveGateThresholdExpr(gate string) string {
	if gp, ok := c.Panel.Gates[gate]; ok && strings.TrimSpace(gp.ApproveThreshold) != "" {
		return gp.ApproveThreshold
	}
	if gate == "adhoc" {
		return c.resolveGateThresholdExpr("bead")
	}
	return c.PanelApproveThresholdExpr()
}

// PanelGateExpectedReviewers returns the expanded reviewer-slot count for
// gate — the creation-time default the spec-110 panel.json writer and
// ms-panel-run step 0 will stamp for that gate — resolved through the R3
// per-field chain. Always equal to len(PanelGateReviewerSlots(gate)) by
// construction (spec 112 R3). Returns an error for a gate name outside
// PanelGateKeys.
func (c *Config) PanelGateExpectedReviewers(gate string) (int, error) {
	if !isValidGateKey(gate) {
		return 0, gateKeyError(gate)
	}
	total := 0
	for _, r := range c.resolveGateReviewers(gate) {
		total += r.CountValue()
	}
	return total, nil
}

// PanelGateApproveThresholdExpr returns gate's RAW approve_threshold
// expression, resolved through the R3 per-field chain — never a resolved
// integer (spec 112 R3; ADR-0037 §3's threshold single home stays
// internal/panel.Panel.ApproveThreshold). Returns an error for a gate name
// outside PanelGateKeys.
func (c *Config) PanelGateApproveThresholdExpr(gate string) (string, error) {
	if !isValidGateKey(gate) {
		return "", gateKeyError(gate)
	}
	return c.resolveGateThresholdExpr(gate), nil
}

// PanelGateReviewerSlots optionally fills an explicit count round-robin over
// the resolved reviewer entries, ignoring their counts. With no explicit list,
// it fills directly over the merged runner order. Without an override it
// returns gate's deterministic, expanded reviewer
// slots (spec 112 R3), resolved through the same per-field chain as
// PanelGateExpectedReviewers. Returns an error for a gate name outside
// PanelGateKeys.
func (c *Config) PanelGateReviewerSlots(gate string, count ...int) ([]ReviewerSlot, error) {
	if !isValidGateKey(gate) {
		return nil, gateKeyError(gate)
	}
	reviewers := c.resolveGateReviewers(gate)
	if len(count) > 0 {
		if len(count) != 1 || count[0] < 2 {
			return nil, fmt.Errorf("a panel needs at least two reviewers; use --reviewers 2")
		}
		resized := make([]Reviewer, count[0])
		if len(c.explicitGateReviewers(gate)) == 0 {
			reviewers = c.runnerReviewers(count[0])
		}
		for i := range resized {
			resized[i] = reviewers[i%len(reviewers)]
			resized[i].Count = nil
		}
		reviewers = resized
	}
	return c.expandSlots(gate, reviewers)
}

// expandSlots preserves explicit lenses and advances the cursor only for missing lenses.
func (c *Config) expandSlots(gate string, reviewers []Reviewer) ([]ReviewerSlot, error) {
	var slots []ReviewerSlot
	lenses := c.GateLenses(gate)
	cursor := 0
	for i, r := range reviewers {
		family := r.Family
		if family == "" {
			family = "harness"
		}
		if _, ok := c.RunnerFor(family); !ok {
			return nil, termsafe.NewFailure(fmt.Sprintf("%s[%d] names family %q with no runner", c.reviewerLabel(gate), i, family), "add a runner for the family to panel.runners")
		}
		model := r.Model
		if model == "" {
			model = c.SubstituteModel(family)
		}
		if err := ValidateModel(model); err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", c.reviewerLabel(gate), i, err)
		}
		for j := 0; j < r.CountValue(); j++ {
			lens := r.Lens
			if lens == "" {
				lens = lenses[cursor%len(lenses)]
				cursor++
			}
			slots = append(slots, ReviewerSlot{Slot: fmt.Sprintf("R%d", len(slots)+1), Family: family, Model: model, Lens: lens})
		}
	}
	return slots, nil
}

// expandSlots retains the package's default expansion helper for callers with validated entries.
func expandSlots(reviewers []Reviewer) []ReviewerSlot {
	slots, _ := DefaultConfig().expandSlots("bead", reviewers)
	return slots
}

// PanelGateAdvisoryDefault is the single-home selection rule (spec 112 R7)
// both Bead 3 caller-side ReviewerCountNote advisory sites resolve through,
// so they cannot drift from each other. recordedGate is the panel.json
// Panel.Gate value (possibly empty or outside PanelGateKeys); isBead is
// whether the panel is a bead panel (Panel.IsBead()). Returns (0, false)
// when the advisory should be SKIPPED — the caller must not print a note
// in that case. Never calls a gate-scoped resolver with a value outside
// PanelGateKeys, so no resolver error can surface through this helper.
//
//   - recordedGate is a known gate key: that gate's PanelGateExpectedReviewers, true.
//   - recordedGate is empty and isBead: the "bead" gate's PanelGateExpectedReviewers, true.
//   - a legacy non-bead panel with no recorded gate and no gate overrides
//     compares against the adhoc default, preserving its historical advisory.
//   - a non-bead panel with no recorded gate and configured gate overrides,
//     or a non-empty recorded value outside the enum: (0, false) — skip the note.
func (c *Config) PanelGateAdvisoryDefault(recordedGate string, isBead bool) (int, bool) {
	if isValidGateKey(recordedGate) {
		n, err := c.PanelGateExpectedReviewers(recordedGate)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	if recordedGate == "" && isBead {
		n, err := c.PanelGateExpectedReviewers("bead")
		if err != nil {
			return 0, false
		}
		return n, true
	}
	if recordedGate == "" && len(c.Panel.Gates) == 0 {
		return c.PanelExpectedReviewers(), true
	}
	return 0, false
}
