package instruct

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupRunTestProject extends setupTestProject with a .git marker so
// workspace.FindLocalRoot/FindRoot can locate the workspace.
func setupRunTestProject(t *testing.T) string {
	t.Helper()
	root := setupTestProject(t)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	return root
}

// TestRun_IdleNoBeads verifies that a workspace with no .beads/ falls back to
// the idle template via the handleNoState path.
func TestRun_IdleNoBeads(t *testing.T) {
	root := setupRunTestProject(t)

	var buf bytes.Buffer
	if err := Run(context.Background(), root, "", "", &buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No Active Work") {
		t.Errorf("expected idle template heading 'No Active Work', got:\n%s", out)
	}
}

// TestRun_JSONFormat verifies the json format path emits parseable JSON to the
// writer.
func TestRun_JSONFormat(t *testing.T) {
	root := setupRunTestProject(t)

	var buf bytes.Buffer
	if err := Run(context.Background(), root, "json", "", &buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	var parsed JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Run JSON output failed to parse: %v\noutput:\n%s", err, buf.String())
	}
	if parsed.Mode == "" {
		t.Errorf("expected non-empty mode in JSON output, got %+v", parsed)
	}
}

// TestRun_WriterIsHonored confirms that Run writes only to the provided writer
// (not to os.Stdout).
func TestRun_WriterIsHonored(t *testing.T) {
	root := setupRunTestProject(t)

	// Capture os.Stdout via a pipe.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	var captured bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&captured, r)
	}()

	var buf bytes.Buffer
	if err := Run(context.Background(), root, "", "", &buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Close the writer so the copier exits.
	_ = w.Close()
	wg.Wait()

	if buf.Len() == 0 {
		t.Errorf("expected output in writer buf, got empty")
	}
	if captured.Len() != 0 {
		t.Errorf("expected os.Stdout untouched, got %d bytes: %q", captured.Len(), captured.String())
	}
}

// TestRun_HonorsContext verifies that an already-canceled context causes Run
// to return ctx.Err() at the first step boundary without writing output.
func TestRun_HonorsContext(t *testing.T) {
	root := setupRunTestProject(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	var buf bytes.Buffer
	err := Run(ctx, root, "", "", &buf)
	if err == nil {
		t.Fatalf("expected error from canceled ctx, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output when ctx is canceled, got %q", buf.String())
	}
}

// TestRun_HonorsContextDeadline confirms that a deadline-expired context is
// reported as context.DeadlineExceeded.
func TestRun_HonorsContextDeadline(t *testing.T) {
	root := setupRunTestProject(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	var buf bytes.Buffer
	err := Run(ctx, root, "", "", &buf)
	if err == nil {
		t.Fatalf("expected error from expired ctx, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}
