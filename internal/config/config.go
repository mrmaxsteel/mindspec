package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// Panel declares the review-panel creation-time defaults (spec 109 R2,
// extended by spec 112 R1). Reviewers/ApproveThreshold/Substitution are the
// ALL-GATES default; Gates overrides them per lifecycle gate. These seed a
// fresh panel.json (the spec-110 writer); an already-recorded panel's own
// fields are the sole authority for its gate decision.
type Panel struct {
	Reviewers []Reviewer `yaml:"reviewers"`
	// ApproveThreshold is the RAW expression ("n-1" or an integer string),
	// never resolved here. Expression resolution is single-homed in
	// internal/panel.Panel.ApproveThreshold; see PanelApproveThresholdExpr.
	ApproveThreshold string       `yaml:"approve_threshold"`
	Substitution     Substitution `yaml:"substitution"`
	// Gates is the optional per-gate override map (spec 112 R1), keyed by
	// one of PanelGateKeys. Absent or present-but-empty are EQUIVALENT
	// everywhere in this package and its callers: every "gates is
	// configured" predicate keys off len(Gates) > 0, never key presence —
	// `gates: {}` behaves identically to an absent `gates:` key. Left empty
	// by DefaultConfig; loadUncached backfills it with nothing (absent means
	// "use the global default", not "use some other default").
	Gates map[string]GatePanel `yaml:"gates"`
	// Note is optional free-text advisory metadata (spec 112 R1) — e.g.
	// "fable-window 2026-07, codex-enabled" — so a point-in-time reviewer
	// mix can self-document. Parsed and echoed verbatim by `config show`;
	// never read by any validation or resolver in this package.
	Note string `yaml:"note"`
}

// GatePanel is one entry of panel.gates: a per-gate override of the
// reviewer mix and/or the approve-threshold expression (spec 112 R1). A
// configured gate must set at least one of the two fields — `Load` refuses
// an entry that sets neither (R4b).
type GatePanel struct {
	Reviewers []Reviewer `yaml:"reviewers"`
	// ApproveThreshold is the RAW expression, exactly like Panel's; never
	// resolved here.
	ApproveThreshold string `yaml:"approve_threshold"`
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

// Reviewer is one entry of a reviewer mix (global panel.reviewers or a
// per-gate panel.gates.<gate>.reviewers list). Model and Lens are both
// open-vocabulary strings — no name-membership validation exists anywhere
// in Load (spec 112 R1, ADR-0040): model ids are runner-specific and ship
// faster than this package can enumerate them. Family is the legacy 109
// field, still parseable; an entry may set Family, Model, or both (Model
// wins for slot expansion — see the unexported model() accessor). Count is
// a pointer so an ABSENT count (nil) is distinguishable from an EXPLICIT
// `count: 0`/negative: nil resolves to 1 (CountValue, the spec 112 R2
// monotone relaxation over 109, which refused count < 1 wholesale);
// non-nil non-positive is refused by Load (R4d).
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

// model returns the reviewer's open-vocabulary model identity for slot
// expansion and validation: Model if set, else the legacy Family string —
// "model wins" when an entry sets both. Returns "" only when neither is
// set, which Load refuses (R4c).
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
	Slot  string
	Model string
	Lens  string
}

// defaultLenses is the interleaved, structural/sharp-alternating default
// lens ordering the slot-expansion cursor walks for lens-less reviewer
// slots (spec 112 R3). The 2026-07-07 live panels showed defect-finding
// power tracked the assigned lens at least as much as the model tier.
var defaultLenses = []string{
	"author-of-record", "empirical-prober", "codebase-pin",
	"adversarial", "contract-stability", "integration",
}

// knownModels is the curated, deliberately non-exhaustive advisory list of
// model ids `config show` (a later bead) warns on absence from. NEVER
// consulted by Load or validateOrchestration (spec 112 R1/R8; ADR-0040's
// open-vocabulary portability principle) — a made-up id must never fail to
// load. New model ids ship faster than this list can be updated (the
// claude-fable-5 precedent).
var knownModels = []string{
	"claude-fable-5", "claude-opus-4-8", "claude-sonnet-5", "gpt-5.5",
	"claude", "codex",
}

