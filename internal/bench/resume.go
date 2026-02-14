package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ResumeConfig holds configuration for resuming a benchmark from phase-1 artifacts.
type ResumeConfig struct {
	SpecID          string
	Timeout         time.Duration
	MaxTurns        int
	MaxRetries      int
	Model           string
	WorkDir         string
	RepoRoot        string
	SkipCleanup     bool
	SkipQualitative bool
	SkipCommit      bool
	Stdout          io.Writer
}

// Resume picks up from a completed phase-1 benchmark run and drives sessions
// through to implementation via a retry loop with auto-approve.
func Resume(cfg *ResumeConfig) error {
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	// Resolve repo root
	if cfg.RepoRoot == "" {
		out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}
		cfg.RepoRoot = trimNewline(string(out))
	}

	// Find existing branches
	branches, err := findBenchBranches(cfg.RepoRoot, cfg.SpecID)
	if err != nil {
		return err
	}

	// Read artifacts
	benchDir := BenchmarkDir(cfg.RepoRoot, cfg.SpecID)
	artifacts := make(map[string]string)
	for _, label := range []string{"a", "b"} {
		content, err := readArtifact(benchDir, fmt.Sprintf("plan-%s.md", label))
		if err != nil {
			return fmt.Errorf("session %s artifact: %w", label, err)
		}
		artifacts[label] = content
	}
	// Session C: try spec-c.md first, then plan-c.md
	cContent, err := readArtifact(benchDir, "spec-c.md")
	if err != nil {
		cContent, err = readArtifact(benchDir, "plan-c.md")
		if err != nil {
			return fmt.Errorf("session c artifact: no spec-c.md or plan-c.md found in %s", benchDir)
		}
	}
	artifacts["c"] = cContent

	// Work directory
	if cfg.WorkDir == "" {
		cfg.WorkDir = filepath.Join(os.TempDir(), "mindspec-bench-"+cfg.SpecID+"-impl")
	}
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return fmt.Errorf("creating work dir: %w", err)
	}

	// Print banner
	fmt.Fprintf(cfg.Stdout, "MindSpec E2E Benchmark — Resume (Implementation Phase)\n")
	fmt.Fprintf(cfg.Stdout, "  Spec:       %s\n", cfg.SpecID)
	fmt.Fprintf(cfg.Stdout, "  MaxRetries: %d\n", cfg.MaxRetries)
	fmt.Fprintf(cfg.Stdout, "  Timeout:    %s per attempt\n", cfg.Timeout)
	fmt.Fprintf(cfg.Stdout, "  Work:       %s\n\n", cfg.WorkDir)

	// Check ports
	for _, port := range []int{portA, portB, portC} {
		if err := CheckPortFree(port); err != nil {
			return fmt.Errorf("prerequisite: %w", err)
		}
	}

	// Create worktrees from existing branches
	fmt.Fprintln(cfg.Stdout, "Creating worktrees from phase-1 branches...")
	sessions := []*SessionDef{
		{Label: "a", Description: "no-docs", Port: portA},
		{Label: "b", Description: "baseline", Port: portB},
		{Label: "c", Description: "mindspec", Port: portC, EnableTrace: true},
	}
	for _, def := range sessions {
		wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
		if err := CheckoutWorktree(cfg.RepoRoot, branches[def.Label], wtPath); err != nil {
			cleanupResumeWorktrees(cfg, sessions)
			return fmt.Errorf("creating worktree %s: %w", def.Label, err)
		}
	}

	defer func() {
		if !cfg.SkipCleanup {
			cleanupResumeWorktrees(cfg, sessions)
		} else {
			fmt.Fprintf(cfg.Stdout, "\nSkipping cleanup. Worktrees at: %s/wt-{a,b,c}\n", cfg.WorkDir)
		}
	}()

	// Prepare session C: set MindSpec state to spec mode
	wtC := filepath.Join(cfg.WorkDir, "wt-c")
	if err := prepareSessionC(wtC, cfg.SpecID); err != nil {
		return fmt.Errorf("preparing session C: %w", err)
	}

	// Build initial prompts
	// Sessions A & B: neutral implementation prompt with plan artifact
	for _, label := range []string{"a", "b"} {
		for _, def := range sessions {
			if def.Label == label {
				def.Prompt = fmt.Sprintf("Implement and commit the feature described below:\n\n---\n\n%s", artifacts[label])
			}
		}
	}
	// Session C: MindSpec workflow prompt
	specRelPath := findSpecRelPath(wtC, cfg.SpecID)
	for _, def := range sessions {
		if def.Label == "c" {
			def.Prompt = fmt.Sprintf(`The specification at %s is ready for review.
Follow the MindSpec workflow:
1. Review the spec, then use /spec-approve to approve it
2. Create a plan at docs/specs/%s/plan.md, then use /plan-approve
3. Implement all code and tests described in the plan
4. Commit your changes when complete`, specRelPath, cfg.SpecID)
		}
	}

	// Run sessions with retries
	ctx := context.Background()
	var results []*SessionResult
	for _, def := range sessions {
		fmt.Fprintf(cfg.Stdout, "\n━━━ Session %s (%s, port %d) ━━━\n\n", def.Label, def.Description, def.Port)
		wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
		result, err := runWithRetries(ctx, cfg, def, wtPath)
		if err != nil {
			return fmt.Errorf("session %s: %w", def.Label, err)
		}
		fmt.Fprintf(cfg.Stdout, "Session %s complete. Events: %d\n", def.Label, result.EventCount)
		results = append(results, result)
	}

	// Generate quantitative report
	fmt.Fprintln(cfg.Stdout, "\nGenerating quantitative report...")
	var parsedSessions []*Session
	sessionLabels := []string{"no-docs", "baseline", "mindspec"}
	for i, r := range results {
		s, err := ParseSession(r.JSONLPath, sessionLabels[i])
		if err != nil {
			return fmt.Errorf("parsing session %s: %w", r.Label, err)
		}
		parsedSessions = append(parsedSessions, s)
	}
	multiReport := CompareN(parsedSessions)
	quantReport := FormatTableN(multiReport)

	// Collect diffs (base = phase-1 branch tip before resume)
	fmt.Fprintln(cfg.Stdout, "Collecting diffs...")
	diffs := make(map[string]string)
	for _, def := range sessions {
		wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
		baseCommit := getPhase1Commit(wtPath, branches[def.Label])
		diffs[def.Label] = generateDiffFrom(wtPath, baseCommit)
	}

	// Collect plans
	plans := collectResumePlans(cfg, cfg.SpecID)

	// Trace summary
	var traceSummary string
	tracePath := filepath.Join(cfg.WorkDir, "trace-c.jsonl")
	if _, err := os.Stat(tracePath); err == nil {
		ms := filepath.Join(cfg.RepoRoot, "bin", "mindspec")
		out, err := exec.Command(ms, "trace", "summary", tracePath).Output()
		if err == nil {
			traceSummary = string(out)
		}
	}

	// Qualitative analysis
	var qual *QualitativeResult
	if !cfg.SkipQualitative {
		// Reuse the existing qualitative pipeline with a RunConfig shim
		shimCfg := &RunConfig{
			Prompt:  "(implementation phase — see per-session prompts in report)",
			WorkDir: cfg.WorkDir,
			Stdout:  cfg.Stdout,
		}
		qual, _ = RunQualitative(shimCfg, quantReport, plans, diffs)
	}

	// Write results
	fmt.Fprintln(cfg.Stdout, "Writing results...")
	if err := WriteResumeResults(cfg, results, quantReport, qual, traceSummary); err != nil {
		return fmt.Errorf("writing results: %w", err)
	}

	// Commit
	if !cfg.SkipCommit {
		fmt.Fprintln(cfg.Stdout, "Committing results...")
		dir := BenchmarkDir(cfg.RepoRoot, cfg.SpecID)
		exec.Command("git", "-C", cfg.RepoRoot, "add", dir).Run()                                                                   //nolint:errcheck
		exec.Command("git", "-C", cfg.RepoRoot, "commit", "-m", "bench("+cfg.SpecID+"): add implementation phase results", "--no-verify").Run() //nolint:errcheck
	}

	dir := BenchmarkDir(cfg.RepoRoot, cfg.SpecID)
	fmt.Fprintf(cfg.Stdout, "\nDone. Results in %s/\n", dir)
	fmt.Fprintln(cfg.Stdout, "  report-impl.md   — implementation phase report")

	return nil
}

