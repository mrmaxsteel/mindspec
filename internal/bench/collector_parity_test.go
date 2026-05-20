package bench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/agentmind/wire"
)

// TestCollectorParity is the Phase-2 NDJSON parity test (spec 083 Bead 2
// step 7, acceptance criterion: "NDJSON byte-for-byte parity (Phase 2,
// alias state) — fixture produced via canonical wire/event.go encoder").
//
// Method (spec lines 247-250):
//
//  1. Reconstruct the same frozen-clock event set the fixture was
//     generated from (see agentmind/cmd/genparity for the source of
//     truth).
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
func TestCollectorParity(t *testing.T) {
	t.Parallel()

	events := parityFixtureEvents()

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

// parityFixtureEvents returns the frozen-clock event set the parity
// fixture was generated from. MUST stay in lock-step with
// agentmind/cmd/genparity. When this test fails because the event set
// drifted, re-run `cd ../agentmind && go run ./cmd/genparity >
// internal/bench/testdata/parity.ndjson` (from the mindspec repo root)
// to regenerate the fixture and commit it in a separate provenance
// commit per Bead 2 step 5.
func parityFixtureEvents() []CollectedEvent {
	return []CollectedEvent{
		{
			TS:    "2026-01-01T00:00:00.000000001Z",
			Event: "claude_code.api_request",
			Data: map[string]any{
				"model":             "claude-opus-4-7",
				"input_tokens":      int64(1024),
				"output_tokens":     int64(256),
				"cache_read_tokens": int64(0),
				"latency_ms":        int64(1234),
				"success":           "true",
			},
			Resource: map[string]any{
				"service.name":    "claude-code",
				"service.version": "1.0.0",
				"host.name":       "frozen-clock-fixture",
			},
		},
		{
			TS:    "2026-01-01T00:00:01.123456789Z",
			Event: "claude_code.token.usage",
			Data: map[string]any{
				"metric": "claude_code.token.usage",
				"value":  float64(1024),
				"type":   "input",
				"model":  "claude-opus-4-7",
			},
			Resource: map[string]any{
				"service.name": "claude-code",
			},
		},
		{
			TS:    "2026-01-01T00:00:02.999999999Z",
			Event: "claude_code.cost.usage",
			Data: map[string]any{
				"metric":   "claude_code.cost.usage",
				"value":    0.0123456,
				"currency": "USD",
				"model":    "claude-opus-4-7",
			},
			Resource: map[string]any{
				"service.name": "claude-code",
			},
		},
		{
			TS:    "2026-01-01T00:00:03Z",
			Event: "claude_code.session.start",
		},
	}
}
