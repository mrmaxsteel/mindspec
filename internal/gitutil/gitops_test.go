package gitutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// TestRevParseRef_DistinguishesAbsentFromTransient pins the round-2 hardening
// (Spec 093): a genuinely-absent ref returns ErrRefNotFound (exit 1), while a
// transient/structural failure (running against a non-repo dir → exit 128)
// returns a non-ErrRefNotFound error. A present ref resolves to its SHA.
func TestRevParseRef_DistinguishesAbsentFromTransient(t *testing.T) {
	repo := initGitRepo(t)

	// Present ref (the HEAD branch) resolves.
	sha, err := RevParseRef(repo, "main")
	if err != nil {
		t.Fatalf("present ref: unexpected error: %v", err)
	}
	if len(sha) < 7 {
		t.Errorf("present ref: expected a sha, got %q", sha)
	}

	// Absent ref → ErrRefNotFound (exit 1, --verify --quiet).
	_, err = RevParseRef(repo, "bead/does-not-exist")
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("absent ref: expected ErrRefNotFound, got %v", err)
	}

	// Transient/structural failure: a non-git directory → git exits 128.
	// MUST NOT be reported as ErrRefNotFound.
	nonRepo := t.TempDir()
	_, err = RevParseRef(nonRepo, "main")
	if err == nil {
		t.Fatal("non-repo: expected an error")
	}
	if errors.Is(err, ErrRefNotFound) {
		t.Errorf("non-repo transient error must NOT be ErrRefNotFound: %v", err)
	}
}

func TestEnsureGitignoreEntry_New(t *testing.T) {
	root := t.TempDir()

	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}

	if !strings.Contains(string(data), ".worktrees/") {
		t.Errorf("expected .worktrees/ in .gitignore, got: %s", data)
	}
}

func TestEnsureGitignoreEntry_Idempotent(t *testing.T) {
	root := t.TempDir()

	// Write twice
	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	count := strings.Count(string(data), ".worktrees/")
	if count != 1 {
		t.Errorf("expected exactly 1 .worktrees/ entry, got %d in:\n%s", count, data)
	}
}

// initGitRepo creates a git repo with an initial commit and returns the path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
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
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestDiffStat(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
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

	// Create a feature branch with changes
	run("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0644)
	run("add", ".")
	run("commit", "-m", "add new file")

	stat, err := DiffStat(dir, "main", "feature")
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}
	if !strings.Contains(stat, "new.txt") {
		t.Errorf("expected new.txt in diffstat, got: %s", stat)
	}
}

func TestCommitCount(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
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

	run("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644)
	run("add", ".")
	run("commit", "-m", "commit 1")
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0644)
	run("add", ".")
	run("commit", "-m", "commit 2")

	count, err := CommitCount(dir, "main", "feature")
	if err != nil {
		t.Fatalf("CommitCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 commits, got %d", count)
	}
}

// TestFirstParentMerges pins the git-I/O primitive behind the
// landed-merge-commit-identity predicate (Spec 119 R4): two sequential
// `git merge --no-ff -m "Merge bead/<id>"` merges onto a spec branch, each
// with its own bead branch carrying one commit, must be listed newest-first
// with their full parent list and exact subject.
func TestFirstParentMerges(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
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

	run("checkout", "-b", "spec/119-test")
	run("checkout", "-b", "bead/one")
	os.WriteFile(filepath.Join(dir, "one.txt"), []byte("one\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work one")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/one", "bead/one")

	run("checkout", "-b", "bead/two")
	os.WriteFile(filepath.Join(dir, "two.txt"), []byte("two\n"), 0644)
	run("add", ".")
	run("commit", "-m", "work two")
	run("checkout", "spec/119-test")
	run("merge", "--no-ff", "-m", "Merge bead/two", "bead/two")

	merges, err := FirstParentMerges(dir, "spec/119-test")
	if err != nil {
		t.Fatalf("FirstParentMerges: %v", err)
	}
	if len(merges) != 2 {
		t.Fatalf("expected 2 first-parent merges, got %d: %+v", len(merges), merges)
	}
	if merges[0].Subject != "Merge bead/two" {
		t.Errorf("newest-first: merges[0].Subject = %q, want %q", merges[0].Subject, "Merge bead/two")
	}
	if merges[1].Subject != "Merge bead/one" {
		t.Errorf("merges[1].Subject = %q, want %q", merges[1].Subject, "Merge bead/one")
	}
	for i, m := range merges {
		if len(m.Parents) != 2 {
			t.Errorf("merges[%d]: expected 2 parents, got %d: %+v", i, len(m.Parents), m.Parents)
		}
		if len(m.SHA) < 7 {
			t.Errorf("merges[%d]: expected a SHA, got %q", i, m.SHA)
		}
	}

	// The bead/two merge's SECOND parent must be bead/two's tip.
	beadTwoTip, err := RevParseRef(dir, "bead/two")
	if err != nil {
		t.Fatalf("RevParseRef bead/two: %v", err)
	}
	if merges[0].Parents[1] != beadTwoTip {
		t.Errorf("merges[0] second parent = %q, want bead/two tip %q", merges[0].Parents[1], beadTwoTip)
	}
}

// TestFirstParentMerges_FreshBranchYieldsNoMerge pins the load-bearing
// AC-10 property: `git merge --no-ff` of an already-ancestor branch (a
// freshly-branched bead with zero own commits) performs no merge
// ("Already up to date") and creates no commit. FirstParentMerges must then
// report zero merges for that bead's subject.
func TestFirstParentMerges_FreshBranchYieldsNoMerge(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
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

	run("checkout", "-b", "spec/119-test")
	run("checkout", "-b", "bead/fresh")
	run("checkout", "spec/119-test")
	// bead/fresh has zero own commits — merging it is a no-op.
	out, err := exec.Command("git", "-C", dir, "merge", "--no-ff", "-m", "Merge bead/fresh", "bead/fresh").CombinedOutput()
	if err != nil {
		t.Fatalf("merge bead/fresh: %s", out)
	}

	merges, err := FirstParentMerges(dir, "spec/119-test")
	if err != nil {
		t.Fatalf("FirstParentMerges: %v", err)
	}
	if len(merges) != 0 {
		t.Fatalf("a fresh zero-commit branch must produce NO merge commit, got %+v", merges)
	}
}

func TestCommitAll_CleanTree(t *testing.T) {
	dir := initGitRepo(t)

	// Clean tree — should be a no-op
	err := CommitAll(dir, "should not commit")
	if err != nil {
		t.Fatalf("unexpected error on clean tree: %v", err)
	}

	// Verify no new commit was created (still just the initial commit)
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected 1 commit, got %s", strings.TrimSpace(string(out)))
	}
}

