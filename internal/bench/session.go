package bench

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// SessionDef describes one benchmark session configuration.
type SessionDef struct {
	Label       string                    // e.g., "a", "b", "c"
	Description string                    // e.g., "no-docs", "baseline", "mindspec"
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
	SessionIDs []string // unique session.id UUIDs from OTLP events for this label
}

// RunSessionWithRetries executes a benchmark session with retry-based auto-approve.
// After each attempt, it checks whether implementation code was produced. If not,
// it auto-approves any pending workflow gates (spec/plan for session C) and retries
// with an escalating prompt directing the agent to implement.
//
// The collector is managed externally (by Run); this function only runs Claude sessions.
// benchEventsPath is the shared JSONL file written by the single collector.
func RunSessionWithRetries(ctx context.Context, cfg *RunConfig, def *SessionDef, wtPath, benchEventsPath string) (*SessionResult, error) {
	outputPath := filepath.Join(cfg.WorkDir, fmt.Sprintf("output-%s.txt", def.Label))

	result := &SessionResult{
		Label:      def.Label,
		JSONLPath:  benchEventsPath,
		OutputPath: outputPath,
	}

	baseCommit := getCurrentCommit(wtPath)

	// Create output file (append across retries)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	env := buildSessionEnv(agentMindPort, cfg.WorkDir, def.Label, def.EnableTrace)

	prompt := def.Prompt
	if prompt == "" {
		prompt = cfg.Prompt
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(cfg.Stdout, "\n── Retry %d/%d (auto-approve) ──\n\n", attempt, maxRetries)
			fmt.Fprintf(outFile, "\n\n--- RETRY %d/%d ---\n\n", attempt, maxRetries)
		}

		exitCode, timedOut, _ := runClaude(ctx, prompt, wtPath, env,
			cfg.MaxTurns, cfg.Model, cfg.Timeout, cfg.Stdout, outFile)
		result.ExitCode = exitCode
		result.TimedOut = timedOut

		// Commit any uncommitted changes
		commitWorktreeChanges(wtPath, fmt.Sprintf("%s-attempt-%d", def.Label, attempt))

		// Check if implementation code was produced
		if hasCodeChanges(wtPath, baseCommit) {
			fmt.Fprintf(cfg.Stdout, "  Implementation detected.\n")
			break
		}

		if attempt < maxRetries {
			fmt.Fprintf(cfg.Stdout, "  No implementation code detected. Auto-approving...\n")
			autoApprove(def.Label, wtPath, cfg.SpecID)
			prompt = buildRetryPrompt(def.Label, wtPath, cfg.SpecID, attempt+1)
		}
	}

	// Extract session IDs and count events for this label from the shared JSONL
	result.SessionIDs = ExtractSessionIDs(benchEventsPath, def.Label)
	result.EventCount = countEventsByLabel(benchEventsPath, def.Label)

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

	exec.Command("git", "-C", wtPath, "add", "-A").Run()                                                      //nolint:errcheck
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

// buildSessionEnv creates the environment for a benchmark Claude session,
// pointing OTLP at the shared bench collector and injecting bench.label
// for session discrimination.
func buildSessionEnv(port int, workDir, label string, enableTrace bool) []string {
	env := os.Environ()
	env = append(env,
		"CLAUDECODE=",
		"CLAUDE_CODE_ENABLE_TELEMETRY=1",
		"OTEL_METRICS_EXPORTER=otlp",
		"OTEL_LOGS_EXPORTER=otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/json",
		fmt.Sprintf("OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:%d", port),
		fmt.Sprintf("OTEL_RESOURCE_ATTRIBUTES=bench.label=%s", label),
	)
	if enableTrace {
		tracePath := filepath.Join(workDir, fmt.Sprintf("trace-%s.jsonl", label))
		env = append(env, "MINDSPEC_TRACE="+tracePath)
	}
	return env
}

