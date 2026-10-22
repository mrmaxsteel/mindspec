package phase_test

// derive_v2_test.go — spec 128 (lifecycle-verb-state-honesty) Bead 2: the
// derive.go warning-emission tests plan step 7/8 requires — AC-5(iv)'s
// direction-conditional recovery (forward keeps `repair phase`, backward
// emits the `bd show` inspection line) and the legacy-notice substitution
// (a marker-absent epic matching LegacyPhaseSignature gets the
// REPLACEMENT notice instead of the general mismatch warning). External
// package, mirroring derive_recovery_test.go, so stderr capture and the
// guard.HasFinalRecoveryLine format-alignment check both work the same
// way (internal/guard imports internal/phase — an import cycle from
// inside the package itself).
//
// AC-5's own close: "No test executes an emitted string" — every
// assertion below checks STRUCTURE (a substring naming the mechanism, a
// final recovery line, the absence of a banned command) never a full
// string match.

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// TestDerivePhase_LegacyNoticeSubstitution: marker absent, stored
// `implement`, all lifecycle children open (the legacy plan-approve
// signature) — the READ path emits the LEGACY NOTICE naming
// `mindspec repair phase <spec-id>`, never the general "disagrees with
// child-derived phase" mismatch warning, and never takes the write lock
// (DerivePhase itself is read-only regardless).
func TestDerivePhase_LegacyNoticeSubstitution(t *testing.T) {
	epicJSON := []byte(`[{"id":"epic-1","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"implement","spec_num":10,"spec_title":"test"}}]`)
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"b1","title":"bead","status":"open","issue_type":"task"}]`), nil
	})
	defer restoreList()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return epicJSON, nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	var got string
	var err error
	stderr := captureStderrPhase(t, func() {
		got, err = phase.DerivePhaseWithStatus("epic-1", "open")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != state.ModeImplement {
		t.Errorf("DerivePhaseWithStatus = %q, want %q (stored still trusted)", got, state.ModeImplement)
	}
	warning := strings.TrimRight(stderr, "\n")
	if !strings.Contains(warning, "legacy phase semantics detected") {
		t.Fatalf("expected the legacy notice to fire; stderr=%q", stderr)
	}
	if strings.Contains(warning, "disagrees with child-derived phase") {
		t.Errorf("legacy notice must REPLACE the general mismatch warning, not accompany it: %q", warning)
	}
	if !guard.HasFinalRecoveryLine(warning) {
		t.Errorf("legacy notice must end with a recovery line: %q", warning)
	}
	if !strings.Contains(warning, "recovery: mindspec repair phase '010-test'") {
		t.Errorf("legacy notice recovery must be the re-runnable repair command: %q", warning)
	}
}

// TestDerivePhase_LegacyNoticeSelfHealsViaRepairPhase is the READ-path
// fixture's second half (plan step 7's sixth fixture): after
// EnsurePhaseSemanticsV2 reconciles the SAME epic (simulating what
// `repair phase`'s own self-heal call does), a subsequent DerivePhase
// call is clean — no legacy notice, no mismatch warning, stored ==
// derived.
func TestDerivePhase_LegacyNoticeSelfHealsViaRepairPhase(t *testing.T) {
	epicMeta := map[string]interface{}{"mindspec_phase": "implement", "spec_num": float64(10), "spec_title": "test"}
	epicJSONFor := func() []byte {
		// Minimal inline JSON builder mirroring the metadata map above —
		// avoids importing encoding/json solely for one fixture.
		marker, _ := epicMeta["mindspec_phase_semantics"].(string)
		markerField := ""
		if marker != "" {
			markerField = `,"mindspec_phase_semantics":"` + marker + `"`
		}
		phaseVal, _ := epicMeta["mindspec_phase"].(string)
		return []byte(`[{"id":"epic-1","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"` + phaseVal + `","spec_num":10,"spec_title":"test"` + markerField + `}}]`)
	}
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if strings.HasPrefix(a, "--parent") {
				return []byte(`[{"id":"b1","title":"bead","status":"open","issue_type":"task"}]`), nil
			}
		}
		return epicJSONFor(), nil
	})
	defer restoreList()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return epicJSONFor(), nil
		}
		return []byte("[]"), nil
	})
	defer restore()
	restoreMerge := phase.SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		for k, v := range updates {
			epicMeta[k] = v
		}
		return nil
	})
	defer restoreMerge()

	// Before self-heal: the legacy notice fires (proven above; re-proven
	// here as the fixture's starting condition).
	preStderr := captureStderrPhase(t, func() {
		_, _ = phase.DerivePhaseWithStatus("epic-1", "open")
	})
	if !strings.Contains(preStderr, "legacy phase semantics detected") {
		t.Fatalf("fixture setup: expected the legacy notice pre-self-heal; stderr=%q", preStderr)
	}

	root := t.TempDir()
	if _, err := phase.EnsurePhaseSemanticsV2(root, "epic-1"); err != nil {
		t.Fatalf("EnsurePhaseSemanticsV2: %v", err)
	}
	if epicMeta["mindspec_phase"] != state.ModePlan {
		t.Fatalf("self-heal must rewrite stored to %q, got %v", state.ModePlan, epicMeta["mindspec_phase"])
	}

	postStderr := captureStderrPhase(t, func() {
		got, err := phase.DerivePhaseWithStatus("epic-1", "open")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != state.ModePlan {
			t.Errorf("post-self-heal DerivePhaseWithStatus = %q, want %q", got, state.ModePlan)
		}
	})
	if postStderr != "" {
		t.Errorf("post-self-heal state must be clean (stored == derived, no notice, no warning): stderr=%q", postStderr)
	}
}

