package bench

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// SessionDef describes one benchmark session configuration.
type SessionDef struct {
	Label       string // e.g., "a", "b", "c"
	Description string // e.g., "no-docs", "baseline", "mindspec"
	Port        int
	Neutralize  func(wtPath string) error // nil for session C (full MindSpec)
	EnableTrace bool
	Prompt      string // per-session prompt override (if non-empty, overrides cfg.Prompt)
}

// SessionResult holds the outcome of a single benchmark session.
type SessionResult struct {
	Label      string
	JSONLPath  string
	OutputPath string
	EventCount int
	ExitCode   int
	TimedOut   bool
}

// RunSession executes a single benchmark session: starts an in-process OTLP
// collector, runs claude -p in the worktree, and captures output.
func RunSession(ctx context.Context, cfg *RunConfig, def *SessionDef, wtPath string) (*SessionResult, error) {
	jsonlPath := filepath.Join(cfg.WorkDir, fmt.Sprintf("session-%s.jsonl", def.Label))
	outputPath := filepath.Join(cfg.WorkDir, fmt.Sprintf("output-%s.txt", def.Label))

	result := &SessionResult{
		Label:      def.Label,
		JSONLPath:  jsonlPath,
		OutputPath: outputPath,
	}

	// Start in-process collector as goroutine
	collector := NewCollector(def.Port, jsonlPath)
	collectorCtx, collectorCancel := context.WithCancel(ctx)
	defer collectorCancel()

	collectorDone := make(chan error, 1)
	go func() {
		collectorDone <- collector.Run(collectorCtx)
	}()

	// Wait for collector port to be ready
	if err := waitForPort(def.Port, 5*time.Second); err != nil {
		collectorCancel()
		return nil, fmt.Errorf("collector failed to start on port %d: %w", def.Port, err)
	}

	// Create output file for tee
	outFile, err := os.Create(outputPath)
	if err != nil {
		collectorCancel()
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	// Determine prompt
	prompt := cfg.Prompt
	if def.Prompt != "" {
		prompt = def.Prompt
	}

	// Build environment
	env := buildSessionEnv(def.Port, cfg.WorkDir, def.Label, def.EnableTrace)

	// Run claude
	exitCode, timedOut, _ := runClaude(ctx, prompt, wtPath, env,
		cfg.MaxTurns, cfg.Model, cfg.Timeout, cfg.Stdout, outFile)
	result.ExitCode = exitCode
	result.TimedOut = timedOut

	// Give collector time to flush remaining events
	time.Sleep(2 * time.Second)
	collectorCancel()
	<-collectorDone

	// Count events
	if data, err := os.ReadFile(jsonlPath); err == nil {
		for _, b := range data {
			if b == '\n' {
				result.EventCount++
			}
		}
	}

	// Auto-commit changes in worktree
	commitWorktreeChanges(wtPath, def.Label)

	return result, nil
}

// commitWorktreeChanges commits any changes made during the session.
func commitWorktreeChanges(wtPath, label string) {
	// Check if there are changes
	cmd := exec.Command("git", "-C", wtPath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return
	}

	exec.Command("git", "-C", wtPath, "add", "-A").Run()                                               //nolint:errcheck
	exec.Command("git", "-C", wtPath, "commit", "-m", "bench: Session "+label+" output", "--no-verify").Run() //nolint:errcheck
}

// waitForPort waits until the given port is accepting TCP connections.
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("localhost:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("port %d not ready after %s", port, timeout)
}

// CheckPortFree returns nil if the port is not in use.
func CheckPortFree(port int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
	if err != nil {
		return nil // port is free
	}
	conn.Close()
	return fmt.Errorf("port %d is already in use", port)
}

// runClaude executes a single claude -p invocation. It is the core execution
// primitive used by both RunSession and runWithRetries.
func runClaude(ctx context.Context, prompt, wtPath string, env []string,
	maxTurns int, model string, timeout time.Duration,
	stdout io.Writer, outFile *os.File) (exitCode int, timedOut bool, err error) {

	args := []string{"-p", prompt, "--dangerously-skip-permissions", "--no-session-persistence"}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	sessionCtx, sessionCancel := context.WithTimeout(ctx, timeout)
	defer sessionCancel()

	cmd := exec.CommandContext(sessionCtx, "claude", args...)
	cmd.Dir = wtPath
	cmd.Stdout = io.MultiWriter(stdout, outFile)
	cmd.Stderr = io.MultiWriter(stdout, outFile)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second
	cmd.Env = env

	runErr := cmd.Run()
	if runErr != nil {
		if sessionCtx.Err() == context.DeadlineExceeded {
			timedOut = true
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return exitCode, timedOut, nil
}

// buildSessionEnv creates the environment for a benchmark Claude session.
func buildSessionEnv(port int, workDir, label string, enableTrace bool) []string {
	env := os.Environ()
	env = append(env,
		"CLAUDECODE=",
		"CLAUDE_CODE_ENABLE_TELEMETRY=1",
		"OTEL_METRICS_EXPORTER=otlp",
		"OTEL_LOGS_EXPORTER=otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/json",
		fmt.Sprintf("OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:%d", port),
		"OTEL_LOG_TOOL_DETAILS=1",
	)
	if enableTrace {
		tracePath := filepath.Join(workDir, fmt.Sprintf("trace-%s.jsonl", label))
		env = append(env, "MINDSPEC_TRACE="+tracePath)
	}
	return env
}
