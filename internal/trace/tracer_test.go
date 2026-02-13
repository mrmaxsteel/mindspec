package trace

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNoopTracerProducesNoOutput(t *testing.T) {
	var buf bytes.Buffer
	prev := SetGlobal(noopTracer{})
	defer SetGlobal(prev)

	Emit(NewEvent("test.noop"))

	if buf.Len() != 0 {
		t.Errorf("noop tracer produced output: %s", buf.String())
	}
}

func TestNDJSONTracerWritesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	SetRunID("test-run-123")
	prev := SetGlobal(&ndjsonTracer{w: nopCloser{&buf}})
	defer SetGlobal(prev)

	e := NewEvent("test.event").
		WithSpec("018-observability").
		WithDuration(150 * time.Millisecond).
		WithTokens(42).
		WithData(map[string]any{"key": "value"})

	Emit(e)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no output produced")
	}

	var parsed Event
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nline: %s", err, line)
	}

	if parsed.Event != "test.event" {
		t.Errorf("event = %q, want %q", parsed.Event, "test.event")
	}
	if parsed.RunID != "test-run-123" {
		t.Errorf("run_id = %q, want %q", parsed.RunID, "test-run-123")
	}
	if parsed.SpecID != "018-observability" {
		t.Errorf("spec_id = %q, want %q", parsed.SpecID, "018-observability")
	}
	if parsed.DurMs < 149 || parsed.DurMs > 151 {
		t.Errorf("dur_ms = %f, want ~150", parsed.DurMs)
	}
	if parsed.Tokens != 42 {
		t.Errorf("tokens = %d, want 42", parsed.Tokens)
	}
	if parsed.Data["key"] != "value" {
		t.Errorf("data[key] = %v, want %q", parsed.Data["key"], "value")
	}
}

func TestNDJSONTracerMultipleEvents(t *testing.T) {
	var buf bytes.Buffer
	SetRunID("multi-run")
	prev := SetGlobal(&ndjsonTracer{w: nopCloser{&buf}})
	defer SetGlobal(prev)

	Emit(NewEvent("event.one"))
	Emit(NewEvent("event.two"))
	Emit(NewEvent("event.three"))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var parsed Event
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestOmitEmptyFields(t *testing.T) {
	var buf bytes.Buffer
	SetRunID("omit-run")
	prev := SetGlobal(&ndjsonTracer{w: nopCloser{&buf}})
	defer SetGlobal(prev)

	Emit(NewEvent("test.minimal"))

	line := strings.TrimSpace(buf.String())

	if strings.Contains(line, `"dur_ms"`) {
		t.Error("dur_ms should be omitted when zero")
	}
	if strings.Contains(line, `"tokens"`) {
		t.Error("tokens should be omitted when zero")
	}
	if strings.Contains(line, `"spec_id"`) {
		t.Error("spec_id should be omitted when empty")
	}
	if strings.Contains(line, `"data"`) {
		t.Error("data should be omitted when nil")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 1},
		{"abcd", 1},
		{"abcde", 2},
		{"hello world", 3}, // 11 bytes → ceil(11/4) = 3
	}

	for _, tt := range tests {
		got := EstimateTokens(tt.input)
		if got != tt.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestEstimateTokensBytes(t *testing.T) {
	got := EstimateTokensBytes([]byte("hello world"))
	if got != 3 {
		t.Errorf("EstimateTokensBytes = %d, want 3", got)
	}

	got = EstimateTokensBytes(nil)
	if got != 0 {
		t.Errorf("EstimateTokensBytes(nil) = %d, want 0", got)
	}
}

func TestInitAndClose(t *testing.T) {
	path := t.TempDir() + "/trace.jsonl"

	if err := Init(path); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Emit(NewEvent("test.init"))

	if err := Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reset to noop for other tests
	global = noopTracer{}
}

func TestEventTimestamp(t *testing.T) {
	SetRunID("ts-test")
	e := NewEvent("test.ts")

	_, err := time.Parse(time.RFC3339Nano, e.TS)
	if err != nil {
		t.Errorf("timestamp %q is not valid RFC3339Nano: %v", e.TS, err)
	}
}