// runWithRetries runs a session with retry-based auto-approve.
// The OTLP collector runs for the entire loop.
func runWithRetries(ctx context.Context, cfg *ResumeConfig, def *SessionDef, wtPath string) (*SessionResult, error) {
	jsonlPath := filepath.Join(cfg.WorkDir, fmt.Sprintf("session-impl-%s.jsonl", def.Label))
	outputPath := filepath.Join(cfg.WorkDir, fmt.Sprintf("output-impl-%s.txt", def.Label))

	result := &SessionResult{
		Label:      def.Label,
		JSONLPath:  jsonlPath,
		OutputPath: outputPath,
	}

	baseCommit := getCurrentCommit(wtPath)

	// Start collector for entire retry loop
	collector := NewCollector(def.Port, jsonlPath)
	collectorCtx, collectorCancel := context.WithCancel(ctx)
	defer collectorCancel()

	collectorDone := make(chan error, 1)
	go func() {
		collectorDone <- collector.Run(collectorCtx)
	}()

	if err := waitForPort(def.Port, 5*time.Second); err != nil {
		collectorCancel()
		return nil, fmt.Errorf("collector failed to start on port %d: %w", def.Port, err)
	}

	// Open output file (append across retries)
	outFile, err := os.Create(outputPath)
	if err != nil {
		collectorCancel()
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	env := buildSessionEnv(def.Port, cfg.WorkDir, def.Label, def.EnableTrace)
	prompt := def.Prompt

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(cfg.Stdout, "\n── Retry %d/%d (auto-approve) ──\n\n", attempt, cfg.MaxRetries)
			// Write separator in output file
			fmt.Fprintf(outFile, "\n\n--- RETRY %d/%d ---\n\n", attempt, cfg.MaxRetries)
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

		if attempt < cfg.MaxRetries {
			fmt.Fprintf(cfg.Stdout, "  No implementation code detected. Auto-approving...\n")
			autoApprove(def.Label, wtPath, cfg.SpecID)
			prompt = buildRetryPrompt(def.Label, wtPath, cfg.SpecID, attempt+1)
		}
	}

	// Flush collector
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

	return result, nil
}

