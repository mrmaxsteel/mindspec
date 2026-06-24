package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initExecRepo creates a configured git repo and returns a MindspecExecutor
// rooted at it.
func initExecRepo(t *testing.T) (string, *MindspecExecutor) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir, NewMindspecExecutor(dir)
}

// TestMindspecExecutor_MoverPrimitives exercises the net-new mover primitives
// surfaced on the Executor interface (spec 106 Bead 3, R4 blocker 2) against a
// real temp git repo, confirming they route to gitutil and behave correctly.
func TestMindspecExecutor_MoverPrimitives(t *testing.T) {
	repo, ex := initExecRepo(t)

	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ex.CommitPaths(repo, "add src/f.txt", []string{"src/f.txt"}); err != nil {
		t.Fatalf("CommitPaths: %v", err)
	}
	head, err := ex.RevParseRef(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// GitMv + commit the rename.
	if err := ex.GitMv(repo, "src", "dst"); err != nil {
		t.Fatalf("GitMv: %v", err)
	}
	if err := ex.CommitPaths(repo, "move src -> dst", nil); err != nil {
		t.Fatalf("CommitPaths rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "dst", "f.txt")); err != nil {
		t.Errorf("dst/f.txt missing: %v", err)
	}

	// ResetHard + CleanForce roll back to the pre-move commit.
	if err := ex.ResetHard(repo, head); err != nil {
		t.Fatalf("ResetHard: %v", err)
	}
	if err := ex.CleanForce(repo); err != nil {
		t.Fatalf("CleanForce: %v", err)
	}
	if now, _ := ex.RevParseRef(repo, "HEAD"); now != head {
		t.Errorf("ResetHard did not restore HEAD: %s != %s", now, head)
	}
	if _, err := os.Stat(filepath.Join(repo, "src", "f.txt")); err != nil {
		t.Errorf("src/f.txt should be restored: %v", err)
	}

	// Ref-discovery primitives.
	locals, err := ex.LocalBranchRefs(repo)
	if err != nil {
		t.Fatalf("LocalBranchRefs: %v", err)
	}
	found := false
	for _, b := range locals {
		if b == "main" {
			found = true
		}
	}
	if !found {
		t.Errorf("LocalBranchRefs missing main: %v", locals)
	}
	if _, err := ex.RemoteTrackingRefs(repo); err != nil {
		t.Fatalf("RemoteTrackingRefs: %v", err)
	}
}
