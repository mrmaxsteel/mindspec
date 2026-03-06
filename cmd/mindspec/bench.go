package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bench"
	"github.com/spf13/cobra"
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Benchmarking tools for comparing Claude Code sessions",
}

var benchSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Print environment configuration for A/B benchmark sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(`# MindSpec Bench — A/B Session Setup
# Copy each block into the respective VSCode session's terminal.

# ─── Session A (MindSpec) ───────────────────────────────────
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export MINDSPEC_TRACE=/tmp/mindspec-bench-a-trace.jsonl

# In a separate terminal, start the collector:
#   mindspec bench collect --port 4318 --output /tmp/bench-session-a.jsonl

# ─── Session B (Baseline) ──────────────────────────────────
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# AgentMind serves as the unified collector — start it if not running:
#   mindspec agentmind serve --output /tmp/bench-session-b.jsonl

# ─── After both sessions complete ──────────────────────────
# mindspec bench report /tmp/bench-session-a.jsonl /tmp/bench-session-b.jsonl --labels "mindspec,baseline"
`)
		return nil
	},
}

var benchCollectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Start an OTLP/HTTP collector to capture Claude Code telemetry",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Deprecated: use 'mindspec agentmind serve --output <path>' instead")
		port, _ := cmd.Flags().GetInt("port")
		output, _ := cmd.Flags().GetString("output")

		if output == "" {
			return fmt.Errorf("--output is required")
		}

		c := bench.NewCollector(port, output)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		return c.Run(ctx)
	},
}

var benchReportCmd = &cobra.Command{
	Use:   "report <session.jsonl> [session.jsonl...]",
	Short: "Compare two or more collected benchmark sessions",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		labels, _ := cmd.Flags().GetString("labels")
		format, _ := cmd.Flags().GetString("format")

		labelParts := splitLabels(labels)

		// Parse all sessions
		var sessions []*bench.Session
		for i, path := range args {
			label := fmt.Sprintf("Session %d", i+1)
			if i < len(labelParts) {
				label = labelParts[i]
			}
			s, err := bench.ParseSession(path, label)
			if err != nil {
				return err
			}
			sessions = append(sessions, s)
		}

		// 2 sessions: pairwise with delta (backward compat)
		if len(sessions) == 2 {
			report := bench.Compare(sessions[0], sessions[1])
			if format == "json" {
				out, err := bench.FormatJSON(report)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}
			fmt.Print(bench.FormatTable(report))
			return nil
		}

		// 3+ sessions: N-way side-by-side (no deltas)
		report := bench.CompareN(sessions)
		fmt.Print(bench.FormatTableN(report))
		return nil
	},
}

var benchRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a full 3-session A/B/C benchmark",
	Long: `Run 3 Claude Code sessions under different conditions and produce a
comparative benchmark report.

  Session A (no-docs):   No CLAUDE.md/.mindspec; hooks stripped; no docs/
  Session B (baseline):  No CLAUDE.md/.mindspec; hooks stripped; docs/ present
  Session C (mindspec):  Full MindSpec tooling`,
	RunE: func(cmd *cobra.Command, args []string) error {
		specID, _ := cmd.Flags().GetString("spec-id")
		prompt, _ := cmd.Flags().GetString("prompt")
		promptFile, _ := cmd.Flags().GetString("prompt-file")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")
		maxTurns, _ := cmd.Flags().GetInt("max-turns")
		model, _ := cmd.Flags().GetString("model")
		workDir, _ := cmd.Flags().GetString("work-dir")
		skipCleanup, _ := cmd.Flags().GetBool("skip-cleanup")
		skipQual, _ := cmd.Flags().GetBool("skip-qualitative")
		skipCommit, _ := cmd.Flags().GetBool("skip-commit")

		if specID == "" {
			return fmt.Errorf("--spec-id is required")
		}

		// Read prompt from file if specified
		if promptFile != "" {
			data, err := os.ReadFile(promptFile)
			if err != nil {
				return fmt.Errorf("reading prompt file: %w", err)
			}
			prompt = string(data)
		}
		if prompt == "" {
			return fmt.Errorf("--prompt or --prompt-file is required")
		}

		parallel, _ := cmd.Flags().GetBool("parallel")
		maxRetries, _ := cmd.Flags().GetInt("max-retries")

		cfg := &bench.RunConfig{
			SpecID:          specID,
			Prompt:          prompt,
			Timeout:         time.Duration(timeoutSec) * time.Second,
			MaxTurns:        maxTurns,
			MaxRetries:      maxRetries,
			Model:           model,
			WorkDir:         workDir,
			SkipCleanup:     skipCleanup,
			SkipQualitative: skipQual,
			SkipCommit:      skipCommit,
			Parallel:        parallel,
			Stdout:          os.Stdout,
		}

		return bench.Run(cfg)
	},
}

func splitLabels(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	current := ""
	for _, c := range s {
		if c == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func init() {
	benchCollectCmd.Flags().Int("port", 4318, "Port to listen on")
	benchCollectCmd.Flags().String("output", "", "Output NDJSON file path")

	benchReportCmd.Flags().String("labels", "", "Comma-separated labels for sessions (e.g., no-docs,baseline,mindspec)")
	benchReportCmd.Flags().String("format", "table", "Output format: table or json")

	benchRunCmd.Flags().String("spec-id", "", "Spec folder ID (e.g., 015-project-bootstrap)")
	benchRunCmd.Flags().String("prompt", "", "Feature prompt for all 3 sessions")
	benchRunCmd.Flags().String("prompt-file", "", "Read prompt from file")
	benchRunCmd.Flags().Int("timeout", 1800, "Per-session timeout in seconds")
	benchRunCmd.Flags().Int("max-turns", 0, "Max agentic turns per session (0 = unlimited)")
	benchRunCmd.Flags().String("model", "", "Claude model for all sessions")
	benchRunCmd.Flags().String("work-dir", "", "Base dir for worktrees (default: /tmp/mindspec-bench-<spec-id>)")
	benchRunCmd.Flags().Bool("skip-cleanup", false, "Preserve worktrees after completion")
	benchRunCmd.Flags().Bool("skip-qualitative", false, "Skip qualitative analysis (quantitative only)")
	benchRunCmd.Flags().Bool("skip-commit", false, "Don't commit results to specs/<id>/benchmark under the active docs root")
	benchRunCmd.Flags().Bool("parallel", false, "Run all sessions concurrently")
	benchRunCmd.Flags().Int("max-retries", 3, "Max auto-approve retry attempts per session")

	benchCmd.AddCommand(benchSetupCmd)
	benchCmd.AddCommand(benchCollectCmd)
	benchCmd.AddCommand(benchReportCmd)
	benchCmd.AddCommand(benchRunCmd)
}
