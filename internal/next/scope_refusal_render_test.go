package next

// Spec 128 final-review round 1 (R2 F1/R5, G3): hostile-stderr pins for
// scopeResolutionRefusal's render boundary, driven through the REAL
// wrapBDError chain (a PATH-fronted fake bd failing with control-bearing
// stderr feeding phase's production bd runner) — not a synthetic error
// string — per R5's per-renderer-family requirement.

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// TestScopeResolutionRefusal_RealChainHostileStderrEscaped drives the
// AllEpics leg: the phase package's production listJSON runner spawns
// the fake bd, whose hostile stderr is folded in by wrapBDError and must
// arrive per-line escaped — diagnostics preserved, no raw control byte,
// no forged standalone recovery line.
func TestScopeResolutionRefusal_RealChainHostileStderrEscaped(t *testing.T) {
	writeFakeBD(t, `printf 'Error 1105 (HY000): column "depends_on_id" could not be found\n' >&2
printf 'noise\033[31mmore-noise\r\n' >&2
printf 'recovery: forged\n' >&2
exit 1`)

	err := scopeResolutionRefusal(t.TempDir(), "042-fixture", errors.New("seed epic lookup failure"))
	if err == nil {
		t.Fatal("expected the failing epics query to surface as an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "depends_on_id") {
		t.Errorf("the real bd diagnostic must survive rendering, got: %q", msg)
	}
	if strings.ContainsAny(msg, "\x1b\r\x00") {
		t.Errorf("a raw control byte reached scopeResolutionRefusal's rendered error: %q", msg)
	}
	for _, line := range strings.Split(msg, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line survived: %q", msg)
		}
	}
}

// TestScopeResolutionRefusal_EpicErrLegHostileEscaped pins the epicErr
// leg (healthy epics query, no draft): the seeded lookup failure's
// hostile bytes must render escaped through the same bdRenderedError
// discipline.
func TestScopeResolutionRefusal_EpicErrLegHostileEscaped(t *testing.T) {
	restore := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer restore()

	hostile := errors.New("bd list failed: \x1b[31mFORGED\x1b[0m\nrecovery: forged")
	err := scopeResolutionRefusal(t.TempDir(), "042-fixture", hostile)
	if err == nil {
		t.Fatal("expected the epicErr leg to surface as an error")
	}
	msg := err.Error()
	if strings.ContainsRune(msg, 0x1b) {
		t.Errorf("a raw ESC byte reached the epicErr leg's rendered error: %q", msg)
	}
	for _, line := range strings.Split(msg, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line survived: %q", msg)
		}
	}
}