// TestDerivePhase_AC5iv_ForwardDisagreementKeepsRepairPhaseRecovery: a
// marker-present epic (round-3 G2 fixture pinning: NOT the legacy
// signature, so the notice branch never fires) whose derived phase is
// AHEAD of stored keeps the existing `mindspec repair phase <spec-id>`
// recovery — unchanged from the pre-bead-2 behavior.
func TestDerivePhase_AC5iv_ForwardDisagreementKeepsRepairPhaseRecovery(t *testing.T) {
	epicJSON := []byte(`[{"id":"epic-1","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"implement","spec_num":10,"spec_title":"test","mindspec_phase_semantics":"v2"}}]`)
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		// All children closed -> derived "review", AHEAD of stored "implement".
		return []byte(`[{"id":"b1","title":"bead","status":"closed","issue_type":"task"}]`), nil
	})
	defer restoreList()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return epicJSON, nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	stderr := captureStderrPhase(t, func() {
		_, _ = phase.DerivePhaseWithStatus("epic-1", "open")
	})
	warning := strings.TrimRight(stderr, "\n")
	if !strings.Contains(warning, "disagrees with child-derived phase") {
		t.Fatalf("expected the general mismatch warning to fire; stderr=%q", stderr)
	}
	if strings.Contains(warning, "legacy phase semantics") {
		t.Errorf("a marker-present epic must never emit the legacy notice: %q", warning)
	}
	if !guard.HasFinalRecoveryLine(warning) {
		t.Errorf("forward disagreement must end with a recovery line: %q", warning)
	}
	if !strings.Contains(warning, "recovery: mindspec repair phase '010-test'") {
		t.Errorf("forward disagreement must keep the repair-phase recovery: %q", warning)
	}
	if strings.Contains(warning, "bd show") {
		t.Errorf("forward disagreement must not emit the backward-only inspection line: %q", warning)
	}
}

// TestDerivePhase_AC5iv_BackwardDisagreementEmitsInspectionLineNeverRepairPhase:
// a marker-present epic whose derived phase is BEHIND stored (the exact
// backward-disagreement leg AC-5(iv) requires — round-1 G6.3) ends with
// an inspection action (`bd show <epic-id>`), NEVER the
// `mindspec repair phase` recovery command that would write the
// backward move.
func TestDerivePhase_AC5iv_BackwardDisagreementEmitsInspectionLineNeverRepairPhase(t *testing.T) {
	// Stored "review" (ahead), all children open -> derived "plan"
	// (BEHIND stored) — marker present so this is NOT the legacy
	// signature (which requires stored == implement).
	epicJSON := []byte(`[{"id":"epic-9","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"review","spec_num":10,"spec_title":"test","mindspec_phase_semantics":"v2"}}]`)
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"b1","title":"bead","status":"open","issue_type":"task"}]`), nil
	})
	defer restoreList()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return epicJSON, nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	var got string
	stderr := captureStderrPhase(t, func() {
		got, _ = phase.DerivePhaseWithStatus("epic-9", "open")
	})
	if got != state.ModeReview {
		t.Errorf("stored phase must still be trusted (never auto-corrected on read): got %q, want %q", got, state.ModeReview)
	}
	warning := strings.TrimRight(stderr, "\n")
	if !strings.Contains(warning, "disagrees with child-derived phase") {
		t.Fatalf("expected the general mismatch warning to fire; stderr=%q", stderr)
	}
	if !strings.Contains(warning, "BEHIND stored") {
		t.Errorf("backward disagreement must name the direction: %q", warning)
	}
	if !guard.HasFinalRecoveryLine(warning) {
		t.Errorf("backward disagreement must still end with a recovery line (ADR-0035): %q", warning)
	}
	if !strings.Contains(warning, "recovery: bd show 'epic-9'") {
		t.Errorf("backward disagreement must end with an inspection action, not a write command: %q", warning)
	}
	if strings.Contains(warning, "recovery: mindspec repair phase") {
		t.Errorf("backward disagreement must NEVER emit the repair-phase write recovery (ADR-0035 round-1 G6.3): %q", warning)
	}
}
