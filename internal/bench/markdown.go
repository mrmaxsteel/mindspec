package bench

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BenchmarkDir returns the benchmark artifact directory for a spec.
func BenchmarkDir(repoRoot, specID string) string {
	return filepath.Join(repoRoot, "docs", "specs", specID, "benchmark")
}

// WriteResults persists all benchmark artifacts to the spec's benchmark directory.
func WriteResults(cfg *RunConfig, results []*SessionResult, quantReport string,
	qual *QualitativeResult, traceSummary string) error {

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

	// Copy telemetry JSONL, output files, and trace
	for _, r := range results {
		copyFile(r.JSONLPath, filepath.Join(dir, fmt.Sprintf("session-%s.jsonl", r.Label)))
		copyFile(r.OutputPath, filepath.Join(dir, fmt.Sprintf("output-%s.txt", r.Label)))
	}

	// Copy trace if it exists (session C)
	tracePath := filepath.Join(cfg.WorkDir, "trace-c.jsonl")
	if _, err := os.Stat(tracePath); err == nil {
		copyFile(tracePath, filepath.Join(dir, "trace-c.jsonl"))
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
	ports := map[string]int{"a": 4318, "b": 4319, "c": 4320}

	b.WriteString("\n## Sessions\n\n")
	b.WriteString("| Session | Description | Port | Events |\n")
	b.WriteString("|---------|-------------|------|--------|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %d |\n",
			labels[r.Label], descriptions[r.Label], ports[r.Label], r.EventCount))
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
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- `session-%s.jsonl` — Session %s OTEL telemetry\n", r.Label, labels[r.Label]))
	}
	b.WriteString("- `trace-c.jsonl` — Session C MindSpec trace\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- `output-%s.txt` — Session %s Claude output\n", r.Label, labels[r.Label]))
	}

	return b.String()
}

// WriteResumeResults persists implementation phase benchmark artifacts.
// Files are suffixed with -impl to avoid overwriting phase-1 results.
func WriteResumeResults(cfg *ResumeConfig, results []*SessionResult, quantReport string,
	qual *QualitativeResult, traceSummary string) error {

	dir := BenchmarkDir(cfg.RepoRoot, cfg.SpecID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating benchmark dir: %w", err)
	}

	report := assembleResumeReportMD(cfg, results, quantReport, qual, traceSummary)
	if err := os.WriteFile(filepath.Join(dir, "report-impl.md"), []byte(report), 0644); err != nil {
		return fmt.Errorf("writing report-impl.md: %w", err)
	}

	if qual != nil && qual.Improvements != "" {
		if err := os.WriteFile(filepath.Join(dir, "improvements-impl.md"), []byte(qual.Improvements+"\n"), 0644); err != nil {
			return fmt.Errorf("writing improvements-impl.md: %w", err)
		}
	}

	for _, r := range results {
		copyFile(r.JSONLPath, filepath.Join(dir, fmt.Sprintf("session-impl-%s.jsonl", r.Label)))
		copyFile(r.OutputPath, filepath.Join(dir, fmt.Sprintf("output-impl-%s.txt", r.Label)))
	}

	tracePath := filepath.Join(cfg.WorkDir, "trace-c.jsonl")
	if _, err := os.Stat(tracePath); err == nil {
		copyFile(tracePath, filepath.Join(dir, "trace-impl-c.jsonl"))
	}

	return nil
}

func assembleResumeReportMD(cfg *ResumeConfig, results []*SessionResult, quantReport string,
	qual *QualitativeResult, traceSummary string) string {

	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Benchmark (Implementation Phase): %s\n\n", cfg.SpecID))
	b.WriteString(fmt.Sprintf("**Date**: %s\n", time.Now().Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("**Phase**: Implementation (resumed from phase-1 plan/spec artifacts)\n"))
	b.WriteString(fmt.Sprintf("**Timeout**: %.0fs per attempt\n", cfg.Timeout.Seconds()))
	b.WriteString(fmt.Sprintf("**Max Retries**: %d\n", cfg.MaxRetries))
	model := cfg.Model
	if model == "" {
		model = "default"
	}
	b.WriteString(fmt.Sprintf("**Model**: %s\n", model))

	descriptions := map[string]string{
		"a": "No CLAUDE.md/.mindspec; hooks stripped; no docs/ — given plan-a.md",
		"b": "No CLAUDE.md/.mindspec; hooks stripped; docs/ present — given plan-b.md",
		"c": "Full MindSpec tooling — follows /spec-approve → plan → /plan-approve → implement",
	}
	labels := map[string]string{
		"a": "A (no-docs)",
		"b": "B (baseline)",
		"c": "C (mindspec)",
	}
	ports := map[string]int{"a": portA, "b": portB, "c": portC}

	b.WriteString("\n## Sessions\n\n")
	b.WriteString("| Session | Description | Port | Events |\n")
	b.WriteString("|---------|-------------|------|--------|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %d |\n",
			labels[r.Label], descriptions[r.Label], ports[r.Label], r.EventCount))
	}

	b.WriteString("\n## Quantitative Comparison\n\n```\n")
	b.WriteString(quantReport)
	b.WriteString("\n```\n")

	if traceSummary != "" {
		b.WriteString("\n## MindSpec Trace Summary (Session C)\n\n```\n")
		b.WriteString(traceSummary)
		b.WriteString("\n```\n")
	}

	if qual != nil && qual.Analysis != "" {
		b.WriteString("\n## Qualitative Analysis\n\n")
		b.WriteString(qual.Analysis)
		b.WriteString("\n")
	} else {
		b.WriteString("\n## Qualitative Analysis\n\n_(skipped)_\n")
	}

	b.WriteString("\n## Raw Data\n\n")
	b.WriteString("Telemetry and output files are in this directory:\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- `session-impl-%s.jsonl` — Session %s OTEL telemetry\n", r.Label, labels[r.Label]))
	}
	b.WriteString("- `trace-impl-c.jsonl` — Session C MindSpec trace\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- `output-impl-%s.txt` — Session %s Claude output\n", r.Label, labels[r.Label]))
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
