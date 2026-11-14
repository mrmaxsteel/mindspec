package panel

import (
	"encoding/json"
	"testing"
)

func TestCoverageManifestUsesSubmittedModel(t *testing.T) {
	target, data, _ := submitFixture(t)
	var v VerdictV2
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	v.Model = "substituted-model"
	seedTestRecords(t, target.Dir, SubstitutionRecord{Record: RecordSubstitution, Spec: "test", Panel: "p", Slot: "R1", Round: 1, FromFamily: "harness", ToFamily: "replacement", Model: v.Model, CreatedAt: ledgerTime})
	resolve := func(Panel, RosterSlot) (SubmitResolution, error) {
		return SubmitResolution{Family: "replacement", Model: v.Model, RunnerKind: "harness", RunnerSource: "default"}, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Submit(target, raw, resolve); err != nil {
		t.Fatal(err)
	}
	res, err := Tally(target.Dir)
	if err != nil {
		t.Fatal(err)
	}
	manifest := DeriveCoverageManifest(res)
	if len(manifest.Slots) != 1 || manifest.Slots[0].Model != v.Model {
		t.Fatalf("manifest must name submitted model %s: %+v", v.Model, manifest)
	}
}
