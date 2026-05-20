// Package bench — collected-event type aliases.
//
// Spec 083 Phase 5 (Bead 5): the OTLP HTTP receiver and its parsing
// helpers (Collector, NewCollector, NewCollectorAppend, handleLogs,
// handleMetrics, extractLogEvents, extractMetricEvents,
// flattenAttributes, parseOTLPTimestamp) were deleted from this file.
// AgentMind now owns the OTLP receiver (one-way ADR-0011 dependency
// over OTLP/HTTP:4318 — see `github.com/mrmaxsteel/agentmind/cmd/agentmind`
// and `github.com/mrmaxsteel/agentmind/internal/otlp`).
//
// What remains here is the Phase-2 alias re-export — `CollectedEvent`,
// `otlpValue`, `otlpKeyValue` — kept because in-mindspec callers
// (`internal/recording/markers.go`, the parity tests) still reference
// the bench-side name. These are type aliases of
// `github.com/mrmaxsteel/agentmind/wire`, so the wire package is the
// single source of truth.
package bench

import "github.com/mrmaxsteel/agentmind/wire"

// CollectedEvent is the normalized NDJSON schema for collected telemetry.
//
// Spec 083 Bead 2 (Phase 2 alias state): re-exported as a type alias of
// wire.CollectedEvent so future beads can swap the OTLP-parsing
// implementation without breaking callers. Bead 5 deleted the OTLP
// parser; the alias remains to spare in-mindspec callers a churn-only
// rename.
type CollectedEvent = wire.CollectedEvent

// otlpValue is the package-private alias of wire.OTLPValue. Retained
// alongside CollectedEvent for symmetry with the wire package; not
// referenced after the OTLP parser was removed but cheap to keep.
type otlpValue = wire.OTLPValue //nolint:unused

// otlpKeyValue is the package-private alias of wire.OTLPKeyValue.
type otlpKeyValue = wire.OTLPKeyValue //nolint:unused
