package gitutil

// Spec 127 bead-6 fix round 3 (O2-1/S1-2's panel finding): direct,
// dedicated unit tests for the six gitutil primitives bead-6 added or
// changed (TreeSHA, CommitMessageBody, CommitTreeMerge, ResetSoft,
// MergeMsgSubject/CommitNoEdit's happy+failure paths, and the new
// UpdateRef/MergeSourceMarkerRef/recordMergeSourceMarker trio) — none of
// these had coverage independent of internal/executor's e2e tests before
// this fix round, so a regression in any ONE primitive's own error
// handling (a bad ref, a non-existent tree, a locked repo) could only be
// caught incidentally, through whichever executor-level scenario happened
// to exercise it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeSHA_HappyPath(t *testing.T) {
	dir := initGitRepo(t)
	tree, err := TreeSHA(dir, "HEAD")
	if err != nil {
		t.Fatalf("TreeSHA: %v", err)
	}
	if len(tree) != 40 {
		t.Errorf("expected a 40-char SHA, got %q", tree)
	}
	// TreeSHA must resolve to the SAME tree `git rev-parse HEAD^{tree}`
	// resolves to directly — a genuine, non-vacuous wrapper.
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD^{tree}: %v", err)
	}
	if want := strings.TrimSpace(string(out)); tree != want {
		t.Errorf("TreeSHA = %q, want %q", tree, want)
	}
}

func TestTreeSHA_BadRefFails(t *testing.T) {
	dir := initGitRepo(t)
	if _, err := TreeSHA(dir, "refs/heads/does-not-exist"); err == nil {
		t.Fatal("expected an error for a non-existent ref, got nil")
	}
}

func TestTreeSHA_OptionLikeRefRejected(t *testing.T) {
	dir := initGitRepo(t)
	if _, err := TreeSHA(dir, "--evil"); err == nil {
		t.Fatal("expected rejectOptionLike to refuse an option-like ref")
	}
}

func TestCommitMessageBody_HappyPath(t *testing.T) {
	dir := initGitRepo(t)
	runGit(t, dir, "commit", "--allow-empty", "-m", "subject line\n\nbody line 1\nbody line 2")
	body, err := CommitMessageBody(dir, "HEAD")
	if err != nil {
		t.Fatalf("CommitMessageBody: %v", err)
	}
	// git's own commit-message cleanup appends a trailing blank line for
	// a multi-line -m message, so `%B` itself carries TWO trailing
	// newlines here; CommitMessageBody's contract strips exactly ONE
	// (the one git itself always appends), otherwise byte-verbatim — so
	// exactly one survives.
	want := "subject line\n\nbody line 1\nbody line 2\n"
	if body != want {
		t.Errorf("CommitMessageBody = %q, want %q", body, want)
	}
}

func TestCommitMessageBody_BadRefFails(t *testing.T) {
	dir := initGitRepo(t)
	if _, err := CommitMessageBody(dir, "does-not-exist"); err == nil {
		t.Fatal("expected an error for a non-existent ref, got nil")
	}
}

