package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompareN(t *testing.T) {
	sessions := []*Session{
		{
			Label: "no-docs", APICallCount: 10,
			InputTokens: 30000, OutputTokens: 10000,
			CacheRead: 100000, CacheCreate: 5000,
			CostUSD: 1.18, DurationMs: 200000,
			ModelBreakdown: map[string]*ModelStats{
				"claude-opus-4-6": {Calls: 5, InputTokens: 20000, OutputTokens: 8000, CostUSD: 0.96},
			},
		},
		{
			Label: "baseline", APICallCount: 8,
			InputTokens: 20000, OutputTokens: 15000,
			CacheRead: 150000, CacheCreate: 8000,
			CostUSD: 1.81, DurationMs: 350000,
			ModelBreakdown: map[string]*ModelStats{
				"claude-opus-4-6": {Calls: 4, InputTokens: 15000, OutputTokens: 12000, CostUSD: 1.62},
			},
		},
		{
			Label: "mindspec", APICallCount: 12,
			InputTokens: 33000, OutputTokens: 36000,
			CacheRead: 2800000, CacheCreate: 140000,
			CostUSD: 2.10, DurationMs: 360000,
			ModelBreakdown: map[string]*ModelStats{
				"claude-opus-4-6":           {Calls: 3, InputTokens: 2000, OutputTokens: 20000, CostUSD: 1.77},
				"claude-haiku-4-5-20251001": {Calls: 9, InputTokens: 31000, OutputTokens: 16000, CostUSD: 0.32},
			},
		},
	}

	report := CompareN(sessions)
	if len(report.Sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(report.Sessions))
	}
}

func TestFormatTableN(t *testing.T) {
	sessions := []*Session{
		{
			Label: "no-docs", APICallCount: 10,
			InputTokens: 30000, OutputTokens: 10000,
			CacheRead: 100000, CacheCreate: 5000,
			CostUSD: 1.18, DurationMs: 200000,
			ModelBreakdown: map[string]*ModelStats{
				"claude-opus-4-6": {Calls: 5, InputTokens: 20000, OutputTokens: 8000, CostUSD: 0.96},
			},
		},
		{
			Label: "baseline", APICallCount: 8,
			InputTokens: 20000, OutputTokens: 15000,
			CacheRead: 150000, CacheCreate: 8000,
			CostUSD: 1.81, DurationMs: 350000,
			ModelBreakdown: map[string]*ModelStats{},
		},
		{
			Label: "mindspec", APICallCount: 12,
			InputTokens: 33000, OutputTokens: 36000,
			CacheRead: 2800000, CacheCreate: 140000,
			CostUSD: 2.10, DurationMs: 360000,
			ModelBreakdown: map[string]*ModelStats{
				"claude-opus-4-6": {Calls: 3, InputTokens: 2000, OutputTokens: 20000, CostUSD: 1.77},
			},
		},
	}

	report := CompareN(sessions)
	table := FormatTableN(report)

	// Should contain all 3 labels
	if !strings.Contains(table, "no-docs") {
		t.Error("table missing 'no-docs'")
	}
	if !strings.Contains(table, "baseline") {
		t.Error("table missing 'baseline'")
	}
	if !strings.Contains(table, "mindspec") {
		t.Error("table missing 'mindspec'")
	}

	// Should contain metric rows
	for _, metric := range []string{"API Calls", "Input Tokens", "Output Tokens", "Total Tokens", "Cost (USD)", "Duration", "Cache Hit Rate"} {
		if !strings.Contains(table, metric) {
			t.Errorf("table missing '%s' row", metric)
		}
	}

	// Should NOT contain "Delta" column
	lines := strings.Split(table, "\n")
	if strings.Contains(lines[0], "Delta") {
		t.Error("N-way table should not have Delta column")
	}
}

