package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ActionEvent represents a single observable action during an agent session.
type ActionEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	Turn        int               `json:"turn"`
	Phase       string            `json:"phase"`
	ActionType  string            `json:"action_type"` // tool_invoke, tool_result, command, hook_block, state_change
	ToolName    string            `json:"tool_name,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        map[string]string `json:"args,omitempty"`
	CWD         string            `json:"cwd,omitempty"`
	ExitCode    int               `json:"exit_code"`
	Blocked     bool              `json:"blocked,omitempty"`
	BlockReason string            `json:"block_reason,omitempty"`
	DurationMS  int64             `json:"duration_ms,omitempty"`
}

// Duration returns the event duration as time.Duration.
func (e ActionEvent) Duration() time.Duration {
	return time.Duration(e.DurationMS) * time.Millisecond
}

// EventLog is an ordered collection of action events with query helpers.
type EventLog struct {
	Events []ActionEvent
}

// ParseEvents reads a JSONL file and returns the parsed events.
func ParseEvents(path string) ([]ActionEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening event log: %w", err)
	}
	defer f.Close()

	var events []ActionEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev ActionEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return events, fmt.Errorf("line %d: %w", lineNum, err)
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scanning event log: %w", err)
	}
	return events, nil
}

// WriteEvents writes events as JSONL to the given path.
func WriteEvents(path string, events []ActionEvent) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			return err
		}
	}
	return nil
}

// NewEventLog creates an EventLog from a slice of events.
func NewEventLog(events []ActionEvent) *EventLog {
	return &EventLog{Events: events}
}

// Filter returns events matching the predicate.
func (l *EventLog) Filter(fn func(ActionEvent) bool) []ActionEvent {
	var out []ActionEvent
	for _, ev := range l.Events {
		if fn(ev) {
			out = append(out, ev)
		}
	}
	return out
}

// Commands returns only events with ActionType "command".
func (l *EventLog) Commands() []ActionEvent {
	return l.Filter(func(ev ActionEvent) bool {
		return ev.ActionType == "command"
	})
}

// Blocked returns only events that were blocked by a hook.
func (l *EventLog) Blocked() []ActionEvent {
	return l.Filter(func(ev ActionEvent) bool {
		return ev.Blocked
	})
}

// ByTurn groups events by turn number.
func (l *EventLog) ByTurn() map[int][]ActionEvent {
	m := make(map[int][]ActionEvent)
	for _, ev := range l.Events {
		m[ev.Turn] = append(m[ev.Turn], ev)
	}
	return m
}

// MaxTurn returns the highest turn number seen.
func (l *EventLog) MaxTurn() int {
	max := 0
	for _, ev := range l.Events {
		if ev.Turn > max {
			max = ev.Turn
		}
	}
	return max
}
