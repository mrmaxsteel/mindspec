package bench

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mrmaxsteel/agentmind/client"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// RunConfig holds the configuration for a full benchmark run.
type RunConfig struct {
	SpecID      string
	Prompt      string
	Timeout     time.Duration
	MaxTurns    int
	MaxRetries  int // Auto-approve retry attempts per session (0 = single attempt)
	Model       string
	WorkDir     string
	RepoRoot    string
	BenchCommit string

	SkipCleanup     bool
	SkipQualitative bool
	SkipCommit      bool

	Parallel bool // Run all sessions concurrently

	Stdout io.Writer
}

// agentMindPort is the OTLP port used for bench collection via AgentMind.
const agentMindPort = client.DefaultOTLPPort

// Run executes the full benchmark pipeline.
func Run(cfg *RunConfig) error {
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}

	// Resolve repo root and commit
	if cfg.RepoRoot == "" {
		top, err := gitutil.RevParseShowToplevel()
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}
		cfg.RepoRoot = top
	}
	if cfg.BenchCommit == "" {
		sha, err := gitutil.RevParseHEAD(cfg.RepoRoot)
		if err != nil {
			return fmt.Errorf("finding HEAD commit: %w", err)
		}
		cfg.BenchCommit = sha
	}

	if cfg.WorkDir == "" {
		cfg.WorkDir = filepath.Join(os.TempDir(), "mindspec-bench-"+cfg.SpecID)
	}

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	// Print banner
	fmt.Fprintf(cfg.Stdout, "MindSpec E2E Benchmark\n")
	fmt.Fprintf(cfg.Stdout, "  Spec:       %s\n", cfg.SpecID)
	fmt.Fprintf(cfg.Stdout, "  Commit:     %s\n", cfg.BenchCommit)
	fmt.Fprintf(cfg.Stdout, "  Timeout:    %s per attempt\n", cfg.Timeout)
	fmt.Fprintf(cfg.Stdout, "  MaxRetries: %d\n", cfg.MaxRetries)
	fmt.Fprintf(cfg.Stdout, "  Work:       %s\n\n", cfg.WorkDir)

	// Prerequisites
	if err := checkPrerequisites(cfg); err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return fmt.Errorf("creating work dir: %w", err)
	}

	// Define sessions
	timestamp := time.Now().Format("20060102-150405")
	sessions := []*SessionDef{
		{Label: "a", Description: "no-docs", Neutralize: NeutralizeNoDocs},
		{Label: "b", Description: "baseline", Neutralize: NeutralizeBaseline},
		{Label: "c", Description: "mindspec", Neutralize: nil, EnableTrace: true},
	}

	// Create worktrees
	fmt.Fprintln(cfg.Stdout, "Creating worktrees...")
	for _, def := range sessions {
		branch := fmt.Sprintf("bench-%s-%s-%s", def.Label, cfg.SpecID, timestamp)
		wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
		if err := CreateWorktree(cfg.RepoRoot, branch, wtPath, cfg.BenchCommit); err != nil {
			cleanupWorktrees(cfg, sessions)
			return fmt.Errorf("creating worktree %s: %w", def.Label, err)
		}
	}

	// Cleanup on exit (unless --skip-cleanup)
	defer func() {
		if !cfg.SkipCleanup {
			cleanupWorktrees(cfg, sessions)
		} else {
			fmt.Fprintf(cfg.Stdout, "\nSkipping cleanup (--skip-cleanup). Worktrees at: %s/wt-{a,b,c}\n", cfg.WorkDir)
		}
	}()

	// Neutralize A and B
	fmt.Fprintln(cfg.Stdout, "Neutralizing sessions A and B...")
	for _, def := range sessions {
		if def.Neutralize != nil {
			wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
			if err := def.Neutralize(wtPath); err != nil {
				return fmt.Errorf("neutralizing %s: %w", def.Label, err)
			}
		}
	}

	// Prepare session C: set MindSpec state to spec mode so hooks work
	wtC := filepath.Join(cfg.WorkDir, "wt-c")
	prepareSessionC(wtC, cfg.SpecID)

	// Start AgentMind as OTLP collector for all sessions.
	//
	// Spec 083 Hard Constraint #4 (batch class): if the agentmind binary
	// is absent, emit the centralized warn line via client.EmitWarnOnce
	// and proceed with exit 0. The bench run continues without telemetry
	// collection — for the bench command, telemetry is a side-effect, not
	// the deliverable.
	benchEventsPath := filepath.Join(cfg.WorkDir, "bench-events.jsonl")
	handle, err := client.AutoStart(cfg.RepoRoot, agentMindPort, client.DefaultUIPort, benchEventsPath)
	switch {
	case err == nil:
		if handle != nil && handle.PID > 0 {
			fmt.Fprintf(cfg.Stdout, "AgentMind started (PID %d) → %s\n", handle.PID, benchEventsPath)
		} else {
			fmt.Fprintf(cfg.Stdout, "AgentMind already running on :%d\n", agentMindPort)
		}
		fmt.Fprintf(cfg.Stdout, "Watch live at http://localhost:%d\n", client.DefaultUIPort)
	case errors.Is(err, client.ErrBinaryNotFound):
		// Batch class: degrade gracefully. EmitWarnOnce is the sync.Once
		// guarded helper that guarantees exactly one warn line per
		// process even if multiple consumers also call it.
		client.EmitWarnOnce(os.Stderr)
	default:
		return fmt.Errorf("starting AgentMind collector: %w", err)
	}

	// Build per-session prompts
	// Sessions A & B: generic feature prompt (no MindSpec workflow)
	// Session C: MindSpec workflow prompt directing through spec→plan→implement
	for _, def := range sessions {
		if def.Label == "c" {
			specRelPath := findSpecRelPath(wtC, cfg.SpecID)
			planRelPath := strings.TrimSuffix(specRelPath, "/spec.md") + "/plan.md"
			def.Prompt = fmt.Sprintf(`The specification at %s is ready for review.
Follow the MindSpec workflow:
1. Review the spec, then use /ms-spec-approve to approve it
2. Create a plan at %s, then use /ms-plan-approve
3. Implement all code and tests described in the plan
4. Commit your changes when complete`, specRelPath, planRelPath)
		} else {
			def.Prompt = cfg.Prompt
		}
	}

	// Run sessions with retry-based auto-approve
	ctx := context.Background()
	results := make([]*SessionResult, len(sessions))

	if cfg.Parallel {
		fmt.Fprintf(cfg.Stdout, "\n━━━ Running %d sessions in parallel ━━━\n\n", len(sessions))
		var wg sync.WaitGroup
		errs := make([]error, len(sessions))
		for i, def := range sessions {
			wg.Add(1)
			go func(idx int, d *SessionDef) {
				defer wg.Done()
				wtPath := filepath.Join(cfg.WorkDir, "wt-"+d.Label)
				result, err := RunSessionWithRetries(ctx, cfg, d, wtPath, benchEventsPath)
				results[idx] = result
				errs[idx] = err
			}(i, def)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				return fmt.Errorf("session %s: %w", sessions[i].Label, err)
			}
		}
		for _, r := range results {
			if r.TimedOut {
				fmt.Fprintf(cfg.Stdout, "WARNING: Session %s timed out after %s\n", r.Label, cfg.Timeout)
			}
			fmt.Fprintf(cfg.Stdout, "Session %s complete. Events: %d\n", r.Label, r.EventCount)
		}
	} else {
		for i, def := range sessions {
			fmt.Fprintf(cfg.Stdout, "\n━━━ Session %s (%s) ━━━\n\n", def.Label, def.Description)
			wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
			result, err := RunSessionWithRetries(ctx, cfg, def, wtPath, benchEventsPath)
			if err != nil {
				return fmt.Errorf("session %s: %w", def.Label, err)
			}
			if result.TimedOut {
				fmt.Fprintf(cfg.Stdout, "\nWARNING: Session %s timed out after %s\n", def.Label, cfg.Timeout)
			}
			fmt.Fprintf(cfg.Stdout, "Session %s complete. Events: %d\n", def.Label, result.EventCount)
			results[i] = result
		}
	}

	// Brief pause to ensure all events are flushed to disk
	time.Sleep(2 * time.Second)

	// Generate N-way quantitative report from single shared JSONL file
	fmt.Fprintln(cfg.Stdout, "\nGenerating quantitative report...")
	var quantReport string
	var parsedSessions []*Session
	displayLabels := []string{"no-docs", "baseline", "mindspec"}
	for i, def := range sessions {
		s, err := ParseSessionByLabel(benchEventsPath, def.Label)
		if err != nil {
			return fmt.Errorf("parsing session %s: %w", def.Label, err)
		}
		s.Label = displayLabels[i] // Use descriptive label for report display
		parsedSessions = append(parsedSessions, s)
	}
	multiReport := CompareN(parsedSessions)
	quantReport = FormatTableN(multiReport)

	// Collect plans and diffs
	fmt.Fprintln(cfg.Stdout, "Collecting diffs and plans...")
	plans := CollectPlans(cfg, cfg.SpecID)
	diffs := GenerateDiffs(cfg, cfg.BenchCommit)

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
		qual, _ = RunQualitative(cfg, quantReport, plans, diffs)
	}

	// Write results
	fmt.Fprintln(cfg.Stdout, "Writing results...")
	if err := WriteResults(cfg, results, quantReport, qual, traceSummary, plans, diffs); err != nil {
		return fmt.Errorf("writing results: %w", err)
	}

	// Commit if requested
	if !cfg.SkipCommit {
		fmt.Fprintln(cfg.Stdout, "Committing results...")
		benchDir, bdErr := BenchmarkDir(cfg.RepoRoot, cfg.SpecID)
		if bdErr != nil {
			return fmt.Errorf("resolving benchmark dir: %w", bdErr)
		}
		_ = gitutil.Add(cfg.RepoRoot, benchDir)
		commitMsg := fmt.Sprintf("bench(%s): add e2e benchmark results", cfg.SpecID)
		_ = gitutil.CommitNoVerify(cfg.RepoRoot, commitMsg)
	}

	benchDir, bdErr := BenchmarkDir(cfg.RepoRoot, cfg.SpecID)
	if bdErr != nil {
		return fmt.Errorf("resolving benchmark dir: %w", bdErr)
	}
	fmt.Fprintf(cfg.Stdout, "\nDone. Results in %s/\n", benchDir)
	fmt.Fprintln(cfg.Stdout, "  report.md        — quantitative + qualitative report")
	if qual != nil && qual.Improvements != "" {
		fmt.Fprintln(cfg.Stdout, "  improvements.md  — actionable findings from A/B")
	}
	fmt.Fprintln(cfg.Stdout, "  plan-{a,b,c}.md  — plan artifacts from each session")
	fmt.Fprintln(cfg.Stdout, "  diff-{a,b,c}.patch — implementation diffs from each session")

	return nil
}