func TestFormatTableNTwoSessions(t *testing.T) {
	// Verify 2-session N-way still works (even though CLI uses pairwise for 2)
	sessions := []*Session{
		{Label: "a", InputTokens: 100, OutputTokens: 50, ModelBreakdown: map[string]*ModelStats{}},
		{Label: "b", InputTokens: 200, OutputTokens: 100, ModelBreakdown: map[string]*ModelStats{}},
	}
	report := CompareN(sessions)
	table := FormatTableN(report)
	if !strings.Contains(table, "a") || !strings.Contains(table, "b") {
		t.Error("table missing labels")
	}
}

func TestNeutralizeBaseline(t *testing.T) {
	dir := t.TempDir()

	// Create fixture files
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("test"), 0644)
	os.MkdirAll(filepath.Join(dir, ".mindspec"), 0755)
	os.WriteFile(filepath.Join(dir, ".mindspec", "focus"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(dir, ".claude", "commands"), 0755)
	os.WriteFile(filepath.Join(dir, ".claude", "commands", "spec-init.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(dir, ".claude", "commands", "spec-approve.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(dir, ".claude", "commands", "custom.md"), []byte("keep"), 0644)
	os.MkdirAll(filepath.Join(dir, "docs", "core"), 0755)
	os.WriteFile(filepath.Join(dir, "docs", "core", "README.md"), []byte("docs"), 0644)

	// Write settings.json with hooks
	settings := map[string]any{
		"hooks":       map[string]any{"pre-tool-use": "echo test"},
		"permissions": map[string]any{"allow": []string{"Read"}},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), data, 0644)

	err := NeutralizeBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}

	// CLAUDE.md should be removed
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("CLAUDE.md should be removed")
	}

	// .mindspec/ should be removed
	if _, err := os.Stat(filepath.Join(dir, ".mindspec")); !os.IsNotExist(err) {
		t.Error(".mindspec/ should be removed")
	}

	// MindSpec commands removed, custom kept
	if _, err := os.Stat(filepath.Join(dir, ".claude", "commands", "spec-init.md")); !os.IsNotExist(err) {
		t.Error("spec-init.md should be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "commands", "custom.md")); os.IsNotExist(err) {
		t.Error("custom.md should be preserved")
	}

	// docs/ should still exist
	if _, err := os.Stat(filepath.Join(dir, "docs", "core", "README.md")); os.IsNotExist(err) {
		t.Error("docs/ should be preserved for baseline")
	}

	// settings.json: hooks removed, permissions preserved
	settingsData, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(settingsData, &result)
	if _, ok := result["hooks"]; ok {
		t.Error("hooks should be removed from settings.json")
	}
	if _, ok := result["permissions"]; !ok {
		t.Error("permissions should be preserved in settings.json")
	}
}

func TestNeutralizeNoDocs(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("test"), 0644)
	os.MkdirAll(filepath.Join(dir, "docs", "core"), 0755)
	os.WriteFile(filepath.Join(dir, "docs", "core", "README.md"), []byte("docs"), 0644)

	err := NeutralizeNoDocs(dir)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("CLAUDE.md should be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "docs")); !os.IsNotExist(err) {
		t.Error("docs/ should be removed for no-docs")
	}
}

func TestStripHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Settings with hooks and other keys
	settings := map[string]any{
		"hooks":       map[string]any{"SessionStart": []any{map[string]any{"matcher": "", "hooks": []any{map[string]any{"type": "command", "command": "mindspec instruct"}}}}},
		"permissions": map[string]any{"allow": []string{"Read", "Write"}},
		"model":       "claude-opus-4-6",
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(path, data, 0644)

	err := stripHooks(path)
	if err != nil {
		t.Fatal(err)
	}

	result, _ := os.ReadFile(path)
	var parsed map[string]any
	json.Unmarshal(result, &parsed)

	if _, ok := parsed["hooks"]; ok {
		t.Error("hooks should be removed")
	}
	if _, ok := parsed["permissions"]; !ok {
		t.Error("permissions should be preserved")
	}
	if _, ok := parsed["model"]; !ok {
		t.Error("model should be preserved")
	}
}

