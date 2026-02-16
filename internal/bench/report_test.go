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
				"cost_usd":              float64(0.03),
				"model":                 "claude-opus-4-6",
			},
		},
		{
			TS:    "2026-02-13T10:05:00.000000000Z",
			Event: "claude_code.api_request",
			Data: map[string]any{
				"input_tokens":  float64(3000),
				"output_tokens": float64(800),
				"cost_usd":      float64(0.02),
				"model":         "claude-opus-4-6",
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

func TestParseSessionMetricEvents(t *testing.T) {
	dir := t.TempDir()
	events := []CollectedEvent{
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "claude_code.token.usage",
			Data:  map[string]any{"type": "input", "value": float64(5000), "model": "claude-opus-4-6"},
		},
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "claude_code.token.usage",
			Data:  map[string]any{"type": "output", "value": float64(1000), "model": "claude-opus-4-6"},
		},
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "claude_code.token.usage",
			Data:  map[string]any{"type": "cacheRead", "value": float64(2000), "model": "claude-opus-4-6"},
		},
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "claude_code.token.usage",
			Data:  map[string]any{"type": "cacheCreation", "value": float64(500), "model": "claude-opus-4-6"},
		},
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "claude_code.cost.usage",
			Data:  map[string]any{"value": float64(0.03), "model": "claude-opus-4-6"},
		},
		// Second batch (delta)
		{
			TS:    "2026-02-13T10:05:00.000000000Z",
			Event: "claude_code.token.usage",
			Data:  map[string]any{"type": "input", "value": float64(3000), "model": "claude-opus-4-6"},
		},
		{
			TS:    "2026-02-13T10:05:00.000000000Z",
			Event: "claude_code.token.usage",
			Data:  map[string]any{"type": "output", "value": float64(800), "model": "claude-opus-4-6"},
		},
		{
			TS:    "2026-02-13T10:05:00.000000000Z",
			Event: "claude_code.cost.usage",
			Data:  map[string]any{"value": float64(0.02), "model": "claude-opus-4-6"},
		},
	}

	path := writeFixture(t, dir, "session-metrics.jsonl", events)
	s, err := ParseSession(path, "test")
	if err != nil {
		t.Fatal(err)
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
	if s.CacheCreate != 500 {
		t.Errorf("CacheCreate = %d, want 500", s.CacheCreate)
	}
	if s.TotalTokens() != 9800 {
		t.Errorf("TotalTokens = %d, want 9800", s.TotalTokens())
	}
	if s.DurationMs != 300000 {
		t.Errorf("DurationMs = %f, want 300000", s.DurationMs)
	}
	// Cost
	expectedCost := 0.05
	if diff := s.CostUSD - expectedCost; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("CostUSD = %f, want %f", s.CostUSD, expectedCost)
	}
	// Model breakdown
	ms, ok := s.ModelBreakdown["claude-opus-4-6"]
	if !ok {
		t.Fatal("missing model breakdown for claude-opus-4-6")
	}
	if ms.InputTokens != 8000 {
		t.Errorf("model InputTokens = %d, want 8000", ms.InputTokens)
	}
	if ms.OutputTokens != 1800 {
		t.Errorf("model OutputTokens = %d, want 1800", ms.OutputTokens)
	}
}

func TestParseSessionCodexMetricEvents(t *testing.T) {
	dir := t.TempDir()
	events := []CollectedEvent{
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "codex.token.usage",
			Data:  map[string]any{"type": "input", "value": float64(4000), "model": "gpt-5-codex"},
		},
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "codex.token.usage",
			Data:  map[string]any{"type": "output", "value": float64(900), "model": "gpt-5-codex"},
		},
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "codex.cost.usage",
			Data:  map[string]any{"value": float64(0.04), "model": "gpt-5-codex"},
		},
	}

	path := writeFixture(t, dir, "session-codex-metrics.jsonl", events)
	s, err := ParseSession(path, "codex")
	if err != nil {
		t.Fatal(err)
	}

	if s.InputTokens != 4000 {
		t.Errorf("InputTokens = %d, want 4000", s.InputTokens)
	}
	if s.OutputTokens != 900 {
		t.Errorf("OutputTokens = %d, want 900", s.OutputTokens)
	}
	if s.TotalTokens() != 4900 {
		t.Errorf("TotalTokens = %d, want 4900", s.TotalTokens())
	}
	if diff := s.CostUSD - 0.04; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("CostUSD = %f, want 0.04", s.CostUSD)
	}
	ms, ok := s.ModelBreakdown["gpt-5-codex"]
	if !ok {
		t.Fatal("missing model breakdown for gpt-5-codex")
	}
	if ms.InputTokens != 4000 {
		t.Errorf("model InputTokens = %d, want 4000", ms.InputTokens)
	}
	if ms.OutputTokens != 900 {
		t.Errorf("model OutputTokens = %d, want 900", ms.OutputTokens)
	}
}

