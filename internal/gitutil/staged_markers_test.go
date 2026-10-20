package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// markerMerge builds a real repo: base, then HEAD (main) and a "side"
// branch each change the files given, then `git merge side` is left in
// progress and its conflicted paths are recorded, as mindspec does when
// its own merge stops. setup runs right after `git init` (for
// attributes or config).
func markerMerge(t *testing.T, setup func(dir string), base, ours, theirs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	if setup != nil {
		setup(dir)
	}
	write := func(files map[string]string) {
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		runGit(t, dir, "add", "-A")
	}
	write(base)
	runGit(t, dir, "commit", "-q", "-m", "base")
	runGit(t, dir, "checkout", "-q", "-b", "side")
	write(theirs)
	runGit(t, dir, "commit", "-q", "-m", "theirs")
	runGit(t, dir, "checkout", "-q", "main")
	write(ours)
	runGit(t, dir, "commit", "-q", "-m", "ours")
	cmd := exec.Command("git", "-C", dir, "merge", "side")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if err := cmd.Run(); err == nil {
		t.Fatal("fixture: the merge must conflict")
	}
	if !MergeInProgress(dir) {
		t.Fatal("fixture: the merge must be in progress")
	}
	if err := RecordMergeConflicts(dir); err != nil {
		t.Fatalf("fixture: recording the conflicted paths: %v", err)
	}
	return dir
}

func stageMarkerFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "--", name)
}

func assertMarkerFiles(t *testing.T, dir string, want []string) {
	t.Helper()
	if len(ConflictedFiles(dir)) != 0 {
		t.Fatal("fixture: no unmerged entries may remain after staging")
	}
	got := StagedConflictMarkerFiles(dir)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StagedConflictMarkerFiles = %q, want %q", got, want)
	}
}

// Git's own conflict output with conflict-marker-size=10, staged as-is.
func TestStagedConflictMarkerFiles_ConflictMarkerSizeAttribute(t *testing.T) {
	dir := markerMerge(t, nil,
		map[string]string{"f": "base\n", ".gitattributes": "f conflict-marker-size=10\n"},
		map[string]string{"f": "ours\n"}, map[string]string{"f": "theirs\n"})
	runGit(t, dir, "add", "--", "f")
	assertMarkerFiles(t, dir, []string{"f"})
}

// A full conflict block with bare markers and CRLF line endings.
func TestStagedConflictMarkerFiles_CRLFBlock(t *testing.T) {
	dir := markerMerge(t, nil, map[string]string{"f": "base\r\n"},
		map[string]string{"f": "ours\r\n"}, map[string]string{"f": "theirs\r\n"})
	stageMarkerFile(t, dir, "f", "<<<<<<<\r\nours\r\n=======\r\ntheirs\r\n>>>>>>>\r\n")
	assertMarkerFiles(t, dir, []string{"f"})
}

// The block's opening comes from HEAD and its closing from MERGE_HEAD,
// but the assembled block exists in neither parent.
func TestStagedConflictMarkerFiles_MixedParentBlock(t *testing.T) {
	dir := markerMerge(t, nil, map[string]string{"f": "base\n"},
		map[string]string{"f": "<<<<<<< HEAD\nours\n"}, map[string]string{"f": "theirs\n>>>>>>> side\n"})
	stageMarkerFile(t, dir, "f", "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> side\n")
	assertMarkerFiles(t, dir, []string{"f"})
}

// diff.interHunkContext must not change what is detected.
func TestStagedConflictMarkerFiles_InterHunkContextConfig(t *testing.T) {
	dir := markerMerge(t, func(dir string) { runGit(t, dir, "config", "diff.interHunkContext", "10") },
		map[string]string{"f": "base\nkeep\nrest\n"},
		map[string]string{"f": "ours\nkeep\nrest\n"}, map[string]string{"f": "theirs\nkeep\nrest\n"})
	stageMarkerFile(t, dir, "f", "ours\nkeep\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> side\nrest\n")
	assertMarkerFiles(t, dir, []string{"f"})
}

// A conflicted file name with a space is reported exactly.
func TestStagedConflictMarkerFiles_PathWithSpace(t *testing.T) {
	dir := markerMerge(t, nil, map[string]string{"space file": "base\n"},
		map[string]string{"space file": "ours\n"}, map[string]string{"space file": "theirs\n"})
	runGit(t, dir, "add", "--", "space file")
	assertMarkerFiles(t, dir, []string{"space file"})
}

// One-side content is never scanned; a lone separator does not flag.
func TestStagedConflictMarkerFiles_SetextAndOneSideFilesPass(t *testing.T) {
	dir := markerMerge(t, nil, map[string]string{"f": "base\n"},
		map[string]string{"f": "ours\n"},
		map[string]string{"f": "theirs\n", "README.md": "Summary\n=======\n", "doc.md": "<<<<<<< a\nx\n=======\ny\n>>>>>>> b\n"})
	stageMarkerFile(t, dir, "f", "Merged\n=======\nours\ntheirs\n")
	assertMarkerFiles(t, dir, nil)
}