func TestStripHooksFileNotExist(t *testing.T) {
	err := stripHooks("/nonexistent/settings.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestWaitForPort(t *testing.T) {
	// Start a test HTTP server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	server := &http.Server{Handler: http.NewServeMux()}
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	// Should succeed
	err = waitForPort(port, 3*time.Second)
	if err != nil {
		t.Errorf("waitForPort should succeed for listening port: %v", err)
	}
}

func TestWaitForPortTimeout(t *testing.T) {
	// Use a port that's likely not in use
	err := waitForPort(59999, 500*time.Millisecond)
	if err == nil {
		t.Error("waitForPort should fail for non-listening port")
	}
}

func TestCheckPortFree(t *testing.T) {
	// Port that's not in use
	err := CheckPortFree(59998)
	if err != nil {
		t.Errorf("CheckPortFree should return nil for free port: %v", err)
	}

	// Start a listener and check it's detected
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	err = CheckPortFree(port)
	if err == nil {
		t.Error("CheckPortFree should return error for occupied port")
	}
}

func TestBenchmarkDir(t *testing.T) {
	dir := BenchmarkDir("/repo", "021-bench-go-command")
	expected := filepath.Join("/repo", "docs", "specs", "021-bench-go-command", "benchmark")
	if dir != expected {
		t.Errorf("BenchmarkDir = %q, want %q", dir, expected)
	}
}

func TestAssembleReportMD(t *testing.T) {
	cfg := &RunConfig{
		SpecID:      "021-test",
		BenchCommit: "abc123",
		Timeout:     30 * time.Minute,
		Prompt:      "test prompt",
	}
	results := []*SessionResult{
		{Label: "a", EventCount: 50},
		{Label: "b", EventCount: 30},
		{Label: "c", EventCount: 70},
	}
	qual := &QualitativeResult{Analysis: "test analysis"}

	report := assembleReportMD(cfg, results, "quant table here", qual, "trace summary here")

	if !strings.Contains(report, "# Benchmark: 021-test") {
		t.Error("missing header")
	}
	if !strings.Contains(report, "abc123") {
		t.Error("missing commit")
	}
	if !strings.Contains(report, "test prompt") {
		t.Error("missing prompt")
	}
	if !strings.Contains(report, "quant table here") {
		t.Error("missing quantitative report")
	}
	if !strings.Contains(report, "test analysis") {
		t.Error("missing qualitative analysis")
	}
	if !strings.Contains(report, "trace summary here") {
		t.Error("missing trace summary")
	}
}

func TestWriteResults(t *testing.T) {
	dir := t.TempDir()
	workDir := t.TempDir()

	// Create fake session artifacts (single shared JSONL + per-session output)
	benchEvents := `{"ts":"2026-01-01T00:00:00Z","event":"claude_code.api_request","data":{"input_tokens":100},"resource":{"bench.label":"a"}}
{"ts":"2026-01-01T00:01:00Z","event":"claude_code.api_request","data":{"input_tokens":200},"resource":{"bench.label":"b"}}
{"ts":"2026-01-01T00:02:00Z","event":"claude_code.api_request","data":{"input_tokens":300},"resource":{"bench.label":"b"}}
{"ts":"2026-01-01T00:03:00Z","event":"claude_code.api_request","data":{"input_tokens":400},"resource":{"bench.label":"c"}}
{"ts":"2026-01-01T00:04:00Z","event":"claude_code.api_request","data":{"input_tokens":500},"resource":{"bench.label":"c"}}
{"ts":"2026-01-01T00:05:00Z","event":"claude_code.api_request","data":{"input_tokens":600},"resource":{"bench.label":"c"}}
`
	benchEventsPath := filepath.Join(workDir, "bench-events.jsonl")
	os.WriteFile(benchEventsPath, []byte(benchEvents), 0644)
	os.WriteFile(filepath.Join(workDir, "output-a.txt"), []byte("output a"), 0644)
	os.WriteFile(filepath.Join(workDir, "output-b.txt"), []byte("output b"), 0644)
	os.WriteFile(filepath.Join(workDir, "output-c.txt"), []byte("output c"), 0644)

	cfg := &RunConfig{
		SpecID:      "test-spec",
		BenchCommit: "abc123",
		Timeout:     30 * time.Minute,
		Prompt:      "test",
		RepoRoot:    dir,
		WorkDir:     workDir,
	}
	results := []*SessionResult{
		{Label: "a", JSONLPath: benchEventsPath, OutputPath: filepath.Join(workDir, "output-a.txt"), EventCount: 1},
		{Label: "b", JSONLPath: benchEventsPath, OutputPath: filepath.Join(workDir, "output-b.txt"), EventCount: 2},
		{Label: "c", JSONLPath: benchEventsPath, OutputPath: filepath.Join(workDir, "output-c.txt"), EventCount: 3},
	}

	plans := map[string]string{
		"a": "# Plan A\nImplement feature X",
		"b": "(No plan artifact found for Session B)",
		"c": "# Plan C\nFull MindSpec plan",
	}
	diffs := map[string]string{
		"a": "diff --git a/foo.go\n+func foo() {}",
		"b": "",
		"c": "diff --git a/bar.go\n+func bar() {}",
	}

	err := WriteResults(cfg, results, "quant report", nil, "", plans, diffs)
	if err != nil {
		t.Fatal(err)
	}

	benchDir := BenchmarkDir(dir, "test-spec")

	// Check report.md exists
	if _, err := os.Stat(filepath.Join(benchDir, "report.md")); os.IsNotExist(err) {
		t.Error("report.md not created")
	}

	// Check artifacts copied
	for _, name := range []string{"bench-events.jsonl", "output-a.txt", "output-b.txt", "output-c.txt"} {
		if _, err := os.Stat(filepath.Join(benchDir, name)); os.IsNotExist(err) {
			t.Errorf("%s not copied", name)
		}
	}

	// Check plan artifacts persisted
	if data, err := os.ReadFile(filepath.Join(benchDir, "plan-a.md")); err != nil {
		t.Error("plan-a.md not created")
	} else if string(data) != plans["a"] {
		t.Errorf("plan-a.md content mismatch: got %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(benchDir, "plan-b.md")); !os.IsNotExist(err) {
		t.Error("plan-b.md should not be created for missing plan")
	}
	if data, err := os.ReadFile(filepath.Join(benchDir, "plan-c.md")); err != nil {
		t.Error("plan-c.md not created")
	} else if string(data) != plans["c"] {
		t.Errorf("plan-c.md content mismatch: got %q", string(data))
	}

	// Check diff artifacts persisted
	if data, err := os.ReadFile(filepath.Join(benchDir, "diff-a.patch")); err != nil {
		t.Error("diff-a.patch not created")
	} else if string(data) != diffs["a"] {
		t.Errorf("diff-a.patch content mismatch: got %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(benchDir, "diff-b.patch")); !os.IsNotExist(err) {
		t.Error("diff-b.patch should not be created for empty diff")
	}
	if data, err := os.ReadFile(filepath.Join(benchDir, "diff-c.patch")); err != nil {
		t.Error("diff-c.patch not created")
	} else if string(data) != diffs["c"] {
		t.Errorf("diff-c.patch content mismatch: got %q", string(data))
	}
}

func TestBuildQualPrompt(t *testing.T) {
	plans := map[string]string{"a": "plan a", "b": "plan b", "c": "plan c"}
	diffs := map[string]string{"a": "diff a", "b": "diff b", "c": "diff c"}
	prompt := buildQualPrompt("implement feature X", "quant report", plans, diffs)

	if !strings.Contains(prompt, "implement feature X") {
		t.Error("missing feature prompt")
	}
	if !strings.Contains(prompt, "quant report") {
		t.Error("missing quantitative report")
	}
	if !strings.Contains(prompt, "plan a") {
		t.Error("missing plan a")
	}
	if !strings.Contains(prompt, "diff c") {
		t.Error("missing diff c")
	}
	if !strings.Contains(prompt, "Planning Quality") {
		t.Error("missing dimension header")
	}
}

func TestBuildImprovPrompt(t *testing.T) {
	plans := map[string]string{"a": "plan a", "b": "plan b", "c": "plan c"}
	diffs := map[string]string{"a": "diff a", "b": "diff b", "c": "diff c"}
	prompt := buildImprovPrompt("implement feature X", "qual analysis", plans, diffs)

	if !strings.Contains(prompt, "implement feature X") {
		t.Error("missing feature prompt")
	}
	if !strings.Contains(prompt, "qual analysis") {
		t.Error("missing qualitative analysis")
	}
	if !strings.Contains(prompt, "Improvements from Non-MindSpec") {
		t.Error("missing improvements format header")
	}
}

func TestCollectPlans(t *testing.T) {
	workDir := t.TempDir()

	// Session C plan
	os.MkdirAll(filepath.Join(workDir, "wt-c", "docs", "specs", "test-spec"), 0755)
	os.WriteFile(filepath.Join(workDir, "wt-c", "docs", "specs", "test-spec", "plan.md"), []byte("# Plan C"), 0644)

	// Session A plan (Claude plans dir)
	os.MkdirAll(filepath.Join(workDir, "wt-a", ".claude", "plans"), 0755)
	os.WriteFile(filepath.Join(workDir, "wt-a", ".claude", "plans", "plan.md"), []byte("# Plan A"), 0644)

	// Session B: no plan
	os.MkdirAll(filepath.Join(workDir, "wt-b"), 0755)

	cfg := &RunConfig{WorkDir: workDir}
	plans := CollectPlans(cfg, "test-spec")

	if plans["c"] != "# Plan C" {
		t.Errorf("plan c = %q, want '# Plan C'", plans["c"])
	}
	if plans["a"] != "# Plan A" {
		t.Errorf("plan a = %q, want '# Plan A'", plans["a"])
	}
	if !strings.Contains(plans["b"], "No plan artifact") {
		t.Errorf("plan b should indicate no plan found, got %q", plans["b"])
	}
}

func TestMergedModelNamesN(t *testing.T) {
	sessions := []*Session{
		{ModelBreakdown: map[string]*ModelStats{"model-a": {}, "model-b": {}}},
		{ModelBreakdown: map[string]*ModelStats{"model-b": {}, "model-c": {}}},
		{ModelBreakdown: map[string]*ModelStats{"model-a": {}}},
	}
	names := mergedModelNamesN(sessions)
	if len(names) != 3 {
		t.Errorf("expected 3 merged model names, got %d", len(names))
	}
	// Should be sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Error("model names not sorted")
		}
	}
}

func TestRunConfigPromptFile(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "prompt.txt")
	os.WriteFile(promptFile, []byte("implement feature X from file"), 0644)

	data, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(data)
	if prompt != "implement feature X from file" {
		t.Errorf("prompt = %q, want 'implement feature X from file'", prompt)
	}
}

