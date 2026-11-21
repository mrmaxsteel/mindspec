package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func beadsExportRepo(t *testing.T, gitignore string, track bool) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "Test")
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".gitignore")
	if track {
		run("add", "-f", ".beads/issues.jsonl")
	}
	run("commit", "-qm", "base")
	return dir
}

func TestBeadsExportPublished(t *testing.T) {
	cases := []struct {
		name      string
		gitignore string
		track     bool
		want      bool
	}{
		{"tracked", "\n", true, true},
		// A tracked file stays published even when an ignore rule matches it.
		{"tracked_and_ignored", ".beads/issues.jsonl\n", true, true},
		{"ignored_untracked", ".beads/issues.jsonl\n", false, false},
		{"untracked_not_ignored", "\n", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := beadsExportRepo(t, tc.gitignore, tc.track)
			got, err := BeadsExportPublished(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("BeadsExportPublished = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBeadsExportPublished_PropagatesGitErrors(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	notRepo := t.TempDir()
	if _, err := BeadsExportPublished(notRepo); err == nil {
		t.Fatal("want an error outside a git repository")
	}
	swapExec(t, "fatal: index file corrupt", 128)
	if _, err := BeadsExportPublished("/tmp/some-root"); err == nil {
		t.Fatal("want the git failure to propagate")
	}
}
