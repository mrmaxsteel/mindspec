package harness

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEventParseRoundTrip(t *testing.T) {
	events := []ActionEvent{
		{
			Timestamp:  time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
			Turn:       0,
			Phase:      "implement",
			ActionType: "command",
			Command:    "mindspec",
			Args:       map[string]string{"0": "next", "1": "--spec=046"},
			CWD:        "/repo/.worktrees/wt-046",
			ExitCode:   0,
			DurationMS: 150,
		},
		{
			Timestamp:   time.Date(2026, 2, 28, 10, 0, 1, 0, time.UTC),
			Turn:        1,
			Phase:       "spec",
			ActionType:  "hook_block",
			ToolName:    "Write",
			Command:     "",
			Args:        map[string]string{"file_path": "internal/foo.go"},
			Blocked:     true,
			BlockReason: "code edits not allowed in spec mode",
		},
	}

	path := filepath.Join(t.TempDir(), "events.jsonl")

	// Write
	if err := WriteEvents(path, events); err != nil {
		t.Fatalf("WriteEvents: %v", err)
	}

	// Read back
	got, err := ParseEvents(path)
	if err != nil {
		t.Fatalf("ParseEvents: %v", err)
	}

	if len(got) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(got))
	}

	// Check first event
	if got[0].Command != "mindspec" {
		t.Errorf("event[0].Command = %q, want %q", got[0].Command, "mindspec")
	}
	if got[0].Phase != "implement" {
		t.Errorf("event[0].Phase = %q, want %q", got[0].Phase, "implement")
	}
	if got[0].DurationMS != 150 {
		t.Errorf("event[0].DurationMS = %d, want 150", got[0].DurationMS)
	}
	if got[0].Duration() != 150*time.Millisecond {
		t.Errorf("event[0].Duration() = %v, want 150ms", got[0].Duration())
	}

	// Check second event (blocked)
	if !got[1].Blocked {
		t.Error("event[1].Blocked = false, want true")
	}
	if got[1].BlockReason != "code edits not allowed in spec mode" {
		t.Errorf("event[1].BlockReason = %q", got[1].BlockReason)
	}
}

func TestEventParseEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := WriteEvents(path, nil); err != nil {
		t.Fatalf("WriteEvents: %v", err)
	}
	got, err := ParseEvents(path)
	if err != nil {
		t.Fatalf("ParseEvents: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 events, got %d", len(got))
	}
}

func TestEventParseNonexistent(t *testing.T) {
	_, err := ParseEvents(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestEventLogQueries(t *testing.T) {
	events := []ActionEvent{
		{Turn: 0, ActionType: "command", Command: "git", ExitCode: 0},
		{Turn: 0, ActionType: "tool_invoke", ToolName: "Read"},
		{Turn: 1, ActionType: "command", Command: "mindspec", ExitCode: 0},
		{Turn: 1, ActionType: "hook_block", Blocked: true, BlockReason: "wrong cwd"},
		{Turn: 2, ActionType: "command", Command: "git", ExitCode: 0},
	}

	log := NewEventLog(events)

	cmds := log.Commands()
	if len(cmds) != 3 {
		t.Errorf("Commands() returned %d, want 3", len(cmds))
	}

	blocked := log.Blocked()
	if len(blocked) != 1 {
		t.Errorf("Blocked() returned %d, want 1", len(blocked))
	}

	byTurn := log.ByTurn()
	if len(byTurn[0]) != 2 {
		t.Errorf("ByTurn()[0] has %d events, want 2", len(byTurn[0]))
	}
	if len(byTurn[1]) != 2 {
		t.Errorf("ByTurn()[1] has %d events, want 2", len(byTurn[1]))
	}

	if log.MaxTurn() != 2 {
		t.Errorf("MaxTurn() = %d, want 2", log.MaxTurn())
	}
}