// findBenchBranches finds existing phase-1 benchmark branches for the given spec ID.
func findBenchBranches(repoRoot, specID string) (map[string]string, error) {
	branches := make(map[string]string)

	for _, label := range []string{"a", "b", "c"} {
		pattern := fmt.Sprintf("bench-%s-%s-*", label, specID)
		cmd := exec.Command("git", "-C", repoRoot, "branch", "--list", pattern)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("listing branches for %s: %w", label, err)
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) == 0 || lines[0] == "" {
			return nil, fmt.Errorf("no phase-1 branch found for session %s (pattern: %s)", label, pattern)
		}

		// Take the last (most recent by timestamp in name)
		branch := strings.TrimSpace(lines[len(lines)-1])
		branch = strings.TrimPrefix(branch, "* ") // in case it's the current branch
		branches[label] = branch
	}

	return branches, nil
}

// readArtifact reads a file from the benchmark directory.
func readArtifact(benchDir, filename string) (string, error) {
	path := filepath.Join(benchDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
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

// prepareSessionC sets MindSpec state to spec mode so hooks emit spec-mode guidance.
func prepareSessionC(wtPath, specID string) error {
	stateDir := filepath.Join(wtPath, ".mindspec")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	state := map[string]string{
		"mode":        "spec",
		"activeSpec":  specID,
		"activeBead":  "",
		"lastUpdated": time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(stateDir, "state.json"), data, 0644)
}

// autoApprove advances workflow gates between retries.
func autoApprove(label, wtPath, specID string) {
	if label != "c" {
		return // A/B: no state to advance, rely on retry prompt
	}

	// Read MindSpec state
	stateData, err := os.ReadFile(filepath.Join(wtPath, ".mindspec", "state.json"))
	if err != nil {
		return
	}
	var state map[string]string
	if err := json.Unmarshal(stateData, &state); err != nil {
		return
	}

	mode := state["mode"]
	switch mode {
	case "spec":
		// Approve the spec: update frontmatter, advance to plan mode
		specPath := findSpecFile(wtPath, specID)
		if specPath != "" {
			updateFrontmatterApproval(specPath)
		}
		writeState(wtPath, "plan", specID, "")

	case "plan":
		// Approve the plan: update frontmatter, advance to implement mode
		planPath := filepath.Join(wtPath, "docs", "specs", specID, "plan.md")
		if _, err := os.Stat(planPath); err == nil {
			updateFrontmatterApproval(planPath)
		}
		writeState(wtPath, "implement", specID, "bench-impl")
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

	// Session C: check MindSpec state and give workflow-appropriate prompt
	stateData, err := os.ReadFile(filepath.Join(wtPath, ".mindspec", "state.json"))
	if err != nil {
		return "Continue implementing. Write all remaining code and commit."
	}
	var state map[string]string
	if err := json.Unmarshal(stateData, &state); err != nil {
		return "Continue implementing. Write all remaining code and commit."
	}

	switch state["mode"] {
	case "plan":
		return fmt.Sprintf("The spec is approved. Create a plan at docs/specs/%s/plan.md, then use /plan-approve to approve it. After approval, implement all code and tests. Commit your changes when complete.", specID)
	case "implement":
		return "The plan is approved. Implement all code and tests described in the plan. Commit your changes when complete."
	default:
		return "Continue implementing. Write all remaining code and commit."
	}
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
		content = approvedByRe.ReplaceAllString(content, "approved_by: bench-resume")
	}

	// Handle markdown-style approval section
	mdApprovedBy := regexp.MustCompile(`(?mi)^(\s*-\s+\*\*Approved By\*\*:\s*).*$`)
	if mdApprovedBy.MatchString(content) {
		content = mdApprovedBy.ReplaceAllString(content, "${1}bench-resume")
	}
	mdApprovedDate := regexp.MustCompile(`(?mi)^(\s*-\s+\*\*Approval Date\*\*:\s*).*$`)
	if mdApprovedDate.MatchString(content) {
		content = mdApprovedDate.ReplaceAllString(content, fmt.Sprintf("${1}%s", time.Now().Format("2006-01-02")))
	}

	os.WriteFile(filePath, []byte(content), 0644) //nolint:errcheck
}

// writeState writes a MindSpec state.json file.
func writeState(wtPath, mode, specID, beadID string) {
	stateDir := filepath.Join(wtPath, ".mindspec")
	os.MkdirAll(stateDir, 0755) //nolint:errcheck

	state := map[string]string{
		"mode":        mode,
		"activeSpec":  specID,
		"activeBead":  beadID,
		"lastUpdated": time.Now().UTC().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	data = append(data, '\n')
	os.WriteFile(filepath.Join(stateDir, "state.json"), data, 0644) //nolint:errcheck
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

// getPhase1Commit returns the first commit of a branch (the phase-1 tip).
// This is the commit that existed before any resume work was done.
func getPhase1Commit(wtPath, branchName string) string {
	// The phase-1 tip is the parent of the first resume commit,
	// but simpler: it's the merge-base with itself before resume.
	// Since we're looking at what the branch had before resume,
	// use the second-to-last commit if there were resume commits.
	// Simplest: just use the commit before the first resume attempt.
	cmd := exec.Command("git", "-C", wtPath, "log", "--oneline", "--format=%H", "-n", "20")
	out, err := cmd.Output()
	if err != nil {
		return "HEAD~1"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "HEAD~1"
	}
	// Return the last commit (oldest in the log output)
	// which should be the phase-1 bench commit
	return lines[len(lines)-1]
}

// generateDiffFrom generates a git diff from a base commit, excluding config dirs.
func generateDiffFrom(wtPath, baseCommit string) string {
	const maxDiffChars = 100000
	cmd := exec.Command("git", "-C", wtPath, "diff", baseCommit, "HEAD",
		"--", ":(exclude).beads", ":(exclude).mindspec", ":(exclude)docs/specs")
	out, err := cmd.Output()
	if err != nil {
		return "(diff generation failed)"
	}
	diff := string(out)
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars] + fmt.Sprintf("\n\n[... truncated at %d chars ...]", maxDiffChars)
	}
	return diff
}

// findSpecFile locates the spec.md for a given spec ID in the worktree.
func findSpecFile(wtPath, specID string) string {
	// Try the standard location first
	p := filepath.Join(wtPath, "docs", "specs", specID, "spec.md")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// Try without the full slug (e.g., "022" instead of "022-agentmind-viz-mvp")
	entries, err := os.ReadDir(filepath.Join(wtPath, "docs", "specs"))
	if err != nil {
		return ""
	}
	prefix := strings.SplitN(specID, "-", 2)[0]
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			p := filepath.Join(wtPath, "docs", "specs", e.Name(), "spec.md")
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
		return fmt.Sprintf("docs/specs/%s/spec.md", specID)
	}
	rel, err := filepath.Rel(wtPath, abs)
	if err != nil {
		return fmt.Sprintf("docs/specs/%s/spec.md", specID)
	}
	return rel
}

// collectResumePlans gathers plan artifacts for the qualitative analysis.
func collectResumePlans(cfg *ResumeConfig, specID string) map[string]string {
	plans := make(map[string]string)
	benchDir := BenchmarkDir(cfg.RepoRoot, specID)

	// Use the original phase-1 artifacts as the "plans"
	for _, label := range []string{"a", "b"} {
		if data, err := os.ReadFile(filepath.Join(benchDir, fmt.Sprintf("plan-%s.md", label))); err == nil {
			plans[label] = string(data)
		} else {
			plans[label] = fmt.Sprintf("(No plan artifact for Session %s)", strings.ToUpper(label))
		}
	}

	// Session C: check for plan.md in the worktree (created during resume)
	planC := filepath.Join(cfg.WorkDir, "wt-c", "docs", "specs", specID, "plan.md")
	if data, err := os.ReadFile(planC); err == nil {
		plans["c"] = string(data)
	} else if data, err := os.ReadFile(filepath.Join(benchDir, "spec-c.md")); err == nil {
		plans["c"] = string(data)
	} else {
		plans["c"] = "(No plan artifact for Session C)"
	}

	return plans
}

func cleanupResumeWorktrees(cfg *ResumeConfig, sessions []*SessionDef) {
	fmt.Fprintln(cfg.Stdout, "\nCleaning up worktrees...")
	for _, def := range sessions {
		wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
		if _, err := os.Stat(wtPath); err == nil {
			RemoveWorktree(cfg.RepoRoot, wtPath) //nolint:errcheck
		}
	}
	exec.Command("git", "-C", cfg.RepoRoot, "worktree", "prune").Run() //nolint:errcheck
}