// KnownModels returns a copy of the curated known-model advisory list (spec
// 112 R8) — a copy so callers cannot mutate the package-level slice.
func KnownModels() []string {
	out := make([]string, len(knownModels))
	copy(out, knownModels)
	return out
}

// Substitution controls quota-driven reviewer-substitution policy.
type Substitution struct {
	ClaudeSubOnQuota bool `yaml:"claude_sub_on_quota"`
	// Substitutes is the model-level, one-step substitution map (spec 112
	// R5): unavailable-model -> substitute-model, both open-vocabulary and
	// validated as non-empty with key != value (R4h); chains are not
	// followed (a mutual pair A->B, B->A is legal). Precedence is crisp:
	// non-empty Substitutes IS the substitution policy and
	// ClaudeSubOnQuota is inert (the supersession *reporting* half is
	// Bead 3's surface); empty Substitutes means ClaudeSubOnQuota keeps its
	// 109 meaning. Global, not per-gate — unavailability is a fact about
	// the environment, not about a gate (ADR-0040 §3).
	Substitutes map[string]string `yaml:"substitutes"`
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
				{Family: "claude", Count: intp(3)},
				{Family: "codex", Count: intp(3)},
			},
			ApproveThreshold: "n-1",
			Substitution:     Substitution{ClaudeSubOnQuota: true},
			// Gates, Note, and Substitution.Substitutes stay empty here
			// (spec 112 R2): the standing per-gate protocol is the
			// documented example, never the default — an existing
			// install's zero-config panel sizes must not change.
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

// validateOrchestration enforces the un-weakenable knobs, the
// threshold-skip / reviewer-floor guards (spec 109 R5), and the per-gate
// refusals (spec 112 R4). Every path returns a guard-style error carrying a
// `recovery: <command>` line (ADR-0035), never a panic.
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

	if err := validateReviewerEntries(cfg.Panel.Reviewers, "panel.reviewers"); err != nil {
		return err
	}

	total := 0
	for _, r := range cfg.Panel.Reviewers {
		total += r.CountValue()
	}
	expr := strings.TrimSpace(cfg.Panel.ApproveThreshold)
	if !strings.EqualFold(expr, "n-1") {
		n, err := strconv.Atoi(expr)
		if err != nil || n < 1 || n > total {
			return fmt.Errorf("panel.approve_threshold %q must be \"n-1\" or an integer in [1, %d] (sum of reviewers.count)\nrecovery: set panel.approve_threshold to \"n-1\" or an integer between 1 and %d in .mindspec/config.yaml", cfg.Panel.ApproveThreshold, total, total)
		}
	}

	if total < 2 {
		return fmt.Errorf("panel.reviewers must sum to at least 2 (got %d): a single-reviewer panel makes the default \"n-1\" threshold resolve to 0, an always-pass gate\nrecovery: add at least one more reviewer under panel.reviewers in .mindspec/config.yaml", total)
	}

	return validateGates(cfg)
}

// validateReviewerEntries applies the spec 112 R4(c)/(d) refusals — a
// reviewer entry with neither `model` nor `family`, and an explicit
// non-positive `count` — to reviewers, a global or per-gate list. label
// identifies the offending list in the error/recovery text (e.g.
// "panel.reviewers" or "panel.gates.bead.reviewers").
func validateReviewerEntries(reviewers []Reviewer, label string) error {
	for i, r := range reviewers {
		if r.model() == "" {
			return fmt.Errorf("%s[%d] sets neither \"model\" nor \"family\": every reviewer entry needs an open-vocabulary model identity\nrecovery: add a model or family value to %s[%d] in .mindspec/config.yaml", label, i, label, i)
		}
		if r.Count != nil && *r.Count < 1 {
			return fmt.Errorf("%s[%d].count must be >= 1 (got %d)\nrecovery: set %s[%d].count to at least 1, or remove the count key to default to 1, in .mindspec/config.yaml", label, i, *r.Count, label, i)
		}
	}
	return nil
}