func TestCommitTreeMerge_HappyPath(t *testing.T) {
	dir := initGitRepo(t)
	parent1 := runGitOut(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "checkout", "-b", "side")
	os.WriteFile(filepath.Join(dir, "side.txt"), []byte("side\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "side work")
	parent2 := runGitOut(t, dir, "rev-parse", "HEAD")
	tree := runGitOut(t, dir, "rev-parse", "HEAD^{tree}")

	sha, err := CommitTreeMerge(dir, tree, parent1, parent2, "Merge side")
	if err != nil {
		t.Fatalf("CommitTreeMerge: %v", err)
	}
	if len(sha) != 40 {
		t.Fatalf("expected a 40-char SHA, got %q", sha)
	}
	// The constructed object is UNREFERENCED (no ref moved) but must be a
	// real, well-formed two-parent commit with the requested tree/subject.
	gotParents := runGitOut(t, dir, "log", "-1", "--format=%P", sha)
	if gotParents != parent1+" "+parent2 {
		t.Errorf("parents = %q, want %q %q", gotParents, parent1, parent2)
	}
	gotTree := runGitOut(t, dir, "rev-parse", sha+"^{tree}")
	if gotTree != tree {
		t.Errorf("tree = %q, want %q", gotTree, tree)
	}
	gotSubject := runGitOut(t, dir, "log", "-1", "--format=%s", sha)
	if gotSubject != "Merge side" {
		t.Errorf("subject = %q, want %q", gotSubject, "Merge side")
	}
	// Unreferenced: no branch or ref points at it.
	if out, err := exec.Command("git", "-C", dir, "branch", "--contains", sha).Output(); err != nil || strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected the constructed commit to be unreferenced; branch --contains = %q err=%v", out, err)
	}
}

func TestCommitTreeMerge_BadTreeFails(t *testing.T) {
	dir := initGitRepo(t)
	parent := runGitOut(t, dir, "rev-parse", "HEAD")
	if _, err := CommitTreeMerge(dir, strings.Repeat("0", 40), parent, parent, "Merge x"); err == nil {
		t.Fatal("expected an error for a non-existent tree SHA, got nil")
	}
}

func TestCommitTreeMerge_OptionLikeOperandRejected(t *testing.T) {
	dir := initGitRepo(t)
	parent := runGitOut(t, dir, "rev-parse", "HEAD")
	tree := runGitOut(t, dir, "rev-parse", "HEAD^{tree}")
	if _, err := CommitTreeMerge(dir, tree, "--evil", parent, "Merge x"); err == nil {
		t.Fatal("expected rejectOptionLike to refuse an option-like parent operand")
	}
}

func TestResetSoft_HappyPath(t *testing.T) {
	dir := initGitRepo(t)
	before := runGitOut(t, dir, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(dir, "reset-soft.txt"), []byte("staged content\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second commit")
	after := runGitOut(t, dir, "rev-parse", "HEAD")
	if before == after {
		t.Fatal("fixture invariant broken: HEAD must have moved")
	}

	if err := ResetSoft(dir, before); err != nil {
		t.Fatalf("ResetSoft: %v", err)
	}
	if got := runGitOut(t, dir, "rev-parse", "HEAD"); got != before {
		t.Errorf("HEAD after ResetSoft = %q, want %q", got, before)
	}
	// reset --soft touches neither the index nor the working tree: the
	// second commit's tree content must still be staged.
	status := runGitOut(t, dir, "status", "--porcelain")
	if status == "" {
		t.Error("expected staged changes (the moved-past commit's tree) after reset --soft, got a clean status")
	}
}

func TestResetSoft_BadTargetFails(t *testing.T) {
	dir := initGitRepo(t)
	if err := ResetSoft(dir, strings.Repeat("0", 40)); err == nil {
		t.Fatal("expected an error for a non-existent target SHA, got nil")
	}
}

func TestResetSoft_OptionLikeTargetRejected(t *testing.T) {
	dir := initGitRepo(t)
	if err := ResetSoft(dir, "--evil"); err == nil {
		t.Fatal("expected rejectOptionLike to refuse an option-like target")
	}
}

func TestUpdateRef_HappyPath(t *testing.T) {
	dir := initGitRepo(t)
	sha := runGitOut(t, dir, "rev-parse", "HEAD")
	if err := UpdateRef(dir, "refs/mindspec/test-marker", sha); err != nil {
		t.Fatalf("UpdateRef: %v", err)
	}
	got := runGitOut(t, dir, "rev-parse", "--verify", "refs/mindspec/test-marker")
	if got != sha {
		t.Errorf("refs/mindspec/test-marker = %q, want %q", got, sha)
	}
}

func TestUpdateRef_MalformedShaFails(t *testing.T) {
	dir := initGitRepo(t)
	// A well-formed-but-nonexistent 40-hex-char SHA is accepted by `git
	// update-ref` for a NEW ref (it does not require the object to
	// already exist) — the genuine failure mode is a malformed operand.
	if err := UpdateRef(dir, "refs/mindspec/test-marker", "not-a-valid-sha"); err == nil {
		t.Fatal("expected an error for a malformed SHA, got nil")
	}
}

func TestUpdateRef_OptionLikeOperandsRejected(t *testing.T) {
	dir := initGitRepo(t)
	sha := runGitOut(t, dir, "rev-parse", "HEAD")
	if err := UpdateRef(dir, "--evil", sha); err == nil {
		t.Fatal("expected rejectOptionLike to refuse an option-like ref name")
	}
	if err := UpdateRef(dir, "refs/mindspec/test-marker", "--evil"); err == nil {
		t.Fatal("expected rejectOptionLike to refuse an option-like sha")
	}
}

func TestMergeSourceMarkerRef_KeyedBySourceName(t *testing.T) {
	if got, want := MergeSourceMarkerRef("bead/mindspec-x.1"), "refs/mindspec/merge-source/bead/mindspec-x.1"; got != want {
		t.Errorf("MergeSourceMarkerRef = %q, want %q", got, want)
	}
}

func TestMergeInto_RecordsMergeSourceMarkerBeforeMerging(t *testing.T) {
	dir := initGitRepo(t)
	runGit(t, dir, "checkout", "-b", "source-branch")
	os.WriteFile(filepath.Join(dir, "s.txt"), []byte("source\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "source work")
	sourceTip := runGitOut(t, dir, "rev-parse", "source-branch")
	runGit(t, dir, "checkout", "main")

	if err := MergeInto(dir, "source-branch"); err != nil {
		t.Fatalf("MergeInto: %v", err)
	}

	got, err := RevParseRef(dir, MergeSourceMarkerRef("source-branch"))
	if err != nil {
		t.Fatalf("reading the merge-start marker: %v", err)
	}
	if got != sourceTip {
		t.Errorf("merge-start marker = %q, want the source tip %q", got, sourceTip)
	}
}

func TestMergeBranch_RecordsMergeSourceMarkerBeforeMerging(t *testing.T) {
	dir := initGitRepo(t)
	runGit(t, dir, "branch", "target-branch")
	runGit(t, dir, "checkout", "-b", "source-branch2")
	os.WriteFile(filepath.Join(dir, "s2.txt"), []byte("source2\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "source2 work")
	sourceTip := runGitOut(t, dir, "rev-parse", "source-branch2")
	runGit(t, dir, "checkout", "main")

	if err := MergeBranch(dir, "source-branch2", "target-branch"); err != nil {
		t.Fatalf("MergeBranch: %v", err)
	}

	got, err := RevParseRef(dir, MergeSourceMarkerRef("source-branch2"))
	if err != nil {
		t.Fatalf("reading the merge-start marker: %v", err)
	}
	if got != sourceTip {
		t.Errorf("merge-start marker = %q, want the source tip %q", got, sourceTip)
	}
}

func TestMergeMsgSubject_BadWorkdirFails(t *testing.T) {
	if _, err := MergeMsgSubject(t.TempDir()); err == nil {
		t.Fatal("expected an error resolving MERGE_MSG outside a git repo, got nil")
	}
}

func TestCommitNoEdit_NoMergeInProgressFails(t *testing.T) {
	dir := initGitRepo(t)
	if err := CommitNoEdit(dir); err == nil {
		t.Fatal("expected an error committing --no-edit with no merge/MERGE_MSG in progress, got nil")
	}
}

// runGit runs a git command in dir with a deterministic committer
// identity, mirroring initGitRepo's own inline helper (kept here as a
// package-level helper so every test in this file can share it without
// redeclaring the closure).
func runGit(t *testing.T, dir string, args ...string) {
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

// runGitOut is runGit for a read-only command whose trimmed stdout is
// needed by the caller.
func runGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}
