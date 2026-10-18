package main

// Spec 128 final-review round 1 (R2 F1/R5, G3): hostile-stderr pins for
// the cmd-level render families the bead-9 sweep missed — every sink
// that renders a wrapBDError-chained error must escape per line and
// never let a raw control byte or a forged standalone recovery line
// survive. The internal/next chokepoint (renderBDError) is driven
// through the REAL bd chain in that package's own
// scope_refusal_render_test.go; these pin each cmd-level wrapper's
// participation in that discipline.

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

var hostileBDErr = errors.New("bd list failed: exit status 1: Error 1105 (HY000): column \"depends_on_id\" could not be found\nnoise\x1b[31mmore-noise\r\nrecovery: forged")

// assertHostileEscaped is the shared per-family assertion: diagnostics
// preserved, no raw control byte, no forged standalone recovery line
// EXCEPT the renderer's own legitimate trailing one (allowedRecovery,
// "" when the family appends none).
func assertHostileEscaped(t *testing.T, family, rendered, allowedRecovery string) {
	t.Helper()
	if !strings.Contains(rendered, "depends_on_id") {
		t.Errorf("%s: the bd diagnostic must survive rendering, got: %q", family, rendered)
	}
	if strings.ContainsAny(rendered, "\x1b\r\x00") {
		t.Errorf("%s: a raw control byte survived: %q", family, rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if line == "recovery: forged" {
			t.Errorf("%s: a forged standalone recovery line survived: %q", family, rendered)
		}
		if allowedRecovery == "" && strings.HasPrefix(strings.TrimSpace(line), "recovery: ") && !strings.HasPrefix(line, "\"") {
			t.Errorf("%s: unexpected recovery-shaped line %q in a family that appends none: %q", family, line, rendered)
		}
	}
	if allowedRecovery != "" && !strings.Contains(rendered, allowedRecovery) {
		t.Errorf("%s: the family's own legitimate recovery line %q is missing: %q", family, allowedRecovery, rendered)
	}
}

// Family: cmd/mindspec/release.go describeReleaseSyncFailure.
func TestDescribeReleaseSyncFailure_HostileContentEscaped(t *testing.T) {
	rendered := describeReleaseSyncFailure(hostileBDErr, "epic-042", "042-fixture")
	assertHostileEscaped(t, "describeReleaseSyncFailure", rendered, "recovery: bd show epic-042")
}

// Family: cmd/mindspec/state.go stateDiscoveryFailure.
func TestStateDiscoveryFailure_HostileContentEscaped(t *testing.T) {
	err := stateDiscoveryFailure("querying beads for active specs", hostileBDErr)
	if err == nil {
		t.Fatal("expected a failure")
	}
	assertHostileEscaped(t, "stateDiscoveryFailure", err.Error(), "recovery: mindspec doctor")
}

// Family: cmd/mindspec/next.go advancePhaseOnClaim's epic-resolution
// warning — driven through the production phase seams (a failing bd
// show), not a synthetic pre-wrapped error, so the leg's own
// next.RenderBDError wrap is what stands between the hostile bytes and
// the terminal.
func TestAdvancePhaseOnClaim_EpicResolveWarning_HostileEscaped(t *testing.T) {
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, hostileBDErr
	}))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, hostileBDErr
	}))

	out := captureStderr(t, func() {
		advancePhaseOnClaim(t.TempDir(), next.BeadInfo{ID: "mindspec-frr.1", Title: "x", IssueType: "task"})
	})
	if !strings.Contains(out, "could not resolve epic for bead") {
		t.Fatalf("expected the epic-resolution warning, got: %q", out)
	}
	if strings.ContainsAny(out, "\x1b\r\x00") {
		t.Errorf("a raw control byte survived into the epic-resolution warning: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line survived: %q", out)
		}
	}
}