// splitLabels is tested via the CLI wiring test above, but also test the bench package version
func TestFormatTableNEmptySessions(t *testing.T) {
	report := CompareN(nil)
	if FormatTableN(report) != "" {
		t.Error("FormatTableN should return empty string for nil sessions")
	}

	report = &MultiReport{Sessions: []*Session{}}
	if FormatTableN(report) != "" {
		t.Error("FormatTableN should return empty string for empty sessions")
	}
}

func TestBuildRetryPrompt(t *testing.T) {
	// Sessions A/B: escalating prompts
	p1 := buildRetryPrompt("a", "/tmp", "test-spec", 1)
	if !strings.Contains(p1, "approved") {
		t.Error("first retry for A should mention approval")
	}
	p2 := buildRetryPrompt("b", "/tmp", "test-spec", 2)
	if !strings.Contains(p2, "required") {
		t.Error("second retry for B should escalate")
	}

	// Session C: state-dependent prompts
	wtDir := t.TempDir()
	os.MkdirAll(filepath.Join(wtDir, ".mindspec"), 0755)

	// Plan mode → should mention plan creation
	os.WriteFile(filepath.Join(wtDir, ".mindspec", "focus"),
		[]byte(`{"mode":"plan","activeSpec":"test-spec"}`), 0644)
	p := buildRetryPrompt("c", wtDir, "test-spec", 1)
	if !strings.Contains(p, "plan") {
		t.Error("plan mode retry should mention plan")
	}

	// Implement mode → should mention implementation
	os.WriteFile(filepath.Join(wtDir, ".mindspec", "focus"),
		[]byte(`{"mode":"implement","activeSpec":"test-spec"}`), 0644)
	p = buildRetryPrompt("c", wtDir, "test-spec", 1)
	if !strings.Contains(p, "Implement") {
		t.Error("implement mode retry should mention implementation")
	}
}

