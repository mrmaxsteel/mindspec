package complete

// Spec 120 R4 cluster 2 (round-5 fix-up): uncoveredPendingObligations and
// CheckPendingObligations receive beadID from the impl-approve gate's
// planBeadIDs (approve.readPlanBeadIDs reads the AGENT-AUTHORED plan.md
// bead_ids YAML frontmatter) — NEVER idvalidate'd. The existing
// panel_advisory_hostile_test.go R3(g) subtests already pin a hostile
// SLOT; these tests pin the beadID parameter itself: every fmt.Errorf
// DISPLAY position in uncoveredPendingObligations/CheckPendingObligations
// must render idrender.Bead(beadID) (forced strconv.Quote for a
// malformed-but-printable value), while the FUNCTIONAL getMeta(beadID)
// metadata read keeps consuming the raw beadID unchanged.

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// hostileObligationBeadID mirrors the idrender_test.go "120-x;evil"
// discriminator: printable ASCII (termsafe.Escape alone is a no-op on
// it), malformed per idvalidate.BeadID's grammar, so only idrender.Bead's
// idvalidate-keyed identity forces it to quote.
const hostileObligationBeadID = "mindspec-x;evil"

func TestCheckPendingObligations_HostileBeadID_MetadataErrorForcedQuoted(t *testing.T) {
	var sawFunctionalRawID bool
	getMeta := func(id string) (map[string]interface{}, error) {
		if id == hostileObligationBeadID {
			sawFunctionalRawID = true
		}
		return nil, errors.New("metadata store unavailable")
	}

	err := CheckPendingObligations(hostileObligationBeadID, getMeta)
	if err == nil {
		t.Fatal("expected a fail-closed error on a metadata read failure")
	}
	if !sawFunctionalRawID {
		t.Fatal("getMeta (the functional metadata read) must receive the raw beadID unchanged")
	}
	msg := err.Error()
	wantQuoted := strconv.Quote(hostileObligationBeadID)
	want := "bead " + wantQuoted + " metadata could not be read"
	if !strings.Contains(msg, want) {
		t.Errorf("metadata-read-error message must render the forced-quoted beadID, got:\n%s\nwant substring: %s", msg, want)
	}
}

func TestCheckPendingObligations_HostileBeadID_UnresolvedObligationForcedQuoted(t *testing.T) {
	getMeta := func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"refutation_pending_entries": []refutationPendingEntry{
				{Slot: "X", Round: 2},
			},
		}, nil
	}

	err := CheckPendingObligations(hostileObligationBeadID, getMeta)
	if err == nil {
		t.Fatal("expected an error naming the uncovered obligation")
	}
	msg := err.Error()
	wantQuoted := strconv.Quote(hostileObligationBeadID)
	want := "bead " + wantQuoted + " carries an unresolved refutation_pending obligation (X@round 2)"
	if !strings.Contains(msg, want) {
		t.Errorf("unresolved-obligation message must render the forced-quoted beadID, got:\n%s\nwant substring: %s", msg, want)
	}
}

// TestCheckPendingObligations_CleanBeadID_ByteIdentical is the
// clean-fixture counterpart (F3 discipline). This mirrors the existing
// "R3(g) clean fixture" subtest in panel_advisory_hostile_test.go, pinned
// standalone here alongside its hostile sibling above.
func TestCheckPendingObligations_CleanBeadID_ByteIdentical(t *testing.T) {
	const cleanID = "mindspec-9cyu.1"
	getMeta := func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"refutation_pending_entries": []refutationPendingEntry{
				{Slot: "X", Round: 2},
			},
		}, nil
	}

	err := CheckPendingObligations(cleanID, getMeta)
	if err == nil {
		t.Fatal("expected an error naming the uncovered obligation")
	}
	want := "bead " + cleanID + " carries an unresolved refutation_pending obligation (X@round 2) not yet covered by a durable panel_refuted record"
	if got := err.Error(); got != want {
		t.Errorf("clean-fixture literal changed:\ngot:  %s\nwant: %s", got, want)
	}
}
