package phase

// semantics_v2.go — spec 128 (lifecycle-verb-state-honesty) Bead 2, plan
// step 7 (round-1 G2): the v2 phase-semantics upgrade reconciliation.
//
// OQ-3 option 1 changes plan-approve's WRITTEN value from `implement` to
// `plan` (semantics_v2.go's sibling change, plan.go:462). An epic that
// went through `plan approve` under the OLD (pre-v2) semantics — stored
// `implement`, at least one lifecycle child, all of them still open (no
// claim was ever made) — carries a value that is now WRONG under v2
// semantics: v2 would have written `plan` for that exact state. This file
// performs the one-shot translation from the old representation to the
// new one, exactly once per epic, gated by a durable marker
// (`mindspec_phase_semantics: "v2"`) so it can never re-fire once applied
// — mirroring ADR-0034's EnsureMigrated pattern (migrate.go) but for a
// semantics generation, not a metadata-presence backfill.
//
// Taxonomy note (plan round-3 G5): this rewrite sits OUTSIDE the
// sanction taxonomy phaselock.go/sanctioned_sync.go define. It is a
// TRANSLATION of an unchanged underlying fact (the epic's real lifecycle
// position never moved) from a pre-v2 to a v2 representation, performed
// once per epic and never again — not an ongoing lifecycle backward
// move. It does NOT route through SanctionedDerivedSync; it is its own
// locked one-shot, keyed on the durable marker rather than "re-derive
// every time" semantics.
//
// Named hazard (bead-1 panel hard-blocked TWICE on this exact class):
// every read this reconciliation performs — the marker lookup and the
// children read — happens on a FRESH per-call Cache constructed INSIDE
// WithPhaseWriteLock's fn (never a cache built before the lock was
// acquired), and a read failure PROPAGATES with no write attempted. The
// `stored` value handed to fn IS the lock's own freshness guarantee
// (phaselock.go's phaseReadOutcome) — no separate stored-phase re-read is
// needed for it.

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// PhaseSemanticsV2MarkerKey is the epic metadata key recording that this
// epic has already been reconciled to v2 phase semantics (or created
// natively under them) — the durable discriminator that makes
// EnsurePhaseSemanticsV2 a true one-shot: once written, the legacy
// rewrite branch below can never fire again for this epic, even if an
// out-of-band raw `bd update` later manufactures the legacy signature
// post-marker (a scoped, disclosed residual — see the file doc comment).
const PhaseSemanticsV2MarkerKey = "mindspec_phase_semantics"

// PhaseSemanticsV2Value is the marker's written/expected value.
const PhaseSemanticsV2Value = "v2"

// LegacyPhaseSignature reports whether stored/children match the
// pre-v2 plan-approve signature: stored phase is `implement`, at least
// one LIFECYCLE child exists (the ADR-0023 task-typed-or-untyped
// discriminator classifyChild already encodes), and EVERY lifecycle
// child's normalized status is EXACTLY `open` — i.e. plan-approve ran
// (writing the old `implement` value) but no bead has ever been claimed.
// Under v2 semantics that exact state would have stored `plan` instead.
//
// Round-1 panel G2/G5 (hard-block-adjacent, R5): this is a POSITIVE
// "all open" check, not a negative "none closed/in_progress" one — a
// `blocked`, empty, or otherwise custom/unknown status is NEITHER closed
// nor in_progress, so a negative check would have wrongly classified a
// blocked-child epic (a genuinely different, non-legacy state) as
// legacy. Every lifecycle child must affirmatively be `open`.
//
// Lock-free and read-only by design (plan round-2 G4): this predicate
// never takes the R3.3 write lock — it exists so READ paths (derive.go's
// stored-vs-derived consistency branch, `state show`) can recognize the
// legacy condition and emit an informative notice without acquiring a
// lock merely to look.
func LegacyPhaseSignature(stored string, children []ChildInfo) bool {
	if stored != state.ModeImplement {
		return false
	}
	var lifecycleTotal, open int
	for _, c := range children {
		if classifyChild(c.IssueType) != childLifecycle {
			continue
		}
		lifecycleTotal++
		if strings.ToLower(strings.TrimSpace(c.Status)) == "open" {
			open++
		}
	}
	return lifecycleTotal > 0 && open == lifecycleTotal
}