func TestAutoApproveSessionC(t *testing.T) {
	wtDir := t.TempDir()
	os.MkdirAll(filepath.Join(wtDir, ".mindspec"), 0755)
	os.MkdirAll(filepath.Join(wtDir, "docs", "specs", "test-spec"), 0755)

	// Write initial focus: spec mode
	os.WriteFile(filepath.Join(wtDir, ".mindspec", "focus"),
		[]byte(`{"mode":"spec","activeSpec":"test-spec"}`), 0644)

	// Write a spec with frontmatter
	os.WriteFile(filepath.Join(wtDir, "docs", "specs", "test-spec", "spec.md"),
		[]byte("---\nstatus: Draft\napproved_at: \"\"\napproved_by: \"\"\n---\n# Spec\n"), 0644)

	// Auto-approve should advance spec → plan
	autoApprove("c", wtDir, "test-spec")

	// Check focus advanced to plan
	data, _ := os.ReadFile(filepath.Join(wtDir, ".mindspec", "focus"))
	if !strings.Contains(string(data), `"plan"`) {
		t.Errorf("focus should be plan, got: %s", string(data))
	}

	// Check spec frontmatter was updated
	specData, _ := os.ReadFile(filepath.Join(wtDir, "docs", "specs", "test-spec", "spec.md"))
	if !strings.Contains(string(specData), "Approved") {
		t.Error("spec status should be Approved")
	}

	// Now write a plan and auto-approve plan → implement
	os.WriteFile(filepath.Join(wtDir, "docs", "specs", "test-spec", "plan.md"),
		[]byte("---\nstatus: Draft\napproved_at: \"\"\napproved_by: \"\"\n---\n# Plan\n"), 0644)

	autoApprove("c", wtDir, "test-spec")

	data, _ = os.ReadFile(filepath.Join(wtDir, ".mindspec", "focus"))
	if !strings.Contains(string(data), `"implement"`) {
		t.Errorf("focus should be implement, got: %s", string(data))
	}
}

