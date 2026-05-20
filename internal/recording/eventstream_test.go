package recording

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/agentmind/client"
)

// TestStartRecordingEventConsumer_ReadsFromSubprocessStdoutPipe is
// the recording-side mirror of the bench-side Bead 3b test: the
// io.Reader passed to client.ReadEvents from the recording collector
// MUST be a subprocess stdout pipe obtained from
// exec.Cmd.StdoutPipe() (Hard Constraint #3 — outbound channel is
// stdout-pipe NDJSON, NOT file-tail).
//
// The test builds cmd/agentmind-fake, invokes client.AutoStart with
// AGENTMIND_BIN pointing at the fake, asserts handle.Stdout is a
// pipe (not a regular file), runs the recording-side consumer
// (startRecordingEventConsumer), and verifies the on-disk NDJSON
// file contains the events the fake emitted.
func TestStartRecordingEventConsumer_ReadsFromSubprocessStdoutPipe(t *testing.T) {
	binPath := buildAgentmindFake(t)
	t.Setenv("AGENTMIND_BIN", binPath)
	t.Setenv("PATH", t.TempDir())

	otlpPort := freePortRec(t)
	uiPort := freePortRec(t)
	outputPath := filepath.Join(t.TempDir(), "events.ndjson")

	handle, err := client.AutoStart(t.TempDir(), otlpPort, uiPort, outputPath)
	if err != nil {
		t.Fatalf("client.AutoStart: %v", err)
	}
	t.Cleanup(func() {
		if handle != nil && handle.PID > 0 {
			if p, perr := os.FindProcess(handle.PID); perr == nil {
				_ = p.Kill()
			}
		}
	})

	// Hard Constraint #3 assertion: handle.Stdout must be a pipe.
	if handle.Stdout == nil {
		t.Fatal("handle.Stdout is nil")
	}
	asFile, ok := handle.Stdout.(*os.File)
	if !ok {
		t.Fatalf("handle.Stdout is %T; want *os.File (subprocess stdout pipe)", handle.Stdout)
	}
	st, statErr := asFile.Stat()
	if statErr != nil {
		t.Fatalf("stat handle.Stdout: %v", statErr)
	}
	if st.Mode().IsRegular() {
		t.Fatalf("handle.Stdout is a regular file (%s); Hard Constraint #3 violation", st.Mode())
	}

	// Drive the recording-side consumer.
	if err := startRecordingEventConsumer(handle, outputPath); err != nil {
		t.Fatalf("startRecordingEventConsumer: %v", err)
	}

	// Give the consumer goroutine a moment to drain the 3 events the
	// fake emitted; then kill the subprocess so the consumer's
	// writer flushes and closes when the pipe closes.
	time.Sleep(300 * time.Millisecond)
	if handle.PID > 0 {
		if p, perr := os.FindProcess(handle.PID); perr == nil {
			_ = p.Kill()
		}
	}

	// Poll for the file to materialize. The 500ms flushInterval may
	// elapse before the writer closes.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, sErr := os.Stat(outputPath); sErr == nil && info.Size() > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	lines := readNDJSONLines(t, outputPath)
	if len(lines) == 0 {
		t.Fatalf("expected NDJSON lines in %s; got 0 after subprocess kill", outputPath)
	}
	// Sanity-check the first line is a wire.CollectedEvent JSON.
	var ev struct {
		Event string `json:"event"`
	}
	if uerr := json.Unmarshal([]byte(lines[0]), &ev); uerr != nil {
		t.Fatalf("first line not JSON: %v (%q)", uerr, lines[0])
	}
	if !strings.HasPrefix(ev.Event, "fake.event.") {
		t.Fatalf("first event name %q; want prefix fake.event.", ev.Event)
	}
}

