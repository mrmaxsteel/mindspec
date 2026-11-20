package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindSlotRoot(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	slot := SlotRoot{Spec: "test", Slug: "p", Round: 1, Slot: "R1", MainRoot: root}
	dir := filepath.Join(root, "scratch", "test", "p", "r1", "R1")
	write := func(dir string, s SlotRoot) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, "tree", "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "slot.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(dir, slot)
	resolve := func(main string) (string, error) {
		if main != root {
			t.Fatalf("wrong main root %s", main)
		}
		return "scratch", nil
	}
	found, ok, err := FindSlotRoot(filepath.Join(dir, "tree", "sub"), resolve)
	if err != nil || !ok || found.MainRoot != root || found.Dir != dir || found.Slot != "R1" {
		t.Fatalf("root: %+v %v %v", found, ok, err)
	}
	// A nested slot.json without a tree sibling is not a slot marker pair.
	if err := os.WriteFile(filepath.Join(dir, "tree", "sub", "slot.json"), []byte(`{"slot":"wrong"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	found, ok, err = FindSlotRoot(filepath.Join(dir, "tree", "sub"), resolve)
	if err != nil || !ok || found.Dir != dir {
		t.Fatalf("nested decoy: %+v %v %v", found, ok, err)
	}
	for _, name := range []string{"wrong slot", "outside isolation", "wrong round", "wrong slug", "wrong spec", "relative root"} {
		t.Run(name, func(t *testing.T) {
			bad := slot
			path := dir
			switch name {
			case "wrong slot":
				bad.Slot = "R2"
			case "outside isolation":
				path = filepath.Join(root, "copy")
			case "wrong round":
				bad.Round = 2
			case "wrong slug":
				bad.Slug = "q"
			case "wrong spec":
				bad.Spec = "other"
			case "relative root":
				bad.MainRoot = "relative"
			}
			write(path, bad)
			_, ok, err := FindSlotRoot(filepath.Join(path, "tree"), resolve)
			if err == nil || !ok || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("accepted bad slot: %v %v", ok, err)
			}
			write(dir, slot)
		})
	}
	if _, ok, err := FindSlotRoot(root, resolve); err != nil || ok {
		t.Fatalf("no slot: %v %v", ok, err)
	}
}

func TestFindSlotRoot_RepositoryBoundary(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "scratch", "test", "p", "r1", "R1")
	tree := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(tree, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(SlotRoot{Spec: "test", Slug: "p", Round: 1, Slot: "R1", MainRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "slot.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tree, ".mindspec"), 0700); err != nil {
		t.Fatal(err)
	}
	// A checkout can use either a git directory or a worktree git file.
	for _, gitFile := range []bool{false, true} {
		t.Run(fmt.Sprintf("git-file-%t", gitFile), func(t *testing.T) {
			marker := filepath.Join(tree, ".git")
			if gitFile {
				if err := os.Rename(marker, filepath.Join(dir, "saved-git")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, []byte("gitdir: ../saved-git\n"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(marker, 0700); err != nil {
				t.Fatal(err)
			}
			resolve := func(string) (string, error) { return "scratch", nil }
			for _, cwd := range []string{dir, tree, filepath.Join(tree, "sub")} {
				found, ok, err := FindSlotRoot(cwd, resolve)
				if err != nil || !ok || found.Dir != dir || found.MainRoot != root {
					t.Fatalf("own checkout %s: %+v %t %v", cwd, found, ok, err)
				}
			}
			for _, rel := range []string{"tmp/fixture", "tree/sub/fixture", "tree/sub/tree"} {
				nested := filepath.Join(dir, fmt.Sprintf("case-%t", gitFile), rel)
				if strings.HasPrefix(rel, "tree/") {
					nested = filepath.Join(tree, fmt.Sprintf("case-%t", gitFile), rel)
				}
				if err := os.MkdirAll(filepath.Join(nested, "sub"), 0700); err != nil {
					t.Fatal(err)
				}
				if gitFile {
					if err := os.WriteFile(filepath.Join(nested, ".git"), []byte("gitdir: /unused\n"), 0600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Mkdir(filepath.Join(nested, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
				for _, cwd := range []string{nested, filepath.Join(nested, "sub")} {
					called := false
					_, ok, err := FindSlotRoot(cwd, func(string) (string, error) { called = true; return "scratch", nil })
					if err != nil || ok || called {
						t.Errorf("nested repository captured at %s: found=%t resolved=%t err=%v", cwd, ok, called, err)
					}
					normal, err := FindRoot(cwd)
					if err != nil || normal != nested {
						t.Errorf("nested root: got %s want %s, err=%v", normal, nested, err)
					}
				}
			}
		})
	}
	t.Run("mindspec-only", func(t *testing.T) {
		for _, nested := range []string{filepath.Join(tree, "tmp", "fixture"), filepath.Join(tree, "tmp", "tree")} {
			if err := os.MkdirAll(filepath.Join(nested, ".mindspec"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(nested, "sub"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, cwd := range []string{nested, filepath.Join(nested, "sub")} {
				called := false
				_, found, err := FindSlotRoot(cwd, func(string) (string, error) { called = true; return "scratch", nil })
				if err != nil || found || called {
					t.Errorf("nested workspace captured at %s: found=%t resolved=%t err=%v", cwd, found, called, err)
				}
				if normal, err := FindRoot(cwd); err != nil || normal != nested {
					t.Errorf("nested root: got %s want %s, err=%v", normal, nested, err)
				}
			}
		}
	})
	scratch := filepath.Join(dir, "scratch")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := FindSlotRoot(scratch, func(string) (string, error) { return "scratch", nil }); err != nil || ok {
		t.Fatalf("slot sibling captured: found=%t err=%v", ok, err)
	}
}
