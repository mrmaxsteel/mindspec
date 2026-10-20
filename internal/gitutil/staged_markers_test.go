package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// markerMerge builds a real repo: base, then HEAD (main) and a "side"
// branch each change the files given, then `git merge side` is left in
// progress. setup runs right after `git init` (for attributes or config).
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
