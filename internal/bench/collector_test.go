package bench

import (
	"encoding/json"
	"testing"
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

func ptr(f float64) *float64 { return &f }
