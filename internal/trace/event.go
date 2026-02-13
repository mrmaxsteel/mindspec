package trace

import (
	"encoding/json"
	"time"
)

// Event represents a single trace event emitted as one NDJSON line.
type Event struct {
	TS     string         `json:"ts"`
	Event  string         `json:"event"`
	RunID  string         `json:"run_id"`
	SpecID string         `json:"spec_id,omitempty"`
	DurMs  float64        `json:"dur_ms,omitempty"`
	Tokens int            `json:"tokens,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

// NewEvent creates an event with the current timestamp and the global run ID.
func NewEvent(name string) Event {
	return Event{
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Event: name,
		RunID: globalRunID,
	}
}

// WithSpec sets the spec ID on the event.
func (e Event) WithSpec(specID string) Event {
	e.SpecID = specID
	return e
}

// WithDuration sets the duration in milliseconds.
func (e Event) WithDuration(d time.Duration) Event {
	e.DurMs = float64(d.Nanoseconds()) / 1e6
	return e
}

// WithTokens sets the token estimate.
func (e Event) WithTokens(tokens int) Event {
	e.Tokens = tokens
	return e
}

// WithData sets the data payload.
func (e Event) WithData(data map[string]any) Event {
	e.Data = data
	return e
}

// MarshalJSON produces compact JSON for NDJSON output.
func (e Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	return json.Marshal(Alias(e))
}
