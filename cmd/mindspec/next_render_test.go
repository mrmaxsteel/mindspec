package main

// next_render_test.go pins spec 128 Bead 8's folded-in ta30.9 render-leg
// follow-up: next.go's two remaining raw-%v render legs on a bd/lock-
// derived error — the RecordReadinessOverride failure message inside
// guard.NewFailure (readinessOverrideFailure) and
// describePhaseSyncFailure's non-timeout leg — must never let a hostile
// control byte from the underlying error survive into the rendered
// message, reusing the bdRenderedError/RenderBDError discipline rather
// than forking a second escaping path.

import (
	"errors"
	"strings"
	"testing"
)

// TestReadinessOverrideFailure_HostileContentEscaped pins that
// readinessOverrideFailure's %v render of a RecordReadinessOverride-style
// failure never leaks a raw control byte, whether or not the error
// arrives already wrapped (RecordReadinessOverride wraps it in
// bdRenderedError today — internal/next/ready_gate.go:130 — but this
// function must not silently regress if that upstream wrapping ever
// changes).
func TestReadinessOverrideFailure_HostileContentEscaped(t *testing.T) {
	hostile := errors.New("bd metadata merge-write failed\x1b[31mFORGED\x1b[0m")
	err := readinessOverrideFailure("mindspec-abcd.1", hostile)
	if err == nil {
		t.Fatal("expected a non-nil failure")
	}
	rendered := err.Error()
	if strings.ContainsRune(rendered, 0x1b) {
		t.Errorf("raw ESC byte survived into readinessOverrideFailure's rendered message: %q", rendered)
	}
}

// TestDescribePhaseSyncFailure_NonTimeoutLeg_HostileContentEscaped pins
// the folded-in follow-up's second leg: describePhaseSyncFailure's
// non-timeout branch now routes err through next.RenderBDError before
// formatting it with %v, so a hostile control byte in a bd/lock-derived
// failure (e.g. a children-read error carrying bd's own stderr) never
// reaches the rendered warning raw.
func TestDescribePhaseSyncFailure_NonTimeoutLeg_HostileContentEscaped(t *testing.T) {
	hostile := errors.New("advance-phase-on-claim: reading children of epic mindspec-xxxx: bd: \x1b[31mFORGED\x1b[0m")
	rendered := describePhaseSyncFailure(hostile, "mindspec repair phase 042-fixture")
	if strings.ContainsRune(rendered, 0x1b) {
		t.Errorf("raw ESC byte survived into describePhaseSyncFailure's non-timeout rendering: %q", rendered)
	}
	if !strings.Contains(rendered, "FORGED") {
		t.Errorf("expected the underlying diagnostic text to survive (quoted, not raw), got: %q", rendered)
	}
	if !strings.Contains(rendered, "recovery: mindspec repair phase 042-fixture") {
		t.Errorf("expected the convergent recovery line to be appended unconditionally, got: %q", rendered)
	}
}

// TestDescribePhaseSyncFailure_NilErrReturnsEmpty pins the existing nil
// contract is unchanged by the RenderBDError wrap.
func TestDescribePhaseSyncFailure_NilErrReturnsEmpty(t *testing.T) {
	if got := describePhaseSyncFailure(nil, "mindspec repair phase 042-fixture"); got != "" {
		t.Errorf("expected \"\" for a nil err, got %q", got)
	}
}
