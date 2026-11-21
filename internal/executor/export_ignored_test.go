package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ignoreExportRepo makes the repo git-ignore .beads/issues.jsonl, as a
// repo that keeps the beads database as its store of record does.
func ignoreExportRepo(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".beads/issues.jsonl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, root, "add", ".gitignore")
	runGitIn(t, root, "commit", "-qm", "ignore export")
}

// countExports replaces the export with one that writes the file and
// counts calls.
func countExports(t *testing.T) *int {
	t.Helper()
	calls := 0
	orig := execBeadExportFn
	execBeadExportFn = func(workdir string) error {
		calls++
		return os.WriteFile(filepath.Join(workdir, ".beads", "issues.jsonl"), []byte("{}\n"), 0o644)
	}
	t.Cleanup(func() { execBeadExportFn = orig })
	return &calls
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitWithExport_IgnoredExportSkipsExportAndCommits(t *testing.T) {
	g, _, root := newRepoExecutor(t)
	ignoreExportRepo(t, root)
	calls := countExports(t)
	if err := os.WriteFile(filepath.Join(root, "work.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.commitWithExport(root, "commit work"); err != nil {
		t.Fatal(err)
	}
	if *calls != 0 {
		t.Fatalf("export ran %d times in a repo that ignores it", *calls)
	}
	if subject := gitOut(t, root, "log", "-1", "--format=%s"); subject != "commit work" {
		t.Fatalf("commit not made: head subject %q", subject)
	}
	if files := gitOut(t, root, "show", "--name-only", "--format=", "HEAD"); files != "work.txt" {
		t.Fatalf("commit content = %q, want work.txt", files)
	}
}

func TestCommitWithExport_UnignoredExportStillExports(t *testing.T) {
	g, _, root := newRepoExecutor(t)
	calls := countExports(t)
	if err := g.commitWithExport(root, "first export"); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Fatalf("export ran %d times, want 1", *calls)
	}
	if tracked := gitOut(t, root, "ls-files", "--", ".beads/issues.jsonl"); tracked != ".beads/issues.jsonl" {
		t.Fatalf("first export of a fresh repo not committed: %q", tracked)
	}
}

func TestPublishBeadClose_LedgersOnlySkipsExport(t *testing.T) {
	g, _, root := newRepoExecutor(t)
	ignoreExportRepo(t, root)
	calls := countExports(t)
	const ledger = "ledger.jsonl"
	if err := os.WriteFile(filepath.Join(root, ledger), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.PublishBeadClose(root, []string{ledger}); err != nil {
		t.Fatal(err)
	}
	if *calls != 0 {
		t.Fatalf("export ran %d times without the export path", *calls)
	}
	if files := gitOut(t, root, "show", "--name-only", "--format=", "HEAD"); files != ledger {
		t.Fatalf("commit content = %q, want %s", files, ledger)
	}
}

func TestPublishBeadClose_EmptyPathsDoesNotSweepIndex(t *testing.T) {
	g, _, root := newRepoExecutor(t)
	calls := countExports(t)
	if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("user work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, root, "add", "staged.txt")
	head := gitOut(t, root, "rev-parse", "HEAD")
	if err := g.PublishBeadClose(root, nil); err != nil {
		t.Fatal(err)
	}
	if *calls != 0 || gitOut(t, root, "rev-parse", "HEAD") != head {
		t.Fatalf("empty publication exported (%d) or committed", *calls)
	}
	if staged := gitOut(t, root, "diff", "--cached", "--name-only"); staged != "staged.txt" {
		t.Fatalf("staged user work changed: %q", staged)
	}
}
