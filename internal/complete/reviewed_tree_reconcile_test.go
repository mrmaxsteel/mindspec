package complete

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestFindLandedMerge_ReviewedTreeConfirms(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	fakeBdOnPath(t)
	const specID, beadID = "132-reviewed", "probe-reviewed.1"
	root, reviewed := setupPanelGateRepo(t, specID, beadID)
	gitRun(t, root, "checkout", "bead/"+beadID)
	writeFile(t, root, ".mindspec/specs/"+specID+"/reviews/round-one/dispositions.jsonl", "{}\n")
	writeFile(t, root, ".beads/issues.jsonl", "{}\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-m", "record tool artifacts")
	second := gateRevParse(t, root, "HEAD")
	gitRun(t, root, "checkout", "spec/"+specID)
	gitRun(t, root, "merge", "--no-ff", "-m", "Merge bead/"+beadID, "bead/"+beadID)
	merge := gateRevParse(t, root, "HEAD")
	gitRun(t, root, "branch", "-D", "bead/"+beadID)
	gitRun(t, root, "checkout", "main")
	writeFile(t, root, ".mindspec/specs/"+specID+"/spec.md", "# Fixture\n")
	writePanel(t, root, "reviewed-tree", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 2,
		ReviewedHeadSHA: reviewed,
	}, approveVerdicts(2))
	var identified *lifecycle.LandedMerge
	mergedUnclosedFn = func(root, branch, id string) (*lifecycle.LandedMerge, bool, error) {
		landed, ok, err := lifecycle.MergedUnclosed(root, branch, id)
		identified = landed
		return landed, ok, err
	}
	var recorded string
	completeMergeMetadataFn = func(_ string, updates map[string]interface{}) error {
		if value, ok := updates["mindspec_reconcile_landed_merge_sha"].(string); ok {
			recorded = value
		}
		return nil
	}
	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "fixture has no domain docs"})
	if err != nil || res == nil || !res.BeadClosed {
		t.Fatalf("branch-less reviewed-tree reconcile must close: result=%+v err=%v", res, err)
	}
	if ex.completeCalled || recorded != merge {
		t.Fatalf("reconcile must record the existing merge: merged=%v recorded=%q want=%s", ex.completeCalled, recorded, merge)
	}
	if identified == nil || identified.SecondParent != second {
		t.Fatalf("landed identity on reconcile path: %+v", identified)
	}
	if !strings.Contains(identified.Provenance, "reviewed-tree") {
		t.Fatalf("confirmation rule missing: %q", identified.Provenance)
	}
}
