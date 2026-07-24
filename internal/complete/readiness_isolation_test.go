package complete

// readiness_isolation_test.go pins spec 124 (impl-readiness-gate) Bead 3's
// AC-9(ii) / ADR-0037 protection: readiness state gains ZERO merge/gate
// authority over `mindspec complete`. A force-claimed bead carrying BOTH
// the durable `--allow-not-ready` override marker and a readiness-attempt
// record (spec 124's two dedicated bd metadata keys,
// bead.MetaKeyReadinessOverride / bead.MetaKeyReadinessAttempt) goes
// through the SAME panel-gate evaluation, byte-identically, as the
// identical bead without them — proving the mechanical/semantic
// readiness layer never becomes a second gate authority alongside the
// panel gate (Spec 099 Bead 2 / ADR-0037).
//
// This is an ANTI-OVERREACH guard (plan-gate Testing Strategy: "AC-9(ii)
// ... anti-overreach guards that pass once written and go red only
// against a non-conforming implementation, deviation stated in-test"):
// today NEITHER panelGate NOR panel.GateFacts/panel.ResolveGateFacts ever
// reads bd metadata for the panel-gate decision at all (panelGate scans
// panel.json files on disk via panel.ForBead/panel.Scan and calls
// git-only I/O — gateRevParseFn/gateStatusFn/gateIsRefNotFoundFn — never
// bead.GetMetadata), so this test is RED-on-a-hypothetical-regression: a
// future change that wires readiness metadata into the panel-gate
// decision would make the two scenarios below diverge, tripping this
// test — the deviation from today's structural isolation would have to
// be stated in-test per the plan's convention.

import (
	"encoding/json"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// readinessIsolationOutcome is the normalized, comparable shape of one
// complete.Run invocation — the exact facts AC-9(ii)'s "byte-identical
// gate behavior" claim is about.
type readinessIsolationOutcome struct {
	completeCalled bool
	beadClosed     bool
	errMsg         string
}

// readinessIsolationMetadataStore is a hermetic, in-memory stand-in for
// bd metadata: readinessIsolationSeedMetadata installs it over
// completeGetMetadataFn/completeMergeMetadataFn so this test never
// depends on a real bd process (fully hermetic, no t.Skip needed).
func readinessIsolationSeedMetadata(t *testing.T, beadID string, seed map[string]interface{}) {
	t.Helper()
	store := map[string]interface{}{}
	for k, v := range seed {
		store[k] = v
	}
	origGet := completeGetMetadataFn
	origMerge := completeMergeMetadataFn
	t.Cleanup(func() {
		completeGetMetadataFn = origGet
		completeMergeMetadataFn = origMerge
	})
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		if id != beadID {
			return map[string]interface{}{}, nil
		}
		out := make(map[string]interface{}, len(store))
		for k, v := range store {
			out[k] = v
		}
		return out, nil
	}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if id == beadID {
			for k, v := range updates {
				store[k] = v
			}
		}
		return nil
	}
}

// runReadinessIsolationScenario builds a fresh panel-gate repo (the
// panel_gate_e2e_test.go setupPanelGateRepo precedent) with the given
// panel verdicts, OPTIONALLY seeding the force-claimed bead's readiness
// state (spec 124's two dedicated metadata keys) BEFORE calling Run, and
// returns the normalized outcome.
func runReadinessIsolationScenario(t *testing.T, specID, beadID, panelSlug string, verdicts map[string]string, seedReadiness bool) readinessIsolationOutcome {
	t.Helper()
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	writePanel(t, root, panelSlug, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
	}, verdicts)

	if seedReadiness {
		readinessIsolationSeedMetadata(t, beadID, map[string]interface{}{
			bead.MetaKeyReadinessOverride: map[string]interface{}{
				"signals":    []string{"MF-1", "MF-2"},
				"overridden": "2026-07-24T00:00:00Z",
			},
			bead.MetaKeyReadinessAttempt: map[string]interface{}{
				"report": []map[string]interface{}{
					{"ordinal": 1, "signal": "SR-2", "reason": "a force-claimed NOT-READY bead's original reason"},
				},
				"clarifications": []map[string]interface{}{
					{"ordinal": 1, "reason": "a force-claimed NOT-READY bead's original reason", "answer": "resolved", "span": "spec.md §R1"},
				},
			},
		})
	}

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: readiness isolation fixture"})

	outcome := readinessIsolationOutcome{completeCalled: ex.completeCalled}
	if res != nil {
		outcome.beadClosed = res.BeadClosed
	}
	if err != nil {
		outcome.errMsg = err.Error()
	}
	return outcome
}