func TestCommitAll_DirtyTree(t *testing.T) {
	dir := initGitRepo(t)

	// Set local git config so CommitAll's real git commands work in CI
	// (initGitRepo uses env vars for its helper, but CommitAll calls git directly).
	for _, kv := range [][2]string{{"user.name", "test"}, {"user.email", "test@test.com"}} {
		cmd := exec.Command("git", "-C", dir, "config", kv[0], kv[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config %s: %s", kv[0], out)
		}
	}

	// Create an untracked file
	os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0644)

	err := CommitAll(dir, "chore: auto-commit spec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the commit happened
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	if strings.TrimSpace(string(out)) != "2" {
		t.Errorf("expected 2 commits (initial + auto), got %s", strings.TrimSpace(string(out)))
	}

	// Verify the commit message
	cmd = exec.Command("git", "-C", dir, "log", "-1", "--format=%s")
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if strings.TrimSpace(string(out)) != "chore: auto-commit spec" {
		t.Errorf("unexpected commit message: %s", strings.TrimSpace(string(out)))
	}

	// Verify tree is clean after
	cmd = exec.Command("git", "-C", dir, "status", "--porcelain")
	out, _ = cmd.Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("tree should be clean after CommitAll, got: %s", out)
	}
}

func TestEnsureGitignoreEntry_ExistingFile(t *testing.T) {
	root := t.TempDir()
	gitignorePath := filepath.Join(root, ".gitignore")

	// Pre-existing content without trailing newline
	if err := os.WriteFile(gitignorePath, []byte("node_modules/"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(content, ".worktrees/") {
		t.Error("new entry should be appended")
	}
}

// --- Tests for new helpers added in ARCH-9 ---

// capturedCall records a single execCommand invocation made during a test.
//
// cmd retains the *exec.Cmd the seam handed back to production. Spec 103 R1
// (GIT_TERMINAL_PROMPT=0 hardening) sets cmd.Env AFTER execCommand returns, so
// name+args alone cannot observe it; tests read (*capturedCall).cmd.Env once
// the production call has completed to assert the env reached the child.
type capturedCall struct {
	name string
	args []string
	cmd  *exec.Cmd
}

// swapExec replaces execCommand for the test's duration, capturing every
// invocation and returning a stub *exec.Cmd that emits the given stdout
// and exits with the given exit code. The captured calls slice is returned
// for assertion.
func swapExec(t *testing.T, stdout string, exitCode int) *[]capturedCall {
	t.Helper()
	calls := &[]capturedCall{}
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Use /bin/sh to emit stdout and control exit code. Quote stdout
		// safely via printf %s.
		shArg := "printf %s " + shellQuote(stdout)
		if exitCode != 0 {
			shArg += "; exit " + itoa(exitCode)
		}
		cmd := exec.Command("/bin/sh", "-c", shArg)
		// Retain the returned cmd so a test can read cmd.Env AFTER production
		// sets GIT_TERMINAL_PROMPT=0 on it (Spec 103 R1).
		*calls = append(*calls, capturedCall{name: name, args: append([]string(nil), args...), cmd: cmd})
		return cmd
	}
	return calls
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func shellQuote(s string) string {
	// Wrap in single quotes and escape any embedded single quotes.
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func assertArgs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args mismatch:\n  got:  %v\n  want: %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("args mismatch at %d:\n  got:  %v\n  want: %v", i, got, want)
		}
	}
}

func TestGitArgs_WithWorkdir(t *testing.T) {
	got := gitArgs("/tmp/wt", "rev-parse", "HEAD")
	assertArgs(t, got, "-C", "/tmp/wt", "rev-parse", "HEAD")
}

func TestGitArgs_NoWorkdir(t *testing.T) {
	got := gitArgs("", "rev-parse", "HEAD")
	assertArgs(t, got, "rev-parse", "HEAD")
}

