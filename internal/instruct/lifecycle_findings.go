// Spec 119 Bead 2 (R7), final-review F1: wires the stale-OPEN,
// finalize-orphan, and stale-tracker doctor checks into the generated
// `mindspec instruct` idle guidance through the SAME exported aggregate
// scan internal/doctor consumes (lifecycle.ScanIntegrityFindings) — never
// a private re-derivation, and never the per-spec-dir bd fan-out the
// original wiring performed (measured at minutes-per-SessionStart on a
// ~118-spec repo). P8 (SHARED-PREDICATE VISIBILITY): the aggregate only
// enumerates; the finding conditions and wording live in the single-home
// internal/lifecycle predicates, so `mindspec doctor` and `mindspec
// instruct` can never disagree on either the trigger or the text
// (AC-12/AC-15).
package instruct

import (
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// instructScanIntegrityFindingsFn is the aggregate-scan seam (mirroring
// internal/doctor's scanIntegrityFindingsFn) so collectLifecycleFindings
// is unit-testable without a live repo or `bd`. Pinned by the AC-12
// anti-drift test (lifecycle_findings_test.go) to be the SAME exported
// symbol internal/doctor's checkLifecycleIntegrity invokes.
var instructScanIntegrityFindingsFn = lifecycle.ScanIntegrityFindings

// collectLifecycleFindings runs the shared cache-aware aggregate scan and
// renders each finding as ONE verbatim line via the finding's own
// Message()/FullMessage() (message + recovery command combined by the SAME
// template `mindspec doctor` renders from — no second copy of the text),
// in the same order doctor reports them. The EXISTING per-invocation
// phase.Cache is threaded in (F1) — no fresh cache, so the epic list is
// commonly already loaded by the idle-mode resolution that preceded this
// call. Best-effort and read-only: any predicate error yields fewer
// findings, never a hard failure of `mindspec instruct`.
func collectLifecycleFindings(root string, cache *phase.Cache) []string {
	findings := instructScanIntegrityFindingsFn(root, cache)

	var out []string
	for _, o := range findings.FinalizeBranches {
		out = append(out, o.FullMessage())
	}
	for _, s := range findings.StaleOpen {
		out = append(out, s.Message())
	}
	for _, o := range findings.StaleTrackers {
		out = append(out, o.FullMessage())
	}
	return out
}