// TestReadinessIsolation_AllowScenario_ByteIdenticalWithAndWithoutReadinessState
// pins AC-9(ii) against an ALLOW (all-APPROVE, fresh SHA) panel: a
// force-claimed bead carrying the override marker + attempt record
// completes IDENTICALLY (same completeCalled/BeadClosed/error) to the
// same bead without them.
func TestReadinessIsolation_AllowScenario_ByteIdenticalWithAndWithoutReadinessState(t *testing.T) {
	const specID, beadID, slug = "124-riso-allow", "mindspec-124risoallow.1", "124-riso-allow-bd"
	without := runReadinessIsolationScenario(t, specID, beadID, slug, approveVerdicts(6), false)
	with := runReadinessIsolationScenario(t, specID, beadID, slug, approveVerdicts(6), true)

	if without.completeCalled != with.completeCalled || without.beadClosed != with.beadClosed || without.errMsg != with.errMsg {
		t.Errorf("readiness metadata changed the ALLOW-scenario gate outcome:\nwithout=%+v\nwith=%+v", without, with)
	}
	if !without.completeCalled || !without.beadClosed || without.errMsg != "" {
		t.Fatalf("baseline ALLOW scenario itself must complete+close cleanly (sanity check the fixture is a genuine pass): %+v", without)
	}
}

// TestReadinessIsolation_BlockScenario_ByteIdenticalWithAndWithoutReadinessState
// pins AC-9(ii)'s other half, and the ADR-0037 Touchpoint language
// directly ("a NOT-READY bead that was force-claimed and implemented
// anyway is judged by the panel exactly as today"): a sub-threshold
// panel BLOCKS identically whether or not the force-claimed bead carries
// readiness state.
func TestReadinessIsolation_BlockScenario_ByteIdenticalWithAndWithoutReadinessState(t *testing.T) {
	const specID, beadID, slug = "124-riso-block", "mindspec-124risoblock.2", "124-riso-block-bd"
	without := runReadinessIsolationScenario(t, specID, beadID, slug, subThresholdVerdicts(), false)
	with := runReadinessIsolationScenario(t, specID, beadID, slug, subThresholdVerdicts(), true)

	if without.completeCalled != with.completeCalled || without.beadClosed != with.beadClosed || without.errMsg != with.errMsg {
		t.Errorf("readiness metadata changed the BLOCK-scenario gate outcome:\nwithout=%+v\nwith=%+v", without, with)
	}
	if without.completeCalled || with.completeCalled {
		t.Errorf("expected BOTH scenarios to BLOCK pre-merge (CompleteBead must not run): without=%+v with=%+v", without, with)
	}
	if without.errMsg == "" || with.errMsg == "" {
		t.Fatalf("baseline BLOCK scenario itself must actually block (sanity check): without=%q with=%q", without.errMsg, with.errMsg)
	}
}

// TestReadinessIsolation_MetadataGenuinelyPersists is a sanity check on
// the seeding helper itself: it proves the readiness metadata really did
// land in the (fake) store the isolation scenarios ran against — so the
// "zero effect" conclusion above is not vacuous (i.e. not simply because
// the seed silently failed to apply).
func TestReadinessIsolation_MetadataGenuinelyPersists(t *testing.T) {
	const beadID = "mindspec-124risometa.1"
	readinessIsolationSeedMetadata(t, beadID, map[string]interface{}{
		bead.MetaKeyReadinessOverride: map[string]interface{}{"signals": []string{"MF-3"}},
	})
	got, err := completeGetMetadataFn(beadID)
	if err != nil {
		t.Fatalf("completeGetMetadataFn: %v", err)
	}
	raw, ok := got[bead.MetaKeyReadinessOverride]
	if !ok {
		t.Fatalf("expected the seeded override marker to be readable back, got %v", got)
	}
	rawJSON, _ := json.Marshal(raw)
	if string(rawJSON) == "" {
		t.Error("expected non-empty seeded marker JSON")
	}
}
