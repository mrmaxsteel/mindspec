package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// blockingSpecBranchPreCommit mirrors the decision internal/hook's
// runPreCommit makes in implement mode: a commit on a spec/ branch is
// refused unless MINDSPEC_ALLOW_MAIN=1. The installed shim
// (`exec mindspec hook pre-commit`) needs a mindspec binary on PATH, so
// the decision is inlined here to keep the test hermetic.
const blockingSpecBranchPreCommit = `#!/bin/sh
[ "$MINDSPEC_ALLOW_MAIN" = "1" ] && exit 0
branch=$(git rev-parse --abbrev-ref HEAD)
case "$branch" in
  spec/*) echo "mindspec: commits on spec branch '$branch' are blocked during implement mode" >&2; exit 1 ;;
esac
exit 0
`

// installPreCommit points the repo's core.hooksPath at a fresh dir with
// script as its pre-commit hook.
func installPreCommit(t *testing.T, dir, script string) {
	t.Helper()
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "config", "core.hooksPath", hooksDir)
}

// preservedConflict runs the first (conflicting) CompleteBead so a merge
// is preserved in the spec worktree, and wires real bead-worktree
// removal for the later successful completion.
func preservedConflict(t *testing.T) (g *MindspecExecutor, dir, specWtPath string) {
	t.Helper()
	return preservedConflictWithBeadFiles(t, nil)
}

// preservedConflictWithBeadFiles is preservedConflict, but the bead also
// commits extraFiles (path -> content), which merge without conflict.
func preservedConflictWithBeadFiles(t *testing.T, extraFiles map[string]string) (g *MindspecExecutor, dir, specWtPath string) {
	t.Helper()
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)
	if len(extraFiles) > 0 {
		for name, content := range extraFiles {
			if err := os.WriteFile(filepath.Join(beadWtDir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			runGitIn(t, beadWtDir, "add", name)
		}
		runGitIn(t, beadWtDir, "commit", "-m", "bead extra files")
	}
	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}
	fake.onRemove = func(name string) {
		if name == "worktree-mindspec-x.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
		}
	}
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the conflict must be preserved after the first attempt")
	}
	return g, dir, specWtPath
}

func stageFile(t *testing.T, specWtPath, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(specWtPath, "c.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")
}

// TestCompleteBead_ResolveMerge_ConcludesUnderBlockingPreCommitHook:
// with a pre-commit hook that blocks spec-branch commits (implement
// mode) and no MINDSPEC_ALLOW_MAIN in the caller's env, `--resolve-merge`
// must still conclude the operator's resolved merge: it is the tool's
// own merge conclusion.
func TestCompleteBead_ResolveMerge_ConcludesUnderBlockingPreCommitHook(t *testing.T) {
	t.Setenv("MINDSPEC_ALLOW_MAIN", "")
	g, dir, specWtPath := preservedConflict(t)
	installPreCommit(t, dir, blockingSpecBranchPreCommit)
	stageFile(t, specWtPath, "resolved\n")

	// Sanity: the hook really blocks a plain commit on the spec branch.
	probe := exec.Command("git", "-C", specWtPath, "commit", "--no-edit")
	probe.Env = append(os.Environ(), "MINDSPEC_ALLOW_MAIN=")
	if out, err := probe.CombinedOutput(); err == nil || !strings.Contains(string(out), "blocked during implement mode") {
		t.Fatalf("fixture hook must block a plain spec-branch commit; err=%v out=%s", err, out)
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the refused probe commit must leave the merge in progress")
	}

	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("--resolve-merge must conclude the merge despite the mindspec gate, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be complete — no MERGE_HEAD should remain")
	}
	if got, err := os.ReadFile(filepath.Join(specWtPath, "c.txt")); err != nil || string(got) != "resolved\n" {
		t.Errorf("spec branch tip must carry the resolved content; got %q, err=%v", got, err)
	}
}

// TestCompleteBead_ResolveMerge_UserPreCommitHookStillRuns: only the
// mindspec gate is bypassed. A user's own pre-commit hook (not the
// mindspec gate, ignores MINDSPEC_ALLOW_MAIN) still runs and can refuse
// the merge conclusion; the merge then stays in progress.
func TestCompleteBead_ResolveMerge_UserPreCommitHookStillRuns(t *testing.T) {
	g, dir, specWtPath := preservedConflict(t)
	installPreCommit(t, dir, "#!/bin/sh\necho 'user hook refused' >&2\nexit 1\n")
	stageFile(t, specWtPath, "resolved\n")

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("a refusing user pre-commit hook must stop --resolve-merge from concluding the merge")
	}
	if !strings.Contains(err.Error(), "user hook refused") {
		t.Errorf("the user hook's refusal must surface; got:\n%v", err)
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must stay in progress after the user hook refuses")
	}
}

