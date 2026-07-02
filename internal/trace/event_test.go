package trace

import (
	"encoding/json"
	"testing"
)

// TestEventNDJSONGolden pins the exact NDJSON bytes json.Marshal produces
// for a representative populated Event and a zero-value Event. The deleted
// Event.MarshalJSON (spec 108 R2) marshaled a type-aliased copy of Event,
// which is byte-identical to Go's default struct marshaling, so its removal
// must leave these bytes unchanged. The goldens capture the current output;
// any drift fails this test.
func TestEventNDJSONGolden(t *testing.T) {
	populated := Event{
		TS:     "2026-07-02T00:00:00.000000001Z",
		Event:  "test.event",
		RunID:  "run-123",
		SpecID: "108-cleanup",
		DurMs:  150,
		Tokens: 42,
		Data:   map[string]any{"key": "value"},
	}
	const wantPopulated = `{"ts":"2026-07-02T00:00:00.000000001Z","event":"test.event","run_id":"run-123","spec_id":"108-cleanup","dur_ms":150,"tokens":42,"data":{"key":"value"}}`
	const wantZero = `{"ts":"","event":"","run_id":""}`

	got, err := json.Marshal(populated)
	if err != nil {
		t.Fatalf("marshal populated: %v", err)
	}
	if string(got) != wantPopulated {
		t.Errorf("populated NDJSON drift:\n got: %s\nwant: %s", got, wantPopulated)
	}

	gotZero, err := json.Marshal(Event{})
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if string(gotZero) != wantZero {
		t.Errorf("zero-value NDJSON drift:\n got: %s\nwant: %s", gotZero, wantZero)
	}
}
