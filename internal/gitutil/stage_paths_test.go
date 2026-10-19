package gitutil

import (
	"os"
	"os/exec"
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
	} else if !strings.Contains(err.Error(), "ignored by one of your .gitignore files") {
		t.Fatalf("refusal must preserve git stderr: %v", err)
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
	write("unstaged.txt", "before\n")
	reviewIgnoreGit(t, root, "add", "src", "outside.txt", "unstaged.txt")
	reviewIgnoreGit(t, root, "commit", "-q", "-m", "source files")
	write("src/tracked.txt", "after\n")
	write("src/new [file].txt", "new\n")
	write("src/ü-note.md", "non-ASCII addition\n")
	write("outside.txt", "unrelated staged change\n")
	write("unstaged.txt", "unrelated unstaged change\n")
	reviewIgnoreGit(t, root, "add", "outside.txt")
	if err := CommitPaths(root, "source changes only", []string{"src"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"src/tracked.txt": "after\n", "src/new [file].txt": "new\n", "src/ü-note.md": "non-ASCII addition\n", "outside.txt": "before\n"} {
		if got := reviewIgnoreGit(t, root, "show", "HEAD:"+path); got != want {
			t.Errorf("committed %s = %q, want %q", path, got, want)
		}
	}
	if got := reviewIgnoreGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(got) != "outside.txt" {
		t.Fatalf("unrelated staged change was not preserved: %s", got)
	}
	if got := reviewIgnoreGit(t, root, "diff", "--name-only"); strings.TrimSpace(got) != "unstaged.txt" {
		t.Fatalf("out-of-pathspec tracked modification must remain unstaged: %s", got)
	}

	// A tracked review directory must accept edits to existing reviews while
	// leaving new ignored siblings out of the index.
	reviewDir := filepath.Dir(ignoredReviewPath)
	reviewIgnoreGit(t, root, "add", "-f", "--", ignoredReviewPath)
	reviewIgnoreGit(t, root, "commit", "-q", "-m", "existing review", "--", ignoredReviewPath)
	write(ignoredReviewPath, "updated review\n")
	sibling := filepath.Join(reviewDir, "sibling.json")
	write(sibling, "ignored sibling\n")
	reviewIgnoreGit(t, root, "check-ignore", "-q", "--", sibling)
	if err := CommitPaths(root, "existing reviews only", []string{reviewDir}); err != nil {
		t.Fatalf("CommitPaths review directory with ignored sibling: %v", err)
	}
	if got := reviewIgnoreGit(t, root, "show", "HEAD:"+ignoredReviewPath); got != "updated review\n" {
		t.Fatalf("directory commit lost tracked review update: %q", got)
	}
	if got := reviewIgnoreGit(t, root, "ls-files", "--", sibling); got != "" {
		t.Fatalf("directory commit tracked ignored sibling review: %s", got)
	}
}

func TestCommitPaths_GitTrace(t *testing.T) {
	for _, path := range []string{"src/tracked.txt", "src"} {
		t.Run(path, func(t *testing.T) {
			root := reviewIgnoreRepo(t)
			if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
				t.Fatal(err)
			}
			tracked := filepath.Join(root, "src/tracked.txt")
			if err := os.WriteFile(tracked, []byte("before\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			reviewIgnoreGit(t, root, "add", "src")
			reviewIgnoreGit(t, root, "commit", "-q", "-m", "tracked source")
			if err := os.WriteFile(tracked, []byte("after\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "src/new.txt"), []byte("new\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Run("trace", func(t *testing.T) {
				t.Setenv("GIT_TRACE", "1")
				if err := CommitPaths(root, "commit with trace enabled", []string{path}); err != nil {
					t.Fatalf("CommitPaths(%q) with GIT_TRACE=1: %v", path, err)
				}
			})
			if got := reviewIgnoreGit(t, root, "show", "HEAD:src/tracked.txt"); got != "after\n" {
				t.Errorf("trace commit lost tracked update: %q", got)
			}
			if path == "src" {
				if got := reviewIgnoreGit(t, root, "show", "HEAD:src/new.txt"); got != "new\n" {
					t.Errorf("trace directory commit lost addition: %q", got)
				}
			}
		})
	}
}

func TestStageExplicitPaths_LiteralUntrackedPaths(t *testing.T) {
	root := reviewIgnoreRepo(t)
	// Git pathspec magic makes the distinction observable even on git versions
	// that prefer an exact filename match before trying ordinary glob syntax.
	const literal, other = ":(glob)ab.txt", "ab.txt"
	if err := os.WriteFile(filepath.Join(root, other), []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewIgnoreGit(t, root, "add", "--", other)
	reviewIgnoreGit(t, root, "commit", "-q", "-m", "glob match")
	if err := os.WriteFile(filepath.Join(root, literal), []byte("literal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) {
		return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	}
	if err := stageExplicitPaths([]string{"."}, run); err != nil {
		t.Fatal(err)
	}
	if got := reviewIgnoreGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(got) != literal {
		t.Fatalf("only literal file must be staged, not its glob match: %q", got)
	}
	if got := reviewIgnoreGit(t, root, "show", ":"+literal); got != "literal\n" {
		t.Fatalf("staged literal content = %q", got)
	}
}
