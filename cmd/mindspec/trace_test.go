package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/trace"
)

// TestTraceSummary_LargeLine verifies that the NDJSON scanner can handle
// individual events larger than bufio.Scanner's default 64 KiB token limit.
// Regression test for SEC-6 (mindspec-36h8).
func TestTraceSummary_LargeLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.ndjson")

	// Build one event whose JSON-encoded length exceeds 64 KiB.
	big := strings.Repeat("x", 200*1024) // 200 KiB payload, well above 64 KiB
	e := trace.Event{
		TS:    "2026-05-14T00:00:00Z",
		Event: "tool.result",
		RunID: "test-run",
		Data:  map[string]any{"payload": big},
	}
	line, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(line) <= 64*1024 {
		t.Fatalf("test event is only %d bytes, need >64 KiB", len(line))
	}

	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
		t.Fatalf("write trace file: %v", err)
	}

	// Capture stdout produced by traceSummaryCmd.
	// Note: the "Events:     1" assertion below is tied to the format string
	// in trace.go ("  Events:     %d\n"). If that format ever changes, this
	// assertion will need to move in lockstep.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	runErr := traceSummaryCmd.RunE(traceSummaryCmd, []string{path})
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	if runErr != nil {
		t.Fatalf("traceSummaryCmd.RunE returned error: %v", runErr)
	}

	out := buf.String()
	if !strings.Contains(out, "Events:") {
		t.Fatalf("expected 'Events:' in summary output, got:\n%s", out)
	}
	if !strings.Contains(out, "Events:     1") {
		t.Fatalf("expected exactly 1 event counted, got:\n%s", out)
	}
	if !strings.Contains(out, "tool.result") {
		t.Fatalf("expected 'tool.result' row in summary, got:\n%s", out)
	}
}
