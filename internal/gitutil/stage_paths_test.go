package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitPaths_TrackedIgnoredReview(t *testing.T) {
	root := reviewIgnoreRepo(t)
	// Seed an already-tracked review, as in a repository predating R8.
	reviewIgnoreGit(t, root, "add", "-f", "--", ignoredReviewPath)
	reviewIgnoreGit(t, root, "commit", "-q", "-m", "existing tracked review")
	if err := os.WriteFile(filepath.Join(root, ignoredReviewPath), []byte("updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitPaths(root, "update tracked review", []string{ignoredReviewPath}); err != nil {
		t.Fatalf("CommitPaths tracked ignored review: %v", err)
	}
	if got := reviewIgnoreGit(t, root, "show", "HEAD:"+ignoredReviewPath); got != "updated\n" {
		t.Fatalf("tracked review update not committed: %q", got)
	}
}

func TestCommitPaths_RefusesUntrackedIgnoredReview(t *testing.T) {
	root := reviewIgnoreRepo(t)
	head := reviewIgnoreGit(t, root, "rev-parse", "HEAD")
	if err := CommitPaths(root, "must not force review", []string{ignoredReviewPath}); err == nil {
		t.Fatal("explicit untracked ignored review must be refused")
	}
	if got := reviewIgnoreGit(t, root, "ls-files", "--", ignoredReviewPath); got != "" {
		t.Fatalf("force-added an untracked ignored review: %s", got)
	}
	if got := reviewIgnoreGit(t, root, "rev-parse", "HEAD"); got != head {
		t.Fatal("refused ignored review changed HEAD")
	}
}

func TestCommitPaths_DirectoryPreservesAdditionsAndScope(t *testing.T) {
	root := reviewIgnoreRepo(t)
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("src/tracked.txt", "before\n")
	write("outside.txt", "before\n")
	reviewIgnoreGit(t, root, "add", "src", "outside.txt")
	reviewIgnoreGit(t, root, "commit", "-q", "-m", "source files")
	write("src/tracked.txt", "after\n")
	write("src/new [file].txt", "new\n")
	write("outside.txt", "unrelated staged change\n")
	reviewIgnoreGit(t, root, "add", "outside.txt")
	if err := CommitPaths(root, "source changes only", []string{"src"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"src/tracked.txt": "after\n", "src/new [file].txt": "new\n", "outside.txt": "before\n"} {
		if got := reviewIgnoreGit(t, root, "show", "HEAD:"+path); got != want {
			t.Errorf("committed %s = %q, want %q", path, got, want)
		}
	}
	if got := reviewIgnoreGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(got) != "outside.txt" {
		t.Fatalf("unrelated staged change was not preserved: %s", got)
	}
	if got := reviewIgnoreGit(t, root, "ls-files", "--", ignoredReviewPath); got != "" {
		t.Fatalf("directory commit tracked ignored review: %s", got)
	}
}
