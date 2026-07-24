package next

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

// ready_gate_test.go pins spec 124 (impl-readiness-gate) Bead 2's
// internal/next.GateReadiness / RecordReadinessOverride — the AC-4 unit
// level: the gate's decision logic (refuse / override / pass) against
// Bead 1's committed NEGATIVE/POSITIVE readiness fixtures, hermetically
// (readiness.FakeBDStore.Install swaps the readiness engine's OWN bd/git
// seams — no real bd, never t.Skip). The full command-level integration
// (the durable marker actually landing in `bd show`, the byte-identical
// refusal audit against real bd/git state, `--force` orthogonality) is
// pinned by cmd/mindspec/next_ready_gate_test.go, which drives the real
// `mindspec next` binary against a real bd/git sandbox — GateReadiness's
// own bd write (RecordReadinessOverride) has no seam reachable from
// outside internal/bead, so its actual bd round-trip can only be proven
// by a real bd process, not faked from this package.

// TestGateReadiness_NegativeFixtureRefuses pins AC-4's refusal shape: a
// failing floor with allowNotReady=false returns a nil result and a
// guard-shaped error carrying the SAME per-signal rendering
// `bead ready-check` prints (ADR-0040 no-restate), each failing signal's
// recovery line, AND both R3 escape hatches (the standalone ready-check
// report, and re-running with --allow-not-ready).
func TestGateReadiness_NegativeFixtureRefuses(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildNegativeFixture(root)
	if err != nil {
		t.Fatalf("BuildNegativeFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	result, err := GateReadiness(root, fx.BeadID, false)
	if err == nil {
		t.Fatal("expected a refusal error for the negative fixture, got nil")
	}
	if result != nil {
		t.Errorf("expected a nil GateResult on refusal, got %+v", result)
	}

	msg := err.Error()
	for _, want := range []string{"MF-1: FAIL", "MF-2: FAIL", "MF-3: FAIL", "MF-4: FAIL"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal message missing %q:\n%s", want, msg)
		}
	}
	if got := strings.Count(msg, "recovery: "); got < 6 {
		// 4 per-signal recovery lines + the 2 R3 escape hatches.
		t.Errorf("expected at least 6 recovery: lines (4 signals + 2 escape hatches), got %d:\n%s", got, msg)
	}
	if !strings.Contains(msg, "bead ready-check") {
		t.Errorf("refusal message missing the standalone ready-check escape hatch:\n%s", msg)
	}
	if !strings.Contains(msg, "--allow-not-ready") {
		t.Errorf("refusal message missing the --allow-not-ready escape hatch:\n%s", msg)
	}
}

// TestGateReadiness_NegativeFixtureAllowNotReady pins the override path:
// allowNotReady=true against a failing floor returns a nil error and a
// GateResult naming every failing signal (no refusal, no partial list).
func TestGateReadiness_NegativeFixtureAllowNotReady(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildNegativeFixture(root)
	if err != nil {
		t.Fatalf("BuildNegativeFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	result, err := GateReadiness(root, fx.BeadID, true)
	if err != nil {
		t.Fatalf("expected no error with allowNotReady=true, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected a non-nil GateResult")
	}
	want := map[string]bool{"MF-1": true, "MF-2": true, "MF-3": true, "MF-4": true}
	if len(result.FailingSignals) != len(want) {
		t.Fatalf("expected %d failing signals, got %d: %v", len(want), len(result.FailingSignals), result.FailingSignals)
	}
	for _, id := range result.FailingSignals {
		if !want[id] {
			t.Errorf("unexpected failing signal %q", id)
		}
		delete(want, id)
	}
	if len(want) != 0 {
		t.Errorf("missing failing signal(s): %v", want)
	}
}

// TestGateReadiness_PositiveFixturePasses pins the "adds no interactive
// step" half of R3: a passing floor returns a nil error and an EMPTY
// GateResult, regardless of allowNotReady's value (a passing floor never
// treats allowNotReady as a signal to warn about).
func TestGateReadiness_PositiveFixturePasses(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildPositiveFixture(root)
	if err != nil {
		t.Fatalf("BuildPositiveFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	for _, allowNotReady := range []bool{false, true} {
		result, err := GateReadiness(root, fx.BeadID, allowNotReady)
		if err != nil {
			t.Fatalf("allowNotReady=%v: expected no error for the positive fixture, got: %v", allowNotReady, err)
		}
		if result == nil {
			t.Fatalf("allowNotReady=%v: expected a non-nil GateResult", allowNotReady)
		}
		if len(result.FailingSignals) != 0 {
			t.Errorf("allowNotReady=%v: expected zero failing signals for the positive fixture, got %v", allowNotReady, result.FailingSignals)
		}
	}
}

// TestGateReadiness_EvaluationErrorPropagates pins the fail-closed
// behavior when the engine itself cannot evaluate (e.g. no discoverable
// bead lineage) — a bare evaluation error, not a NOT-READY guard failure,
// still returns (nil, err) so the caller never proceeds to claim.
func TestGateReadiness_EvaluationErrorPropagates(t *testing.T) {
	root := t.TempDir()
	store := readiness.NewFakeBDStore() // no lineage recorded for any bead
	restore := store.Install()
	t.Cleanup(restore)

	result, err := GateReadiness(root, "mindspec-nope.1", false)
	if err == nil {
		t.Fatal("expected an evaluation error for an unrecorded bead, got nil")
	}
	if result != nil {
		t.Errorf("expected a nil GateResult on evaluation error, got %+v", result)
	}
}

// TestRecordReadinessOverride_WritesMarkerViaMergeMetadata pins the
// durable-marker write shape: RecordReadinessOverride calls
// bead.MergeMetadata (via the mergeMetadataFn seam) with EXACTLY the
// bead.MetaKeyReadinessOverride key, carrying the signal list and a
// parseable UTC RFC3339 timestamp.
func TestRecordReadinessOverride_WritesMarkerViaMergeMetadata(t *testing.T) {
	orig := mergeMetadataFn
	t.Cleanup(func() { mergeMetadataFn = orig })

	var gotID string
	var gotUpdates map[string]interface{}
	mergeMetadataFn = func(issueID string, updates map[string]interface{}) error {
		gotID = issueID
		gotUpdates = updates
		return nil
	}

	signals := []string{"MF-2", "MF-4"}
	if err := RecordReadinessOverride("mindspec-abcd.1", signals); err != nil {
		t.Fatalf("RecordReadinessOverride: %v", err)
	}

	if gotID != "mindspec-abcd.1" {
		t.Errorf("expected issueID %q, got %q", "mindspec-abcd.1", gotID)
	}
	if len(gotUpdates) != 1 {
		t.Fatalf("expected exactly one metadata key written, got %d: %v", len(gotUpdates), gotUpdates)
	}
	raw, ok := gotUpdates[bead.MetaKeyReadinessOverride]
	if !ok {
		t.Fatalf("expected the %q key to be written, got keys %v", bead.MetaKeyReadinessOverride, gotUpdates)
	}
	marker, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected the marker value to be a map, got %T", raw)
	}
	gotSignals, ok := marker["signals"].([]string)
	if !ok || len(gotSignals) != 2 || gotSignals[0] != "MF-2" || gotSignals[1] != "MF-4" {
		t.Errorf("expected signals %v, got %v", signals, marker["signals"])
	}
	ts, ok := marker["overridden"].(string)
	if !ok || ts == "" {
		t.Fatalf("expected a non-empty overridden timestamp string, got %v", marker["overridden"])
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("overridden timestamp %q is not a valid RFC3339 UTC timestamp: %v", ts, err)
	}
}

// TestRecordReadinessOverride_PropagatesWriteFailure pins that a
// MergeMetadata failure is surfaced, never swallowed inside
// RecordReadinessOverride itself (the caller decides whether to warn-and-
// continue or fail hard — see cmd/mindspec/next.go's call site).
func TestRecordReadinessOverride_PropagatesWriteFailure(t *testing.T) {
	orig := mergeMetadataFn
	t.Cleanup(func() { mergeMetadataFn = orig })

	mergeMetadataFn = func(issueID string, updates map[string]interface{}) error {
		return errors.New("bd metadata merge-write failed")
	}

	if err := RecordReadinessOverride("mindspec-abcd.1", []string{"MF-1"}); err == nil {
		t.Fatal("expected the MergeMetadata failure to propagate, got nil")
	}
}
