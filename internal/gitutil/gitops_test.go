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
type capturedCall struct {
	name string
	args []string
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
		*calls = append(*calls, capturedCall{name: name, args: append([]string(nil), args...)})
		// Use /bin/sh to emit stdout and control exit code. Quote stdout
		// safely via printf %s.
		shArg := "printf %s " + shellQuote(stdout)
		if exitCode != 0 {
			shArg += "; exit " + itoa(exitCode)
		}
		return exec.Command("/bin/sh", "-c", shArg)
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
