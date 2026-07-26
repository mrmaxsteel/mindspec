package approve

// Shared real-git fixture helpers for adopt_test.go / adopt_lattice_test.go
// (spec 127 bead 3). Mirrors internal/approve/plan_fault_test.go's
// planGitRun/planGitCommit house pattern — a dedicated adopt-prefixed set
// so the two test files stay independently readable.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func adoptGitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
	}
}

func adoptGitRunAllowFail(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	)
	return cmd.CombinedOutput()
}

// adoptGitStatusPorcelain returns `git status --porcelain` for dir —
// used by the G1-B3-01 dirty-workdir counterfixtures to prove an
// operator's unrelated tracked/untracked/staged file survives an adopt
// call byte- and index-identical (never swept into the finalize-export
// commit).
func adoptGitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	out, err := adoptGitRunAllowFail(dir, "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status --porcelain in %s: %v\n%s", dir, err, out)
	}
	return string(out)
}

// adoptWriteFile writes content to a path inside dir, creating parent
// directories as needed.
func adoptWriteFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

// adoptInitRepo creates a fresh git repo at a new temp dir with an
// initial commit on main, and returns the dir.
func adoptInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	adoptGitRun(t, dir, "init", "-q", "-b", "main")
	adoptWriteFile(t, dir, "README.md", "root\n")
	adoptGitRun(t, dir, "add", ".")
	adoptGitRun(t, dir, "commit", "-q", "-m", "root commit")
	return dir
}

// adoptCommit stages everything and commits with msg (no-op if clean).
func adoptCommit(t *testing.T, dir, msg string) {
	t.Helper()
	adoptGitRun(t, dir, "add", "-A")
	out, err := adoptGitRunAllowFail(dir, "commit", "-q", "-m", msg)
	if err != nil && !strings.Contains(string(out), "nothing to commit") {
		t.Fatalf("git commit in %s: %v\n%s", dir, err, out)
	}
}

// adoptRefHash resolves ref to its SHA in dir.
func adoptRefHash(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("rev-parse %s in %s: %v", ref, dir, err)
	}
	return strings.TrimSpace(string(out))
}

// makeStaleRecreatedBranch builds the #218 step-2 shape bead 1's
// predicate exists to catch: a branch that reverts target's own later
// work back to an earlier snapshot target's history still carries — the
// "recreated from the target's current tip, carrying an old tree" shape
// (internal/gitutil/workdestruction.go's package doc comment). Getting
// the git mechanics right matters: the branch must be a DESCENDANT of
// target's CURRENT tip (so merge-base(target, branch) == target's tip,
// not some older common ancestor) whose OWN commit removes content
// target added since — a `git reset --hard <old>` on a freshly branched
// ref does NOT produce this shape, because reset discards the branch's
// ancestry to target's tip entirely (merge-base would then resolve to
// the OLD commit instead, and PreviewDeletedPaths' three-way merge would
// cleanly re-add the "removed" content from target's side, masking the
// very deletion this fixture wants).
//
// Concretely: target gets a second commit (adding extra.txt); branch is
// created AS A CHILD of target's new tip, then reverts that addition in
// one commit of its own (deleting extra.txt) — reconstructing the ROOT
// commit's tree, which target's own history still contains.
func makeStaleRecreatedBranch(t *testing.T, dir, branch string) {
	t.Helper()
	adoptWriteFile(t, dir, "extra.txt", "landed after the branch's snapshot\n")
	adoptCommit(t, dir, "advance main past the branch's old snapshot")

	adoptGitRun(t, dir, "checkout", "-q", "-b", branch, "main")
	if err := os.Remove(filepath.Join(dir, "extra.txt")); err != nil {
		t.Fatalf("remove extra.txt: %v", err)
	}
	adoptCommit(t, dir, "revert to the old snapshot (stale recreation)")
	adoptGitRun(t, dir, "checkout", "-q", "main")
}

// adoptBareOrigin creates a bare repo at a new temp dir, wires it as
// dir's "origin" remote, and pushes main. Returns the bare repo's path.
func adoptBareOrigin(t *testing.T, dir string) string {
	t.Helper()
	origin := t.TempDir()
	adoptGitRun(t, origin, "init", "-q", "--bare", "-b", "main")
	adoptGitRun(t, dir, "remote", "add", "origin", origin)
	adoptGitRun(t, dir, "push", "-q", "-u", "origin", "main")
	return origin
}