// countEventsByLabel counts events in an NDJSON file where Resource["bench.label"] matches.
func countEventsByLabel(path, label string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e CollectedEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		bl, _ := e.Resource["bench.label"].(string)
		if bl == label {
			count++
		}
	}
	return count
}

// getCurrentCommit returns the HEAD commit SHA of a worktree.
func getCurrentCommit(wtPath string) string {
	cmd := exec.Command("git", "-C", wtPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return trimNewline(string(out))
}

// hasCodeChanges checks if implementation source files were created since baseCommit.
func hasCodeChanges(wtPath, baseCommit string) bool {
	cmd := exec.Command("git", "-C", wtPath, "diff", "--name-only", baseCommit+"..HEAD")
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	codeExts := regexp.MustCompile(`\.(go|js|ts|html|css|jsx|tsx)$`)
	excludeDirs := []string{"docs/", ".claude/", ".mindspec/", ".beads/"}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !codeExts.MatchString(line) {
			continue
		}
		excluded := false
		for _, dir := range excludeDirs {
			if strings.HasPrefix(line, dir) {
				excluded = true
				break
			}
		}
		if !excluded {
			return true
		}
	}
	return false
}

// autoApprove advances workflow gates between retries.
func autoApprove(label, wtPath, specID string) {
	if label != "c" {
		return // A/B: no state to advance, rely on retry prompt
	}

	// Read MindSpec focus
	cacheData, err := os.ReadFile(filepath.Join(wtPath, ".mindspec", "focus"))
	if err != nil {
		return
	}
	var cache map[string]string
	if err := json.Unmarshal(cacheData, &cache); err != nil {
		return
	}

	mode := cache["mode"]
	switch mode {
	case "spec":
		// Approve the spec: update frontmatter, advance to plan mode
		specPath := findSpecFile(wtPath, specID)
		if specPath != "" {
			updateFrontmatterApproval(specPath)
		}
		writeFocus(wtPath, "plan", specID, "")

	case "plan":
		// Approve the plan: update frontmatter, advance to implement mode
		planPath := filepath.Join(workspace.SpecDir(wtPath, specID), "plan.md")
		if _, err := os.Stat(planPath); err == nil {
			updateFrontmatterApproval(planPath)
		}
		writeFocus(wtPath, "implement", specID, "bench-impl")
	}
}

// buildRetryPrompt generates the prompt for a retry attempt.
func buildRetryPrompt(label, wtPath, specID string, attempt int) string {
	if label != "c" {
		// Sessions A/B: escalating implementation prompts
		if attempt == 1 {
			return "Your plan is approved. Proceed to implementation. Write all code and tests, then commit your changes."
		}
		return "Implementation is required. Write the code now and commit all changes."
	}

	// Session C: check MindSpec focus and give workflow-appropriate prompt
	cacheData, err := os.ReadFile(filepath.Join(wtPath, ".mindspec", "focus"))
	if err != nil {
		return "Continue implementing. Write all remaining code and commit."
	}
	var cache map[string]string
	if err := json.Unmarshal(cacheData, &cache); err != nil {
		return "Continue implementing. Write all remaining code and commit."
	}

	switch cache["mode"] {
	case "plan":
		specRel := findSpecRelPath(wtPath, specID)
		planRel := strings.TrimSuffix(specRel, "/spec.md") + "/plan.md"
		return fmt.Sprintf("The spec is approved. Create a plan at %s, then use /ms-plan-approve to approve it. After approval, implement all code and tests. Commit your changes when complete.", planRel)
	case "implement":
		return "The plan is approved. Implement all code and tests described in the plan. Commit your changes when complete."
	default:
		return "Continue implementing. Write all remaining code and commit."
	}
}

// prepareSessionC sets MindSpec focus to spec mode so hooks emit spec-mode guidance.
func prepareSessionC(wtPath, specID string) {
	writeFocus(wtPath, "spec", specID, "")
}

