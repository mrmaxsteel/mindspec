package harness

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

// requireValidBeadID is the shared harness id-gate (spec 120 R7, round
// 9/10): every harness call site that builds a `bd`-wrapper invocation
// with a bead/spec/epic id-position operand routes the id through this
// helper BEFORE any subprocess spawn. A malformed id fails the test
// fast, before `bd` ever runs — the same discipline
// gitutil.RejectOptionLike gives the harness's dynamic git operands
// (see Sandbox.BranchExists/ListBranches), applied to the id class
// instead of the option-injection class.
//
// This gates scenario-authored test-scaffolding ids, not an
// agent-writable runtime vector — but per the round-9
// no-provenance-exemption rule, every id-position operand is gated
// uniformly regardless of who authored it, so the R6(g) tree-wide scan
// has zero un-classified sites.
func requireValidBeadID(t testing.TB, id string) {
	t.Helper()
	if err := idvalidate.BeadID(id); err != nil {
		t.Fatalf("harness: invalid bead ID %q: %v", id, err)
	}
}

// requireValidSpecID is requireValidBeadID's spec-id sibling, for
// caller sites whose id-position operand is a spec id rather than a
// bead id.
func requireValidSpecID(t testing.TB, id string) {
	t.Helper()
	if err := idvalidate.SpecID(id); err != nil {
		t.Fatalf("harness: invalid spec ID %q: %v", id, err)
	}
}
