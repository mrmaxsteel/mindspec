package phase

// order.go — spec 128 (lifecycle-verb-state-honesty) Bead 1, R3.3: the
// lifecycle phase total order (ADR-0023 §3: spec < plan < implement <
// review < done). `mindspec repair phase`'s backward-move guard
// (AC-5(ii)/(iii)) uses IsBackwardMove to refuse a write that would move
// an epic's stored mindspec_phase backward in this order unless the
// operator explicitly forces it with a recorded reason.

import "github.com/mrmaxsteel/mindspec/internal/state"

// phaseRank assigns each of the five named lifecycle phases its position
// in the total order spec < plan < implement < review < done. A phase
// value absent from this map — "", or any string that is not one of the
// five named phases — has NO rank.
var phaseRank = map[string]int{
	state.ModeSpec:      0,
	state.ModePlan:      1,
	state.ModeImplement: 2,
	state.ModeReview:    3,
	state.ModeDone:      4,
}

// IsBackwardMove reports whether moving an epic's stored mindspec_phase
// FROM `from` TO `to` is a backward move in the lifecycle order (spec <
// plan < implement < review < done).
//
// Guard-conservative by design: when either value is UNKNOWN — not one of
// the five named phases, including the empty string "" (no stored phase
// yet) — the two values are INCOMPARABLE, and an incomparable pair is
// treated as backward. The guard this function serves must never let an
// unrecognized or absent phase value slip past unchecked: "we can't
// order-compare these" defaults to "refuse without --force-backward", not
// "allow". This also means a from="" (never-migrated / not-yet-written)
// case counts as backward — a deliberately conservative choice for the
// repair guard specifically; callers that need a true absent-only
// backfill semantics (e.g. a legacy migration) must not route through
// this comparator and instead test for the empty string directly.
func IsBackwardMove(from, to string) bool {
	fromRank, fromOK := phaseRank[from]
	toRank, toOK := phaseRank[to]
	if !fromOK || !toOK {
		return true
	}
	return toRank < fromRank
}
