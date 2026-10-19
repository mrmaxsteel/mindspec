package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const ignoredReviewPath = ".mindspec/specs/001-x/reviews/a/panel.json"

func reviewIgnoreRepo(t *testing.T) string {
	t.Helper()
	// Keep local git fixtures independent of the operator's configuration.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	t.Setenv("GIT_AUTHOR_NAME", "spec129 test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@mindspec.local")
	t.Setenv("GIT_COMMITTER_NAME", "spec129 test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@mindspec.local")
	root := t.TempDir()
	reviewIgnoreGit(t, root, "init", "-q")
	if err := EnsureGitignoreEntriesWithHeader(root, "# local panel reviews", ReviewIgnoreEntries...); err != nil {
		t.Fatal(err)
	}
	reviewIgnoreGit(t, root, "add", ".gitignore")
	reviewIgnoreGit(t, root, "commit", "-q", "-m", "base")
	path := filepath.Join(root, ignoredReviewPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"bead_id":"mindspec-test.1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func reviewIgnoreGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestCommitAll_SkipsIgnoredReviews(t *testing.T) {
	root := reviewIgnoreRepo(t)
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("commit this\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(root, "auto commit"); err != nil {
		t.Fatal(err)
	}
	if got := reviewIgnoreGit(t, root, "ls-files", "--", ignoredReviewPath); got != "" {
		t.Errorf("CommitAll tracked an ignored review: %s", got)
	}
	if got := reviewIgnoreGit(t, root, "show", "HEAD:source.txt"); got != "commit this\n" {
		t.Errorf("CommitAll did not commit ordinary work: %q", got)
	}
	if got := reviewIgnoreGit(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("ignored review must leave a clean status: %s", got)
	}
}

func TestSnapshotWorktreeTree_SkipsIgnoredReviews(t *testing.T) {
	root := reviewIgnoreRepo(t)
	// With only an ignored review, neither the live index nor status changes.
	tree, err := SnapshotWorktreeTree(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := reviewIgnoreGit(t, root, "ls-tree", "-r", "--name-only", tree, "--", ignoredReviewPath); got != "" {
		t.Errorf("snapshot contains an ignored review: %s", got)
	}
	if got := reviewIgnoreGit(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("snapshot must leave a clean status: %s", got)
	}
	// Pin real staging too: an ordinary untracked file must enter the snapshot.
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("snapshot this\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err = SnapshotWorktreeTree(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := reviewIgnoreGit(t, root, "show", tree+":source.txt"); got != "snapshot this\n" {
		t.Errorf("snapshot lost ordinary work: %q", got)
	}
	if got := reviewIgnoreGit(t, root, "ls-tree", "-r", "--name-only", tree); strings.Contains(got, ignoredReviewPath) {
		t.Errorf("snapshot with ordinary work contains an ignored review: %s", got)
	}
}
