package kernel_test

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

// TestS134Audit_LeaseKeepsRawClaim drives the real lease writer over a tracker
// claim. The audit row must survive the close of the step.
func TestS134Audit_LeaseKeepsRawClaim(t *testing.T) {
	f, id, deps := actorFixture(t)
	f.Must(f.Store.SetStatusIf(id, "open", "in_progress"))
	leaseActor(t, f, id, deps)
	claims, ok, err := kernel.Decode[[]kernel.RawClaim](f.State().Records[id].Metadata, kernel.KeyRawClaims)
	if err != nil || !ok || len(claims) != 1 || !claims[0].At.Equal(fixedTime) {
		t.Fatalf("lease dropped the raw claim: %+v %v %v", claims, ok, err)
	}
	// A take of the held lease adds no claim.
	if _, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, deps); err != nil {
		t.Fatal(err)
	}
	claims, _, err = kernel.Decode[[]kernel.RawClaim](f.State().Records[id].Metadata, kernel.KeyRawClaims)
	if err != nil || len(claims) != 1 {
		t.Fatalf("take added a raw claim: %+v %v", claims, err)
	}
	rows, err := kernel.Audit(f.State(), nil)
	f.Must(err)
	for _, row := range rows {
		if row.Event == "raw-claims" && row.Count != 1 {
			t.Fatalf("raw-claims=%d", row.Count)
		}
	}
}
