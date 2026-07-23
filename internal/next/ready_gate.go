package next

// ready_gate.go wires spec 124 (impl-readiness-gate) Bead 1's mechanical
// readiness floor into `mindspec next`'s claim path — the gate-before-
// mutate preflight leg ADR-0041's fourth-verb clause names (spec 124 R3 /
// R9 / AC-4 / AC-16): GateReadiness runs after bead selection and BEFORE
// any claim/branch/worktree mutation; RecordReadinessOverride writes the
// durable `--allow-not-ready` marker AFTER a claim succeeds and BEFORE the
// worktree is created (the write-ordering choice recorded in the spec 124
// plan preamble: a claim lost to contention leaves no stray marker).

import (
	"fmt"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

// Package-level function variables for testability (the same
// func-var-seam convention beads.go/guard.go already use in this
// package) — tests swap these to exercise GateReadiness/
// RecordReadinessOverride without a real bd process or a real
// internal/validate/readiness engine evaluation.
var (
	evaluateReadinessFn = readiness.EvaluateReadiness
	mergeMetadataFn     = bead.MergeMetadata
)

// GateResult is GateReadiness's outcome when the gate does not refuse.
type GateResult struct {
	// FailingSignals lists the MF-x signal IDs (e.g. "MF-2") that failed
	// evaluation, in signal order. Empty when the floor fully passed;
	// non-empty ONLY when the caller passed allowNotReady=true and the
	// gate is proceeding past a refusal the operator deliberately
	// bypassed.
	FailingSignals []string
}

// GateReadiness evaluates spec 124's mechanical readiness floor
// (internal/validate/readiness.EvaluateReadiness, Bead 1's MF-1..MF-4
// engine) for beadID against root. Callers MUST invoke this immediately
// after bead selection and BEFORE any lifecycle mutation (no
// bd update --claim, no bead/<id> branch, no worktree) — the ADR-0041
// "preflight-leg-only addition" this function's call site cites.
//
// On a failing floor with allowNotReady=false, it returns a nil result and
// a guard-shaped refusal error: the SAME per-signal rendering
// `mindspec bead ready-check` prints (readiness.Render — ADR-0040
// no-restate, never re-implemented here) plus every failing signal's own
// recovery line, plus the two escape hatches R3 names (the standalone
// `bead ready-check` report, and re-running with `--allow-not-ready`).
//
// On a failing floor with allowNotReady=true, it returns a nil error and a
// GateResult naming every failing signal — the caller emits the R3
// stderr warning and, once ClaimBead succeeds, calls
// RecordReadinessOverride with this same list.
//
// On a passing floor it returns a nil error and an empty GateResult — no
// interactive step, no output beyond the caller's existing claim lines
// (R3: "adds no interactive step and no output beyond a single OK line").
func GateReadiness(root, beadID string, allowNotReady bool) (*GateResult, error) {
	report, err := evaluateReadinessFn(root, beadID)
	if err != nil {
		return nil, fmt.Errorf("evaluating readiness for %s: %w", idrender.Bead(beadID), err)
	}

	if report.AllPass() {
		return &GateResult{}, nil
	}

	failing := report.FailingSignals()
	ids := make([]string, 0, len(failing))
	for _, s := range failing {
		ids = append(ids, s.ID)
	}

	if allowNotReady {
		return &GateResult{FailingSignals: ids}, nil
	}

	recovery := append([]string{}, report.RecoveryCommands()...)
	recovery = append(recovery,
		fmt.Sprintf("mindspec bead ready-check %s   (the standalone per-signal report)", idrender.Bead(beadID)),
		"re-run this exact `mindspec next` invocation with --allow-not-ready to claim deliberately despite the failing signal(s) above",
	)
	return nil, guard.NewFailure(readiness.Render(report), recovery...)
}

// RecordReadinessOverride writes the durable `--allow-not-ready` override
// marker (spec 124 R3 / AC-4) naming the bypassed mechanical signals plus
// a UTC timestamp, via the existing bead.MergeMetadata helper (no bd
// schema change, ADR-0023-advisory) under bead.MetaKeyReadinessOverride.
//
// AC-4 makes the marker a GUARANTEE, not best-effort: `--allow-not-ready`
// success ⟹ (marker durably written AND bead claimed). Callers MUST
// therefore invoke this on the override-proceed path BEFORE ClaimBead and
// FAIL-CLOSED on its error — refusing the whole command (nothing claimed,
// no worktree) rather than claiming without the durable marker. Writing
// advisory metadata to the not-yet-claimed bead is safe: the bead exists
// in bd regardless of claim status, and a plain refusal (no
// --allow-not-ready) never reaches this call, so AC-4's
// refusal-zero-mutation audit stays intact. This function itself never
// swallows the write error — it returns it for the caller to fail on.
func RecordReadinessOverride(beadID string, signals []string) error {
	return mergeMetadataFn(beadID, map[string]interface{}{
		bead.MetaKeyReadinessOverride: map[string]interface{}{
			"signals":    signals,
			"overridden": time.Now().UTC().Format(time.RFC3339),
		},
	})
}