// validateGates applies the spec 112 R4 refusals scoped to panel.gates: (a)
// an unknown gates key, (b) a configured gate entry setting neither
// reviewers nor approve_threshold, (e) a per-gate reviewers list whose own
// expanded sum is < 2, and (f)+(g) the resolved threshold-vs-sum check run
// per configured gate over the R3 per-field inheritance chain. (c)/(d) ride
// validateReviewerEntries for each gate's own reviewers list; (h)
// (substitutes) is validated separately.
func validateGates(cfg *Config) error {
	unknown := make([]string, 0, len(cfg.Panel.Gates))
	for k := range cfg.Panel.Gates {
		if !isValidGateKey(k) {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("panel.gates key %q is not one of the five valid gate keys (%s) — distinct from loop.gate_authority's bead_merge/impl_approve vocabulary, which names approval acts rather than review events\nrecovery: rename panel.gates.%s to one of %s in .mindspec/config.yaml", unknown[0], strings.Join(PanelGateKeys, ", "), unknown[0], strings.Join(PanelGateKeys, ", "))
	}

	for _, gate := range PanelGateKeys {
		gp, ok := cfg.Panel.Gates[gate]
		if !ok {
			continue
		}
		hasReviewers := len(gp.Reviewers) > 0
		hasThreshold := strings.TrimSpace(gp.ApproveThreshold) != ""
		if !hasReviewers && !hasThreshold {
			return fmt.Errorf("panel.gates.%s sets neither reviewers nor approve_threshold: a configured gate entry that sets neither is a likely indentation mistake\nrecovery: add a reviewers list or an approve_threshold under panel.gates.%s in .mindspec/config.yaml, or remove the empty panel.gates.%s entry", gate, gate, gate)
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
				return fmt.Errorf("panel.gates.%s.reviewers must sum to at least 2 (got %d): a single-reviewer gate makes the default \"n-1\" threshold resolve to 0, an always-pass gate\nrecovery: add at least one more reviewer under panel.gates.%s.reviewers in .mindspec/config.yaml", gate, ownSum, gate)
			}
		}

		// (f)+(g): the resolved threshold must lie in [1, the resolved
		// reviewer sum] at every link of the adhoc->bead->global chain —
		// run for every configured gate regardless of which field(s) it
		// sets itself, because an inherited half must still respect the
		// INHERITING gate's own resolved sum.
		resolvedExpr := strings.TrimSpace(cfg.resolveGateThresholdExpr(gate))
		if !strings.EqualFold(resolvedExpr, "n-1") {
			resolvedSum := 0
			for _, r := range cfg.resolveGateReviewers(gate) {
				resolvedSum += r.CountValue()
			}
			n, err := strconv.Atoi(resolvedExpr)
			if err != nil || n < 1 || n > resolvedSum {
				return fmt.Errorf("panel.gates.%s's resolved approve_threshold %q must be \"n-1\" or an integer in [1, %d] (that gate's resolved reviewer sum)\nrecovery: set panel.gates.%s.approve_threshold (or the value it inherits from bead/global) to \"n-1\" or an integer between 1 and %d in .mindspec/config.yaml", gate, cfg.resolveGateThresholdExpr(gate), resolvedSum, gate, resolvedSum)
			}
		}
	}

	subKeys := make([]string, 0, len(cfg.Panel.Substitution.Substitutes))
	for k := range cfg.Panel.Substitution.Substitutes {
		subKeys = append(subKeys, k)
	}
	sort.Strings(subKeys)
	for _, k := range subKeys {
		v := cfg.Panel.Substitution.Substitutes[k]
		if k == "" || v == "" {
			return fmt.Errorf("panel.substitution.substitutes has an empty-sided entry (key %q -> value %q): both the unavailable and substitute model ids must be non-empty\nrecovery: fill in both sides of the panel.substitution.substitutes entry, or remove it, in .mindspec/config.yaml", k, v)
		}
		if k == v {
			return fmt.Errorf("panel.substitution.substitutes maps %q to itself: a substitution must name a different model\nrecovery: change panel.substitution.substitutes.%s to a different substitute model id, or remove the self-mapping, in .mindspec/config.yaml", k, k)
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

// PanelExpectedReviewers returns the sum of the configured panel.reviewers
// counts — the creation-time default the spec-110 panel.json writer will
// stamp as a fresh panel's expected_reviewers. Defaults to 6 (the 3+3 mix)
// when Reviewers is unset, independent of whether this Config went through
// Load's backfill.
func (c *Config) PanelExpectedReviewers() int {
	total := 0
	for _, r := range c.globalReviewers() {
		total += r.CountValue()
	}
	return total
}

// globalReviewers returns panel.reviewers with the built-in 3+3 fallback
// (mirroring PanelExpectedReviewers' historical sum semantics as a list
// rather than a sum) — the single "global" tier every gate-scoped resolver
// falls back to.
func (c *Config) globalReviewers() []Reviewer {
	if len(c.Panel.Reviewers) == 0 {
		return DefaultConfig().Panel.Reviewers
	}
	return c.Panel.Reviewers
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
	return fmt.Errorf("gate %q is not one of the five valid panel gate keys (%s)\nrecovery: pass one of %s to the gate-scoped config resolver, or fix the panel.gates key spelling in .mindspec/config.yaml", gate, strings.Join(PanelGateKeys, ", "), strings.Join(PanelGateKeys, ", "))
}

// resolveGateReviewers walks the R3 per-field resolution chain for a gate's
// reviewer list: the gate's own configured reviewers -> (adhoc only)
// bead's resolved reviewers -> the global list (with its own built-in
// fallback via globalReviewers). gate is assumed already validated against
// PanelGateKeys by the caller.
func (c *Config) resolveGateReviewers(gate string) []Reviewer {
	if gp, ok := c.Panel.Gates[gate]; ok && len(gp.Reviewers) > 0 {
		return gp.Reviewers
	}
	if gate == "adhoc" {
		return c.resolveGateReviewers("bead")
	}
	return c.globalReviewers()
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

// PanelGateReviewerSlots returns gate's deterministic, expanded reviewer
// slots (spec 112 R3), resolved through the same per-field chain as
// PanelGateExpectedReviewers. Returns an error for a gate name outside
// PanelGateKeys.
func (c *Config) PanelGateReviewerSlots(gate string) ([]ReviewerSlot, error) {
	if !isValidGateKey(gate) {
		return nil, gateKeyError(gate)
	}
	return expandSlots(c.resolveGateReviewers(gate)), nil
}

// expandSlots deterministically expands reviewers into "R1".."Rn" slots in
// declaration order (spec 112 R3): each entry's CountValue() is expanded;
// an explicit Lens applies to every slot that entry produces and does NOT
// advance the default-lens cursor; a lens-less slot takes
// defaultLenses[cursor % 6], and the single cursor — starting at index 0 —
// advances only over lens-less slots. The worked example (a 9-reviewer,
// all-lens-less, 3-entries-of-count-3 panel) must expand to exactly R1
// author-of-record, R2 empirical-prober, R3 codebase-pin, R4 adversarial,
// R5 contract-stability, R6 integration, R7 author-of-record, R8
// empirical-prober, R9 codebase-pin — this is normative, not illustrative.
func expandSlots(reviewers []Reviewer) []ReviewerSlot {
	var slots []ReviewerSlot
	cursor := 0
	n := 0
	for _, r := range reviewers {
		model := r.model()
		for i := 0; i < r.CountValue(); i++ {
			n++
			lens := r.Lens
			if lens == "" {
				lens = defaultLenses[cursor%len(defaultLenses)]
				cursor++
			}
			slots = append(slots, ReviewerSlot{
				Slot:  fmt.Sprintf("R%d", n),
				Model: model,
				Lens:  lens,
			})
		}
	}
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
//   - len(Gates) == 0 (gates not configured): every panel compares against
//     the global default, exactly as 109 ships it — (PanelExpectedReviewers(), true).
//   - recordedGate is a known gate key: that gate's PanelGateExpectedReviewers, true.
//   - recordedGate is empty and isBead: the "bead" gate's PanelGateExpectedReviewers, true.
//   - anything else (a non-bead panel with no recorded gate, or a recorded
//     value outside the enum): (0, false) — skip the note.
func (c *Config) PanelGateAdvisoryDefault(recordedGate string, isBead bool) (int, bool) {
	if len(c.Panel.Gates) == 0 {
		return c.PanelExpectedReviewers(), true
	}
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
	return 0, false
}
