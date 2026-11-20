package kernel_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func subjectFilesCommit(t *testing.T, f *kerneltest.Fixture, branch string, files map[string][]byte) string {
	t.Helper()
	base := reducerGit(t, f, "rev-parse", branch)
	prior, had := os.LookupEnv("GIT_INDEX_FILE")
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	defer func() {
		if had {
			f.Must(os.Setenv("GIT_INDEX_FILE", prior))
		} else {
			f.Must(os.Unsetenv("GIT_INDEX_FILE"))
		}
	}()
	reducerGit(t, f, "read-tree", base)
	for name, data := range files {
		file := filepath.Join(t.TempDir(), "blob")
		f.Must(os.WriteFile(file, data, 0600))
		blob := reducerGit(t, f, "hash-object", "-w", file)
		reducerGit(t, f, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+filepath.ToSlash(name))
	}
	tree := reducerGit(t, f, "write-tree")
	head := reducerGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit-tree", tree, "-p", base, "-m", "subject files")
	reducerGit(t, f, "update-ref", "refs/heads/"+branch, head)
	return head
}

func TestS134Subject_RegistrationBoundary(t *testing.T) {
	for _, final := range []bool{false, true} {
		for _, mode := range []string{"untracked-panel", "untracked-ledger", "untracked-same-slug", "untracked-link-panel", "untracked-link-ledger", "ignored-panel", "leaf-physical", "leaf-lexical", "parent-lexical", "tracked-panel", "tracked-ledger", "stored-panel"} {
			t.Run(fmt.Sprintf("final=%t/%s", final, mode), func(t *testing.T) {
				f := reducerFixture(t)
				gate, branch, slug := f.Items[0]+".review", "bead/"+f.Items[0], "panel"
				if final {
					gate, branch, slug = f.Epic+".final", "spec/"+f.SpecID, "review"
					for _, item := range f.Items {
						doneItem(t, f, item)
					}
					registerGate(t, f, gate, slug, 1)
				} else {
					passingPanel(t, f)
				}
				reduce(t, f, true)
				old := recoveryRound(t, f, gate)
				if !final {
					f.Must(kernel.CheckPassedGate(f.State(), gate))
				}
				base := f.Root
				segment := "review"
				if (final && mode != "untracked-same-slug") || mode == "stored-panel" {
					base, segment = f.SpecDir, "reviews"
				}
				dirSlug := "evil"
				if mode == "untracked-same-slug" || mode == "stored-panel" {
					dirSlug = slug
				}
				dir := filepath.Join(base, segment, dirSlug)
				name := "panel.json"
				if mode == "untracked-ledger" || mode == "tracked-ledger" || mode == "untracked-link-ledger" {
					name = "dispositions.jsonl"
				}
				data, err := os.ReadFile(filepath.Join(f.SpecDir, "reviews", slug, name))
				f.Must(err)
				f.Must(os.MkdirAll(dir, 0755))
				f.Must(os.WriteFile(filepath.Join(dir, name), data, 0644))
				rel, err := filepath.Rel(f.Root, dir)
				f.Must(err)
				tracked := mode == "tracked-panel" || mode == "tracked-ledger" || mode == "leaf-physical" || mode == "leaf-lexical" || mode == "parent-lexical"
				if tracked {
					subjectFilesCommit(t, f, branch, map[string][]byte{filepath.Join(rel, name): data})
				}
				if mode == "ignored-panel" {
					f.Must(os.WriteFile(filepath.Join(f.Root, ".git", "info", "exclude"), []byte(filepath.ToSlash(rel)+"/\n"), 0644))
				}

				changed := filepath.Join(rel, "code.go")
				if mode == "leaf-physical" || mode == "leaf-lexical" || mode == "untracked-link-panel" || mode == "untracked-link-ledger" {
					target := filepath.Join(f.Root, "src")
					f.Must(os.Rename(dir, target))
					f.Must(os.Symlink(target, dir))
					if mode == "leaf-physical" || mode == "untracked-link-panel" || mode == "untracked-link-ledger" {
						changed = "src/code.go"
					}
				}
				if mode == "parent-lexical" {
					parent := filepath.Dir(dir)
					f.Must(os.Rename(parent, parent+"-real"))
					f.Must(os.Symlink(parent+"-real", parent))
				}
				head := subjectFilesCommit(t, f, branch, map[string][]byte{changed: []byte("package source\n")})
				honest := mode == "tracked-panel" || mode == "tracked-ledger" || mode == "stored-panel"
				if !final {
					err := kernel.CheckPassedGate(f.State(), gate)
					if honest && err != nil {
						t.Fatalf("tracked panel-only commit refused: %v", err)
					}
					if !honest && err == nil {
						t.Fatal("unreviewed commit passed the bead gate")
					}
				}
				reduce(t, f, true)
				if final {
					next := recoveryRound(t, f, gate)
					if honest {
						if next.Round != old.Round || next.Subject != old.Subject {
							t.Fatalf("tracked panel-only commit moved subject: %+v", next)
						}
					} else if next.Round != old.Round+1 || next.Subject != head {
						t.Fatalf("unreviewed commit did not move final subject: %+v", next)
					}
				} else if !honest && !action(reduce(t, f, false), gate+".judge.a1") {
					t.Fatal("unreviewed commit did not create a change judge")
				}
			})
		}
	}
}
