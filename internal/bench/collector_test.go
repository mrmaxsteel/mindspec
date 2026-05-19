package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractLogEvents(t *testing.T) {
	body := []byte(`{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1707840000000000000",
					"body": {"stringValue": "api_request"},
					"attributes": [
						{"key": "event.name", "value": {"stringValue": "claude_code.api_request"}},
						{"key": "input_tokens", "value": {"intValue": "5234"}},
						{"key": "output_tokens", "value": {"intValue": "1201"}},
						{"key": "cache_read_tokens", "value": {"intValue": "1000"}},
						{"key": "cache_creation_tokens", "value": {"intValue": "500"}},
						{"key": "cost_usd", "value": {"doubleValue": 0.0315}},
						{"key": "duration_ms", "value": {"intValue": "2847"}},
						{"key": "model", "value": {"stringValue": "claude-sonnet-4-5-20250929"}}
					]
				}]
			}]
		}]
	}`)

	events := extractLogEvents(body)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Event != "claude_code.api_request" {
		t.Errorf("event = %q, want %q", e.Event, "claude_code.api_request")
	}
	if e.TS == "" {
		t.Error("timestamp is empty")
	}

	if v, ok := e.Data["input_tokens"].(int64); !ok || v != 5234 {
		t.Errorf("input_tokens = %v, want 5234", e.Data["input_tokens"])
	}
	if v, ok := e.Data["output_tokens"].(int64); !ok || v != 1201 {
		t.Errorf("output_tokens = %v, want 1201", e.Data["output_tokens"])
	}
	if v, ok := e.Data["model"].(string); !ok || v != "claude-sonnet-4-5-20250929" {
		t.Errorf("model = %v, want claude-sonnet-4-5-20250929", e.Data["model"])
	}
}