func TestAutoApproveNoopForAB(t *testing.T) {
	// autoApprove should be a no-op for sessions A and B
	autoApprove("a", "/nonexistent", "test-spec") // should not panic
	autoApprove("b", "/nonexistent", "test-spec") // should not panic
}

// Verify N-way backward compat: 2 sessions via bench report still works
func TestPairwiseReportStillWorks(t *testing.T) {
	a := &Session{
		Label: "mindspec", APICallCount: 10,
		InputTokens: 50000, OutputTokens: 10000,
		CostUSD: 0.50, DurationMs: 300000,
		ModelBreakdown: map[string]*ModelStats{},
	}
	b := &Session{
		Label: "baseline", APICallCount: 15,
		InputTokens: 80000, OutputTokens: 12000,
		CostUSD: 0.85, DurationMs: 450000,
		ModelBreakdown: map[string]*ModelStats{},
	}

	// Pairwise still works
	report := Compare(a, b)
	table := FormatTable(report)
	if !strings.Contains(table, "Delta") {
		t.Error("pairwise table should have Delta column")
	}
	if !strings.Contains(table, "mindspec") {
		t.Error("pairwise table should have labels")
	}

	// N-way also works for 2
	nReport := CompareN([]*Session{a, b})
	nTable := FormatTableN(nReport)
	if strings.Contains(nTable, "Delta") {
		t.Error("N-way table should NOT have Delta column")
	}

	fmt.Println("--- Pairwise ---")
	fmt.Print(table)
	fmt.Println("\n--- N-way (2 sessions) ---")
	fmt.Print(nTable)
}
