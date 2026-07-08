package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config represents .mindspec/config.yaml settings.
type Config struct {
	ProtectedBranches []string      `yaml:"protected_branches"`
	MergeStrategy     string        `yaml:"merge_strategy"`
	WorktreeRoot      string        `yaml:"worktree_root"`
	AutoFinalize      bool          `yaml:"auto_finalize"`
	Enforcement       Enforcement   `yaml:"enforcement"`
	Recording         Recording     `yaml:"recording"`
	Decomposition     Decomposition `yaml:"decomposition"`

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
}

// Panel declares the review-panel creation-time defaults (spec 109 R2): the
// reviewer mix, the approve-threshold expression, and the
// quota-substitution policy. These seed a fresh panel.json (the spec-110
// writer); an already-recorded panel's own fields are the sole authority
// for its gate decision.
type Panel struct {
	Reviewers []Reviewer `yaml:"reviewers"`
	// ApproveThreshold is the RAW expression ("n-1" or an integer string),
	// never resolved here. Expression resolution is single-homed in
	// internal/panel.Panel.ApproveThreshold; see PanelApproveThresholdExpr.
	ApproveThreshold string       `yaml:"approve_threshold"`
	Substitution     Substitution `yaml:"substitution"`
}

// Reviewer is one entry of the panel.reviewers mix.
type Reviewer struct {
	Family string `yaml:"family"`
	Count  int    `yaml:"count"`
}

// Substitution controls quota-driven reviewer-substitution policy.
type Substitution struct {
	ClaudeSubOnQuota bool `yaml:"claude_sub_on_quota"`
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
		ProtectedBranches: []string{"main", "master"},
		MergeStrategy:     "auto",
		WorktreeRoot:      ".worktrees",
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
			Reviewers: []Reviewer{
				{Family: "claude", Count: 3},
				{Family: "codex", Count: 3},
			},
			ApproveThreshold: "n-1",
			Substitution:     Substitution{ClaudeSubOnQuota: true},
		},
		Models: map[string]string{},
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
		Runner: "claude-code-skills",
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
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
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
	// Substitution.ClaudeSubOnQuota needs no backfill of its own: like
	// Enforcement.PanelGate above, the pre-populated `true` default
	// survives an absent key (yaml.v3 leaves it untouched) and an explicit
	// `false` intentionally overrides it.
	if len(cfg.Panel.Reviewers) == 0 {
		cfg.Panel.Reviewers = DefaultConfig().Panel.Reviewers
	}
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
		cfg.Runner = "claude-code-skills"
	}

	if err := validateOrchestration(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateOrchestration enforces the un-weakenable knobs and the
// threshold-skip / reviewer-floor guards (spec 109 R5). Every path returns a
// guard-style error carrying a `recovery: <command>` line (ADR-0035), never
// a panic.
func validateOrchestration(cfg *Config) error {
	if _, ok := cfg.Loop.GateAuthority["panel_skip"]; ok {
		return fmt.Errorf("loop.gate_authority may not declare \"panel_skip\": panel-skip is permanently human (MINDSPEC_SKIP_PANEL is env-only, ADR-0037 §7)\nrecovery: remove the panel_skip key from loop.gate_authority in .mindspec/config.yaml")
	}
	if cfg.Loop.Halt.OnReject != "halt" {
		return fmt.Errorf("loop.halt.on_reject must be \"halt\" (got %q): a REJECT halts the track, auto-fixing a rejection is verification debt\nrecovery: set loop.halt.on_reject: halt in .mindspec/config.yaml", cfg.Loop.Halt.OnReject)
	}
	for k, v := range cfg.Loop.GateAuthority {
		if v != "panel" && v != "human" {
			return fmt.Errorf("loop.gate_authority.%s must be \"panel\" or \"human\" (got %q)\nrecovery: set loop.gate_authority.%s to panel or human in .mindspec/config.yaml", k, v, k)
		}
	}
	if ch := cfg.Loop.Context.ControllerHandoff; ch != "per-spec" && ch != "at-usage-threshold" {
		return fmt.Errorf("loop.context.controller_handoff must be \"per-spec\" or \"at-usage-threshold\" (got %q)\nrecovery: set loop.context.controller_handoff to per-spec or at-usage-threshold in .mindspec/config.yaml", ch)
	}
	switch cfg.Runner {
	case "claude-code-skills", "claude-code-workflow", "external":
	default:
		return fmt.Errorf("runner %q is not a recognized orchestration adapter (want claude-code-skills, claude-code-workflow, or external)\nrecovery: set runner to claude-code-skills, claude-code-workflow, or external in .mindspec/config.yaml", cfg.Runner)
	}

	total := 0
	for _, r := range cfg.Panel.Reviewers {
		total += r.Count
	}
	expr := strings.TrimSpace(cfg.Panel.ApproveThreshold)
	if !strings.EqualFold(expr, "n-1") {
		n, err := strconv.Atoi(expr)
		if err != nil || n < 1 || n > total {
			return fmt.Errorf("panel.approve_threshold %q must be \"n-1\" or an integer in [1, %d] (sum of reviewers.count)\nrecovery: set panel.approve_threshold to \"n-1\" or an integer between 1 and %d in .mindspec/config.yaml", cfg.Panel.ApproveThreshold, total, total)
		}
	}

	for _, r := range cfg.Panel.Reviewers {
		if r.Count < 1 {
			return fmt.Errorf("panel.reviewers[%s].count must be >= 1 (got %d)\nrecovery: set panel.reviewers.%s.count to at least 1, or remove that entry, in .mindspec/config.yaml", r.Family, r.Count, r.Family)
		}
	}
	if total < 2 {
		return fmt.Errorf("panel.reviewers must sum to at least 2 (got %d): a single-reviewer panel makes the default \"n-1\" threshold resolve to 0, an always-pass gate\nrecovery: add at least one more reviewer under panel.reviewers in .mindspec/config.yaml", total)
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

// PanelExpectedReviewers returns the sum of the configured panel.reviewers
// counts — the creation-time default the spec-110 panel.json writer will
// stamp as a fresh panel's expected_reviewers. Defaults to 6 (the 3+3 mix)
// when Reviewers is unset, independent of whether this Config went through
// Load's backfill.
func (c *Config) PanelExpectedReviewers() int {
	if len(c.Panel.Reviewers) == 0 {
		return 6
	}
	total := 0
	for _, r := range c.Panel.Reviewers {
		total += r.Count
	}
	return total
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