// TestCompleteBead_ResolveMerge_RefusesStagedConflictMarkers: a file
// staged with its conflict markers still in it has no unmerged index
// entry, but --resolve-merge must not commit it. It re-prints the
// resolution steps (naming the file, no `git commit` step) and keeps
// the merge in progress. Once the markers are gone (trailing whitespace
// alone is not a marker), it concludes.
func TestCompleteBead_ResolveMerge_RefusesStagedConflictMarkers(t *testing.T) {
	g, _, specWtPath := preservedConflict(t)
	runGitIn(t, specWtPath, "add", "c.txt") // markers still in the file
	if len(gitutil.ConflictedFiles(specWtPath)) != 0 {
		t.Fatal("fixture: after `git add` there must be no unmerged entries")
	}

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("--resolve-merge must refuse staged conflict markers")
	}
	msg := err.Error()
	if !strings.Contains(msg, "leftover conflict markers in: c.txt") || !strings.Contains(msg, ResolveMergeFlag) {
		t.Errorf("refusal must name the file and the --resolve-merge re-run; got:\n%s", msg)
	}
	if strings.Contains(msg, "conflicted") {
		t.Errorf("a staged file is not conflicted; the refusal must not say so; got:\n%s", msg)
	}
	if strings.Contains(msg, "git commit") {
		t.Errorf("resolution steps must not tell the operator to run `git commit`; got:\n%s", msg)
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the merge must stay in progress after the refusal")
	}

	stageFile(t, specWtPath, "resolved \n")
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("with markers removed, --resolve-merge must conclude, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be complete")
	}
}

// setextReadme is a Markdown file whose heading underline is exactly
// seven '=' — the same text as a conflict separator line.
const setextReadme = "Summary\n=======\n\nBody.\n"

// TestCompleteBead_ResolveMerge_BeadSetextHeadingDoesNotBlock: the bead
// adds a Markdown file with a '=======' underline; the conflict is in
// another file. That line comes from the bead side, not from the
// resolution, so once c.txt is resolved --resolve-merge concludes.
func TestCompleteBead_ResolveMerge_BeadSetextHeadingDoesNotBlock(t *testing.T) {
	g, _, specWtPath := preservedConflictWithBeadFiles(t, map[string]string{"README.md": setextReadme})
	stageFile(t, specWtPath, "resolved\n")
	if files := gitutil.StagedConflictMarkerFiles(specWtPath); len(files) != 0 {
		t.Errorf("bead-authored setext underline must not count as a marker; got %v", files)
	}
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("--resolve-merge must conclude, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be complete")
	}
	if got, err := os.ReadFile(filepath.Join(specWtPath, "README.md")); err != nil || string(got) != setextReadme {
		t.Errorf("README.md must land unchanged; got %q, err=%v", got, err)
	}
}

// TestCompleteBead_ResolveMerge_ResolutionSetextHeadingConcludes: the
// resolution itself writes a Markdown heading with a '=======' underline
// into the conflicted file and no other marker lines. A lone separator
// is not a conflict block, so --resolve-merge concludes.
func TestCompleteBead_ResolveMerge_ResolutionSetextHeadingConcludes(t *testing.T) {
	g, _, specWtPath := preservedConflict(t)
	stageFile(t, specWtPath, "Merged\n=======\nspec side\nbead side\n")
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("a lone '=======' line must not block --resolve-merge, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be complete")
	}
}

// TestCompleteBead_ResolveMerge_BeadAuthoredMarkerBlockDoesNotBlock: a
// bead file that documents a full conflict block is content from the
// bead side, not a leftover of the resolution. It passes because the
// conflict record does not list it, not because of the fallback scan.
func TestCompleteBead_ResolveMerge_BeadAuthoredMarkerBlockDoesNotBlock(t *testing.T) {
	doc := "Example:\n<<<<<<< ours\na\n=======\nb\n>>>>>>> theirs\n"
	g, _, specWtPath := preservedConflictWithBeadFiles(t, map[string]string{"conflicts.md": doc})
	// The stopped merge recorded git's conflicted paths; conflicts.md is
	// not one of them. Without the record, the fallback would scan it.
	if paths, ok := gitutil.MergeConflictRecord(specWtPath); !ok || len(paths) != 1 || paths[0] != "c.txt" {
		t.Fatalf("conflict record = %q, %v; want [c.txt], true", paths, ok)
	}
	stageFile(t, specWtPath, "resolved\n")
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("bead-authored marker lines must not block --resolve-merge, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be complete")
	}
}

// TestCompleteBead_ResolveMerge_TruncatedRecordRefuses: a conflict record
// cut off after its header lists no paths. It must not make the check
// scan nothing: the fallback scans the staged files and refuses.
func TestCompleteBead_ResolveMerge_TruncatedRecordRefuses(t *testing.T) {
	g, _, specWtPath := preservedConflict(t)
	gitOut := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", specWtPath}, args...)...).Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	recPath := gitOut("rev-parse", "--git-path", "MINDSPEC_MERGE_CONFLICTS")
	if !filepath.IsAbs(recPath) {
		recPath = filepath.Join(specWtPath, recPath)
	}
	header := gitOut("rev-parse", "MERGE_HEAD") + " " + gitOut("rev-parse", "HEAD") + "\n"
	if err := os.WriteFile(recPath, []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, specWtPath, "add", "c.txt") // markers still in the file

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil || !strings.Contains(err.Error(), "leftover conflict markers in: c.txt") {
		t.Fatalf("a truncated record must not let markers through; got: %v", err)
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must stay in progress after the refusal")
	}
}
