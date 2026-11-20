package executor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

func TestS134Finalize_PendingSource(t *testing.T) {
	for _, location := range []string{"spec", "bead"} {
		for _, mode := range []string{"untracked", "modified", "deleted"} {
			t.Run(location+"/"+mode, func(t *testing.T) {
				g, fake, root := newRepoExecutor(t)
				writePendingFile(t, root, "source.go", "package source\n")
				runGitIn(t, root, "add", "source.go")
				runGitIn(t, root, "commit", "-qm", "source")
				accepted := refHash(t, root, "main")
				spec := "001-artifacts"
				branch := "spec/" + spec
				wt := filepath.Join(root, ".worktrees", "worktree-spec-"+spec)
				runGitIn(t, root, "worktree", "add", "-b", branch, wt)
				dirty := wt
				if location == "bead" {
					dirty = filepath.Join(root, ".worktrees", "worktree-bead-1")
					runGitIn(t, root, "worktree", "add", "-b", "bead/bead-1", dirty)
					writePendingFile(t, dirty, ".mindspec/specs/"+spec+"/note.md", "review note\n")
					runGitIn(t, dirty, "add", ".")
					runGitIn(t, dirty, "commit", "-qm", "bead evidence")
					fake.listEntries = []bead.WorktreeListEntry{{Name: "worktree-bead-1", Path: dirty, Branch: "bead/bead-1"}}
				}
				name := "source.go"
				if mode == "untracked" {
					name = "unreviewed.go"
				}
				if mode == "deleted" {
					if err := os.Rename(filepath.Join(dirty, name), filepath.Join(t.TempDir(), name)); err != nil {
						t.Fatal(err)
					}
				} else {
					writePendingFile(t, dirty, name, "package unreviewed\n")
				}
				before := refHash(t, root, branch)
				beadBefore := ""
				if location == "bead" {
					beadBefore = refHash(t, root, "bead/bead-1")
				}
				exports, writes := 0, 0
				execBeadExportFn = func(string) error { exports++; return nil }
				old := finalizeMetadataWriteFn
				finalizeMetadataWriteFn = func(string, map[string]interface{}) error { writes++; return nil }
				t.Cleanup(func() { finalizeMetadataWriteFn = old })
				_, err := g.FinalizeEpic("epic-artifacts", spec, branch, []string{"bead-1"}, "", false, false, FinalizeArtifactPolicy{SpecDir: filepath.Join(wt, ".mindspec/specs", spec), AcceptedSubject: accepted})
				if err == nil || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "normal workflow") {
					t.Fatalf("pending source not refused: %v", err)
				}
				if exports != 0 || writes != 0 || refHash(t, root, branch) != before || refHash(t, root, "main") != accepted {
					t.Fatalf("refusal wrote: exports=%d metadata=%d", exports, writes)
				}
				if location == "bead" && refHash(t, root, "bead/bead-1") != beadBefore {
					t.Fatal("refusal committed bead work")
				}
				runGitIn(t, dirty, "diff", "--cached", "--exit-code")
			})
		}
	}
}

func writePendingFile(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestS134Finalize_PendingArtifacts(t *testing.T) {
	t.Run("recorded", testFinalizeRecordedArtifacts)
	for _, name := range unregisteredFinalizePaths() {
		t.Run(name, func(t *testing.T) { testFinalizeUnregistered(t, name, false) })
	}
}

func testFinalizeRecordedArtifacts(t *testing.T) {
	g, fake, root := newRepoExecutor(t)
	const spec = "001-artifacts"
	branch := "spec/" + spec
	wt := filepath.Join(root, ".worktrees", "worktree-spec-"+spec)
	runGitIn(t, root, "worktree", "add", "-b", branch, wt)
	accepted := refHash(t, root, branch)
	names := []string{".mindspec/specs/" + spec + "/spec.md", ".mindspec/specs/" + spec + "/reviews/panel/note.md", "review/panel/note.md", "reviews/panel/note.md", ".mindspec/review/panel/note.md", ".mindspec/reviews/panel/note.md", ".beads/issues.jsonl"}
	for _, name := range names {
		writePendingFile(t, wt, name, "artifact\n")
	}
	// Stop after the artifact commit, before cleanup, to inspect the retained tree.
	old := finalizeStepHookFn
	finalizeStepHookFn = func(stage string) error {
		if stage == "auto_merge" {
			return errors.New("inspect artifacts")
		}
		return nil
	}
	t.Cleanup(func() { finalizeStepHookFn = old })
	fake.listEntries = nil
	_, err := g.FinalizeEpic("epic-artifacts", spec, branch, []string{}, "", false, false, FinalizeArtifactPolicy{SpecDir: filepath.Join(wt, ".mindspec/specs", spec), AcceptedSubject: accepted, PanelDirs: []string{"review/panel/", "reviews/panel/", ".mindspec/review/panel/", ".mindspec/reviews/panel/"}})
	if err == nil || !strings.Contains(err.Error(), "inspect artifacts") {
		t.Fatal(err)
	}
	if refHash(t, root, branch) == accepted {
		t.Fatal("pending artifacts were not committed")
	}
	for _, name := range names {
		runGitIn(t, root, "cat-file", "-e", branch+":"+name)
	}
}

func unregisteredFinalizePaths() []string {
	var names []string
	for _, dir := range []string{"review/evil", "reviews/evil", ".mindspec/review/evil", ".mindspec/reviews/evil", "reviews/other-spec-panel", ".mindspec/specs/999-other/reviews/panel"} {
		for _, file := range []string{"x.go", "x_test.go"} {
			names = append(names, dir+"/"+file)
		}
	}
	return names
}

func TestS134Finalize_PublicationContent(t *testing.T) {
	for _, name := range append([]string{"source.go"}, unregisteredFinalizePaths()...) {
		t.Run(name, func(t *testing.T) { testFinalizeUnregistered(t, name, true) })
	}
}

func testFinalizeUnregistered(t *testing.T, name string, committed bool) {
	t.Helper()
	g, _, root := newRepoExecutor(t)
	const spec = "001-artifacts"
	branch := "spec/" + spec
	accepted := refHash(t, root, "main")
	wt := filepath.Join(root, ".worktrees", "worktree-spec-"+spec)
	runGitIn(t, root, "worktree", "add", "-b", branch, wt)
	writePendingFile(t, wt, name, "package unreviewed\n")
	if committed {
		runGitIn(t, wt, "add", name)
		runGitIn(t, wt, "commit", "-qm", "unreviewed subject")
	}
	before := refHash(t, root, branch)
	exports, writes := 0, 0
	execBeadExportFn = func(string) error { exports++; return nil }
	old := finalizeMetadataWriteFn
	finalizeMetadataWriteFn = func(string, map[string]interface{}) error { writes++; return nil }
	t.Cleanup(func() { finalizeMetadataWriteFn = old })
	_, err := g.FinalizeEpic("epic-artifacts", spec, branch, []string{}, "", false, false, FinalizeArtifactPolicy{SpecDir: filepath.Join(wt, ".mindspec/specs", spec), AcceptedSubject: accepted, PanelDirs: []string{"review/honest/", "reviews/honest/"}})
	if err == nil || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "normal workflow") {
		t.Fatalf("unreviewed publication was not refused: %v", err)
	}
	if exports != 0 || writes != 0 || refHash(t, root, branch) != before || refHash(t, root, "main") != accepted {
		t.Fatalf("refusal wrote: exports=%d metadata=%d", exports, writes)
	}
	runGitIn(t, wt, "diff", "--cached", "--exit-code")
}
