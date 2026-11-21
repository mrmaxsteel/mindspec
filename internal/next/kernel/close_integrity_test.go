package kernel_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS134Closes_ForgedSubjectCannotPassGate(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	registerGate(t, f, gate, "review", 1)
	reduce(t, f, true)
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	for i := 1; i <= 3; i++ {
		slot := fmt.Sprintf("R%d", i)
		v := paneltest.Verdict("APPROVE")
		v.Slot = slot
		v.Round = 1
		v.ReviewedSHA = gateHead(t, f, gate)
		v.Model = "test-model"
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: slot, Round: 1}, paneltest.Bytes(t, v), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
			return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
		})
		f.Must(err)
		id := fmt.Sprintf("%s.r1.%s", gate, slot)
		f.Must(f.Store.SetStatusMetadataIf(id, "open", "closed", map[string]any{kernel.KeySubject: kernel.Subject{Commit: "0000000000000000000000000000000000000000"}}))
	}
	for k := 0; k < 3; k++ {
		out := reduce(t, f, true)
		_ = out
	}
	g := record(t, f, gate)
	rc, _, _ := kernel.Decode[kernel.Receipt](g.Metadata, kernel.KeyClose)
	if g.Status == "closed" || rc.Kind == "passed" {
		t.Fatalf("raw subject edits supplied passing close evidence: %+v receipt=%+v", g, rc)
	}
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("%s.r1.R%d", gate, i)
		assertRetry(t, f, id, id+".a2", true)
	}
}