// updateFrontmatterApproval updates a markdown file's YAML frontmatter to set
// status: Approved and record the approval date.
func updateFrontmatterApproval(filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	content := string(data)

	// Replace status field
	statusRe := regexp.MustCompile(`(?mi)^(\s*(?:-\s+)?\*?\*?Status\*?\*?\s*:\s*).*$`)
	if statusRe.MatchString(content) {
		content = statusRe.ReplaceAllString(content, "${1}APPROVED")
	}

	// Also handle YAML frontmatter status field
	fmStatusRe := regexp.MustCompile(`(?m)^status:\s*.*$`)
	if fmStatusRe.MatchString(content) {
		content = fmStatusRe.ReplaceAllString(content, "status: Approved")
	}

	// Set approval date in frontmatter
	approvedAtRe := regexp.MustCompile(`(?m)^approved_at:\s*.*$`)
	now := time.Now().UTC().Format(time.RFC3339)
	if approvedAtRe.MatchString(content) {
		content = approvedAtRe.ReplaceAllString(content, fmt.Sprintf("approved_at: %q", now))
	}

	approvedByRe := regexp.MustCompile(`(?m)^approved_by:\s*.*$`)
	if approvedByRe.MatchString(content) {
		content = approvedByRe.ReplaceAllString(content, "approved_by: bench")
	}

	// Handle markdown-style approval section
	mdApprovedBy := regexp.MustCompile(`(?mi)^(\s*-\s+\*\*Approved By\*\*:\s*).*$`)
	if mdApprovedBy.MatchString(content) {
		content = mdApprovedBy.ReplaceAllString(content, "${1}bench")
	}
	mdApprovedDate := regexp.MustCompile(`(?mi)^(\s*-\s+\*\*Approval Date\*\*:\s*).*$`)
	if mdApprovedDate.MatchString(content) {
		content = mdApprovedDate.ReplaceAllString(content, fmt.Sprintf("${1}%s", time.Now().Format("2006-01-02")))
	}

	os.WriteFile(filePath, []byte(content), 0644) //nolint:errcheck
}

// writeFocus writes a MindSpec focus cursor file.
func writeFocus(wtPath, mode, specID, beadID string) {
	stateDir := filepath.Join(wtPath, ".mindspec")
	os.MkdirAll(stateDir, 0755) //nolint:errcheck

	cache := map[string]string{
		"mode":       mode,
		"activeSpec": specID,
		"activeBead": beadID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(cache, "", "  ")
	data = append(data, '\n')
	os.WriteFile(filepath.Join(stateDir, "focus"), data, 0644) //nolint:errcheck
}

// findSpecFile locates the spec.md for a given spec ID in the worktree.
func findSpecFile(wtPath, specID string) string {
	// Try the standard location first
	p := filepath.Join(workspace.SpecDir(wtPath, specID), "spec.md")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// Try without the full slug (e.g., "022" instead of "022-agentmind-viz-mvp")
	entries, err := os.ReadDir(filepath.Join(workspace.DocsDir(wtPath), "specs"))
	if err != nil {
		return ""
	}
	prefix := strings.SplitN(specID, "-", 2)[0]
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			p := filepath.Join(workspace.DocsDir(wtPath), "specs", e.Name(), "spec.md")
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// findSpecRelPath returns the relative path to spec.md from the worktree root.
func findSpecRelPath(wtPath, specID string) string {
	abs := findSpecFile(wtPath, specID)
	if abs == "" {
		rel, err := filepath.Rel(wtPath, filepath.Join(workspace.SpecDir(wtPath, specID), "spec.md"))
		if err != nil {
			return fmt.Sprintf(".mindspec/docs/specs/%s/spec.md", specID)
		}
		return filepath.ToSlash(rel)
	}
	rel, err := filepath.Rel(wtPath, abs)
	if err != nil {
		return fmt.Sprintf(".mindspec/docs/specs/%s/spec.md", specID)
	}
	return filepath.ToSlash(rel)
}
