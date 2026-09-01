package phase

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

// TestIsBackwardMove_TotalOrder pins the total order spec < plan <
// implement < review < done: every strictly-earlier-to-strictly-later
// pair is forward (not backward), every same-value pair is not backward
// (a no-op write), and every later-to-earlier pair is backward.
func TestIsBackwardMove_TotalOrder(t *testing.T) {
	order := []string{state.ModeSpec, state.ModePlan, state.ModeImplement, state.ModeReview, state.ModeDone}
	for i, from := range order {
		for j, to := range order {
			want := j < i
			got := IsBackwardMove(from, to)
			if got != want {
				t.Errorf("IsBackwardMove(%q, %q) = %v, want %v", from, to, got, want)
			}
		}
	}
}

// TestIsBackwardMove_UnknownValuesAreConservativelyBackward pins the
// guard-conservative disposition: an unknown value on either side of the
// comparison (including "", and a bogus/typo'd phase string) makes the
// pair incomparable, and IsBackwardMove treats an incomparable pair as
// backward — "we can't tell" must never resolve to "allow" for a guard
// that exists to refuse unsafe writes by default.
func TestIsBackwardMove_UnknownValuesAreConservativelyBackward(t *testing.T) {
	cases := []struct {
		name, from, to string
	}{
		{"empty from, known to", "", state.ModePlan},
		{"known from, empty to", state.ModeImplement, ""},
		{"both empty", "", ""},
		{"bogus from", "bogus-phase", state.ModePlan},
		{"bogus to", state.ModeImplement, "bogus-phase"},
		{"both bogus", "bogus-1", "bogus-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsBackwardMove(tc.from, tc.to) {
				t.Errorf("IsBackwardMove(%q, %q) = false, want true (incomparable → conservative backward)", tc.from, tc.to)
			}
		})
	}
}

// TestIsBackwardMove_SameValueIsNotBackward guards the AC-5-adjacent
// "already consistent" repair path: writing the same phase value back
// (an idempotent refresh) must not be classified as a backward move.
func TestIsBackwardMove_SameValueIsNotBackward(t *testing.T) {
	for _, p := range []string{state.ModeSpec, state.ModePlan, state.ModeImplement, state.ModeReview, state.ModeDone} {
		if IsBackwardMove(p, p) {
			t.Errorf("IsBackwardMove(%q, %q) = true, want false (same value, not backward)", p, p)
		}
	}
}
