package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents .mindspec/config.yaml settings.
type Config struct {
	ProtectedBranches []string    `yaml:"protected_branches"`
	MergeStrategy     string      `yaml:"merge_strategy"`
	WorktreeRoot      string      `yaml:"worktree_root"`
	AutoFinalize      bool        `yaml:"auto_finalize"`
	Enforcement       Enforcement `yaml:"enforcement"`
	Recording         Recording   `yaml:"recording"`
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
		},
	}
}

// Load reads .mindspec/config.yaml under root.
// Returns DefaultConfig if the file does not exist.
func Load(root string) (*Config, error) {
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
