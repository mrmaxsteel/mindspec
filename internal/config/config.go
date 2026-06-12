package config

import (
	"fmt"
	"os"
	"path/filepath"
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

	return cfg, nil
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