func TestParseSessionMixedClaudeAndCodex(t *testing.T) {
	dir := t.TempDir()
	events := []CollectedEvent{
		{
			TS:    "2026-02-13T10:00:00.000000000Z",
			Event: "claude_code.api_request",
			Data: map[string]any{
				"input_tokens":  float64(1000),
				"output_tokens": float64(200),
				"cost_usd":      float64(0.01),
				"model":         "claude-opus-4-6",
			},
		},
		{
			TS:    "2026-02-13T10:00:01.000000000Z",
			Event: "codex.api_request",
			Data: map[string]any{
				"input_tokens":  float64(1200),
				"output_tokens": float64(300),
				"cost_usd":      float64(0.02),
				"model":         "gpt-5-codex",
			},
		},
		{
			TS:    "2026-02-13T10:00:02.000000000Z",
			Event: "codex.token.usage",
			Data: map[string]any{
				"type":  "cacheRead",
				"value": float64(500),
				"model": "gpt-5-codex",
			},
		},
	}

	path := writeFixture(t, dir, "session-mixed.jsonl", events)
	s, err := ParseSession(path, "mixed")
	if err != nil {
		t.Fatal(err)
	}

	if s.APICallCount != 2 {
		t.Errorf("APICallCount = %d, want 2", s.APICallCount)
	}
	if s.InputTokens != 2200 {
		t.Errorf("InputTokens = %d, want 2200", s.InputTokens)
	}
	if s.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", s.OutputTokens)
	}
	if s.CacheRead != 500 {
		t.Errorf("CacheRead = %d, want 500", s.CacheRead)
	}
	if diff := s.CostUSD - 0.03; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("CostUSD = %f, want 0.03", s.CostUSD)
	}
	if _, ok := s.ModelBreakdown["claude-opus-4-6"]; !ok {
		t.Error("expected claude model breakdown")
	}
	if _, ok := s.ModelBreakdown["gpt-5-codex"]; !ok {
		t.Error("expected codex model breakdown")
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

func TestParseSessionByLabel(t *testing.T) {
	dir := t.TempDir()

	// Shared JSONL with events from 3 sessions differentiated by bench.label
	events := []CollectedEvent{
		{
			TS:       "2026-01-01T00:00:00Z",
			Event:    "claude_code.api_request",
			Data:     map[string]any{"input_tokens": float64(100), "output_tokens": float64(50), "cost_usd": float64(0.01), "model": "opus"},
			Resource: map[string]any{"bench.label": "a"},
		},
		{
			TS:       "2026-01-01T00:01:00Z",
			Event:    "claude_code.api_request",
			Data:     map[string]any{"input_tokens": float64(200), "output_tokens": float64(100), "cost_usd": float64(0.02), "model": "opus"},
			Resource: map[string]any{"bench.label": "b"},
		},
		{
			TS:       "2026-01-01T00:02:00Z",
			Event:    "claude_code.api_request",
			Data:     map[string]any{"input_tokens": float64(300), "output_tokens": float64(150), "cost_usd": float64(0.03), "model": "sonnet"},
			Resource: map[string]any{"bench.label": "a"},
		},
		{
			TS:       "2026-01-01T00:03:00Z",
			Event:    "claude_code.api_request",
			Data:     map[string]any{"input_tokens": float64(400), "output_tokens": float64(200), "cost_usd": float64(0.04), "model": "opus"},
			Resource: map[string]any{"bench.label": "c"},
		},
	}

	path := writeFixture(t, dir, "bench-events.jsonl", events)

	// Parse only session A events
	s, err := ParseSessionByLabel(path, "a")
	if err != nil {
		t.Fatalf("ParseSessionByLabel(a): %v", err)
	}
	if s.APICallCount != 2 {
		t.Errorf("session A: APICallCount = %d, want 2", s.APICallCount)
	}
	if s.InputTokens != 400 { // 100 + 300
		t.Errorf("session A: InputTokens = %d, want 400", s.InputTokens)
	}
	if s.OutputTokens != 200 { // 50 + 150
		t.Errorf("session A: OutputTokens = %d, want 200", s.OutputTokens)
	}

	// Parse only session B events
	s, err = ParseSessionByLabel(path, "b")
	if err != nil {
		t.Fatalf("ParseSessionByLabel(b): %v", err)
	}
	if s.APICallCount != 1 {
		t.Errorf("session B: APICallCount = %d, want 1", s.APICallCount)
	}
	if s.InputTokens != 200 {
		t.Errorf("session B: InputTokens = %d, want 200", s.InputTokens)
	}

	// Parse only session C events
	s, err = ParseSessionByLabel(path, "c")
	if err != nil {
		t.Fatalf("ParseSessionByLabel(c): %v", err)
	}
	if s.APICallCount != 1 {
		t.Errorf("session C: APICallCount = %d, want 1", s.APICallCount)
	}

	// Non-existent label returns empty session
	s, err = ParseSessionByLabel(path, "x")
	if err != nil {
		t.Fatalf("ParseSessionByLabel(x): %v", err)
	}
	if s.APICallCount != 0 {
		t.Errorf("session X: APICallCount = %d, want 0", s.APICallCount)
	}
}

func TestParseSessionBackwardCompat(t *testing.T) {
	dir := t.TempDir()

	// Standalone JSONL without bench.label (legacy format)
	events := []CollectedEvent{
		{
			TS:    "2026-01-01T00:00:00Z",
			Event: "claude_code.api_request",
			Data:  map[string]any{"input_tokens": float64(100), "output_tokens": float64(50), "cost_usd": float64(0.01)},
		},
		{
			TS:    "2026-01-01T00:01:00Z",
			Event: "claude_code.api_request",
			Data:  map[string]any{"input_tokens": float64(200), "output_tokens": float64(100), "cost_usd": float64(0.02)},
		},
	}

	path := writeFixture(t, dir, "legacy-session.jsonl", events)

	// ParseSession aggregates all events regardless of bench.label
	s, err := ParseSession(path, "legacy")
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if s.APICallCount != 2 {
		t.Errorf("APICallCount = %d, want 2", s.APICallCount)
	}
	if s.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", s.InputTokens)
	}
	if s.Label != "legacy" {
		t.Errorf("Label = %q, want 'legacy'", s.Label)
	}
}

