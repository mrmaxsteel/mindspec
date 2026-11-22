package approve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// TestAdoptSpec_IgnoredExportSkipsExportStage: a repo that git-ignores
// .beads/issues.jsonl and does not track it keeps the beads database as
// the store of record. Adopt must not export or try to commit the file,
// and must still record that its export stage is done.
func TestAdoptSpec_IgnoredExportSkipsExportStage(t *testing.T) {
	dir := adoptInitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".beads/issues.jsonl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adoptGitRun(t, dir, "add", ".gitignore")
	adoptGitRun(t, dir, "commit", "-qm", "ignore export")
	makeLandedBeadBranch(t, dir, "test-b1")
	stubAdoptListEpicBeads(t, []adoptEpicBead{{ID: "test-b1", Status: "closed"}})
	closedCalls, metadata := wireAdoptSeams(t, "epic-1")
	exports := adoptStubExport(t)

	preTip := adoptRefHash(t, dir, "main")
	exec := &realAdoptCommitExecutor{MockExecutor: &executor.MockExecutor{BeadsExportPublishedFn: gitutil.BeadsExportPublished}, t: t}

	result, err := AdoptSpec(dir, "042-test", exec, AdoptOpts{Reason: "landed via an external GH merge"})
	if err != nil {
		t.Fatalf("AdoptSpec: unexpected error: %v", err)
	}
	if !result.Verified || *closedCalls != 1 {
		t.Fatalf("adopt did not finish: verified=%v closes=%d", result.Verified, *closedCalls)
	}
	if *exports != 0 {
		t.Fatalf("export ran %d times in a repo that ignores it", *exports)
	}
	if exec.calls != 0 {
		t.Fatalf("CommitPaths ran %d times in a repo that ignores the export", exec.calls)
	}
	if n := adoptCommitCount(t, dir, preTip, "main"); n != 0 {
		t.Fatalf("adopt made %d commits; want none", n)
	}
	if metadata[adoptExportCommittedMetaKey] != true {
		t.Fatal("export stage completion not recorded")
	}
}
