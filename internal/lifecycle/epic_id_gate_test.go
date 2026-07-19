package lifecycle

import (
	"strings"
	"testing"
)

// TestBDListParentEpicIDGate is spec 120 AC-26 (round-8 epicID gate): the
// listOpenBeadsFn (stale_open.go) and listClosedBeadsFn (orphans.go)
// DEFAULT seams, given "--help", "x;evil", and the 116 hostile
// control-byte triple, return the validation error with ZERO bd spawn —
// the gate precedes bead.RunBD, so the returned error is the validation
// class, never an exec error, the value escaped-only in the assertion
// below (never interpolated raw into a t.Errorf format needing that, but
// asserted not to crash on it). A well-formed epicID (mindspec-s2mf)
// produces byte-identical argv to today (exactly one call) — proven via
// PATH-starvation: a real bd spawn attempt surfaces a distinctive
// "executable file not found" error, which a malformed id must NEVER
// produce (proving no spawn was attempted for it), while a well-formed id
// legitimately does hit that PATH-starved spawn attempt.
func TestBDListParentEpicIDGate(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	hostileEpicIDs := []string{
		"--help",
		"x;evil",
		"x\x00\x1b[31m\nrecovery: forged",
	}

	for _, epicID := range hostileEpicIDs {
		if _, err := listOpenBeadsFn(epicID); err == nil {
			t.Errorf("listOpenBeadsFn(%q) accepted a hostile epic id", epicID)
		} else if strings.Contains(err.Error(), "executable file not found") {
			t.Errorf("listOpenBeadsFn(%q) attempted a real bd spawn (should be gated before any spawn): %v", epicID, err)
		}

		if _, err := listClosedBeadsFn(epicID); err == nil {
			t.Errorf("listClosedBeadsFn(%q) accepted a hostile epic id", epicID)
		} else if strings.Contains(err.Error(), "executable file not found") {
			t.Errorf("listClosedBeadsFn(%q) attempted a real bd spawn (should be gated before any spawn): %v", epicID, err)
		}
	}

	// A well-formed epicID reaches the real (PATH-starved) spawn attempt —
	// proving the gate does not block a valid id.
	if _, err := listOpenBeadsFn("mindspec-s2mf"); err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Errorf("listOpenBeadsFn(mindspec-s2mf) expected a PATH-starved spawn-attempt error, got: %v", err)
	}
	if _, err := listClosedBeadsFn("mindspec-s2mf"); err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Errorf("listClosedBeadsFn(mindspec-s2mf) expected a PATH-starved spawn-attempt error, got: %v", err)
	}
}
