package lifecycle

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestReviewedHeadSHASet_IncludesRebindFrom(t *testing.T) {
	for _, variant := range []string{"event", "carried", "hand changed", "unbacked", "prefix only"} {
		t.Run(variant, func(t *testing.T) {
			root, run := initLandedRepo(t, "119-test")
			mergeBead(t, run, root, "bead-one", "spec/119-test")
			from := revParseIn(t, root, "bead/bead-one")
			run("branch", "-D", "bead/bead-one")
			dir := filepath.Join(root, "review", "rebound")
			to := strings.Repeat("a", 40)
			digest := "sha256:" + strings.Repeat("b", 64)
			beadID := "bead-one"
			if err := panel.Create(dir, panel.CreateInput{Spec: "119-test", BeadID: &beadID, Gate: "bead", Target: "bead/bead-one", Round: 1, ExpectedReviewers: 2,
				ReviewedHeadSHA: from, SubjectDigest: digest, Roster: []panel.RosterSlot{{Slot: "R1", Model: "codex", Lens: "a"}, {Slot: "R2", Model: "claude", Lens: "b"}}}); err != nil {
				t.Fatal(err)
			}
			entry := panel.Rebind{Round: 1, FromSHA: from, ToSHA: to, Paths: []string{"source.go"}, Reason: "max: checked", CreatedAt: "2026-10-20T00:00:00Z"}
			event, _ := json.Marshal(panel.RebindRecord{Record: panel.RecordRebind, Spec: "119-test", Panel: "rebound", SubjectDigest: digest, Rebind: entry})
			if variant != "unbacked" {
				if err := panel.AppendRecords(dir, event); err != nil {
					t.Fatal(err)
				}
			}
			res, err := panel.Tally(dir)
			if err != nil || res.Panel == nil {
				t.Fatalf("fixture: %v", err)
			}
			p := *res.Panel
			p.ReviewedHeadSHA = to
			p.SubjectDigest = digest
			p.Rebinds = []panel.Rebind{entry}
			if variant == "hand changed" {
				p.Rebinds[0].FromSHA = strings.Repeat("c", 40)
			}
			if err := panel.WriteSnapshot(dir, p); err != nil {
				t.Fatal(err)
			}
			if variant == "carried" {
				if err := panel.Create(dir, panel.CreateInput{Spec: "119-test", BeadID: &beadID, Gate: "bead", Target: "bead/bead-one", Round: 2, ExpectedReviewers: 2, ReviewedHeadSHA: to, SubjectDigest: digest}); err != nil {
					t.Fatal(err)
				}
			}
			oldScan := landedPanelScanFn
			oldMerges := firstParentMergesFn
			t.Cleanup(func() { landedPanelScanFn = oldScan; firstParentMergesFn = oldMerges })
			landedPanelScanFn = func(...string) []panel.Registration { return panel.Scan(root) }
			set := reviewedHeadSHAForBead(root, "spec/119-test", beadID)
			_, hasFrom := set[from]
			if want := variant != "hand changed" && variant != "unbacked"; hasFrom != want {
				t.Fatalf("from SHA membership = %v, want %v: %v", hasFrom, want, set)
			}
			if variant == "hand changed" {
				if _, ok := set[p.Rebinds[0].FromSHA]; ok {
					t.Fatal("hand-changed from_sha entered set")
				}
			}
			for _, prefix := range []string{from[:7], from[:39]} {
				if _, ok := set[prefix]; ok {
					t.Fatalf("prefix %s entered exact set", prefix)
				}
			}
			if variant == "prefix only" {
				// Keep the real merge/tree, but ask the corroborator about an abbreviated
				// parent. A prefix is not exact evidence even when git could resolve it.
				firstParentMergesFn = func(workdir, ref string) ([]gitutil.MergeCommit, error) {
					merges, err := oldMerges(workdir, ref)
					for i := range merges {
						merges[i].Parents[1] = from[:7]
					}
					return merges, err
				}
			}
			got, err := FindLandedMerge(root, "spec/119-test", beadID)
			if variant == "hand changed" || variant == "unbacked" || variant == "prefix only" {
				if !errors.Is(err, ErrLandedMergeNotFound) {
					t.Fatalf("non-exact/unbacked rebind confirmed merge: %+v %v", got, err)
				}
			} else if err != nil || got == nil || got.SecondParent != from {
				t.Fatalf("matched rebind did not corroborate exact parent: %+v %v", got, err)
			}
		})
	}
}
