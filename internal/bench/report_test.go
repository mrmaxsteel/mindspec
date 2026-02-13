package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, dir, name string, events []CollectedEvent) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, e := range events {
		data, _ := json.Marshal(e)
		f.Write(data)
		f.Write([]byte("\n"))
	}
	return path
}

func TestParseSession(t *testing.T) {
	dir := t.TempDir()
	events := []CollectedEvent{
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "claude_code.api_request",
			Data: map[string]any{
				"input_tokens":          float64(5000),
				"output_tokens":         float64(1000),
				"cache_read_tokens":     float64(2000),
				"cache_creation_tokens": float64(500),
				"cost_usd":             float64(0.03),
				"model":                "claude-opus-4-6",
			},
		},
		{
			TS:    "2026-02-13T10:05:00.000000000Z",
			Event: "claude_code.api_request",
			Data: map[string]any{
				"input_tokens":  float64(3000),
				"output_tokens": float64(800),
				"cost_usd":     float64(0.02),
				"model":        "claude-opus-4-6",
			},
		},
	}

	path := writeFixture(t, dir, "session.jsonl", events)
	s, err := ParseSession(path, "test")
	if err != nil {
		t.Fatal(err)
	}

	if s.APICallCount != 2 {
		t.Errorf("APICallCount = %d, want 2", s.APICallCount)
	}
	if s.InputTokens != 8000 {
		t.Errorf("InputTokens = %d, want 8000", s.InputTokens)
	}
	if s.OutputTokens != 1800 {
		t.Errorf("OutputTokens = %d, want 1800", s.OutputTokens)
	}
	if s.CacheRead != 2000 {
		t.Errorf("CacheRead = %d, want 2000", s.CacheRead)
	}
	if s.TotalTokens() != 9800 {
		t.Errorf("TotalTokens = %d, want 9800", s.TotalTokens())
	}
	if s.DurationMs != 300000 { // 5 minutes
		t.Errorf("DurationMs = %f, want 300000", s.DurationMs)
	}
}

func TestCompare(t *testing.T) {
	a := &Session{
		Label:        "mindspec",
		APICallCount: 10,
		InputTokens:  50000,
		OutputTokens: 10000,
		CostUSD:      0.50,
		DurationMs:   300000,
	}
	b := &Session{
		Label:        "baseline",
		APICallCount: 15,
		InputTokens:  80000,
		OutputTokens: 12000,
		CostUSD:      0.85,
		DurationMs:   450000,
	}

	r := Compare(a, b)

	if r.Delta.APICallCount != -5 {
		t.Errorf("Delta.APICallCount = %d, want -5", r.Delta.APICallCount)
	}
	if r.Delta.InputTokens != -30000 {
		t.Errorf("Delta.InputTokens = %d, want -30000", r.Delta.InputTokens)
	}
	if r.Delta.TotalTokens != -32000 {
		t.Errorf("Delta.TotalTokens = %d, want -32000", r.Delta.TotalTokens)
	}
}

func TestFormatTable(t *testing.T) {
	a := &Session{
		Label:        "mindspec",
		APICallCount: 10,
		InputTokens:  50000,
		OutputTokens: 10000,
		CostUSD:      0.50,
		DurationMs:   300000,
	}
	b := &Session{
		Label:        "baseline",
		APICallCount: 15,
		InputTokens:  80000,
		OutputTokens: 12000,
		CostUSD:      0.85,
		DurationMs:   450000,
	}

	r := Compare(a, b)
	table := FormatTable(r)

	if !strings.Contains(table, "mindspec") {
		t.Error("table missing label 'mindspec'")
	}
	if !strings.Contains(table, "baseline") {
		t.Error("table missing label 'baseline'")
	}
	if !strings.Contains(table, "API Calls") {
		t.Error("table missing 'API Calls' row")
	}
	if !strings.Contains(table, "Total Tokens") {
		t.Error("table missing 'Total Tokens' row")
	}
}

func TestFormatJSON(t *testing.T) {
	a := &Session{Label: "a", ModelBreakdown: map[string]*ModelStats{}}
	b := &Session{Label: "b", ModelBreakdown: map[string]*ModelStats{}}
	r := Compare(a, b)

	out, err := FormatJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(out)) {
		t.Error("output is not valid JSON")
	}
}
