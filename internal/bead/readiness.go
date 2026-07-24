// Package bead. readiness.go declares the two dedicated bd metadata key
// constants spec 124 (impl-readiness-gate) uses to carry ADVISORY audit
// annotations about a bead's readiness evaluation history (ADR-0023: bd/
// Dolt stays the single lifecycle-state authority; these keys are never
// lifecycle state and are never read by any mechanical readiness signal).
//
// Both keys are written EXCLUSIVELY via the existing bead.MergeMetadata
// helper by their owning verbs (the override key is additionally REMOVED
// via DeleteMetadataKeys on `next`'s claim-failure rollback path —
// final-review r1 G3-OVERRIDE-ORPHAN):
//   - MetaKeyReadinessOverride is written by `mindspec next --allow-not-ready`
//     (spec 124 Bead 2 / R3) immediately BEFORE ClaimBead (marker-before-
//     claim, FAIL-CLOSED — `--allow-not-ready` success guarantees a durable
//     marker, and a claim failure after the write rolls the marker back),
//     naming the mechanical signals (MF-1..MF-4) the operator deliberately
//     bypassed plus a UTC timestamp — the durable override marker the R4
//     dispatch ingress re-check (Bead 3) honors, for exactly the recorded
//     signals, instead of re-blocking a deliberately force-claimed bead.
//   - MetaKeyReadinessAttempt is written by `mindspec bead clarify` (spec 124
//     Bead 3 / R8) exactly once per bead — the append-only readiness-attempt
//     record: the original ordinal-keyed NOT-READY report plus the
//     span-grounded clarification entries.
//
// Neither key is ever read by internal/validate/readiness's mechanical
// signals (MF-1..MF-4): the layer boundary (spec 124 R8e / AC-12) holds by
// construction because the engine's bd reads never consult these keys at
// all, not because of a runtime check. internal/bead itself never reads or
// interprets the values under these keys — it only names them so every
// writer/reader across the three beads shares one literal definition.
package bead

const (
	// MetaKeyReadinessOverride is the bd metadata key naming a durable,
	// deliberate `--allow-not-ready` override: the mechanical signals that
	// were bypassed at claim time, plus a UTC timestamp. Advisory only
	// (ADR-0023) — never consulted by any mechanical signal.
	MetaKeyReadinessOverride = "mindspec_readiness_override"

	// MetaKeyReadinessAttempt is the bd metadata key naming the append-only
	// readiness-attempt record: the original NOT-READY report (ordinal,
	// verbatim reason, signal tag) plus the grounded clarification entries
	// ({ordinal, reason, answer, span}). Written at most once per bead
	// (spec 124 R8d's categorical per-bead cap) — never consulted by any
	// mechanical signal.
	MetaKeyReadinessAttempt = "mindspec_readiness_attempt"
)
