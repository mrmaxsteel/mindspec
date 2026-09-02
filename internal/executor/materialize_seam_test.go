package executor

// Spec 128 Bead 6: thin-wrap conformance for the eight prospective-
// snapshot / write-ahead materialization seams. Each MindspecExecutor
// method under test is a one-line pass-through to internal/gitutil; these
// tests exercise them against a real temp git repo so a regression that
// severs the wrap (e.g. a wrong argument order) is caught here rather than
// only inside internal/complete's higher-level fixtures.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

func runGitForMaterializeSeam(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func revParseForMaterializeSeam(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func initRepoForMaterializeSeam(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitForMaterializeSeam(t, dir, "init", "-q", "-b", "main")
	runGitForMaterializeSeam(t, dir, "config", "user.email", "test@test.com")
	runGitForMaterializeSeam(t, dir, "config", "user.name", "test")
	runGitForMaterializeSeam(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForMaterializeSeam(t, dir, "add", "-A")
	runGitForMaterializeSeam(t, dir, "commit", "-q", "-m", "base")
	return dir
}

func TestMindspecExecutor_SnapshotWorktreeTreeAndChangedFilesTree(t *testing.T) {
	dir := initRepoForMaterializeSeam(t)
	base := revParseForMaterializeSeam(t, dir, "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &MindspecExecutor{Root: dir}
	tree, err := g.SnapshotWorktreeTree(dir, nil)
	if err != nil {
		t.Fatalf("SnapshotWorktreeTree: %v", err)
	}
	files, err := g.ChangedFilesTree(dir, base, tree)
	if err != nil {
		t.Fatalf("ChangedFilesTree: %v", err)
	}
	var found bool
	for _, f := range files {
		if f == "new.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected new.go in changed files, got %v", files)
	}
}

func TestMindspecExecutor_MaterializeStageCommitPicksSubjectByStage(t *testing.T) {
	dir := initRepoForMaterializeSeam(t)
	parent := revParseForMaterializeSeam(t, dir, "HEAD")
	tree := revParseForMaterializeSeam(t, dir, "HEAD^{tree}")

	g := &MindspecExecutor{Root: dir}
	c, err := g.MaterializeStageCommit(dir, tree, parent, gitutil.StageMaterialization, "")
	if err != nil {
		t.Fatalf("MaterializeStageCommit: %v", err)
	}
	stage, present, err := gitutil.ReadStageTrailer(dir, c)
	if err != nil || !present || stage != gitutil.StageMaterialization {
		t.Fatalf("expected stage=%q present=true, got stage=%q present=%v err=%v", gitutil.StageMaterialization, stage, present, err)
	}
	body, err := gitutil.CommitMessageBody(dir, c)
	if err != nil {
		t.Fatalf("CommitMessageBody: %v", err)
	}
	if !strings.Contains(body, "mindspec: materialize bead completion") {
		t.Errorf("expected the default materialization subject on an empty subject arg, got %q", body)
	}

	d, err := g.MaterializeStageCommit(dir, tree, c, gitutil.StageArtifactSync, "")
	if err != nil {
		t.Fatalf("MaterializeStageCommit (artifact-sync): %v", err)
	}
	dBody, err := gitutil.CommitMessageBody(dir, d)
	if err != nil {
		t.Fatalf("CommitMessageBody: %v", err)
	}
	if !strings.Contains(dBody, "chore: sync beads artifact") {
		t.Errorf("expected the default artifact-sync subject on an empty subject arg, got %q", dBody)
	}
}

// TestMindspecExecutor_MaterializeStageCommitPreservesCallerSubject is
// fix-round G6's seam-level pin: a non-empty subject passed by the
// caller (internal/complete, carrying the operator's --commit-msg text)
// survives verbatim into the commit — the executor's own fixed default
// is used ONLY as a fallback for an empty subject, never overriding one.
func TestMindspecExecutor_MaterializeStageCommitPreservesCallerSubject(t *testing.T) {
	dir := initRepoForMaterializeSeam(t)
	parent := revParseForMaterializeSeam(t, dir, "HEAD")
	tree := revParseForMaterializeSeam(t, dir, "HEAD^{tree}")

	g := &MindspecExecutor{Root: dir}
	callerSubject := "impl(mindspec-seam.1): a real operator message"
	c, err := g.MaterializeStageCommit(dir, tree, parent, gitutil.StageMaterialization, callerSubject)
	if err != nil {
		t.Fatalf("MaterializeStageCommit: %v", err)
	}
	body, err := gitutil.CommitMessageBody(dir, c)
	if err != nil {
		t.Fatalf("CommitMessageBody: %v", err)
	}
	if !strings.Contains(body, callerSubject) {
		t.Errorf("expected the caller's own subject to survive verbatim, got %q", body)
	}
	if strings.Contains(body, "mindspec: materialize bead completion") {
		t.Errorf("the fixed default subject must NOT appear alongside a caller-supplied one, got %q", body)
	}
}

func TestMindspecExecutor_StageMarkerAndBranchAdvanceRoundTrip(t *testing.T) {
	dir := initRepoForMaterializeSeam(t)
	a := revParseForMaterializeSeam(t, dir, "HEAD")
	runGitForMaterializeSeam(t, dir, "checkout", "-q", "-b", "bead/mindspec-seam.1")

	g := &MindspecExecutor{Root: dir}
	tree := revParseForMaterializeSeam(t, dir, "HEAD^{tree}")
	c, err := g.MaterializeStageCommit(dir, tree, a, gitutil.StageMaterialization, "")
	if err != nil {
		t.Fatalf("MaterializeStageCommit: %v", err)
	}

	beadID := "mindspec-seam.1"
	if err := g.PublishStageMarker(beadID, c, ""); err != nil {
		t.Fatalf("PublishStageMarker: %v", err)
	}
	oid, stage, present, err := g.ReadStageMarker(beadID)
	if err != nil || !present || oid != c || stage != gitutil.StageMaterialization {
		t.Fatalf("ReadStageMarker: oid=%q stage=%q present=%v err=%v (want oid=%s stage=%s)", oid, stage, present, err, c, gitutil.StageMaterialization)
	}

	if err := g.AdvanceBranchExpectedOld("bead/mindspec-seam.1", c, a); err != nil {
		t.Fatalf("AdvanceBranchExpectedOld: %v", err)
	}
	if got := revParseForMaterializeSeam(t, dir, "bead/mindspec-seam.1"); got != c {
		t.Errorf("expected branch to advance to %s, got %s", c, got)
	}

	if err := g.ResetIndexToHead(dir); err != nil {
		t.Fatalf("ResetIndexToHead: %v", err)
	}
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected a clean status after ResetIndexToHead, got %q", out)
	}

	if err := g.ClearStageMarker(beadID, c); err != nil {
		t.Fatalf("ClearStageMarker: %v", err)
	}
	_, _, present, err = g.ReadStageMarker(beadID)
	if err != nil || present {
		t.Errorf("expected the marker to be cleared, present=%v err=%v", present, err)
	}
}
