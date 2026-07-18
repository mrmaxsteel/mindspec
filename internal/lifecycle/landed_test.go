package lifecycle

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// initLandedRepo builds a real throwaway git repo with a spec/<id> branch
// forked from main — the fixture shape FindLandedMerge/MergedUnclosed
// operate over. run is a t.Helper closure for issuing further git commands
// against the same repo.
func initLandedRepo(t *testing.T, specID string) (dir string, run func(args ...string)) {
	t.Helper()
	dir = t.TempDir()
	run = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")
	run("checkout", "-b", "spec/"+specID)
	return dir, run
}

// mergeBead creates bead/<id> off the current HEAD, commits one change, and
// merges it (--no-ff, the deterministic gitutil.MergeInto message) back into
// whatever branch is currently checked out.
func mergeBead(t *testing.T, run func(args ...string), dir, beadID, specBranch string) {
	t.Helper()
	run("checkout", "-b", "bead/"+beadID)
	os.WriteFile(filepath.Join(dir, beadID+".txt"), []byte(beadID+"\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work "+beadID)
	run("checkout", specBranch)
	run("merge", "--no-ff", "-m", "Merge bead/"+beadID, "bead/"+beadID)
}

func TestFindLandedMerge_Identified(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")

	landed, err := FindLandedMerge(dir, "spec/119-test", "one")
	if err != nil {
		t.Fatalf("FindLandedMerge: %v", err)
	}
	if len(landed.SHA) < 7 || len(landed.FirstParent) < 7 || len(landed.SecondParent) < 7 {
		t.Fatalf("expected populated SHAs, got %+v", landed)
	}
}

// TestFindLandedMerge_FreshBranchNotFound pins the AC-10 load-bearing
// property: a freshly-claimed bead branch (zero own commits, trivially an
// ancestor of the spec branch) produces no merge commit at all — so
// FindLandedMerge correctly reports ErrLandedMergeNotFound, never a false
// positive.
func TestFindLandedMerge_FreshBranchNotFound(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/fresh")
	run("checkout", "spec/119-test")

	_, err := FindLandedMerge(dir, "spec/119-test", "fresh")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound, got %v", err)
	}
}

// TestFindLandedMerge_NoBranchAtAllNotFound: a bead that was never even
// branched (no bead/<id> ref ever existed, no merge commit) also reports
// ErrLandedMergeNotFound.
func TestFindLandedMerge_NoBranchAtAllNotFound(t *testing.T) {
	dir, _ := initLandedRepo(t, "119-test")

	_, err := FindLandedMerge(dir, "spec/119-test", "never-existed")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound, got %v", err)
	}
}

// TestFindLandedMerge_BranchDeletedAfterMerge: the common recovery
// scenario — the bead branch was merged AND then deleted (mimicking
// CompleteBead's best-effort branch cleanup). FindLandedMerge must still
// identify the merge from the spec branch's own history alone.
func TestFindLandedMerge_BranchDeletedAfterMerge(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")
	run("branch", "-D", "bead/one")

	landed, err := FindLandedMerge(dir, "spec/119-test", "one")
	if err != nil {
		t.Fatalf("FindLandedMerge after branch deletion: %v", err)
	}
	if landed.SHA == "" {
		t.Fatal("expected a populated merge SHA")
	}
}

// TestFindLandedMerge_ReviewedHeadSHACorroborates: a registered panel whose
// reviewed_head_sha equals the merge's second parent corroborates the
// match (still identified).
func TestFindLandedMerge_ReviewedHeadSHACorroborates(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")

	landed, err := FindLandedMerge(dir, "spec/119-test", "one")
	if err != nil {
		t.Fatalf("baseline FindLandedMerge: %v", err)
	}

	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })
	beadID := "one"
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{
			Dir: "/fake/review/one",
			Panel: panel.Panel{
				BeadID:          &beadID,
				ReviewedHeadSHA: landed.SecondParent,
			},
		}}
	}

	got, err := FindLandedMerge(dir, "spec/119-test", "one")
	if err != nil {
		t.Fatalf("FindLandedMerge with corroborating panel: %v", err)
	}
	if got.SHA != landed.SHA {
		t.Errorf("SHA = %q, want %q", got.SHA, landed.SHA)
	}
}

