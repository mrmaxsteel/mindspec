package kernel_test

import (
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Submit_RefreshesPublicationState(t *testing.T) {
	f, id, lease, subject := slotFixture(t)
	stale := f.State()
	gate := f.Items[0] + ".review"
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "current", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
	artifact := filepath.Join(f.SpecDir, "reviews", "current", "verdict.json")
	relative, err := filepath.Rel(f.Root, artifact)
	f.Must(err)
	actorCommit(t, f, relative, verdictBytes(subject, "R1"), "")
	called := false
	_, err = kernel.Submit(stale, kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: artifact, BeforeClose: func(fresh kernel.State, receipt kernel.Receipt) error {
		called = true
		if len(fresh.Panels) != 1 || fresh.Panels[0].Slug() != "current" {
			t.Fatal("publication received the panel state from before the lock")
		}
		row := actorRecord(t, f, id)
		_, closed, decodeErr := kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
		f.Must(decodeErr)
		if row.Status != "in_progress" || closed {
			t.Fatal("publication ran after receipt or close")
		}
		// The hook state holds the stored verdict that the receipt names.
		_, err := kernel.ReceiptBytes(fresh.Records[id], receipt)
		return err
	}})
	f.Must(err)
	if !called || actorRecord(t, f, id).Status != "closed" {
		t.Fatal("publication did not close the submitted slot")
	}
}
