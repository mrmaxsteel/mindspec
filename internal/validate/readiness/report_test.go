package readiness

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestRender_MF3NonMember(t *testing.T) {
	const spec, current = "996-membership", "owner-epic.1"
	origParent, origLanded := fetchDependencyParentFn, findLandedMergeFn
	t.Cleanup(func() { fetchDependencyParentFn, findLandedMergeFn = origParent, origLanded })
	fetchDependencyParentFn = func(string) (string, error) { return "other-epic", nil }
	findLandedMergeFn = func(string, string, string) (*lifecycle.LandedMerge, error) {
		t.Fatal("a non-member must pass on closed status alone")
		return nil, nil
	}
	r := &Report{BeadID: current, EpicID: "owner-epic", SpecID: spec}
	s := evaluateMF3("", r, "", []dependencyEdge{{ID: "plain-task", Status: "closed"}})
	if !s.Pass {
		t.Fatalf("AC-10 leg a must pass: %+v", s)
	}
	for _, id := range []string{"plain-task", "plain\x1b[31m-task", "plain\x1b[31m-task\nforged"} {
		t.Run(id, func(t *testing.T) {
			confirmation := s
			// Hostile IDs are rejected by evaluation; inject at the render
			// boundary to pin escaping in the dependency-ID position too.
			confirmation.Detail = strings.ReplaceAll(s.Detail, "plain-task", id)
			r.Signals = []Signal{
				{ID: SignalPlanSection, Pass: true},
				{ID: SignalTokens, Pass: true},
				confirmation,
				{ID: SignalBlocking, Pass: true},
			}
			output := Render(r)
			lines := strings.Split(output, "\n")
			want := "MF-3: PASS — " + termsafe.Escape("dependency "+id+": closed; not a bead of spec "+spec+" — landed-merge test not applicable")
			if len(lines) != 5 || lines[3] != want || strings.ContainsRune(output, '\x1b') {
				t.Fatalf("non-member PASS must render its escaped sentence on one MF-3 line: %q; want %q", output, want)
			}
			if lines[1] != "MF-1: PASS" || lines[2] != "MF-2: PASS" || lines[4] != "MF-4: PASS" {
				t.Fatalf("empty passing Details must keep their existing output: %q", output)
			}
		})
	}
}
