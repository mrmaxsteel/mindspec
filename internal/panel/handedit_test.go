package panel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func snapshotFromRound(t *testing.T, raw []byte) Panel {
	t.Helper()
	var p Panel
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHandEdit_FieldMatcher(t *testing.T) {
	records := ledgerFixtures(t, "p")
	baseline := snapshotFromRound(t, records[0])
	cases := []struct {
		name string
		edit func(*Panel)
	}{
		{"round", func(p *Panel) { p.Round++ }},
		{"roster", func(p *Panel) { p.Roster = []RosterSlot{{"R1", "changed", "lens"}, {"R2", "model-b", "architecture"}} }},
		{"gate", func(p *Panel) { p.Gate = "adhoc" }},
		{"target", func(p *Panel) { p.Target = "main" }},
		{"bead_id", func(p *Panel) { id := "different"; p.BeadID = &id }},
		{"expected_reviewers", func(p *Panel) { p.ExpectedReviewers++ }},
		{"telemetry", func(p *Panel) { p.Telemetry = "" }},
		{"telemetry", func(p *Panel) { p.Telemetry = "optional" }},
		{"reviewed_head_sha", func(p *Panel) { p.ReviewedHeadSHA = "changed" }},
		{"refutations[0]", func(p *Panel) { p.Refutations = []Refutation{{Slot: "R1", Round: 1, Reason: "manual"}} }},
		{"deferrals[0]", func(p *Panel) { p.Deferrals = []Deferral{{Slot: "R1", Round: 1, Bead: "manual"}} }},
		{"rebinds[0]", func(p *Panel) { p.Rebinds = []Rebind{{Round: 1, FromSHA: "manual", ToSHA: "manual"}} }},
		{"abandoned", func(p *Panel) { p.Abandoned = true }},
		{"abandon_reason", func(p *Panel) { p.AbandonReason = "manual" }},
	}
	if field, err := FirstUnbackedField(baseline, records[:1]); err != nil || field != "" {
		t.Fatalf("baseline: %q %v", field, err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := baseline
			tc.edit(&p)
			field, err := FirstUnbackedField(p, records[:1])
			if err != nil || field != tc.name {
				t.Fatalf("field=%q want=%q err=%v", field, tc.name, err)
			}
		})
	}
	// The later rebind sets the SHA; an older rebind cannot override a new round.
	p := baseline
	p.ReviewedHeadSHA = "def456"
	if field, err := FirstUnbackedField(p, records[:4]); err != nil || field != "" {
		t.Fatalf("rebind SHA: %q %v", field, err)
	}
	next := changeLedger(t, records[0], "round", 2)
	p = baseline
	p.Round = 2
	if field, err := FirstUnbackedField(p, append(records[:4:4], next)); err != nil || field != "" {
		t.Fatalf("new round SHA: %q %v", field, err)
	}
}

func TestHandEdit_EntryPayloadsAndAbandonment(t *testing.T) {
	records := ledgerFixtures(t, "p")
	p := snapshotFromRound(t, records[0])
	var ref Refutation
	var def Deferral
	var reb Rebind
	for _, item := range []struct {
		raw  []byte
		dest interface{}
	}{{records[1], &ref}, {records[2], &def}, {records[3], &reb}} {
		if err := json.Unmarshal(item.raw, item.dest); err != nil {
			t.Fatal(err)
		}
	}
	p.Refutations = []Refutation{ref}
	p.Deferrals = []Deferral{def}
	p.Rebinds = []Rebind{reb}
	p.ReviewedHeadSHA = reb.ToSHA
	p.Abandoned = true
	p.AbandonReason = "halt"
	if field, err := FirstUnbackedField(p, records); err != nil || field != "" {
		t.Fatalf("backed snapshot: %q %v", field, err)
	}
	p.Refutations[0].CreatedAt = ledgerLater
	p.Deferrals[0].CreatedAt = ledgerLater
	p.Rebinds[0].CreatedAt = ledgerLater
	if field, err := FirstUnbackedField(p, records); err != nil || field != "" {
		t.Fatalf("timestamp rerun: %q %v", field, err)
	}
	baseline := mustMarshal(t, p)
	for _, tc := range []struct {
		name  string
		entry func(*Panel) reflect.Value
	}{
		{"refutations[0]", func(p *Panel) reflect.Value { return reflect.ValueOf(&p.Refutations[0]).Elem() }},
		{"deferrals[0]", func(p *Panel) reflect.Value { return reflect.ValueOf(&p.Deferrals[0]).Elem() }},
		{"rebinds[0]", func(p *Panel) reflect.Value { return reflect.ValueOf(&p.Rebinds[0]).Elem() }},
	} {
		entryType := tc.entry(&p).Type()
		for i := 0; i < entryType.NumField(); i++ {
			field := entryType.Field(i)
			if !field.IsExported() || field.Name == "CreatedAt" {
				continue
			}
			t.Run(tc.name+"/"+field.Name, func(t *testing.T) {
				edited := snapshotFromRound(t, baseline)
				value := tc.entry(&edited).Field(i)
				switch value.Kind() {
				case reflect.String:
					value.SetString(value.String() + "-changed")
				case reflect.Int:
					value.SetInt(value.Int() + 1)
				case reflect.Slice:
					if value.Type().Elem().Kind() != reflect.String {
						t.Fatalf("unhandled slice type for %s: %s", field.Name, value.Type())
					}
					changed := reflect.MakeSlice(value.Type(), value.Len()+1, value.Len()+1)
					reflect.Copy(changed, value)
					changed.Index(value.Len()).SetString("changed")
					value.Set(changed)
				default:
					t.Fatalf("unhandled kind for %s: %s", field.Name, value.Kind())
				}
				if field, err := FirstUnbackedField(edited, records); err != nil || field != tc.name {
					t.Fatalf("field=%q want=%q err=%v", field, tc.name, err)
				}
			})
		}
	}
	p.Abandoned = false
	if field, err := FirstUnbackedField(p, records); err != nil || field != "abandoned" {
		t.Fatalf("hand-cleared abandonment: %q %v", field, err)
	}
}

