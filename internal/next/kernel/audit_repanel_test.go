package kernel_test

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

// TestS134Audit_RePanelCountsEachJudgement: the judge writes its scope on the new
// gate attempt and the lens step copies it. One judgement is one re-panel.
func TestS134Audit_RePanelCountsEachJudgement(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	reduce(t, f, true)
	attempt, lens, other := gate+".a2", gate+".r3.lens", gate+".r4.lens"
	for _, id := range []string{attempt, lens, other} {
		f.Create(id, gate)
	}
	f.Must(f.Store.WriteMetadata(attempt, kernel.Metadata(kernel.KeyRePanel, kernel.RePanel{Judge: "judge-1", SlotCount: 3})))
	f.Must(f.Store.WriteMetadata(lens, kernel.Metadata(kernel.KeyRePanel, kernel.RePanel{Judge: "judge-1", SlotCount: 3})))
	f.Must(f.Store.WriteMetadata(other, kernel.Metadata(kernel.KeyRePanel, kernel.RePanel{Judge: "judge-2", SlotCount: 2})))
	rows, err := kernel.Audit(f.State(), nil)
	f.Must(err)
	for _, row := range rows {
		if row.Event != "re-panels" {
			continue
		}
		if row.Count != 2 || len(row.Details) != 2 || row.Details[0] != attempt+": 3 slots" || row.Details[1] != other+": 2 slots" {
			t.Fatalf("re-panels: %+v", row)
		}
		return
	}
	t.Fatal("no re-panels row")
}
