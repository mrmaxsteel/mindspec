package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mindspec/mindspec/internal/bench"
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
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export MINDSPEC_TRACE=/tmp/mindspec-bench-a-trace.jsonl

# In a separate terminal, start the collector:
#   mindspec bench collect --port 4318 --output /tmp/bench-session-a.jsonl

# ─── Session B (Baseline) ──────────────────────────────────
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4319

# In a separate terminal, start the collector:
#   mindspec bench collect --port 4319 --output /tmp/bench-session-b.jsonl

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
	Use:   "report <session-a.jsonl> <session-b.jsonl>",
	Short: "Compare two collected benchmark sessions",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		labels, _ := cmd.Flags().GetString("labels")
		format, _ := cmd.Flags().GetString("format")

		labelA, labelB := "Session A", "Session B"
		if labels != "" {
			parts := splitLabels(labels)
			if len(parts) >= 2 {
				labelA, labelB = parts[0], parts[1]
			}
		}

		a, err := bench.ParseSession(args[0], labelA)
		if err != nil {
			return err
		}

		b, err := bench.ParseSession(args[1], labelB)
		if err != nil {
			return err
		}

		report := bench.Compare(a, b)

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
	},
}

func splitLabels(s string) []string {
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

	benchReportCmd.Flags().String("labels", "", "Comma-separated labels for sessions (e.g., mindspec,baseline)")
	benchReportCmd.Flags().String("format", "table", "Output format: table or json")

	benchCmd.AddCommand(benchSetupCmd)
	benchCmd.AddCommand(benchCollectCmd)
	benchCmd.AddCommand(benchReportCmd)
}
