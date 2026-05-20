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
// The end-to-end OTLP-parser-to-CollectedEvent path used to be covered
// by TestCollectorParity_OTLPRoundTrip below; that test was deleted in
// spec 083 Bead 5 alongside the OTLP parser it covered (Collector,
// handleLogs, handleMetrics, extractLogEvents, extractMetricEvents).
// The OTLP receiver lives in agentmind now (one-way ADR-0011 dependency
// over OTLP/HTTP:4318); its OTLP round-trip coverage runs in the
// agentmind repo.
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
