package bench

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// BenchmarkDir returns the benchmark artifact directory for a spec.
func BenchmarkDir(repoRoot, specID string) string {
	return filepath.Join(workspace.SpecDir(repoRoot, specID), "benchmark")
}

// WriteResults persists all benchmark artifacts to the spec's benchmark directory.
func WriteResults(cfg *RunConfig, results []*SessionResult, quantReport string,
	qual *QualitativeResult, traceSummary string, plans, diffs map[string]string) error {

	dir := BenchmarkDir(cfg.RepoRoot, cfg.SpecID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating benchmark dir: %w", err)
	}

	// Write report.md
	report := assembleReportMD(cfg, results, quantReport, qual, traceSummary)
	if err := os.WriteFile(filepath.Join(dir, "report.md"), []byte(report), 0644); err != nil {
		return fmt.Errorf("writing report.md: %w", err)
	}

	// Write improvements.md if available
	if qual != nil && qual.Improvements != "" {
		if err := os.WriteFile(filepath.Join(dir, "improvements.md"), []byte(qual.Improvements+"\n"), 0644); err != nil {
			return fmt.Errorf("writing improvements.md: %w", err)
		}
	}

	// Copy shared telemetry JSONL and per-session output files
	if len(results) > 0 {
		copyFile(results[0].JSONLPath, filepath.Join(dir, "bench-events.jsonl"))
	}
	for _, r := range results {
		copyFile(r.OutputPath, filepath.Join(dir, fmt.Sprintf("output-%s.txt", r.Label)))
	}

	// Copy trace if it exists (session C)
	tracePath := filepath.Join(cfg.WorkDir, "trace-c.jsonl")
	if _, err := os.Stat(tracePath); err == nil {
		copyFile(tracePath, filepath.Join(dir, "trace-c.jsonl"))
	}

	// Persist plan artifacts (needed by bench resume and for review)
	for _, label := range []string{"a", "b", "c"} {
		content, ok := plans[label]
		if !ok || strings.HasPrefix(content, "(No plan") {
			continue
		}
		// Session C: save as both plan-c.md and spec-c.md (resume checks spec-c.md first)
		name := fmt.Sprintf("plan-%s.md", label)
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0644) //nolint:errcheck
	}

	// Persist implementation diffs
	for _, label := range []string{"a", "b", "c"} {
		content, ok := diffs[label]
		if !ok || content == "" || content == "(diff generation failed)" {
			continue
		}
		name := fmt.Sprintf("diff-%s.patch", label)
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0644) //nolint:errcheck
	}

	return nil
}

func assembleReportMD(cfg *RunConfig, results []*SessionResult, quantReport string,
	qual *QualitativeResult, traceSummary string) string {

	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Benchmark: %s\n\n", cfg.SpecID))
	b.WriteString(fmt.Sprintf("**Date**: %s\n", time.Now().Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("**Commit**: %s\n", cfg.BenchCommit))
	b.WriteString(fmt.Sprintf("**Timeout**: %.0fs\n", cfg.Timeout.Seconds()))
	model := cfg.Model
	if model == "" {
		model = "default"
	}
	b.WriteString(fmt.Sprintf("**Model**: %s\n", model))

	// Prompt
	prompt := cfg.Prompt
	if len(prompt) > 500 {
		prompt = prompt[:500] + "..."
	}
	b.WriteString(fmt.Sprintf("\n## Prompt\n\n%s\n", prompt))

	// Sessions table
	descriptions := map[string]string{
		"a": "No CLAUDE.md/.mindspec; hooks stripped; no docs/",
		"b": "No CLAUDE.md/.mindspec; hooks stripped; docs/ present",
		"c": "Full MindSpec tooling",
	}
	labels := map[string]string{
		"a": "A (no-docs)",
		"b": "B (baseline)",
		"c": "C (mindspec)",
	}

	b.WriteString("\n## Sessions\n\n")
	b.WriteString("| Session | Description | Label | Events |\n")
	b.WriteString("|---------|-------------|-------|--------|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %d |\n",
			labels[r.Label], descriptions[r.Label], r.Label, r.EventCount))
	}

	// Quantitative comparison
	b.WriteString("\n## Quantitative Comparison\n\n```\n")
	b.WriteString(quantReport)
	b.WriteString("\n```\n")

	// Trace summary
	if traceSummary != "" {
		b.WriteString("\n## MindSpec Trace Summary (Session C)\n\n```\n")
		b.WriteString(traceSummary)
		b.WriteString("\n```\n")
	}

	// Qualitative analysis
	if qual != nil && qual.Analysis != "" {
		b.WriteString("\n## Qualitative Analysis\n\n")
		b.WriteString(qual.Analysis)
		b.WriteString("\n")
	} else {
		b.WriteString("\n## Qualitative Analysis\n\n_(skipped)_\n")
	}

	// Raw data note
	b.WriteString("\n## Raw Data\n\n")
	b.WriteString("Telemetry and output files are in this directory:\n")
	b.WriteString("- `bench-events.jsonl` — Shared OTEL telemetry (all sessions, filtered by `bench.label`)\n")
	b.WriteString("- `trace-c.jsonl` — Session C MindSpec trace\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- `plan-%s.md` — Session %s plan artifact\n", r.Label, labels[r.Label]))
	}
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- `diff-%s.patch` — Session %s implementation diff\n", r.Label, labels[r.Label]))
	}
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- `output-%s.txt` — Session %s Claude output\n", r.Label, labels[r.Label]))
	}

	return b.String()
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()

	io.Copy(out, in) //nolint:errcheck
}
