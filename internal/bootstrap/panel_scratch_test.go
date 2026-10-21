package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func TestBootstrap_ScratchIgnoredExisting(t *testing.T) {
	for _, isolation := range []string{".mindspec-scratch", "review-scratch", "review/", "review[1]", "#review", "!review", "review*", "review?", "review  "} {
		t.Run(isolation, func(t *testing.T) {
			root := reviewBootstrapRepo(t)
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
				t.Fatal(err)
			}
			if isolation != ".mindspec-scratch" {
				if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("panel: {isolation_root: "+strconv.Quote(isolation)+"}\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			seed := "# existing project\n*.log\n"
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0644); err != nil {
				t.Fatal(err)
			}
			config.ResetCache()
			t.Cleanup(config.ResetCache)
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(before), seed) {
				t.Fatal("existing ignore text changed")
			}
			if err := os.MkdirAll(filepath.Join(root, isolation), 0755); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{isolation, ".mindspec/config.local.yaml"} {
				if (path == ".mindspec-scratch" || path == "review-scratch" || path == ".mindspec/config.local.yaml") && !strings.Contains(string(before), "\n"+path+"\n") {
					t.Fatalf("missing ignore %s: %s", path, before)
				}
				mustGit(t, root, "check-ignore", "--", path+"/probe")
			}
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("second init changed ignores")
			}
		})
	}
}

func TestBootstrap_ScratchRootRefused(t *testing.T) {
	for _, isolation := range []string{"scratch\n!.mindspec/config.local.yaml", "review\r", "./review", "review//x", `review\files`} {
		t.Run(strconv.Quote(isolation), func(t *testing.T) {
			root := reviewBootstrapRepo(t)
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("panel: {isolation_root: "+strconv.Quote(isolation)+"}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			seed := []byte("# existing project\n*.log\n")
			path := filepath.Join(root, ".gitignore")
			if err := os.WriteFile(path, seed, 0644); err != nil {
				t.Fatal(err)
			}
			config.ResetCache()
			t.Cleanup(config.ResetCache)
			_, err := Run(root, false)
			want := "\nrecovery: set panel.isolation_root to an untracked relative directory outside .git, .worktrees and .mindspec"
			if err == nil || !strings.Contains(err.Error(), "invalid panel.isolation_root "+strconv.Quote(isolation)) || !strings.HasSuffix(err.Error(), want) || strings.Count(err.Error(), "recovery:") != 1 {
				t.Errorf("init accepted root or returned wrong recovery: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(seed, after) {
				t.Errorf("refused init changed .gitignore: before %q; after %q", seed, after)
			}
		})
	}
}
