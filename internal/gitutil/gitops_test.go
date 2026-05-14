package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	assertArgs(t, (*calls)[0].args, "diff", "--name-only", "HEAD~1")
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
	assertArgs(t, (*calls)[0].args, "-C", "/wt", "checkout", "-b", "feature")
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
