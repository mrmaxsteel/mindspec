package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunSession executes an agent session for a given scenario in a sandbox.
// It runs the scenario's setup, invokes the agent, collects recorded events,
// and returns a SessionResult with timing and event data.
func RunSession(ctx context.Context, agent Agent, scenario Scenario, sandbox *Sandbox) (*SessionResult, error) {
	// Run scenario setup to prepare sandbox state
	if scenario.Setup != nil {
		if err := scenario.Setup(sandbox); err != nil {
			return nil, fmt.Errorf("scenario setup: %w", err)
		}
	}

	// Build run options from scenario
	opts := RunOpts{
		MaxTurns: scenario.MaxTurns,
		Model:    scenario.Model,
	}

	// Spec 092 Req 16: resolve the agent's starting directory after Setup
	// (worktree paths embed dynamically assigned bead IDs, so StartDir may
	// be a glob). Default remains sandbox.Root (empty Dir).
	if scenario.StartDir != "" {
		dir, err := resolveStartDir(sandbox.Root, scenario.StartDir)
		if err != nil {
			return nil, fmt.Errorf("resolving StartDir %q: %w", scenario.StartDir, err)
		}
		opts.Dir = dir
	}

	start := time.Now()

	result, err := agent.Run(ctx, sandbox, scenario.Prompt, opts)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	result.Duration = time.Since(start)

	return result, nil
}

// resolveStartDir resolves a Scenario.StartDir value (relative to root,
// optionally containing a glob pattern) to exactly one existing directory.
func resolveStartDir(root, startDir string) (string, error) {
	pattern := filepath.Join(root, startDir)
	if !strings.ContainsAny(startDir, "*?[") {
		info, err := os.Stat(pattern)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s is not a directory", pattern)
		}
		return pattern, nil
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && info.IsDir() {
			dirs = append(dirs, m)
		}
	}
	if len(dirs) != 1 {
		return "", fmt.Errorf("pattern matched %d directories, want exactly 1: %v", len(dirs), dirs)
	}
	return dirs[0], nil
}
