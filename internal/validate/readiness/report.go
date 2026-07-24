// Package readiness implements spec 124 (impl-readiness-gate)'s
// deterministic mechanical floor: four signals (MF-1..MF-4) evaluated
// against a bead's plan section, bd description, and bd dependency edges,
// exposed as EvaluateReadiness. It lives in its own sub-package —
// internal/validate/readiness, NOT internal/validate itself — because the
// engine must consume internal/lifecycle.FindLandedMerge (MF-3), and
// internal/lifecycle's white-box test imports internal/validate; placing
// the engine directly in internal/validate would close the cycle
// `lifecycle[test] -> validate -> lifecycle` and break
// `go test ./internal/lifecycle/...`. The sub-package is a leaf edge
// lifecycle's own tests never traverse.
//
// report.go declares the ReadinessReport shape and its renderer as a
// reachable, exported API so spec 124 Bead 2's `next` gate-before-mutate
// refusal reuses the SAME rendering `bead ready-check` prints (ADR-0040:
// no gate logic is restated).
package readiness

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// Signal IDs, stable across releases (spec 124 R1: "a stable signal ID").
const (
	SignalPlanSection  = "MF-1"
	SignalTokens       = "MF-2"
	SignalDependencies = "MF-3"
	SignalBlocking     = "MF-4"
)

// Signal is one mechanical signal's evaluated outcome.
type Signal struct {
	// ID is one of the SignalXxx constants above.
	ID string
	// Pass reports whether this signal passed.
	Pass bool
	// Detail is the evidence text: for a FAIL, the offending or missing
	// element plus its evidence path (spec 124 R1); empty or a short
	// confirmation for a PASS. Detail is agent-influenced (it quotes bead
	// descriptions and plan prose) and is escaped by Render, never by the
	// caller.
	Detail string
	// Recovery is the one actionable, copy-pastable lever named for a
	// FAILing signal (spec 124 ADR-0035 Touchpoint: "one recovery: line
	// per failing signal"). Empty when Pass is true. Recovery text is
	// operator-authored (constructed by this package, never from raw bd/
	// plan text) so it is never escaped and must never embed agent-
	// controlled bytes.
	Recovery string
}

// Report is the full per-bead readiness evaluation: exactly the four
// MF-1..MF-4 signals, in stable order.
type Report struct {
	// BeadID is the evaluated bead's ID (already idvalidate-clean by the
	// time EvaluateReadiness constructs a Report).
	BeadID string
	// Signals is always exactly four entries, ordered MF-1, MF-2, MF-3,
	// MF-4 — AllPass/FailingSignals/Render all rely on this invariant.
	Signals []Signal
}

// AllPass reports whether every signal in r passed (spec 124 R1: "Exit 0
// when all four pass").
func (r *Report) AllPass() bool {
	for _, s := range r.Signals {
		if !s.Pass {
			return false
		}
	}
	return true
}

// FailingSignals returns the signals that did not pass, in signal order.
func (r *Report) FailingSignals() []Signal {
	var out []Signal
	for _, s := range r.Signals {
		if !s.Pass {
			out = append(out, s)
		}
	}
	return out
}

// RecoveryCommands returns one recovery command per failing signal, in
// signal order — the exact slice callers pass to guard.NewFailure's
// variadic commands (spec 124 R1: "on any FAIL, exit non-zero via
// internal/guard with one recovery: line per failing signal").
func (r *Report) RecoveryCommands() []string {
	var out []string
	for _, s := range r.FailingSignals() {
		out = append(out, s.Recovery)
	}
	return out
}

// Render renders r as the per-signal report text: one line per signal,
// PASS/FAIL plus (for a FAIL) the escaped evidence. Every byte of
// agent-influenced text (bead descriptions, plan prose) reaches this
// output only through termsafe.Escape — spec 124 R1 / AC-8: "All
// bead/plan-derived text rendered in the report passes through
// termsafe.Escape before reaching the terminal."
//
// This is the SOLE renderer for the mechanical-floor report: `bead
// ready-check` (Bead 1) and `next`'s gate-before-mutate refusal (Bead 2)
// both call it, so the report format never drifts between the two call
// sites (ADR-0040 no-restate).
func Render(r *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Readiness report for %s:\n", idrender.Bead(r.BeadID))
	for _, s := range r.Signals {
		if s.Pass {
			fmt.Fprintf(&b, "%s: PASS\n", s.ID)
			continue
		}
		fmt.Fprintf(&b, "%s: FAIL — %s\n", s.ID, termsafe.Escape(s.Detail))
	}
	return strings.TrimRight(b.String(), "\n")
}