// A rename/modify conflict sits at the rename destination.
func TestStagedConflictMarkerFiles_RenameModify(t *testing.T) {
	keep := strings.Repeat("keep\n", 30)
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("f", "base\n"+keep)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	runGit(t, dir, "checkout", "-q", "-b", "side")
	write("f", "theirs\n"+keep)
	runGit(t, dir, "commit", "-q", "-am", "theirs")
	runGit(t, dir, "checkout", "-q", "main")
	write("f", "ours\n"+keep)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "mv", "f", "renamed")
	runGit(t, dir, "commit", "-q", "-m", "ours+rename")
	if err := exec.Command("git", "-C", dir, "merge", "side").Run(); err == nil {
		t.Fatal("fixture: the merge must conflict")
	}
	if err := RecordMergeConflicts(dir); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "--", "renamed")
	assertMarkerFiles(t, dir, []string{"renamed"})
}

// Criss-cross history: two merge bases; git's recursive merge still
// conflicts in f, and the staged conflict output is flagged.
func TestStagedConflictMarkerFiles_CrissCross(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	write := func(content string) {
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", "f")
	}
	write("base\n")
	runGit(t, dir, "commit", "-q", "-m", "base")
	runGit(t, dir, "checkout", "-q", "-b", "side")
	write("theirs\n")
	runGit(t, dir, "commit", "-q", "-m", "B")
	runGit(t, dir, "checkout", "-q", "main")
	write("ours\n")
	runGit(t, dir, "commit", "-q", "-m", "A")
	a := runGitOut(t, dir, "rev-parse", "main")
	b := runGitOut(t, dir, "rev-parse", "side")
	runGit(t, dir, "merge", "-q", "-s", "ours", "-m", "M1", b)
	runGit(t, dir, "checkout", "-q", "side")
	runGit(t, dir, "merge", "-q", "-s", "ours", "-m", "M2", a)
	runGit(t, dir, "checkout", "-q", "main")
	if bases := runGitOut(t, dir, "merge-base", "--all", "main", "side"); len(strings.Fields(bases)) != 2 {
		t.Fatalf("fixture: want two merge bases, got %q", bases)
	}
	if err := exec.Command("git", "-C", dir, "merge", "side").Run(); err == nil {
		t.Fatal("fixture: the merge must conflict")
	}
	if err := RecordMergeConflicts(dir); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "--", "f")
	assertMarkerFiles(t, dir, []string{"f"})
}

// fallbackMerge leaves a conflict in f plus a side-only doc.md that
// documents a full marker block. Only the fallback scan flags doc.md.
func fallbackMerge(t *testing.T) string {
	t.Helper()
	dir := markerMerge(t, nil, map[string]string{"f": "base\n"},
		map[string]string{"f": "ours\n"},
		map[string]string{"f": "theirs\n", "doc.md": "<<<<<<< a\nx\n=======\ny\n>>>>>>> b\n"})
	runGit(t, dir, "add", "--", "f")
	return dir
}

// With a valid record, only the recorded path is scanned.
func TestStagedConflictMarkerFiles_RecordLimitsScan(t *testing.T) {
	dir := fallbackMerge(t)
	if paths, ok := MergeConflictRecord(dir); !ok || !reflect.DeepEqual(paths, []string{"f"}) {
		t.Fatalf("record = %q, %v; want [f], true", paths, ok)
	}
	assertMarkerFiles(t, dir, []string{"f"})
}

// Record missing: every path staged against HEAD is scanned.
func TestStagedConflictMarkerFiles_MissingRecordFallsBack(t *testing.T) {
	dir := fallbackMerge(t)
	ClearMergeConflictRecord(dir)
	if _, ok := MergeConflictRecord(dir); ok {
		t.Fatal("record must be gone")
	}
	assertMarkerFiles(t, dir, []string{"doc.md", "f"})
}

// Record from another merge (stale MERGE_HEAD): ignored, fallback used.
func TestStagedConflictMarkerFiles_StaleRecordIgnored(t *testing.T) {
	dir := fallbackMerge(t)
	path, err := mergeConflictRecordPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	stale := runGitOut(t, dir, "rev-parse", "HEAD") + "\nf\x00"
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := MergeConflictRecord(dir); ok {
		t.Fatal("a record for another MERGE_HEAD must be ignored")
	}
	assertMarkerFiles(t, dir, []string{"doc.md", "f"})
}

// The record is removed when the merge is concluded or aborted.
func TestMergeConflictRecord_ClearedOnCommitAndAbort(t *testing.T) {
	dir := fallbackMerge(t)
	if err := AbortMerge(dir); err != nil {
		t.Fatal(err)
	}
	if p, _ := mergeConflictRecordPath(dir); fileExistsT(p) {
		t.Error("abort must remove the record")
	}
	dir = markerMerge(t, nil, map[string]string{"f": "base\n"},
		map[string]string{"f": "ours\n"}, map[string]string{"f": "theirs\n"})
	stageMarkerFile(t, dir, "f", "resolved\n")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	if err := CommitNoEdit(dir); err != nil {
		t.Fatal(err)
	}
	if p, _ := mergeConflictRecordPath(dir); fileExistsT(p) {
		t.Error("a successful merge commit must remove the record")
	}
}

func fileExistsT(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