// TestFindLandedMerge_ReviewedHeadSHAContradicts: a registered panel whose
// reviewed_head_sha is neither equal to nor an ancestor of the merge's
// second parent CONTRADICTS the match — not a positive identification.
func TestFindLandedMerge_ReviewedHeadSHAContradicts(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")

	// A second, UNRELATED branch/commit — neither ancestor nor descendant
	// of bead/one's tip — stands in for a contradicting reviewed_head_sha.
	run("checkout", "main")
	run("checkout", "-b", "unrelated")
	os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x\n"), 0644)
	run("add", ".")
	run("commit", "-m", "unrelated work")
	cmd := exec.Command("git", "-C", dir, "rev-parse", "unrelated")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse unrelated: %v", err)
	}
	unrelatedSHA := string(out)
	if len(unrelatedSHA) > 0 && unrelatedSHA[len(unrelatedSHA)-1] == '\n' {
		unrelatedSHA = unrelatedSHA[:len(unrelatedSHA)-1]
	}

	origScan := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = origScan })
	beadID := "one"
	landedPanelScanFn = func(roots ...string) []panel.Registration {
		return []panel.Registration{{
			Dir: "/fake/review/one",
			Panel: panel.Panel{
				BeadID:          &beadID,
				ReviewedHeadSHA: unrelatedSHA,
			},
		}}
	}

	_, err = FindLandedMerge(dir, "spec/119-test", "one")
	if !errors.Is(err, ErrLandedMergeNotFound) {
		t.Fatalf("expected ErrLandedMergeNotFound on contradiction, got %v", err)
	}
}

func TestMergedUnclosed_BranchDeleted(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")
	run("branch", "-D", "bead/one")

	landed, ok, err := MergedUnclosed(dir, "spec/119-test", "one")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if !ok {
		t.Fatal("expected merged-unclosed to be true")
	}
	if landed == nil || landed.SHA == "" {
		t.Fatalf("expected populated landed evidence, got %+v", landed)
	}
}

func TestMergedUnclosed_BranchSurvivesAsAncestor(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")
	// bead/one still exists but carries no NEW commits beyond the merge —
	// benign merged-but-undeleted.

	_, ok, err := MergedUnclosed(dir, "spec/119-test", "one")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if !ok {
		t.Fatal("expected merged-unclosed to be true for a merged, surviving, non-diverged branch")
	}
}

// TestMergedUnclosed_BranchDivergedNotFlagged pins the anti-false-flag
// requirement: if bead/<id> carries NEW commits landed AFTER the identified
// merge (real still-unmerged work), it must NOT be reported merged-unclosed.
func TestMergedUnclosed_BranchDivergedNotFlagged(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	mergeBead(t, run, dir, "one", "spec/119-test")

	run("checkout", "bead/one")
	os.WriteFile(filepath.Join(dir, "one-more.txt"), []byte("more\n"), 0644)
	run("add", ".")
	run("commit", "-m", "more work not yet merged")
	run("checkout", "spec/119-test")

	_, ok, err := MergedUnclosed(dir, "spec/119-test", "one")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if ok {
		t.Fatal("a branch carrying new unlanded commits must NOT be flagged merged-unclosed")
	}
}

func TestMergedUnclosed_NoLandedMerge(t *testing.T) {
	dir, run := initLandedRepo(t, "119-test")
	run("checkout", "-b", "bead/fresh")
	run("checkout", "spec/119-test")

	_, ok, err := MergedUnclosed(dir, "spec/119-test", "fresh")
	if err != nil {
		t.Fatalf("MergedUnclosed: %v", err)
	}
	if ok {
		t.Fatal("expected merged-unclosed to be false when no landed merge exists")
	}
}
