package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Agent abstracts a coding agent (Claude Code, Copilot, Codex) so that
// behavioral test scenarios are agent-agnostic.
type Agent interface {
	// Run executes an agent session in the sandbox with the given prompt.
	// Returns when the agent finishes or the context is cancelled.
	Run(ctx context.Context, sandbox *Sandbox, prompt string, opts RunOpts) (*SessionResult, error)

	// Name returns the agent identifier (e.g. "claude-code", "copilot", "codex").
	Name() string
}

// RunOpts configures an agent session.
type RunOpts struct {
	MaxTurns int      // turn budget (0 = unlimited)
	Model    string   // model override (e.g. "haiku", "sonnet")
	Env      []string // additional env vars
	Dir      string   // working directory for the agent process (empty = sandbox.Root)
}

// SessionResult holds the output of an agent session.
type SessionResult struct {
	Output   string        // agent's text output
	Duration time.Duration // wall-clock duration
	ExitCode int           // agent process exit code
}

// ClaudeCodeAgent runs sessions via the `claude` CLI using existing auth.
type ClaudeCodeAgent struct{}

// Name returns the agent identifier.
func (a *ClaudeCodeAgent) Name() string { return "claude-code" }

// Run executes a Claude Code session in the sandbox.
func (a *ClaudeCodeAgent) Run(ctx context.Context, sandbox *Sandbox, prompt string, opts RunOpts) (*SessionResult, error) {
	args := []string{"-p", prompt, "--permission-mode", "bypassPermissions"}

	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	// Spec 092 Req 16: scenarios may start the agent in a directory other
	// than the sandbox root (e.g. inside a bead worktree). Default remains
	// sandbox.Root.
	cmd.Dir = sandbox.Root
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	// Merge sandbox env (recording shims) with any extra opts.Env.
	// Filter out all Claude Code vars to allow launching a clean nested session.
	env := filterEnvPrefix(sandbox.Env(), "CLAUDECODE=", "CLAUDE_CODE_")
	env = append(env, opts.Env...)
	cmd.Env = env

	start := time.Now()
	out, err := cmd.CombinedOutput()
	dur := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("running claude: %w", err)
		}
	}

	return &SessionResult{
		Output:   string(out),
		Duration: dur,
		ExitCode: exitCode,
	}, nil
}

// ResolveAgent returns an Agent implementation based on the name.
// Supported: "claude-code". Others return an error (extensible for copilot, codex).
func ResolveAgent(name string) (Agent, error) {
	switch strings.ToLower(name) {
	case "claude-code", "":
		return &ClaudeCodeAgent{}, nil
	default:
		return nil, fmt.Errorf("unknown agent %q (supported: claude-code)", name)
	}
}

// DefaultAgentName returns the agent name from BENCH_AGENT env var, defaulting to "claude-code".
func DefaultAgentName() string {
	if name := os.Getenv("BENCH_AGENT"); name != "" {
		return name
	}
	return "claude-code"
}

// ClaudeCodeAvailable checks if the claude CLI is installed and authed.
func ClaudeCodeAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}

// filterEnv removes environment variables matching any of the given keys.
func filterEnv(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, k := range keys {
			if strings.HasPrefix(e, k+"=") {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, e)
		}
	}
	return out
}

// filterEnvPrefix removes environment variables whose name starts with any of
// the given prefixes (e.g. "CLAUDE_CODE_" matches CLAUDE_CODE_SSE_PORT=...).
func filterEnvPrefix(env []string, prefixes ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, p := range prefixes {
			if strings.HasPrefix(e, p) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, e)
		}
	}
	return out
}
