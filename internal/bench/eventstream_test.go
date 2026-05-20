package bench

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

// TestStreamConsumer_ReaderIsSubprocessStdoutPipe is the bead-3b
// load-bearing test: the io.Reader fed to client.ReadEvents from the
// bench consumer MUST be a subprocess stdout pipe obtained from
// exec.Cmd.StdoutPipe(), NOT a file handle opened against the
// agentmind --output file path.
//
// Spec 083 Hard Constraint #3 (outbound channel is stdout-pipe NDJSON,
// not file-tail). The test:
//
//  1. Builds the in-repo cmd/agentmind-fake stand-in binary.
//  2. Invokes client.AutoStart with AGENTMIND_BIN pointed at the
//     fake; AutoStart attaches the subprocess's stdout pipe to
//     handle.Stdout.
//  3. Asserts handle.Stdout is a *os.File whose .Name() begins with
//     "|" (Go's exec package convention for anonymous pipes returned
//     by StdoutPipe).
//  4. Runs the bench-side StreamConsumer against handle.Stdout and
//     verifies the synthetic NDJSON the fake emitted ends up in the
//     on-disk file.
func TestStreamConsumer_ReaderIsSubprocessStdoutPipe(t *testing.T) {
	// Panel bead-3b-v1 REV-6: the previous `if testing.Short() &&
	// os.Getenv("MINDSPEC_BEAD3B_LIVE") == "" { ... }` block had a
	// comment-only body — no t.Skip, no return — so it was a no-op
	// that misled readers about gating. Removed; the synthetic-reader
	// test (TestStreamConsumer_SyntheticReader) provides short-mode
	// coverage of the decode-and-write logic, and buildAgentmindFake
	// is a fast in-module `go build`.

	binPath := buildAgentmindFake(t)
	t.Setenv("AGENTMIND_BIN", binPath)
	t.Setenv("PATH", t.TempDir())

	otlpPort := freePortBench(t)
	uiPort := freePortBench(t)
	outputPath := filepath.Join(t.TempDir(), "events.ndjson")

	handle, err := client.AutoStart(t.TempDir(), otlpPort, uiPort, outputPath)
	if err != nil {
		t.Fatalf("client.AutoStart: %v", err)
	}
	if handle == nil {
		t.Fatal("AutoStart returned nil handle")
	}
	t.Cleanup(func() {
		if handle.PID > 0 {
			if p, err := os.FindProcess(handle.PID); err == nil {
				_ = p.Kill()
			}
		}
	})

	// === Load-bearing assertion: handle.Stdout MUST be a subprocess
	// stdout pipe (i.e., the read end of exec.Cmd.StdoutPipe()), NOT
	// an *os.File obtained via os.Open against a regular file path.
	//
	// Go's exec package implements StdoutPipe() by returning an
	// *os.File whose internal type is a pipe (Stat reports a non-
	// regular mode). We assert that here.
	if handle.Stdout == nil {
		t.Fatal("handle.Stdout is nil — consumer cannot read NDJSON via client.ReadEvents")
	}
	asFile, ok := handle.Stdout.(*os.File)
	if !ok {
		t.Fatalf("handle.Stdout is %T; want *os.File (pipe). Hard Constraint #3 violation: consumer must read from a subprocess stdout pipe.", handle.Stdout)
	}
	st, statErr := asFile.Stat()
	if statErr != nil {
		t.Fatalf("stat handle.Stdout: %v", statErr)
	}
	if (st.Mode() & os.ModeNamedPipe) == 0 {
		// Subprocess StdoutPipe is a kernel-anonymous pipe, not a
		// "named pipe" — but it does have ModeIrregular set on POSIX
		// and is not a regular file. Reject regular files explicitly.
		if st.Mode().IsRegular() {
			t.Fatalf("handle.Stdout mode is %s (regular file); want a pipe (Hard Constraint #3 file-tail prohibition)", st.Mode())
		}
	}

	// Drive the bench-side consumer end-to-end. This is the path
	// runner.go takes via startBenchCollector → ConsumeHandleToFile.
	consumer, err := ConsumeHandleToFile(handle, outputPath)
	if err != nil {
		t.Fatalf("ConsumeHandleToFile: %v", err)
	}
	if consumer == nil {
		t.Fatal("ConsumeHandleToFile returned nil consumer for a Started handle")
	}
	// The fake emits its full event burst within microseconds but
	// keeps the listener up (its --linger default is 30s) so the
	// parent's WaitForPort succeeds. We give the consumer goroutine
	// a moment to drain, flush, then kill the subprocess so its
	// stdout pipe closes and the consumer's Done channel fires.
	time.Sleep(300 * time.Millisecond)
	if err := consumer.Flush(); err != nil {
		t.Fatalf("consumer Flush: %v", err)
	}
	if handle.PID > 0 {
		if p, perr := os.FindProcess(handle.PID); perr == nil {
			_ = p.Kill()
		}
	}
	select {
	case <-consumer.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not drain within 5s after subprocess kill")
	}

	// Verify the on-disk file matches what the fake emitted (3 events,
	// fake.event.0..2). The format is canonical NDJSON via the
	// bench consumer's ndjson.Writer.
	got := readNDJSONLines(t, outputPath)
	if len(got) != 3 {
		t.Fatalf("expected 3 events in %s; got %d (%v)", outputPath, len(got), got)
	}
	for i, line := range got {
		var ev struct {
			Event string `json:"event"`
		}
		if uerr := json.Unmarshal([]byte(line), &ev); uerr != nil {
			t.Fatalf("line %d not JSON: %v (%q)", i, uerr, line)
		}
		want := "fake.event." + string(rune('0'+i))
		if ev.Event != want {
			t.Fatalf("line %d event = %q; want %q", i, ev.Event, want)
		}
	}
}