func TestExtractLogEventsEmpty(t *testing.T) {
	events := extractLogEvents([]byte(`{"resourceLogs":[]}`))
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestExtractLogEventsInvalidJSON(t *testing.T) {
	events := extractLogEvents([]byte(`not json`))
	if events != nil {
		t.Errorf("expected nil, got %v", events)
	}
}

func TestExtractMetricEvents(t *testing.T) {
	body := []byte(`{
		"resourceMetrics": [{
			"scopeMetrics": [{
				"metrics": [{
					"name": "claude_code.token.usage",
					"sum": {
						"dataPoints": [{
							"timeUnixNano": "1707840000000000000",
							"asInt": 42000,
							"attributes": [
								{"key": "type", "value": {"stringValue": "input"}},
								{"key": "model", "value": {"stringValue": "claude-opus-4-6"}}
							]
						}]
					}
				}]
			}]
		}]
	}`)

	events := extractMetricEvents(body)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Event != "claude_code.token.usage" {
		t.Errorf("event = %q, want %q", e.Event, "claude_code.token.usage")
	}
	if v, ok := e.Data["value"].(float64); !ok || v != 42000 {
		t.Errorf("value = %v, want 42000", e.Data["value"])
	}
	if v, ok := e.Data["type"].(string); !ok || v != "input" {
		t.Errorf("type = %v, want input", e.Data["type"])
	}
}

func TestFlattenAttributes(t *testing.T) {
	attrs := []otlpKeyValue{
		{Key: "str", Value: otlpValue{StringValue: "hello"}},
		{Key: "num", Value: otlpValue{IntValue: json.RawMessage(`"42"`)}},
		{Key: "dbl", Value: otlpValue{DoubleValue: ptr(3.14)}},
	}

	m := flattenAttributes(attrs)

	if m["str"] != "hello" {
		t.Errorf("str = %v, want hello", m["str"])
	}
	if m["num"] != int64(42) {
		t.Errorf("num = %v, want 42", m["num"])
	}
	if m["dbl"] != 3.14 {
		t.Errorf("dbl = %v, want 3.14", m["dbl"])
	}
}

func TestFlattenAttributesIntValueVariants(t *testing.T) {
	attrs := []otlpKeyValue{
		{Key: "bare", Value: otlpValue{IntValue: json.RawMessage(`123`)}},
		{Key: "neg", Value: otlpValue{IntValue: json.RawMessage(`"-7"`)}},
		{Key: "bad", Value: otlpValue{IntValue: json.RawMessage(`"abc"`)}},
		{Key: "empty", Value: otlpValue{IntValue: json.RawMessage(`""`)}},
		{Key: "big", Value: otlpValue{IntValue: json.RawMessage(`"9223372036854775807"`)}},
	}

	m := flattenAttributes(attrs)

	if m["bare"] != int64(123) {
		t.Errorf("bare = %v (type %T), want int64(123)", m["bare"], m["bare"])
	}
	if m["neg"] != int64(-7) {
		t.Errorf("neg = %v, want -7", m["neg"])
	}
	// Legacy zero-on-parse-error semantics: malformed input yields int64(0)
	// with the key still present, matching the previous fmt.Sscanf behavior.
	if v, ok := m["bad"]; !ok || v != int64(0) {
		t.Errorf("bad = %v (ok=%v), want int64(0) present", v, ok)
	}
	if v, ok := m["empty"]; !ok || v != int64(0) {
		t.Errorf("empty = %v (ok=%v), want int64(0) present", v, ok)
	}
	if m["big"] != int64(9223372036854775807) {
		t.Errorf("big = %v, want 9223372036854775807", m["big"])
	}
}

func TestParseOTLPTimestamp(t *testing.T) {
	const wantFixed = "2024-02-13T16:00:00Z"

	// Quoted nanos (OTLP/JSON wire format).
	if got := parseOTLPTimestamp(`"1707840000000000000"`); got != wantFixed {
		t.Errorf("quoted: got %q, want %q", got, wantFixed)
	}
	// Bare nanos (already-unquoted upstream).
	if got := parseOTLPTimestamp(`1707840000000000000`); got != wantFixed {
		t.Errorf("bare: got %q, want %q", got, wantFixed)
	}
	// Empty input: zero triggers fallback to time.Now(); must still parse as RFC3339Nano.
	if got := parseOTLPTimestamp(""); got == "" {
		t.Errorf("empty: got empty string, want RFC3339Nano fallback")
	} else if _, err := time.Parse(time.RFC3339Nano, got); err != nil {
		t.Errorf("empty: got %q, not parseable as RFC3339Nano: %v", got, err)
	}
	// Garbage input: ParseInt returns (0, err); zero triggers fallback to time.Now()
	// (legacy fmt.Sscanf behavior preserved).
	if got := parseOTLPTimestamp(`"not-a-number"`); got == "" {
		t.Errorf("garbage: got empty string, want RFC3339Nano fallback")
	} else if _, err := time.Parse(time.RFC3339Nano, got); err != nil {
		t.Errorf("garbage: got %q, not parseable as RFC3339Nano: %v", got, err)
	}
}

func TestExtractLogEventsWithResourceAttrs(t *testing.T) {
	body := []byte(`{
		"resourceLogs": [{
			"resource": {
				"attributes": [
					{"key": "service.name", "value": {"stringValue": "my-agent"}},
					{"key": "service.instance.id", "value": {"stringValue": "abc123"}}
				]
			},
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1707840000000000000",
					"body": {"stringValue": "claude_code.api_request"},
					"attributes": [
						{"key": "model", "value": {"stringValue": "claude-sonnet-4-5-20250929"}}
					]
				}]
			}]
		}]
	}`)

	events := extractLogEvents(body)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Resource == nil {
		t.Fatal("expected Resource to be populated")
	}
	if v, ok := e.Resource["service.name"].(string); !ok || v != "my-agent" {
		t.Errorf("service.name = %v, want my-agent", e.Resource["service.name"])
	}
	if v, ok := e.Resource["service.instance.id"].(string); !ok || v != "abc123" {
		t.Errorf("service.instance.id = %v, want abc123", e.Resource["service.instance.id"])
	}
}

func TestExtractLogEventsNoResource(t *testing.T) {
	body := []byte(`{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1707840000000000000",
					"body": {"stringValue": "claude_code.api_request"},
					"attributes": [
						{"key": "model", "value": {"stringValue": "claude-sonnet-4-5-20250929"}}
					]
				}]
			}]
		}]
	}`)

	events := extractLogEvents(body)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Resource != nil {
		t.Errorf("expected nil Resource when no resource block, got %v", events[0].Resource)
	}
}

func TestExtractMetricEventsWithResourceAttrs(t *testing.T) {
	body := []byte(`{
		"resourceMetrics": [{
			"resource": {
				"attributes": [
					{"key": "agent.name", "value": {"stringValue": "sub-agent-1"}}
				]
			},
			"scopeMetrics": [{
				"metrics": [{
					"name": "claude_code.token.usage",
					"sum": {
						"dataPoints": [{
							"timeUnixNano": "1707840000000000000",
							"asInt": 1000,
							"attributes": [
								{"key": "model", "value": {"stringValue": "claude-opus-4-6"}}
							]
						}]
					}
				}]
			}]
		}]
	}`)

	events := extractMetricEvents(body)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Resource == nil {
		t.Fatal("expected Resource to be populated")
	}
	if v := events[0].Resource["agent.name"]; v != "sub-agent-1" {
		t.Errorf("agent.name = %v, want sub-agent-1", v)
	}
}

func ptr(f float64) *float64 { return &f }

// freePort grabs a free TCP port on 127.0.0.1.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return port
}