func TestHandEdit_LegacyMarkersAndCarried(t *testing.T) {
	for _, tc := range []struct {
		name      string
		telemetry bool
		round     bool
		invalid   bool
	}{{"legacy", false, false, false}, {"modern", true, true, false}, {"missing-telemetry", false, true, true}, {"missing-round", true, false, true}, {"empty-telemetry", true, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "p")
			records := ledgerFixtures(t, "p")
			p := snapshotFromRound(t, records[0])
			if !tc.telemetry {
				p.Telemetry = ""
			}
			if err := WriteSnapshot(dir, p); err != nil {
				t.Fatal(err)
			}
			if tc.name == "empty-telemetry" {
				if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"telemetry":""}`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.round {
				if err := AppendRecords(dir, records[0]); err != nil {
					t.Fatal(err)
				}
			}
			legacy, err := LegacyPanel(dir)
			if (err != nil) != tc.invalid {
				t.Fatalf("marker error=%v want invalid=%v", err, tc.invalid)
			}
			if !tc.invalid && legacy == tc.telemetry {
				t.Fatalf("wrong legacy classification: %v", legacy)
			}
			if tc.name == "legacy" {
				p.Refutations = []Refutation{{Slot: "R1", Round: 1}}
				if field, err := CheckHandEdits(dir, p); err != nil || field != "" {
					t.Fatalf("legacy not exempt: %q %v", field, err)
				}
			}
		})
	}
	records := ledgerFixtures(t, "p")
	legacy := Refutation{Slot: "R1", Round: 1, Reason: "legacy", Evidence: "commit"}
	round := changeLedger(t, records[0], "carried", CarriedEntries{Refutations: []Refutation{legacy}})
	p := snapshotFromRound(t, round)
	p.Refutations = []Refutation{legacy}
	if field, err := FirstUnbackedField(p, [][]byte{round}); err != nil || field != "" {
		t.Fatalf("carried baseline rejected: %q %v", field, err)
	}
	p.Refutations[0].Evidence = "edited"
	if field, err := FirstUnbackedField(p, [][]byte{round}); err != nil || field != "refutations[0]" {
		t.Fatalf("changed carried baseline accepted: %q %v", field, err)
	}
}

func TestHandEdit_UnparseableRegistrationRefuses(t *testing.T) {
	for _, data := range []string{"", "{"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LegacyPanel(dir); err == nil {
			t.Fatalf("malformed registration %q treated as legacy", data)
		}
	}
}

func TestHandEdit_NewerRoundClearsAbandonment(t *testing.T) {
	records := ledgerFixtures(t, "p")
	next := changeLedger(t, records[0], "round", 2)
	next = changeLedger(t, next, "created_at", ledgerLater)
	p := snapshotFromRound(t, next)
	lines := [][]byte{records[0], records[4], next}
	if field, err := FirstUnbackedField(p, lines); err != nil || field != "" {
		t.Fatalf("new round must expect abandoned=false: field=%q err=%v", field, err)
	}
	p.Abandoned = true
	p.AbandonReason = "halt"
	if field, err := FirstUnbackedField(p, lines); err != nil || field != "abandoned" {
		t.Fatalf("stale abandonment: field=%q want=abandoned err=%v", field, err)
	}
}