// EnsurePhaseSemanticsV2 runs the one-shot, idempotent, lock-protected v2
// phase-semantics reconciliation for epicID (root locates the R3.3 phase
// write lock, matching every other locked writer in this package). Like
// EnsureMigrated and SanctionedDerivedSync, it writes via this package's
// OWN mergeMetadataFn seam (mergeMetadataFn's doc comment; migrate.go) —
// deliberately NOT an injected caller seam. A caller-supplied seam
// (implPhaseMetadataFn, planMergeMetadataFn, ...) exists so that VERB's
// OWN phase-value writes are individually observable/stubbable in its
// own tests (e.g. "implPhaseMetadataFn must not be called on a preflight
// refusal", impl_test.go); EnsurePhaseSemanticsV2 is a DIFFERENT
// writer — the same ahead-of-preflight ONE-SHOT MIGRATION-SHAPED
// reconciliation EnsureMigrated already is — and sharing a caller's own
// per-verb seam would make an unrelated verb-specific "this seam was
// never called" assertion observe a write that has nothing to do with
// that verb's own value semantics. Tests that need to observe or stub
// this reconciliation's write use SetMergeMetadataForTest, exactly as
// EnsureMigrated's own tests already do.
//
// Returns (rewrote bool, err error): rewrote reports whether the legacy
// stored-phase rewrite actually fired (the marker is always written on
// first run regardless; rewrote distinguishes the "legacy → plan" leg
// from the "marker-only, phase untouched" leg for callers that log or
// test the distinction).
//
// Three outcomes, in order (plan step 7):
//   - marker PRESENT (any value at all, round-1 panel G3) → no-op (both
//     marker and phase untouched).
//   - marker absent AND LegacyPhaseSignature(stored, children) holds →
//     rewrite stored to `plan` AND write the marker, in the SAME
//     critical section (never two writes).
//   - marker absent, any other state (already progressed, released,
//     review, done, or simply created natively under v2) → write the
//     marker only; the phase is left exactly as it is.
func EnsurePhaseSemanticsV2(root, epicID string) (bool, error) {
	var rewrote bool
	lockErr := WithPhaseWriteLock(root, epicID, mergeMetadataFn, func(stored string, _ bool) (map[string]interface{}, error) {
		// Fresh per-call Cache — never a Cache built before this call
		// reached the lock (the exact hazard bead 1 was hard-blocked on
		// twice). Both the marker lookup and the children read go
		// through it.
		freshCache := NewCache()
		epic, err := freshCache.FindEpic(epicID)
		if err != nil {
			return nil, fmt.Errorf("ensure-phase-semantics-v2: reading epic %s: %w", idrender.Bead(epicID), err)
		}
		// Round-1 panel G3: forward-compat marker discrimination. ANY
		// present value — "v2", a future "v3"/"v4", or even a malformed
		// non-string value from an out-of-band edit — means this epic
		// has ALREADY been discriminated (natively created under some
		// known-or-future semantics generation, or already reconciled)
		// and must never be overwritten with "v2" or have the legacy
		// rewrite run over it. Only a genuinely ABSENT key is eligible
		// for this one-shot.
		if epic != nil && epic.Metadata != nil {
			if _, ok := epic.Metadata[PhaseSemanticsV2MarkerKey]; ok {
				return nil, nil // marker present (any value) -> no-op, phase untouched.
			}
		}

		children, cerr := freshCache.GetChildren(epicID)
		if cerr != nil {
			return nil, fmt.Errorf("ensure-phase-semantics-v2: reading children of epic %s: %w", idrender.Bead(epicID), cerr)
		}

		updates := map[string]interface{}{PhaseSemanticsV2MarkerKey: PhaseSemanticsV2Value}
		if LegacyPhaseSignature(stored, children) {
			updates["mindspec_phase"] = state.ModePlan
			rewrote = true
		}
		return updates, nil
	})
	if lockErr != nil {
		return false, fmt.Errorf("ensure-phase-semantics-v2 %s: %w", idrender.Bead(epicID), lockErr)
	}
	return rewrote, nil
}

// hasV2MarkerWithCache reports whether epicID's metadata already carries
// the phase-semantics discriminator marker (ANY value — round-1 panel
// G3 forward-compat, matching EnsurePhaseSemanticsV2's own presence-only
// check), via the SAME cache c (memoized by Cache.FindEpic) the caller
// is already using — round-2 G4's lock-free read-side consumer
// (derive.go's stored-vs-derived consistency branch): it never takes the
// R3.3 write lock, and when the caller already resolved the epic on this
// cache moments earlier (as derive.go's readStoredPhaseWithCache does),
// this is a zero-extra-bd-call lookup.
func hasV2MarkerWithCache(c *Cache, epicID string) bool {
	epic, err := c.FindEpic(epicID)
	if err != nil || epic == nil || epic.Metadata == nil {
		return false
	}
	_, ok := epic.Metadata[PhaseSemanticsV2MarkerKey]
	return ok
}
