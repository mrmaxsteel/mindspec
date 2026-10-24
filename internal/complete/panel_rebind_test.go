package complete

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestComplete_CopiesRebinds(t *testing.T) {
	for _, variant := range []string{"success", "write failure", "hatch", "abandoned", "retry union"} {
		t.Run(variant, func(t *testing.T) {
			const specID, beadID = "130-rebind", "mindspec-rebind.1"
			root, sha := setupPanelGateRepo(t, specID, beadID)
			store := newFakeMetadataStore()
			store.wire(t)
			entries := []panel.Rebind{{Round: 1, FromSHA: strings.Repeat("a", 40), ToSHA: sha, Paths: []string{"source.go"}, Reason: "max: checked", CreatedAt: "2026-10-20T00:00:00Z"}}
			p := panel.Panel{BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha, Rebinds: entries}
			if variant == "abandoned" {
				p.Abandoned = true
				p.AbandonReason = "max: dropped"
			}
			writePanel(t, root, "rebound", p, approveVerdicts(6))
			if variant == "hatch" {
				previous := panelSkipEnvFn
				t.Cleanup(func() { panelSkipEnvFn = previous })
				panelSkipEnvFn = func() bool { return true }
			}
			if variant == "write failure" {
				store.failMerge = failOnKey("panel_rebound_entries")
			}
			var old panel.Rebind
			if variant == "retry union" {
				old = entries[0]
				old.FromSHA = strings.Repeat("b", 40)
				store.data[beadID] = map[string]interface{}{"panel_rebound_entries": []panel.Rebind{old, entries[0]}}
			}
			closed := false
			closeBeadFn = func(...string) error { closed = true; return nil }
			ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
			_, err := Run(root, beadID, specID, "", ex, CompleteOpts{DocOwner: "fixture-doc-owner", AllowDocSkew: "test: fixture"})
			if variant == "write failure" {
				if err == nil || closed || ex.completeCalled {
					t.Fatalf("audit failure must refuse before close/merge: %v closed=%v merged=%v", err, closed, ex.completeCalled)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !closed || !ex.completeCalled {
				t.Fatal("completion did not close and merge")
			}
			raw := store.data[beadID]["panel_rebound_entries"]
			if variant == "hatch" || variant == "abandoned" {
				if raw != nil {
					t.Fatalf("Warn copied rebinds: %v", raw)
				}
				return
			}
			data, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			var got []panel.Rebind
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			want := entries
			if variant == "retry union" {
				want = []panel.Rebind{old, entries[0]}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("panel_rebound_entries = %+v, want %+v", got, want)
			}
		})
	}
}
