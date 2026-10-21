package panel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSubstitutionRecord_Validate(t *testing.T) {
	good := SubstitutionRecord{Record: RecordSubstitution, Spec: "test", Panel: "p", Slot: "R1", Round: 1, FromFamily: "source", ToFamily: "target", Model: "m", CreatedAt: ledgerTime}
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"valid", "", nil},
		{"round zero", "round", 0}, {"round negative", "round", -1},
		{"round string", "round", "1"}, {"round fraction", "round", 1.5},
		{"from empty", "from_family", ""}, {"from blank", "from_family", " "},
		{"from type", "from_family", 1}, {"from null", "from_family", nil},
		{"to empty", "to_family", ""}, {"to blank", "to_family", " "},
		{"to type", "to_family", false}, {"to null", "to_family", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(good)
			if err != nil {
				t.Fatal(err)
			}
			if tc.field != "" {
				var m map[string]any
				if err := json.Unmarshal(raw, &m); err != nil {
					t.Fatal(err)
				}
				m[tc.field] = tc.value
				raw, err = json.Marshal(m)
				if err != nil {
					t.Fatal(err)
				}
			}
			err = Validate(raw)
			if tc.field == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("want %s refusal, got %v", tc.field, err)
			}
		})
	}
}
