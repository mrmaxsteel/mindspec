package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS134Finalize_ArtifactSymlinks(t *testing.T) {
	for _, mode := range []string{"review-link", "outside-spec-link", "parent-link"} {
		t.Run(mode, func(t *testing.T) {
			g, _, root := newRepoExecutor(t)
			const spec = "001-artifacts"
			runGitIn(t, root, "branch", "spec/"+spec)
			wt := filepath.Join(root, ".worktrees", "worktree-spec-"+spec)
			if err := os.MkdirAll(filepath.Dir(wt), 0755); err != nil {
				t.Fatal(err)
			}
			runGitIn(t, root, "worktree", "add", wt, "spec/"+spec)
			dir := filepath.Join(wt, ".mindspec", "specs", spec, "reviews")
			if mode == "outside-spec-link" {
				dir = filepath.Join(wt, "src")
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			target := t.TempDir()
			if err := os.WriteFile(filepath.Join(target, "dispositions.jsonl"), []byte("evidence\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if mode == "parent-link" {
				dir = filepath.Join(dir, "nested")
			}
			if err := os.Symlink(target, filepath.Join(dir, "evil")); err != nil {
				if !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(dir, "evil")); err != nil {
					t.Fatal(err)
				}
			}
			before := refHash(t, root, "spec/"+spec)
			exports := 0
			execBeadExportFn = func(string) error { exports++; return nil }
			_, err := g.FinalizeEpic("epic-artifacts", spec, "spec/"+spec, nil, "", false, false)
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("artifact symlink was not refused: %v", err)
			}
			if exports != 0 || refHash(t, root, "spec/"+spec) != before {
				t.Fatal("refusal exported or committed artifacts")
			}
			runGitIn(t, wt, "diff", "--cached", "--exit-code")
		})
	}
}