// TestCollectorRunEndToEnd exercises the full bench writer wiring: it starts
// Run, posts a synthetic OTLP log payload to /v1/logs, cancels Run, and asserts
// the output file contains a well-formed NDJSON line. This covers writeEvents,
// the atomic counter, and the Run open/close lifecycle that collector_test.go
// historically did not exercise.
func TestCollectorRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "events.ndjson")
	port := freePort(t)
	c := NewCollector(port, outPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- c.Run(ctx)
	}()

	// Wait for the server to be reachable (Run prints "Collecting on...").
	endpoint := "http://127.0.0.1:" + itoa(port)
	if err := waitForServer(endpoint+"/v1/logs", 2*time.Second); err != nil {
		cancel()
		t.Fatalf("server never came up: %v", err)
	}

	body := []byte(`{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1707840000000000000",
					"body": {"stringValue": "claude_code.api_request"},
					"attributes": [
						{"key": "model", "value": {"stringValue": "test-model"}},
						{"key": "input_tokens", "value": {"intValue": "100"}}
					]
				}]
			}]
		}]
	}`)

	resp, err := http.Post(endpoint+"/v1/logs", "application/json", bytes.NewReader(body))
	if err != nil {
		cancel()
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Cancel and wait for Run to return (which closes the writer + flushes).
	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}

	// Verify counter was incremented exactly once.
	if got := c.count.Load(); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}

	// Read output and verify one well-formed NDJSON line.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		t.Fatal("output file is empty")
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1: %q", len(lines), trimmed)
	}

	var ev CollectedEvent
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("output line not valid JSON: %v", err)
	}
	if ev.Event != "claude_code.api_request" {
		t.Errorf("Event = %q, want claude_code.api_request", ev.Event)
	}
	if v, ok := ev.Data["model"].(string); !ok || v != "test-model" {
		t.Errorf("Data[model] = %v, want test-model", ev.Data["model"])
	}
}

// TestCollectorRunAppendMode verifies that NewCollectorAppend preserves
// pre-existing content rather than truncating it.
func TestCollectorRunAppendMode(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "events.ndjson")

	// Seed with an existing line.
	preexisting := `{"ts":"2026-01-01T00:00:00Z","event":"pre.existing"}` + "\n"
	if err := os.WriteFile(outPath, []byte(preexisting), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	c := NewCollectorAppend(port, outPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- c.Run(ctx)
	}()

	endpoint := "http://127.0.0.1:" + itoa(port)
	if err := waitForServer(endpoint+"/v1/logs", 2*time.Second); err != nil {
		cancel()
		t.Fatalf("server never came up: %v", err)
	}

	body := []byte(`{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1707840000000000000",
					"body": {"stringValue": "claude_code.api_request"},
					"attributes": []
				}]
			}]
		}]
	}`)
	resp, err := http.Post(endpoint+"/v1/logs", "application/json", bytes.NewReader(body))
	if err != nil {
		cancel()
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "pre.existing") {
		t.Error("append mode dropped pre-existing content")
	}
	if !strings.Contains(string(data), "claude_code.api_request") {
		t.Error("append mode did not write new event")
	}
}

// itoa is a tiny helper to avoid pulling in strconv just for this.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// waitForServer polls the endpoint until it accepts a connection or deadline hits.
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// A POST with empty body returns 400, but the server is up if it answers.
		resp, err := http.Post(url, "application/json", bytes.NewReader([]byte("{}")))
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return context.DeadlineExceeded
}
