package next

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestS134Next_PreKernelSelectsRecordedIDs(t *testing.T) {
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	// A later local heading grants no identity to an unrecorded foreign child.
	text := kerneltest.EnrolmentPlan(f.SpecID, 4)
	text = strings.Replace(text, "status: Draft", "status: Approved\nbead_ids: ["+strings.Join(f.Items, ",")+"]", 1)
	f.Must(os.WriteFile(filepath.Join(f.SpecDir, "plan.md"), []byte(text), 0600))
	foreign := kernel.Record{ID: f.Epic + ".4", Parent: f.Epic, Title: "foreign task", Status: "open", Type: "task"}
	f.Fake.Records[foreign.ID] = foreign
	oldReady, oldFind := readyJSONFn, findEpicForBeadFn
	t.Cleanup(func() { readyJSONFn, findEpicForBeadFn = oldReady, oldFind })
	readyJSONFn = func(...string) ([]byte, error) {
		return json.Marshal([]BeadInfo{{ID: f.Items[0], Parent: f.Epic, Status: "open", IssueType: "task"}, {ID: foreign.ID, Parent: f.Epic, Status: "open", IssueType: "task"}})
	}
	findEpicForBeadFn = func(*phase.Cache, string) (string, string, phase.LineageProvenance, error) {
		return f.Epic, f.SpecID, phase.LineageProvenance{ParentChild: true}, nil
	}
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg == "--type=epic" {
				return json.Marshal([]kernel.Record{f.Fake.Records[f.Epic]})
			}
		}
		return f.Fake.Run(append([]string{"list"}, args...)...)
	}))
	before := kerneltest.Snapshot(t, f)
	rs, err := BuildReadySet(f.Root, f.SpecID)
	f.Must(err)
	if len(rs.Filtered) != 1 || rs.Filtered[0].ID != f.Items[0] {
		t.Fatalf("selected foreign child: %+v", rs.Filtered)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("selection wrote state")
	}
	empty := strings.Replace(text, "bead_ids: ["+strings.Join(f.Items, ",")+"]", "bead_ids: []", 1)
	f.Must(os.WriteFile(filepath.Join(f.SpecDir, "plan.md"), []byte(empty), 0600))
	rs, err = BuildReadySet(f.Root, f.SpecID)
	if err != nil || len(rs.Filtered) != 0 {
		t.Fatalf("empty recorded set selected work: %+v %v", rs.Filtered, err)
	}
}
