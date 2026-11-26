package panel

import (
	"errors"
	"strings"
	"testing"
)

// A record source that cannot read the stored slot verdicts blocks the gate
// and names the tracker failure, not an unreadable panel.json. No disk file
// is tallied in its place.
func TestResolveGateFacts_RecordSourceFailureNamesTracker(t *testing.T) {
	dir := t.TempDir()
	reg := Registration{Dir: dir}
	f := ResolveGateFacts(reg, "", dir, GateIO{Records: func(string) (map[string][]byte, error) {
		return nil, errors.New("tracker unavailable")
	}})
	if f.Res == nil || f.Res.RecordsErr == nil || f.Res.PanelErr == nil {
		t.Fatalf("record source failure not kept fail-closed: %+v", f.Res)
	}
	d := PanelGateDecision(f)
	if d.Action != Block {
		t.Fatalf("record source failure did not block: %+v", d)
	}
	if !strings.Contains(d.Message, "tracker unavailable") || !strings.Contains(d.Message, "tracker store") || strings.Contains(d.Message, "panel.json") {
		t.Fatalf("decision does not name the tracker failure: %q", d.Message)
	}
}
