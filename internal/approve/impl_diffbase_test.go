package approve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// diffNames lists the paths changed between base and ref in dir.
func diffNames(t *testing.T, dir, base, ref string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "diff", "--name-only", base, ref).Output()
	if err != nil {
		t.Fatalf("git diff %s %s: %v", base, ref, err)
	}
	return strings.Fields(string(out))
}

// TestImplDiffBase_StaleLocalMainUsesOriginMergeBase reproduces the
// stale-local-main refusal: local main is behind origin/main, and the spec
// branch has merged origin/main. With the local merge-base, the file that
// origin/main added reads as a spec change. The base must be the
// origin/main merge-base, so only the spec's own file is in the range.
func TestImplDiffBase_StaleLocalMainUsesOriginMergeBase(t *testing.T) {
	dir := adoptInitRepo(t)
	staleMain := adoptRefHash(t, dir, "main")

	// origin/main moves ahead with a file the spec never wrote.
	adoptGitRun(t, dir, "checkout", "-q", "-b", "upstream")
	adoptWriteFile(t, dir, ".github/copilot-instructions.md", "upstream\n")
	adoptCommit(t, dir, "upstream change")
	upstream := adoptRefHash(t, dir, "upstream")
	adoptGitRun(t, dir, "update-ref", "refs/remotes/origin/main", upstream)

	// The spec branch forks from the stale main, writes its own file, and
	// merges origin/main.
	adoptGitRun(t, dir, "checkout", "-q", "-b", "spec/999-x", staleMain)
	adoptWriteFile(t, dir, "spec-only.txt", "spec\n")
	adoptCommit(t, dir, "spec change")
	adoptGitRun(t, dir, "merge", "-q", "--no-edit", "origin/main")

	if got := adoptRefHash(t, dir, "main"); got != staleMain {
		t.Fatalf("fixture: local main moved to %s", got)
	}

	base, err := implDiffBase(dir, executor.NewMindspecExecutor(dir), "spec/999-x")
	if err != nil {
		t.Fatalf("implDiffBase: %v", err)
	}
	if base != upstream {
		t.Errorf("base = %s, want the origin/main merge-base %s (local main merge-base is %s)", base, upstream, staleMain)
	}
	for _, p := range diffNames(t, dir, base, "spec/999-x") {
		if p == ".github/copilot-instructions.md" {
			t.Errorf("a file that only origin/main changed is in the spec range %s..spec/999-x", base)
		}
	}
}

// TestImplDiffBase_LocalMainAheadKeepsLocal: when local main is ahead of
// origin/main (and the spec merged local main), the local merge-base is
// the newer one and stays the base.
func TestImplDiffBase_LocalMainAheadKeepsLocal(t *testing.T) {
	dir := adoptInitRepo(t)
	root := adoptRefHash(t, dir, "main")
	adoptGitRun(t, dir, "update-ref", "refs/remotes/origin/main", root)
	adoptWriteFile(t, dir, "local.txt", "local\n")
	adoptCommit(t, dir, "local main ahead")
	localTip := adoptRefHash(t, dir, "main")
	adoptGitRun(t, dir, "checkout", "-q", "-b", "spec/999-x")
	adoptWriteFile(t, dir, "spec-only.txt", "spec\n")
	adoptCommit(t, dir, "spec change")

	base, err := implDiffBase(dir, executor.NewMindspecExecutor(dir), "spec/999-x")
	if err != nil {
		t.Fatalf("implDiffBase: %v", err)
	}
	if base != localTip {
		t.Errorf("base = %s, want local merge-base %s", base, localTip)
	}
}

// TestImplDiffBase_NoOriginRefKeepsLocal: a repo with no origin/main ref
// keeps the local merge-base.
func TestImplDiffBase_NoOriginRefKeepsLocal(t *testing.T) {
	dir := adoptInitRepo(t)
	mainTip := adoptRefHash(t, dir, "main")
	adoptGitRun(t, dir, "checkout", "-q", "-b", "spec/999-x")
	adoptWriteFile(t, dir, "spec-only.txt", "spec\n")
	adoptCommit(t, dir, "spec change")

	base, err := implDiffBase(dir, executor.NewMindspecExecutor(dir), "spec/999-x")
	if err != nil {
		t.Fatalf("implDiffBase: %v", err)
	}
	if base != mainTip {
		t.Errorf("base = %s, want %s", base, mainTip)
	}
}

// TestApproveImpl_ContentGatesConsultOriginMain pins the wiring: the
// whole-branch gates resolve their base through implDiffBase, which asks
// for the origin/main merge-base as well as the local one. The old code
// asked only for merge-base(main, spec).
func TestApproveImpl_ContentGatesConsultOriginMain(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	if err := os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	saveAndRestore(t)
	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		ChangedFilesResult: []string{".mindspec/docs/specs/010-test/spec.md"},
	}
	_, _ = ApproveImpl(tmp, "010-test", mock)
	sawOrigin := false
	for _, c := range mock.CallsTo("MergeBase") {
		if len(c.Args) > 0 && c.Args[0] == "origin/main" {
			sawOrigin = true
		}
	}
	if !sawOrigin {
		t.Errorf("ApproveImpl never asked for merge-base(origin/main, spec); calls: %+v", mock.CallsTo("MergeBase"))
	}
}

func TestImplDiffBase_SpecAlreadyMergedIntoOriginKeepsFullRange(t *testing.T) {
	for _, later := range []bool{false, true} {
		t.Run(fmt.Sprint("upstreamLater=", later), func(t *testing.T) {
			dir := adoptInitRepo(t)
			local := adoptRefHash(t, dir, "main")
			adoptGitRun(t, dir, "checkout", "-q", "-b", "spec/999-x")
			adoptWriteFile(t, dir, "internal/feature.go", "package feature\n")
			adoptCommit(t, dir, "spec source change")
			tip := adoptRefHash(t, dir, "HEAD")
			adoptGitRun(t, dir, "checkout", "-q", "-b", "upstream", local)
			adoptGitRun(t, dir, "merge", "-q", "--no-ff", "--no-edit", "spec/999-x")
			if later {
				adoptWriteFile(t, dir, "later.txt", "later\n")
				adoptCommit(t, dir, "later upstream change")
			}
			adoptGitRun(t, dir, "update-ref", "refs/remotes/origin/main", adoptRefHash(t, dir, "HEAD"))
			base, err := implDiffBase(dir, executor.NewMindspecExecutor(dir), "spec/999-x")
			if err != nil {
				t.Fatal(err)
			}
			files := diffNames(t, dir, base, "spec/999-x")
			if base != local || !slices.Contains(files, "internal/feature.go") {
				t.Fatalf("content gates lost spec range: base=%s local=%s spec=%s files=%v", base, local, tip, files)
			}
		})
	}
}