func checkPrerequisites(cfg *RunConfig) error {
	var errors []string

	for _, cmd := range []string{"claude", "git"} {
		if _, err := exec.LookPath(cmd); err != nil {
			errors = append(errors, fmt.Sprintf("%s not found on PATH", cmd))
		}
	}

	msPath := filepath.Join(cfg.RepoRoot, "bin", "mindspec")
	if _, err := os.Stat(msPath); err != nil {
		errors = append(errors, "mindspec binary not found (run 'make build')")
	}

	// Check clean git tree
	if err := gitutil.DiffQuiet(cfg.RepoRoot); err != nil {
		errors = append(errors, "git working tree is not clean — commit or stash changes")
	} else {
		if err := gitutil.DiffCachedQuiet(cfg.RepoRoot); err != nil {
			errors = append(errors, "git index has staged changes — commit or stash")
		}
	}

	// No port check needed — AgentMind may already be running (reused)

	if len(errors) > 0 {
		msg := "prerequisites check failed:\n"
		for _, e := range errors {
			msg += "  - " + e + "\n"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func cleanupWorktrees(cfg *RunConfig, sessions []*SessionDef) {
	fmt.Fprintln(cfg.Stdout, "\nCleaning up worktrees...")
	for _, def := range sessions {
		wtPath := filepath.Join(cfg.WorkDir, "wt-"+def.Label)
		if _, err := os.Stat(wtPath); err == nil {
			RemoveWorktree(cfg.RepoRoot, wtPath) //nolint:errcheck
		}
	}
	_ = gitutil.WorktreePrune(cfg.RepoRoot)
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