func TestRevParseHEAD_TrimsNewline(t *testing.T) {
	calls := swapExec(t, "abc123\n", 0)
	sha, err := RevParseHEAD("/tmp/wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc123" {
		t.Errorf("expected trimmed sha, got %q", sha)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args, "-C", "/tmp/wt", "rev-parse", "HEAD")
}

func TestRevParseShowToplevel(t *testing.T) {
	calls := swapExec(t, "/home/user/repo\n", 0)
	top, err := RevParseShowToplevel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if top != "/home/user/repo" {
		t.Errorf("expected trimmed path, got %q", top)
	}
	assertArgs(t, (*calls)[0].args, "rev-parse", "--show-toplevel")
}

func TestIsInsideWorkTree_True(t *testing.T) {
	swapExec(t, "true\n", 0)
	if !IsInsideWorkTree("/some/path") {
		t.Error("expected true on exit 0")
	}
}

func TestIsInsideWorkTree_False(t *testing.T) {
	swapExec(t, "", 1)
	if IsInsideWorkTree("/some/path") {
		t.Error("expected false on exit 1")
	}
}

func TestStatus_PassesC(t *testing.T) {
	calls := swapExec(t, "?? new.txt\n", 0)
	out, err := Status("/tmp/wt")
	if err != nil {
		t.Fatal(err)
	}
	if out != "?? new.txt\n" {
		t.Errorf("unexpected output: %q", out)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/tmp/wt", "status", "--porcelain")
}

func TestStatus_NoWorkdir(t *testing.T) {
	calls := swapExec(t, "", 0)
	if _, err := Status(""); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "status", "--porcelain")
}

func TestStatusWithStderr_PreservesOutputOnFailure(t *testing.T) {
	swapExec(t, "fatal: not a git repository", 128)
	_, err := StatusWithStderr("/tmp/wt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

func TestLsFiles_PassesArgs(t *testing.T) {
	calls := swapExec(t, ".beads/issues.jsonl\n", 0)
	out, err := LsFiles("/r", ".beads/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "issues.jsonl") {
		t.Errorf("unexpected output: %q", out)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "ls-files", ".beads/")
}

func TestLsFilesErrorUnmatch_NilOnTracked(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := LsFilesErrorUnmatch("/r", "tracked.txt"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "ls-files", "--error-unmatch", "--", "tracked.txt")
}

func TestLsFilesErrorUnmatch_ErrOnUntracked(t *testing.T) {
	swapExec(t, "", 1)
	if err := LsFilesErrorUnmatch("/r", "untracked.txt"); err == nil {
		t.Error("expected non-nil error")
	}
}

func TestLsFilesFullName(t *testing.T) {
	calls := swapExec(t, "issues.jsonl\n", 0)
	out, err := LsFilesFullName("/r", "issues.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "issues.jsonl") {
		t.Errorf("unexpected output: %q", out)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "ls-files", "--full-name", "--", "issues.jsonl")
}

func TestCheckIgnore_NilOnIgnored(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := CheckIgnore("/r", "build/"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "check-ignore", "--quiet", "--", "build/")
}

func TestCheckIgnore_ErrOnNotIgnored(t *testing.T) {
	swapExec(t, "", 1)
	if err := CheckIgnore("/r", "src/"); err == nil {
		t.Error("expected non-nil error")
	}
}

func TestDiffNameOnly_SplitsLines(t *testing.T) {
	calls := swapExec(t, "a.go\nb.go\n\nc.go\n", 0)
	files, err := DiffNameOnly("/r", "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.go", "b.go", "c.go"}
	if len(files) != len(want) {
		t.Fatalf("expected %d files, got %d (%v)", len(want), len(files), files)
	}
	for i, w := range want {
		if files[i] != w {
			t.Errorf("[%d]: got %q want %q", i, files[i], w)
		}
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "diff", "--name-only", "main..feature")
}

func TestDiffNameOnly_Empty(t *testing.T) {
	swapExec(t, "", 0)
	files, err := DiffNameOnly("/r", "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty slice, got %v", files)
	}
}

func TestDiffNameOnlyRef_NoWorkdir(t *testing.T) {
	calls := swapExec(t, "x.go\n", 0)
	files, err := DiffNameOnlyRef("", "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "x.go" {
		t.Errorf("unexpected files: %v", files)
	}
	assertArgs(t, (*calls)[0].args, "diff", "--name-only", "HEAD~1", "--")
}

func TestDiffPathspec_InsertsSeparator(t *testing.T) {
	calls := swapExec(t, "diff --git a/x b/x\n", 0)
	pathspecs := []string{":(exclude).beads", ":(exclude)docs/"}
	_, err := DiffPathspec("/wt", "base", "HEAD", pathspecs)
	if err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args,
		"-C", "/wt", "diff", "base", "HEAD", "--",
		":(exclude).beads", ":(exclude)docs/")
}

func TestDiffQuiet_NilOnClean(t *testing.T) {
	swapExec(t, "", 0)
	if err := DiffQuiet("/wt"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestDiffQuiet_ErrOnDirty(t *testing.T) {
	swapExec(t, "", 1)
	if err := DiffQuiet("/wt"); err == nil {
		t.Error("expected non-nil")
	}
}

func TestDiffCachedQuiet(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := DiffCachedQuiet("/wt"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "diff", "--cached", "--quiet")
}

func TestAdd_PassesArgs(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := Add("/wt", "-A"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "add", "-A")
}

func TestCommitNoVerify_PassesMessage(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := CommitNoVerify("/wt", "hello"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "commit", "-m", "hello", "--no-verify")
}

func TestRmCached(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := RmCached("/wt", ".mindspec/session.json"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "rm", "--cached", "--", ".mindspec/session.json")
}

func TestWorktreeAddDetach(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := WorktreeAddDetach("/r", "/wt-a", "abc"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "worktree", "add", "--detach", "/wt-a", "abc")
}

func TestWorktreeAdd(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := WorktreeAdd("/r", "/wt-a", "feature"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "worktree", "add", "/wt-a", "feature")
}

func TestWorktreeRemoveForce(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := WorktreeRemoveForce("/r", "/wt-a"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "worktree", "remove", "--force", "/wt-a")
}

func TestWorktreePrune(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := WorktreePrune("/r"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/r", "worktree", "prune")
}

func TestCheckoutNewBranch(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := CheckoutNewBranch("/wt", "feature"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "checkout", "-b", "feature", "--")
}

// --- Integration tests against a real git repo ---

func TestStatus_RealRepo(t *testing.T) {
	dir := initGitRepo(t)
	// Tree is clean post-init.
	out, err := Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected clean tree, got %q", out)
	}

	// Make a change.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err = Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "new.txt") {
		t.Errorf("expected new.txt in porcelain output, got %q", out)
	}
}

func TestRevParseHEAD_RealRepo(t *testing.T) {
	dir := initGitRepo(t)
	sha, err := RevParseHEAD(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sha) < 7 {
		t.Errorf("expected sha, got %q", sha)
	}
}

func TestDiffNameOnly_RealRepo(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
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
	run("checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "add feat")

	files, err := DiffNameOnly(dir, "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "feat.txt" {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestCheckIgnore_RealRepo(t *testing.T) {
	dir := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CheckIgnore(dir, "ignored.txt"); err != nil {
		t.Errorf("expected ignored.txt to be ignored: %v", err)
	}
	if err := CheckIgnore(dir, "README.md"); err == nil {
		t.Error("expected README.md to not be ignored")
	}
}

func TestIsInsideWorkTree_RealRepo(t *testing.T) {
	dir := initGitRepo(t)
	if !IsInsideWorkTree(dir) {
		t.Error("expected inside work tree")
	}
	if IsInsideWorkTree(t.TempDir()) {
		t.Error("expected outside work tree")
	}
}

// --- Bead 9 punch-list B16: defensive error branches ---

// TestAbortMerge_FailureWrapsOutput covers AbortMerge's error branch:
// a failing `git merge --abort` (e.g. no merge to abort) surfaces the
// workdir and git's combined output.
func TestAbortMerge_FailureWrapsOutput(t *testing.T) {
	calls := swapExec(t, "fatal: There is no merge to abort (MERGE_HEAD missing).", 1)

	err := AbortMerge("/wd")
	if err == nil {
		t.Fatal("expected an error when git merge --abort fails")
	}
	if !strings.Contains(err.Error(), "aborting merge in /wd") {
		t.Errorf("error should name the workdir, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no merge to abort") {
		t.Errorf("error should carry git's output, got: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 git call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wd", "merge", "--abort")
}

// TestConflictedFiles_GitFailureReturnsNil covers ConflictedFiles'
// best-effort error branch: a git failure yields nil, never a partial
// or garbage list.
func TestConflictedFiles_GitFailureReturnsNil(t *testing.T) {
	swapExec(t, "", 1)

	if got := ConflictedFiles("/wd"); got != nil {
		t.Errorf("ConflictedFiles on git failure = %v, want nil (best-effort)", got)
	}
}

// TestRevParseRef exercises the real git-resolution glue behind
// liveBranchSHA (Spec 093 panel-state R2 coverage): an existing ref
// peels to a SHA; a deleted/absent ref returns a non-nil error so the
// caller can map it to the missing-ref pass-through (Req 11).
func TestRevParseRef(t *testing.T) {
	dir := initGitRepo(t)
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
	run("branch", "bead/x.1")

	// Existing ref → 40-char SHA, no error.
	sha, err := RevParseRef(dir, "bead/x.1")
	if err != nil {
		t.Fatalf("RevParseRef on existing branch: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected a 40-char SHA, got %q (len %d)", sha, len(sha))
	}
	// Same commit as HEAD (the branch was forked at HEAD).
	head, err := RevParseHEAD(dir)
	if err != nil {
		t.Fatalf("RevParseHEAD: %v", err)
	}
	if sha != head {
		t.Errorf("RevParseRef(bead/x.1)=%s, want HEAD %s", sha, head)
	}

	// Absent ref → non-nil error (the branch-gone signal liveBranchSHA
	// maps to exists=false). --verify --quiet keeps stderr clean.
	if _, err := RevParseRef(dir, "bead/does-not-exist"); err == nil {
		t.Error("RevParseRef on a missing ref should error (branch-gone signal)")
	}
}

// TestLogOneline exercises the in-progress-beads last-commit glue
// (Spec 093 Req 14): a live ref yields a "<short-sha> <subject>" line;
// a deleted ref errors so the caller renders no detail.
func TestLogOneline(t *testing.T) {
	dir := initGitRepo(t)

	line, err := LogOneline(dir, "HEAD")
	if err != nil {
		t.Fatalf("LogOneline(HEAD): %v", err)
	}
	if !strings.Contains(line, "initial") {
		t.Errorf("expected the commit subject in the oneline, got %q", line)
	}
	if strings.Contains(line, "\n") {
		t.Errorf("LogOneline must be a single trimmed line, got %q", line)
	}

	if _, err := LogOneline(dir, "bead/does-not-exist"); err == nil {
		t.Error("LogOneline on a missing ref should error")
	}
}

// --- SEC-5 / spec 097 R1 (finding obxo): option-injection guard ---

// assertOptionLikeRejected verifies an error is an ADR-0035-shaped
// rejection of a `-`-prefixed operand: non-nil, naming the operand, and
// ending with a final `recovery:` line (guard.HasFinalRecoveryLine).
func assertOptionLikeRejected(t *testing.T, err error, operand string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a rejection error for hostile operand %q, got nil", operand)
	}
	if !strings.Contains(err.Error(), operand) {
		t.Errorf("error should name the hostile operand %q, got: %v", operand, err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("error must end with an ADR-0035 recovery line, got: %q", err.Error())
	}
}

// TestRejectOptionLike_Predicate pins the boundary guard itself: any
// `-`-prefixed operand is rejected with a final recovery line; controlled
// refs and the empty string pass.
func TestRejectOptionLike_Predicate(t *testing.T) {
	for _, hostile := range []string{"-x", "--upload-pack=/evil", "-", "--no-ff"} {
		if err := rejectOptionLike(hostile); err == nil {
			t.Errorf("rejectOptionLike(%q) = nil, want rejection", hostile)
		} else {
			assertOptionLikeRejected(t, err, hostile)
		}
	}
	for _, ok := range []string{"main", "spec/097-x", "bead/mindspec-r04i.2", "HEAD", "HEAD~1", ""} {
		if err := rejectOptionLike(ok); err != nil {
			t.Errorf("rejectOptionLike(%q) = %v, want nil (controlled ref)", ok, err)
		}
	}
}

// TestRejectOptionLike_ExportedMatchesUnexported pins the exported
// RejectOptionLike (consumed by sibling boundary packages such as
// internal/executor, spec 097 R1 executor gap) as behaviorally identical to
// the in-package alias: same rejection on hostile operands, same pass on
// controlled refs, with the ADR-0035 SEC-5 sentinel + recovery line.
func TestRejectOptionLike_ExportedMatchesUnexported(t *testing.T) {
	for _, hostile := range []string{"-x", "--upload-pack=/evil", "-"} {
		exp := RejectOptionLike(hostile)
		if exp == nil {
			t.Errorf("RejectOptionLike(%q) = nil, want rejection", hostile)
			continue
		}
		if !strings.Contains(exp.Error(), "SEC-5") {
			t.Errorf("RejectOptionLike(%q) error %q lacks SEC-5 sentinel", hostile, exp.Error())
		}
		if !guard.HasFinalRecoveryLine(exp.Error()) {
			t.Errorf("RejectOptionLike(%q) error must end with an ADR-0035 recovery line, got: %q", hostile, exp.Error())
		}
		if (rejectOptionLike(hostile) == nil) != (exp == nil) {
			t.Errorf("RejectOptionLike/rejectOptionLike disagree on %q", hostile)
		}
	}
	for _, ok := range []string{"main", "spec/097-x", "HEAD", ""} {
		if err := RejectOptionLike(ok); err != nil {
			t.Errorf("RejectOptionLike(%q) = %v, want nil", ok, err)
		}
	}
}

// TestSEC5_SingleRefSites_RejectHostileOperand covers a single-ref-class
// entry point: a hostile `-`-prefixed ref errors with a recovery line and
// NEVER reaches git (no exec call captured).
func TestSEC5_SingleRefSites_RejectHostileOperand(t *testing.T) {
	calls := swapExec(t, "", 0)

	// CreateBranch (branch -- name from): both operands guarded.
	assertOptionLikeRejected(t, CreateBranch("-x", "main"), "-x")
	assertOptionLikeRejected(t, CreateBranch("feature", "--upload-pack=x"), "--upload-pack=x")

	// MergeBranch (checkout -- target; merge ... -- source).
	assertOptionLikeRejected(t, MergeBranch("/wt", "-x", "main"), "-x")
	assertOptionLikeRejected(t, MergeBranch("/wt", "feature", "-x"), "-x")

	// MergeInto (merge ... -- source).
	assertOptionLikeRejected(t, MergeInto("/wt", "--upload-pack=x"), "--upload-pack=x")

	// DeleteBranch (branch -D -- name).
	assertOptionLikeRejected(t, DeleteBranch("-x"), "-x")

	// CheckoutNewBranch (checkout -b branch --).
	assertOptionLikeRejected(t, CheckoutNewBranch("/wt", "-x"), "-x")

	// LogOneline / DiffNameOnlyRef (trailing-`--` single-ref).
	_, err := LogOneline("/wt", "-x")
	assertOptionLikeRejected(t, err, "-x")
	_, err = DiffNameOnlyRef("/wt", "--upload-pack=x")
	assertOptionLikeRejected(t, err, "--upload-pack=x")

	if len(*calls) != 0 {
		t.Errorf("hostile operands must be rejected BEFORE git is invoked, got %d exec calls: %v", len(*calls), *calls)
	}
}

// TestSEC5_RefOnlySites_RejectHostileOperand covers the ref-only class
// (no `--`): a hostile operand errors (or, for BranchExists which returns
// bool, reports false) and never reaches git.
func TestSEC5_RefOnlySites_RejectHostileOperand(t *testing.T) {
	calls := swapExec(t, "", 0)

	assertOptionLikeRejected(t, PushBranch("-x"), "-x")
	_, err := IsAncestor("/wt", "-x", "main")
	assertOptionLikeRejected(t, err, "-x")
	_, err = IsAncestor("/wt", "main", "--upload-pack=x")
	assertOptionLikeRejected(t, err, "--upload-pack=x")
	_, err = RevParseRef("/wt", "-x")
	assertOptionLikeRejected(t, err, "-x")
	assertOptionLikeRejected(t, WorktreeAddDetach("/wt", "/p", "-x"), "-x")
	assertOptionLikeRejected(t, WorktreeAdd("/wt", "/p", "--upload-pack=x"), "--upload-pack=x")

	if BranchExists("-x") {
		t.Error("BranchExists(\"-x\") = true, want false (option-like name rejected)")
	}

	if len(*calls) != 0 {
		t.Errorf("hostile ref-only operands must be rejected BEFORE git is invoked, got %d exec calls: %v", len(*calls), *calls)
	}
}

// TestSEC5_RangeSites_RejectHostileOperand covers the revision-range class:
// a hostile base OR head errors with a recovery line and never reaches git.
func TestSEC5_RangeSites_RejectHostileOperand(t *testing.T) {
	calls := swapExec(t, "", 0)

	_, err := DiffStat("/wt", "-x", "main")
	assertOptionLikeRejected(t, err, "-x")
	_, err = DiffStat("/wt", "main", "--upload-pack=x")
	assertOptionLikeRejected(t, err, "--upload-pack=x")

	_, err = CommitCount("/wt", "-x", "main")
	assertOptionLikeRejected(t, err, "-x")

	_, err = DiffNameOnly("/wt", "main", "-x")
	assertOptionLikeRejected(t, err, "-x")

	// DiffPathspec guards base/head too (pathspecs sit safely after `--`).
	_, err = DiffPathspec("/wt", "-x", "main", []string{"a.go"})
	assertOptionLikeRejected(t, err, "-x")

	if len(*calls) != 0 {
		t.Errorf("hostile range operands must be rejected BEFORE git is invoked, got %d exec calls: %v", len(*calls), *calls)
	}
}

// TestSEC5_RangeSites_NoSeparator pins that the range operands DO NOT get
// a `--` separator (a `--` would reinterpret `base..head` as a pathspec).
// RED if a `--` is wrongly inserted into a range argv.
func TestSEC5_RangeSites_NoSeparator(t *testing.T) {
	calls := swapExec(t, "1\n", 0)
	if _, err := DiffStat("/wt", "main", "feature"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "diff", "--stat", "main..feature")

	calls = swapExec(t, "3\n", 0)
	if _, err := CommitCount("/wt", "main", "feature"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "rev-list", "--count", "main..feature")

	calls = swapExec(t, "a.go\n", 0)
	if _, err := DiffNameOnly("/wt", "main", "feature"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "diff", "--name-only", "main..feature")
}

// TestSEC5_SingleRefSites_InsertSeparator pins the `--` separators on the
// single-ref subcommands. RED if a separator is dropped.
func TestSEC5_SingleRefSites_InsertSeparator(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := CreateBranch("feature", "main"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "branch", "--", "feature", "main")

	calls = swapExec(t, "", 0)
	if err := DeleteBranch("feature"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "branch", "-D", "--", "feature")

	calls = swapExec(t, "", 0)
	if err := MergeBranch("/wt", "feature", "main"); err != nil {
		t.Fatal(err)
	}
	// Two calls: checkout target (trailing `--`), then merge with `-m ... -- source`.
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "checkout", "main", "--")
	assertArgs(t, (*calls)[1].args, "-C", "/wt", "merge", "--no-ff", "-m", "Merge feature into main", "--", "feature")

	calls = swapExec(t, "", 0)
	if err := MergeInto("/wt", "feature"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "merge", "--no-ff", "-m", "Merge feature", "--", "feature")

	calls = swapExec(t, "", 0)
	if _, err := LogOneline("/wt", "main"); err != nil {
		t.Fatal(err)
	}
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "log", "-1", "--oneline", "main", "--")
}

// TestSEC5_ControlledRefs_StillSucceed proves every classified ref shape
// (`main`, `spec/<id>`, `bead/<id>`) still flows through to git unchanged.
func TestSEC5_ControlledRefs_StillSucceed(t *testing.T) {
	for _, ref := range []string{"main", "spec/097-code-review-cleanup", "bead/mindspec-r04i.2"} {
		calls := swapExec(t, "", 0)
		if err := PushBranch(ref); err != nil {
			t.Errorf("PushBranch(%q) rejected a controlled ref: %v", ref, err)
		}
		assertArgs(t, (*calls)[0].args, "push", "-u", "origin", ref)

		calls = swapExec(t, "", 0)
		if err := CheckoutNewBranch("/wt", ref); err != nil {
			t.Errorf("CheckoutNewBranch(%q) rejected a controlled ref: %v", ref, err)
		}
		assertArgs(t, (*calls)[0].args, "-C", "/wt", "checkout", "-b", ref, "--")
	}
}

// --- Spec 101 Bead 4 (R4, k9a8/#76): fetch + default-branch detect ----------

// swapExecFunc is a per-command-aware variant of swapExec: the supplied
// reply function inspects the (name, args) of each invocation and returns the
// stdout + exit code that stub should emit. This is required for the
// symbolic-ref → `git remote show origin` fall-through, where ONE git
// subprocess must succeed-but-empty (or garbage) while the NEXT must return a
// parseable "HEAD branch" — swapExec's single stdout/exitCode for all commands
// cannot express that.
func swapExecFunc(t *testing.T, reply func(name string, args []string) (stdout string, exitCode int)) *[]capturedCall {
	t.Helper()
	calls := &[]capturedCall{}
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = func(name string, args ...string) *exec.Cmd {
		stdout, exitCode := reply(name, append([]string(nil), args...))
		shArg := "printf %s " + shellQuote(stdout)
		if exitCode != 0 {
			shArg += "; exit " + itoa(exitCode)
		}
		cmd := exec.Command("/bin/sh", "-c", shArg)
		// Retain the returned cmd so a test can read cmd.Env AFTER production
		// sets GIT_TERMINAL_PROMPT=0 on it (Spec 103 R1).
		*calls = append(*calls, capturedCall{name: name, args: append([]string(nil), args...), cmd: cmd})
		return cmd
	}
	return calls
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// TestFetchRemote_RunsFetch asserts FetchRemote shells out to `git fetch
// <remote>` and surfaces a non-zero exit as an error (the offline / auth
// failure case the executor funnels into its WARN fallback).
func TestFetchRemote_RunsFetch(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := FetchRemote("origin"); err != nil {
		t.Fatalf("FetchRemote: unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args, "fetch", "origin")

	// Non-zero exit (offline / auth failure) must surface as an error.
	swapExec(t, "fatal: could not read from remote", 1)
	if err := FetchRemote("origin"); err == nil {
		t.Fatal("FetchRemote: expected error on non-zero git exit, got nil")
	}
}

// TestFetchRemoteBranch_RunsNarrowFetch asserts FetchRemoteBranch shells out
// to the NARROW `git fetch <remote> <branch>` (bug wu7t's protected-main
// finalize check — not FetchRemote's full multi-branch fetch) and surfaces a
// non-zero exit (offline / missing remote ref) as an error, which the
// executor funnels into its warn-and-fall-back path.
func TestFetchRemoteBranch_RunsNarrowFetch(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := FetchRemoteBranch("origin", "main"); err != nil {
		t.Fatalf("FetchRemoteBranch: unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args, "fetch", "origin", "main")

	swapExec(t, "fatal: couldn't find remote ref main", 1)
	if err := FetchRemoteBranch("origin", "main"); err == nil {
		t.Fatal("FetchRemoteBranch: expected error on non-zero git exit, got nil")
	}
}

// TestRemoteHeadSHA_ParsesLsRemote pins bug wu7t's three-way remote-state
// probe: a present branch yields its SHA (first ls-remote field), an ABSENT
// branch yields "" with a NIL error (empty output, zero exit), and a git
// failure (offline / auth) is an ERROR — never conflated with absence,
// which would downgrade the lease push to a plain (rejectable) push.
func TestRemoteHeadSHA_ParsesLsRemote(t *testing.T) {
	calls := swapExec(t, "abc123\trefs/heads/chore/finalize-077-x\n", 0)
	sha, err := RemoteHeadSHA("origin", "chore/finalize-077-x")
	if err != nil {
		t.Fatalf("RemoteHeadSHA: unexpected error: %v", err)
	}
	if sha != "abc123" {
		t.Errorf("sha = %q, want %q", sha, "abc123")
	}
	assertArgs(t, (*calls)[0].args, "ls-remote", "--heads", "origin", "chore/finalize-077-x")

	// Absent branch: empty output with a zero exit → "" and nil error.
	swapExec(t, "", 0)
	sha, err = RemoteHeadSHA("origin", "chore/finalize-077-x")
	if err != nil {
		t.Fatalf("RemoteHeadSHA on absent branch: unexpected error: %v", err)
	}
	if sha != "" {
		t.Errorf("absent branch sha = %q, want empty", sha)
	}

	// Git failure is an error, NOT an absent branch.
	swapExec(t, "", 128)
	if _, err := RemoteHeadSHA("origin", "chore/finalize-077-x"); err == nil {
		t.Fatal("RemoteHeadSHA: expected error on non-zero git exit, got nil")
	}
}

// TestPushBranchForceWithLease_ArgvAndLease pins the compare-and-swap push
// bug wu7t's retried chore-branch flow uses: the lease names the exact
// refs/heads/<branch>:<expectedSHA> pair, and a failed lease (the remote
// tip moved under us) surfaces as an error rather than clobbering it.
func TestPushBranchForceWithLease_ArgvAndLease(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := PushBranchForceWithLease("chore/finalize-077-x", "abc123"); err != nil {
		t.Fatalf("PushBranchForceWithLease: unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args,
		"push", "--force-with-lease=refs/heads/chore/finalize-077-x:abc123", "-u", "origin", "chore/finalize-077-x")

	swapExec(t, "stale info", 1)
	if err := PushBranchForceWithLease("chore/finalize-077-x", "abc123"); err == nil {
		t.Fatal("PushBranchForceWithLease: expected error on a failed lease, got nil")
	}
}

// TestDefaultBranch_DetectedFromSymbolicRef proves the default branch is
// DETECTED from `git symbolic-ref refs/remotes/origin/HEAD` — and is NOT
// hardcoded `main`: a NON-main remote HEAD (develop) is returned verbatim.
func TestDefaultBranch_DetectedFromSymbolicRef(t *testing.T) {
	calls := swapExec(t, "refs/remotes/origin/develop\n", 0)
	got, err := DetectDefaultBranch("origin")
	if err != nil {
		t.Fatalf("DetectDefaultBranch: unexpected error: %v", err)
	}
	if got != "develop" {
		t.Fatalf("default branch = %q, want %q (detected, not hardcoded main)", got, "develop")
	}
	// Only the cheap symbolic-ref call should run on the happy path; no
	// fall-through to `git remote show`.
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call (symbolic-ref only), got %d: %v", len(*calls), *calls)
	}
	assertArgs(t, (*calls)[0].args, "symbolic-ref", "refs/remotes/origin/HEAD")
}

// TestDefaultBranch_EmptyOrGarbageSymbolicRefFallsThrough proves an empty or
// unparseable symbolic-ref output is treated as a MISS and falls THROUGH to
// `git remote show origin` (parsing the "HEAD branch:" line). Uses the
// per-command stub so symbolic-ref and remote-show can answer differently.
func TestDefaultBranch_EmptyOrGarbageSymbolicRefFallsThrough(t *testing.T) {
	for _, symRefOut := range []string{"", "totally-not-a-ref\n", "refs/heads/oops\n"} {
		calls := swapExecFunc(t, func(name string, args []string) (string, int) {
			switch {
			case hasArg(args, "symbolic-ref"):
				// Empty/garbage cached ref: not a valid refs/remotes/origin/<name>.
				return symRefOut, 0
			case hasArg(args, "show"): // `git remote show origin`
				return "* remote origin\n  Fetch URL: x\n  HEAD branch: develop\n  Remote branches:\n", 0
			default:
				return "", 0
			}
		})
		got, err := DetectDefaultBranch("origin")
		if err != nil {
			t.Fatalf("symRef=%q: DetectDefaultBranch fell through but errored: %v", symRefOut, err)
		}
		if got != "develop" {
			t.Fatalf("symRef=%q: default = %q, want develop (from remote show fall-through)", symRefOut, got)
		}
		if len(*calls) != 2 {
			t.Fatalf("symRef=%q: expected 2 calls (symbolic-ref then remote show), got %d: %v", symRefOut, len(*calls), *calls)
		}
		assertArgs(t, (*calls)[0].args, "symbolic-ref", "refs/remotes/origin/HEAD")
		assertArgs(t, (*calls)[1].args, "remote", "show", "origin")
	}
}

// TestDefaultBranch_BothSourcesFail returns an error when neither the cached
// symbolic-ref nor `git remote show` yields a default (the detect-failure case
// the executor funnels into its WARN fallback, never a hard failure).
func TestDefaultBranch_BothSourcesFail(t *testing.T) {
	swapExecFunc(t, func(name string, args []string) (string, int) {
		if hasArg(args, "symbolic-ref") {
			return "", 1 // no cached ref
		}
		return "fatal: could not read from remote", 1 // remote show fails (offline)
	})
	if _, err := DetectDefaultBranch("origin"); err == nil {
		t.Fatal("DetectDefaultBranch: expected error when both detection sources fail, got nil")
	}
}

// --- Spec 103 Bead 1 (R1, o7tp): network ops fast-fail via GIT_TERMINAL_PROMPT=0 ---

// assertNoPromptEnv asserts the captured child cmd carries GIT_TERMINAL_PROMPT=0
// (so git fast-fails on a slow/auth-prompting origin instead of hanging on
// stdin) AND that the inherited environment is preserved (e.g. PATH is still
// present), proving the env was APPENDED to os.Environ() rather than clobbered.
func assertNoPromptEnv(t *testing.T, c capturedCall) {
	t.Helper()
	if c.cmd == nil {
		t.Fatalf("captured call %s %v: nil cmd (seam did not retain it)", c.name, c.args)
	}
	var sawNoPrompt, sawInherited bool
	for _, kv := range c.cmd.Env {
		if kv == "GIT_TERMINAL_PROMPT=0" {
			sawNoPrompt = true
		}
		if strings.HasPrefix(kv, "PATH=") {
			sawInherited = true
		}
	}
	if !sawNoPrompt {
		t.Errorf("%s %v: cmd.Env missing GIT_TERMINAL_PROMPT=0 (git can prompt/hang on stdin); env=%v", c.name, c.args, c.cmd.Env)
	}
	if !sawInherited {
		t.Errorf("%s %v: cmd.Env missing inherited PATH (env was clobbered, not appended to os.Environ()); env=%v", c.name, c.args, c.cmd.Env)
	}
}

// TestFetchRemote_SetsNoPromptEnv pins R1: FetchRemote's `git fetch` carries
// GIT_TERMINAL_PROMPT=0 (appended, not clobbered). RED before the fix (no env
// set), GREEN after.
func TestFetchRemote_SetsNoPromptEnv(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := FetchRemote("origin"); err != nil {
		t.Fatalf("FetchRemote: unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args, "fetch", "origin")
	assertNoPromptEnv(t, (*calls)[0])
}

// TestPushBranch_SetsNoPromptEnv pins R1 for PushBranch's `git push`.
func TestPushBranch_SetsNoPromptEnv(t *testing.T) {
	calls := swapExec(t, "", 0)
	if err := PushBranch("bead/x.1"); err != nil {
		t.Fatalf("PushBranch: unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args, "push", "-u", "origin", "bead/x.1")
	assertNoPromptEnv(t, (*calls)[0])
}

// TestDefaultBranch_SetsNoPromptEnv pins R1 for both network/credential ops
// in DetectDefaultBranch: the cheap `git symbolic-ref` step AND the
// `git remote show` fall-through both carry GIT_TERMINAL_PROMPT=0. Drives the
// fall-through so both subprocesses are exercised in one run.
func TestDefaultBranch_SetsNoPromptEnv(t *testing.T) {
	calls := swapExecFunc(t, func(name string, args []string) (string, int) {
		switch {
		case hasArg(args, "symbolic-ref"):
			return "", 1 // miss → fall through to remote show
		case hasArg(args, "show"):
			return "* remote origin\n  HEAD branch: develop\n", 0
		default:
			return "", 0
		}
	})
	if _, err := DetectDefaultBranch("origin"); err != nil {
		t.Fatalf("DetectDefaultBranch: unexpected error: %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 calls (symbolic-ref then remote show), got %d: %v", len(*calls), *calls)
	}
	assertArgs(t, (*calls)[0].args, "symbolic-ref", "refs/remotes/origin/HEAD")
	assertNoPromptEnv(t, (*calls)[0])
	assertArgs(t, (*calls)[1].args, "remote", "show", "origin")
	assertNoPromptEnv(t, (*calls)[1])
}
