package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/agentmind/wire"
	"github.com/mrmaxsteel/mindspec/internal/ndjson"
)

// TestCollectorParity is the Phase-2 NDJSON parity test (spec 083 Bead 2
// step 7, acceptance criterion: "NDJSON byte-for-byte parity (Phase 2,
// alias state) — fixture produced via canonical wire/event.go encoder").
//
// Method (spec lines 247-250):
//
//  1. Source the same frozen-clock event set the fixture was generated
//     from via wire.ParityFixtureEvents() — the single source of truth
//     shared with agentmind/cmd/genparity so the slice cannot drift.
//  2. Re-emit the events via the canonical encoder in
//     github.com/mrmaxsteel/agentmind/wire (lex-sorted JSON keys,
//     strconv.FormatFloat(v, 'f', -1, 64) for floats, UTC RFC3339Nano
//     timestamps).
//  3. Diff the resulting bytes against
//     internal/bench/testdata/parity.ndjson via byte-for-byte equality.
//
// The events are declared as `[]CollectedEvent` (the bench-side alias) to
// prove that the Phase-2 alias state — `type CollectedEvent =
// wire.CollectedEvent` — round-trips identically to direct
// wire.CollectedEvent construction.
//
// NOTE: This test anchors alias transparency + encoder determinism only.
// The end-to-end OTLP-parser-to-CollectedEvent path is covered by
// TestCollectorParity_OTLPRoundTrip below.
func TestCollectorParity(t *testing.T) {
	t.Parallel()

	// Pull the event set through the bench-side alias to prove the alias
	// is type-identical to wire.CollectedEvent.
	wireEvents := wire.ParityFixtureEvents()
	events := make([]CollectedEvent, len(wireEvents))
	for i, e := range wireEvents {
		events[i] = CollectedEvent(e)
	}

	var buf bytes.Buffer
	for _, e := range events {
		b, err := wire.Marshal(e)
		if err != nil {
			t.Fatalf("wire.Marshal: %v", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	got := buf.Bytes()

	fixturePath := filepath.Join("testdata", "parity.ndjson")
	want, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("Phase-2 NDJSON parity mismatch.\nFixture (want):\n%s\nCanonical encoder (got):\n%s",
			string(want), string(got))
	}
}

// TestCollectorParity_AliasIdentity is the structural check that backs the
// byte-equality test above: it proves that `bench.CollectedEvent` is the
// same type as `wire.CollectedEvent` (alias transparency), and that the
// fixture's JSON deserializes into either form interchangeably.
//
// This is the "key parity check" — bench's CollectedEvent flows through
// the same canonical encoding as wire's CollectedEvent because they are
// the same type at the language level (Go type alias).
func TestCollectorParity_AliasIdentity(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "parity.ndjson")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// Decode each fixture line into both bench.CollectedEvent (alias)
	// and wire.CollectedEvent (target). They MUST be byte-identical
	// when re-encoded through the canonical encoder; if they were not
	// the same type, this would fail at the type-checker level.
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	for i, line := range lines {
		var asBench CollectedEvent
		var asWire wire.CollectedEvent
		if err := json.Unmarshal(line, &asBench); err != nil {
			t.Fatalf("line %d: unmarshal into bench.CollectedEvent: %v", i, err)
		}
		if err := json.Unmarshal(line, &asWire); err != nil {
			t.Fatalf("line %d: unmarshal into wire.CollectedEvent: %v", i, err)
		}

		benchBytes, err := wire.Marshal(asBench)
		if err != nil {
			t.Fatalf("line %d: wire.Marshal(bench): %v", i, err)
		}
		wireBytes, err := wire.Marshal(asWire)
		if err != nil {
			t.Fatalf("line %d: wire.Marshal(wire): %v", i, err)
		}
		if !bytes.Equal(benchBytes, wireBytes) {
			t.Errorf("line %d: alias-identity mismatch.\nbench: %s\nwire:  %s",
				i, string(benchBytes), string(wireBytes))
		}
		if !bytes.Equal(benchBytes, line) {
			t.Errorf("line %d: canonical re-encoding diverged from fixture.\nfixture: %s\nencoded: %s",
				i, string(line), string(benchBytes))
		}
	}
}

// TestCollectorParity_OTLPRoundTrip closes the panel-flagged
// tautology gap: it constructs synthetic OTLP/HTTP JSON payloads
// (one ExportLogsServiceRequest, one ExportMetricsServiceRequest),
// POSTs them through mindspec's actual Collector HTTP handler
// (collector.handleLogs / collector.handleMetrics), reads the NDJSON
// the collector wrote, decodes each line back through the bench-side
// CollectedEvent alias, re-emits via the canonical encoder, and
// asserts byte-equality against canonical encodings of the expected
// post-parser events.
//
// This proves the alias chain works end-to-end with the actual OTLP
// parser, not just with the canonical encoder. A regression in
// extractLogEvents / extractMetricEvents / handleLogs / handleMetrics
// will now fail this test.
func TestCollectorParity_OTLPRoundTrip(t *testing.T) {
	t.Parallel()

	// Frozen timestamps (RFC3339Nano UTC, exactly as the canonical
	// encoder normalizes them).
	const (
		logTS    = "2026-02-01T00:00:00.000000001Z"
		metricTS = "2026-02-01T00:00:01.123456789Z"
	)

	logTSNanos, err := time.Parse(time.RFC3339Nano, logTS)
	if err != nil {
		t.Fatalf("parse logTS: %v", err)
	}
	metricTSNanos, err := time.Parse(time.RFC3339Nano, metricTS)
	if err != nil {
		t.Fatalf("parse metricTS: %v", err)
	}

	// Build a synthetic OTLP ExportLogsServiceRequest body that
	// extractLogEvents knows how to parse.
	logBody := fmt.Sprintf(`{
		"resourceLogs": [{
			"resource": {
				"attributes": [
					{"key": "service.name", "value": {"stringValue": "claude-code"}}
				]
			},
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "%d",
					"body": {"stringValue": "claude_code.api_request"},
					"attributes": [
						{"key": "model", "value": {"stringValue": "claude-opus-4-7"}},
						{"key": "input_tokens", "value": {"intValue": "1024"}},
						{"key": "output_tokens", "value": {"intValue": "256"}}
					]
				}]
			}]
		}]
	}`, logTSNanos.UnixNano())

	// Build a synthetic OTLP ExportMetricsServiceRequest body that
	// extractMetricEvents knows how to parse (Sum metric with one data
	// point).
	metricBody := fmt.Sprintf(`{
		"resourceMetrics": [{
			"resource": {
				"attributes": [
					{"key": "service.name", "value": {"stringValue": "claude-code"}}
				]
			},
			"scopeMetrics": [{
				"metrics": [{
					"name": "claude_code.token.usage",
					"sum": {
						"dataPoints": [{
							"timeUnixNano": "%d",
							"asInt": 1024,
							"attributes": [
								{"key": "model", "value": {"stringValue": "claude-opus-4-7"}},
								{"key": "type", "value": {"stringValue": "input"}}
							]
						}]
					}
				}]
			}]
		}]
	}`, metricTSNanos.UnixNano())

	// Stand up the collector against an NDJSON writer that we control,
	// so the test reads back exactly what the HTTP handler emitted.
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "collected.ndjson")

	w, err := ndjson.New(outPath, ndjson.Opts{
		BufSize:       64 << 10,
		FlushInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("ndjson.New: %v", err)
	}
	c := &Collector{
		port:   0, // unused; httptest provides the listener.
		output: outPath,
		w:      w,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/logs", c.handleLogs)
	mux.HandleFunc("/v1/metrics", c.handleMetrics)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// POST the synthetic OTLP requests through the real HTTP handler.
	postOTLP(t, srv.URL+"/v1/logs", logBody)
	postOTLP(t, srv.URL+"/v1/metrics", metricBody)

	// Force a flush + close so the bufio buffer is on disk before we
	// read it back.
	if err := w.Close(); err != nil {
		t.Fatalf("ndjson.Close: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read NDJSON output: %v", err)
	}

	// Decode each NDJSON line into the bench-side alias, re-emit via
	// the canonical encoder, and compare against canonical encodings
	// of the expected post-parser events. This is the load-bearing
	// assertion: the alias must survive the full HTTP -> parser ->
	// NDJSON -> decode -> canonical-encode roundtrip.
	lines := bytes.Split(bytes.TrimRight(got, "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines (1 log, 1 metric), got %d:\n%s",
			len(lines), string(got))
	}

	// Build the canonical "want" encodings for the two events we
	// expect the OTLP parser to produce.
	wantLog := CollectedEvent{
		TS:    logTS,
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model":         "claude-opus-4-7",
			"input_tokens":  int64(1024),
			"output_tokens": int64(256),
		},
		Resource: map[string]any{
			"service.name": "claude-code",
		},
	}
	wantMetric := CollectedEvent{
		TS:    metricTS,
		Event: "claude_code.token.usage",
		Data: map[string]any{
			"metric": "claude_code.token.usage",
			"value":  float64(1024),
			"model":  "claude-opus-4-7",
			"type":   "input",
		},
		Resource: map[string]any{
			"service.name": "claude-code",
		},
	}

	wantLogBytes, err := wire.Marshal(wantLog)
	if err != nil {
		t.Fatalf("marshal wantLog: %v", err)
	}
	wantMetricBytes, err := wire.Marshal(wantMetric)
	if err != nil {
		t.Fatalf("marshal wantMetric: %v", err)
	}

	// For each line, decode -> re-encode canonically -> compare
	// against the canonical "want" for the matching event.
	gotByEvent := make(map[string][]byte)
	for i, line := range lines {
		var ev CollectedEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("line %d: unmarshal: %v\nline: %s", i, err, string(line))
		}
		canon, err := wire.Marshal(ev)
		if err != nil {
			t.Fatalf("line %d: canonical re-encode: %v", i, err)
		}
		gotByEvent[ev.Event] = canon
	}

	if !bytes.Equal(gotByEvent["claude_code.api_request"], wantLogBytes) {
		t.Errorf("log event OTLP roundtrip mismatch.\nwant: %s\n got: %s",
			string(wantLogBytes),
			string(gotByEvent["claude_code.api_request"]))
	}
	if !bytes.Equal(gotByEvent["claude_code.token.usage"], wantMetricBytes) {
		t.Errorf("metric event OTLP roundtrip mismatch.\nwant: %s\n got: %s",
			string(wantMetricBytes),
			string(gotByEvent["claude_code.token.usage"]))
	}

	// Sanity check: deterministic event ordering on the output.
	keys := make([]string, 0, len(gotByEvent))
	for k := range gotByEvent {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if got, want := strings.Join(keys, ","), "claude_code.api_request,claude_code.token.usage"; got != want {
		t.Errorf("unexpected event set: got %q want %q", got, want)
	}
}

// postOTLP POSTs a JSON body to the given URL and fails the test on any
// non-2xx response. Helper for TestCollectorParity_OTLPRoundTrip.
func postOTLP(t *testing.T, url, body string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: status %d: %s", url, resp.StatusCode, string(b))
	}
}