// TestStartRecordingEventConsumer_NilHandle is the no-op assertion:
// nil handle / nil Stdout MUST NOT touch disk and MUST return nil
// error.
func TestStartRecordingEventConsumer_NilHandle(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "events.ndjson")
	if err := startRecordingEventConsumer(nil, outputPath); err != nil {
		t.Fatalf("nil handle: unexpected err: %v", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected output file absent on nil handle; stat err: %v", statErr)
	}

	// Reused-instance case: handle != nil, Stdout == nil.
	if err := startRecordingEventConsumer(&client.Handle{Reused: true}, outputPath); err != nil {
		t.Fatalf("reused handle: unexpected err: %v", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected output file absent on reused handle; stat err: %v", statErr)
	}
}

// TestStartRecordingEventConsumer_RestartInSameProcess asserts that
// two back-to-back calls to startRecordingEventConsumer in the same
// process both spawn a draining goroutine and both produce on-disk
// NDJSON output. This is the regression test for the package-level
// sync.Once that REV-3 removed: previously the second call silently
// no-op'd, dropping the second spec's events on the floor.
//
// The test uses io.Pipe handles directly (no subprocess) — the
// goroutine drain path is what we want to exercise, and synthetic
// readers prove the per-call goroutine lifecycle without paying the
// subprocess-build cost twice.
func TestStartRecordingEventConsumer_RestartInSameProcess(t *testing.T) {
	const ndjsonA = `{"ts":"2026-01-01T00:00:00Z","event":"a0","data":{"i":0}}
{"ts":"2026-01-01T00:00:01Z","event":"a1","data":{"i":1}}
`
	const ndjsonB = `{"ts":"2026-01-02T00:00:00Z","event":"b0","data":{"i":0}}
{"ts":"2026-01-02T00:00:01Z","event":"b1","data":{"i":1}}
{"ts":"2026-01-02T00:00:02Z","event":"b2","data":{"i":2}}
`

	driveConsumer := func(name, body string) []string {
		t.Helper()
		// Build a pipe-backed *os.File pair so handle.Stdout has the
		// same shape as a real subprocess pipe (non-regular file).
		// io.Pipe would not satisfy the *os.File type assertion in
		// the live-binary test, but startRecordingEventConsumer only
		// requires handle.Stdout to be non-nil io.Reader. We use
		// os.Pipe() to mirror the subprocess case more faithfully.
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		go func() {
			defer w.Close()
			_, _ = w.WriteString(body)
		}()

		outputPath := filepath.Join(t.TempDir(), name+".ndjson")
		handle := &client.Handle{Stdout: r}
		if err := startRecordingEventConsumer(handle, outputPath); err != nil {
			t.Fatalf("%s: startRecordingEventConsumer: %v", name, err)
		}

		// Wait for the on-disk file to be flushed. The writer's
		// flushInterval is 500ms; on Close (when the goroutine sees
		// the input channel closed) it flushes synchronously. We
		// poll up to 5s.
		deadline := time.Now().Add(5 * time.Second)
		var lines []string
		for time.Now().Before(deadline) {
			if info, sErr := os.Stat(outputPath); sErr == nil && info.Size() > 0 {
				lines = readNDJSONLines(t, outputPath)
				// Wait until we have all the expected lines (the
				// writer flushes on close so this is deterministic
				// once the pipe drains).
				if len(lines) >= strings.Count(body, "\n") {
					return lines
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s: timed out waiting for output %s; got %d lines", name, outputPath, len(lines))
		return nil
	}

	// First invocation: previously the package-level sync.Once would
	// fire here and any second call would be silently skipped.
	gotA := driveConsumer("a", ndjsonA)
	if len(gotA) != 2 {
		t.Fatalf("first call: expected 2 lines; got %d", len(gotA))
	}

	// Second invocation in the same test process. Pre-REV-3 this
	// would have failed (no goroutine launched, output file empty).
	gotB := driveConsumer("b", ndjsonB)
	if len(gotB) != 3 {
		t.Fatalf("second call (post-REV-3 restart): expected 3 lines; got %d (would be 0 with package-level sync.Once)", len(gotB))
	}

	// Cross-check: the two outputs are independent and the second's
	// events did not leak into the first's file.
	for _, line := range gotA {
		if strings.Contains(line, "b0") || strings.Contains(line, "b1") || strings.Contains(line, "b2") {
			t.Fatalf("first call file contains second call's event: %q", line)
		}
	}
}

// === helpers =============================================================

func buildAgentmindFake(t *testing.T) string {
	t.Helper()
	mod := goModRoot(t)
	binPath := filepath.Join(t.TempDir(), "agentmind")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/agentmind-fake")
	cmd.Dir = mod
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build cmd/agentmind-fake: %v\n%s", err, out)
	}
	return binPath
}

func goModRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == "/dev/null" {
		t.Fatalf("no go.mod resolved; got %q", gomod)
	}
	return filepath.Dir(gomod)
}

func freePortRec(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func readNDJSONLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