func TestExtractSessionIDs(t *testing.T) {
	dir := t.TempDir()

	events := []CollectedEvent{
		{TS: "2026-01-01T00:00:00Z", Event: "claude_code.api_request", Data: map[string]any{"session.id": "uuid-1"}, Resource: map[string]any{"bench.label": "a"}},
		{TS: "2026-01-01T00:01:00Z", Event: "claude_code.api_request", Data: map[string]any{"session.id": "uuid-2"}, Resource: map[string]any{"bench.label": "b"}},
		{TS: "2026-01-01T00:02:00Z", Event: "claude_code.api_request", Data: map[string]any{"session.id": "uuid-1"}, Resource: map[string]any{"bench.label": "a"}},
		{TS: "2026-01-01T00:03:00Z", Event: "claude_code.api_request", Data: map[string]any{"session.id": "uuid-3"}, Resource: map[string]any{"bench.label": "a"}},
	}

	path := writeFixture(t, dir, "bench-events.jsonl", events)

	ids := ExtractSessionIDs(path, "a")
	if len(ids) != 2 {
		t.Fatalf("expected 2 session IDs for label a, got %d: %v", len(ids), ids)
	}
	// Sorted: uuid-1, uuid-3
	if ids[0] != "uuid-1" || ids[1] != "uuid-3" {
		t.Errorf("unexpected session IDs: %v", ids)
	}

	ids = ExtractSessionIDs(path, "b")
	if len(ids) != 1 || ids[0] != "uuid-2" {
		t.Errorf("expected [uuid-2] for label b, got %v", ids)
	}

	ids = ExtractSessionIDs(path, "x")
	if len(ids) != 0 {
		t.Errorf("expected 0 session IDs for label x, got %v", ids)
	}
}

func TestCountEventsByLabel(t *testing.T) {
	dir := t.TempDir()

	events := []CollectedEvent{
		{TS: "2026-01-01T00:00:00Z", Event: "e", Data: map[string]any{}, Resource: map[string]any{"bench.label": "a"}},
		{TS: "2026-01-01T00:01:00Z", Event: "e", Data: map[string]any{}, Resource: map[string]any{"bench.label": "b"}},
		{TS: "2026-01-01T00:02:00Z", Event: "e", Data: map[string]any{}, Resource: map[string]any{"bench.label": "a"}},
		{TS: "2026-01-01T00:03:00Z", Event: "e", Data: map[string]any{}, Resource: map[string]any{"bench.label": "a"}},
	}

	path := writeFixture(t, dir, "bench-events.jsonl", events)

	if c := countEventsByLabel(path, "a"); c != 3 {
		t.Errorf("expected 3 events for label a, got %d", c)
	}
	if c := countEventsByLabel(path, "b"); c != 1 {
		t.Errorf("expected 1 event for label b, got %d", c)
	}
	if c := countEventsByLabel(path, "x"); c != 0 {
		t.Errorf("expected 0 events for label x, got %d", c)
	}
}