// TestStreamConsumer_SyntheticReader exercises the same code path
// without spawning a subprocess. The reader here is an in-memory
// strings.NewReader rather than a subprocess pipe — useful for
// covering the consumer's NDJSON-decode-and-write logic in unit
// scope without paying the build+spawn cost.
func TestStreamConsumer_SyntheticReader(t *testing.T) {
	const ndjson = `{"ts":"2026-01-01T00:00:00Z","event":"a","data":{"i":0}}
{"ts":"2026-01-01T00:00:01Z","event":"b","data":{"i":1}}
`
	outputPath := filepath.Join(t.TempDir(), "out.ndjson")
	c := &StreamConsumer{
		Reader:     strings.NewReader(ndjson),
		OutputPath: outputPath,
	}
	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("synthetic consumer did not drain")
	}
	lines := readNDJSONLines(t, outputPath)
	if len(lines) != 2 {
		t.Fatalf("expected 2 events; got %d (%v)", len(lines), lines)
	}
}

// TestConsumeHandleToFile_NilHandleIsNoop confirms the degraded /
// reused paths (Handle==nil or Handle.Stdout==nil) return (nil, nil)
// without attempting to open the file.
func TestConsumeHandleToFile_NilHandleIsNoop(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "out.ndjson")
	c, err := ConsumeHandleToFile(nil, outputPath)
	if err != nil {
		t.Fatalf("nil handle: unexpected err: %v", err)
	}
	if c != nil {
		t.Fatalf("nil handle: expected nil consumer; got %+v", c)
	}
	// File should NOT exist — the consumer must not touch disk on
	// the no-op path.
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected output file to be absent; got stat err: %v", statErr)
	}

	// Also: handle with nil Stdout (the reused-instance case).
	c, err = ConsumeHandleToFile(&client.Handle{Reused: true}, outputPath)
	if err != nil {
		t.Fatalf("reused handle: unexpected err: %v", err)
	}
	if c != nil {
		t.Fatalf("reused handle: expected nil consumer; got %+v", c)
	}
}

// === helpers =============================================================

// buildAgentmindFake builds cmd/agentmind-fake under a temp directory
// and returns the resulting binary path. Located via go env GOMOD so
// the test works from any package within the module.
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

func freePortBench(t *testing.T) int {
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

