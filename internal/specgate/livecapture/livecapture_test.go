//go:build livecapture

package livecapture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/agentmind/client"
)

// TestLiveCapture is the spec 083 Test D gate. The test orchestrates
// the full live-capture flow:
//
//  1. Resolve the agentmind binary via $AGENTMIND_BIN (fail if unset
//     — the Makefile target is responsible for populating it).
//  2. Spawn it under `client.AutoStart` with a free OTLP port and a
//     deterministic --output file path.
//  3. Wait for the OTLP port to become reachable.
//  4. POST a synthetic OTLP log payload to /v1/logs. (Fake binaries
//     accept-and-close the connection; real binaries parse it and
//     emit a corresponding wire.CollectedEvent.)
//  5. Stop the subprocess and inspect the --output NDJSON file.
//  6. Assert it is non-empty, every line parses as a
//     wire.CollectedEvent, AND — when $AGENTMIND_REAL_BINARY=1 —
//     contains an event whose `name` field is `claude_code.api_request`
//     (the spec-canonical real-binary assertion).
func TestLiveCapture(t *testing.T) {
	binPath := os.Getenv("AGENTMIND_BIN")
	if binPath == "" {
		t.Fatal("AGENTMIND_BIN is not set — the Makefile target test-live-capture must populate it")
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("AGENTMIND_BIN=%q is not stat-able: %v", binPath, err)
	}

	realBinary := os.Getenv("AGENTMIND_REAL_BINARY") == "1"

	otlpPort := freePort(t)
	uiPort := freePort(t)
	outputPath := filepath.Join(t.TempDir(), "em.ndjson")

	t.Setenv("AGENTMIND_BIN", binPath)
	t.Setenv("PATH", t.TempDir())

	handle, err := client.AutoStart(t.TempDir(), otlpPort, uiPort, outputPath)
	if err != nil {
		t.Fatalf("client.AutoStart: %v", err)
	}
	if handle == nil {
		t.Fatal("AutoStart returned nil handle")
	}
	t.Cleanup(func() {
		if handle.PID > 0 {
			if p, perr := os.FindProcess(handle.PID); perr == nil {
				_ = p.Kill()
			}
		}
	})

	// Wait for the OTLP listener — AutoStart already does this but
	// we re-confirm here so a step-3 failure is locatable.
	if err := client.WaitForPort(otlpPort, 3*time.Second); err != nil {
		t.Fatalf("OTLP port %d never opened: %v", otlpPort, err)
	}

	// POST a synthetic OTLP log payload. Real binaries parse this and
	// surface a wire.CollectedEvent; fake binaries accept the
	// connection but ignore the body.
	postSyntheticOTLP(t, otlpPort)

	// Give the binary a moment to emit (the fake emits its synthetic
	// burst within microseconds; a real OTLP-parsing implementation
	// needs time to ingest the POST). Drain via a deadline rather
	// than a fixed sleep.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if info, statErr := os.Stat(outputPath); statErr == nil && info.Size() > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Stop the subprocess so its --output is fully flushed.
	if handle.PID > 0 {
		if p, perr := os.FindProcess(handle.PID); perr == nil {
			_ = p.Kill()
		}
	}

	// Drain handle.Stdout to a sink so the subprocess doesn't block
	// on stdout backpressure during shutdown. ReadEvents covers that
	// path under the bench consumer; for the live-capture test we
	// care only about the --output file.
	if handle.Stdout != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				if _, rerr := handle.Stdout.Read(buf); rerr != nil {
					return
				}
			}
		}()
	}

	lines := readLines(t, outputPath)
	if len(lines) == 0 {
		t.Fatalf("expected at least one event in %s; got empty file", outputPath)
	}

	// Every line MUST decode as a wire.CollectedEvent (or at least
	// JSON object — the test does not yet import wire here because
	// the agentmind sibling's wire pkg is the canonical shape).
	type ev struct {
		TS    string         `json:"ts"`
		Event string         `json:"event"`
		Name  string         `json:"name"`
		Data  map[string]any `json:"data"`
	}
	var parsed []ev
	for i, line := range lines {
		var e ev
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("line %d not JSON: %v\nline=%q", i, err, line)
		}
		parsed = append(parsed, e)
	}

	if realBinary {
		// Spec Test D real-binary assertion: at least one event has
		// name == "claude_code.api_request".
		found := false
		for _, e := range parsed {
			// Accept either top-level `name` (real binary) or
			// `event` field (synthetic) — but the strict spec form
			// is `name`.
			if e.Name == "claude_code.api_request" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("AGENTMIND_REAL_BINARY=1 set but no event with name=\"claude_code.api_request\" found in %s\nlines=%v",
				outputPath, lines)
		}
	} else {
		// Fake-binary strict assertion (spec 083 Bead 4 panel
		// revision): the default path MUST verify that the in-repo
		// cmd/agentmind-fake produced its expected `fake.event.N`
		// records. A binary that emits an empty `{}` JSON object
		// would previously have passed this gate silently; the
		// hard-fail below closes that hole. Real-binary mode
		// (AGENTMIND_REAL_BINARY=1) keeps the
		// `claude_code.api_request` check above.
		anyFake := false
		for _, e := range parsed {
			if strings.HasPrefix(e.Event, "fake.event.") {
				anyFake = true
				break
			}
		}
		if !anyFake {
			t.Fatalf("no fake.event.N records found in %s — binary at %s did not emit cmd/agentmind-fake's expected event shape (got %d records). If you intentionally pointed AGENTMIND_BIN at a different binary, set AGENTMIND_REAL_BINARY=1 to switch to the real-binary assertion.",
				outputPath, binPath, len(parsed))
		}
	}

	t.Logf("live-capture OK: %d events captured in %s (real_binary=%v)",
		len(parsed), outputPath, realBinary)
}

// postSyntheticOTLP sends a minimal OTLP/HTTP log payload to
// http://127.0.0.1:<port>/v1/logs. The payload mirrors the shape
// the real agentmind binary expects (resourceLogs → scopeLogs →
// logRecords with a log body carrying claude_code.api_request).
//
// Errors are non-fatal (the fake doesn't run an HTTP server on the
// bound OTLP port; the test still proceeds to assert NDJSON output).
func postSyntheticOTLP(t *testing.T, port int) {
	t.Helper()

	body := []byte(`{
	  "resourceLogs": [{
	    "resource": {"attributes": [
	      {"key": "service.name", "value": {"stringValue": "claude-code"}}
	    ]},
	    "scopeLogs": [{
	      "scope": {"name": "claude-code"},
	      "logRecords": [{
	        "timeUnixNano": "1715000000000000000",
	        "severityNumber": 9,
	        "severityText": "INFO",
	        "body": {"stringValue": "claude_code.api_request"},
	        "attributes": [
	          {"key": "name", "value": {"stringValue": "claude_code.api_request"}}
	        ]
	      }]
	    }]
	  }]
	}`)

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/v1/logs", port),
		bytes.NewReader(body))
	if err != nil {
		t.Logf("warn: build OTLP request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Expected when running against cmd/agentmind-fake (no HTTP
		// server, just an accept-and-close TCP listener). Logged so
		// CI logs make the cause visible.
		t.Logf("info: POST /v1/logs failed (expected with fake binary): %v", err)
		return
	}
	defer resp.Body.Close()
	t.Logf("info: POST /v1/logs -> %d", resp.StatusCode)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			out = append(out, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return out
}

// keep exec import resolvable when the test file is built with the
// livecapture tag but no test actually references exec yet.
var _ = exec.Command
