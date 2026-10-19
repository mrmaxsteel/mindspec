package bead

import "testing"

func TestPlanBeadIndex(t *testing.T) {
	for _, tc := range []struct {
		epic, id string
		want     int
	}{
		{"mindspec-e", "mindspec-e.1", 1},
		{"mindspec-e", "mindspec-e.12", 12},
		{"mindspec-e", "mindspec-e.0", 0},
		{"mindspec-e", "mindspec-e.01", 0},
		{"mindspec-e", "mindspec-e.1.2", 0},
		{"mindspec-e", "mindspec-other.1", 0},
		{"mindspec-e", "mindspec-ex.1", 0},
		{"mindspec-e", "mindspec-e.", 0},
		{"mindspec-e", "mindspec-e.+1", 0},
		{"mindspec-e", "mindspec-e. 1", 0},
		{"mindspec-e", "mindspec-e.999999999999999999999999999999", 0},
		{"", ".1", 0},
	} {
		t.Run(tc.epic+"/"+tc.id, func(t *testing.T) {
			n, ok := PlanBeadIndex(tc.epic, tc.id)
			if n != tc.want || ok != (tc.want > 0) {
				t.Fatalf("PlanBeadIndex(%q, %q)=(%d,%v), want (%d,%v)", tc.epic, tc.id, n, ok, tc.want, tc.want > 0)
			}
		})
	}
}
