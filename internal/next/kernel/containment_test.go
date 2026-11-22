package kernel_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134Submit_RecordContainment(t *testing.T) {
	for _, mode := range []string{"moved", "locked-absent", "not-directory", "loop-error", "link-in-tree", "link-parent-target", "link-parent-spelling", "relative-link-parent", "case-alias", "data-volume", "separate-git-dir", "name-prefix"} {
		t.Run(mode, func(t *testing.T) {
			f, id, lease, data := recordFixture(t, "lens")
			path := recordFile(t, data)
			refuse := false
			switch mode {
			case "moved", "locked-absent", "not-directory", "loop-error":
				parent := filepath.Join(t.TempDir(), "registered")
				f.Must(os.Mkdir(parent, 0700))
				wt := filepath.Join(parent, "tree")
				reducerGit(t, f, "worktree", "add", "--detach", wt, "HEAD")
				if mode == "locked-absent" {
					reducerGit(t, f, "worktree", "lock", wt)
				}
				f.Must(os.Rename(wt, filepath.Join(t.TempDir(), "moved")))
				if mode == "not-directory" {
					f.Must(os.Rename(parent, parent+"-moved"))
					f.Must(os.WriteFile(parent, []byte("not a directory"), 0600))
				}
				if mode == "loop-error" {
					f.Must(os.Symlink(wt, wt))
					refuse = true
				}
			case "link-in-tree":
				alias := filepath.Join(f.Root, "alias.json")
				f.Must(os.Symlink(path, alias))
				path = alias
				refuse = true
			case "link-parent-target":
				child := filepath.Join(f.Root, "child")
				f.Must(os.Mkdir(child, 0700))
				f.Must(os.WriteFile(filepath.Join(f.Root, "record.json"), []byte(data), 0600))
				outside := filepath.Dir(path)
				alias := filepath.Join(outside, "alias")
				f.Must(os.Symlink(child, alias))
				path = alias + string(filepath.Separator) + ".." + string(filepath.Separator) + "record.json"
				refuse = true
			case "link-parent-spelling", "relative-link-parent":
				outside := filepath.Dir(path)
				child := filepath.Join(outside, "one", "two")
				f.Must(os.MkdirAll(child, 0700))
				alias := filepath.Join(f.Root, "alias")
				f.Must(os.Symlink(child, alias))
				// The clean spelling names another existing external record.
				f.Must(os.WriteFile(filepath.Join(filepath.Dir(f.Root), "record.json"), []byte(data), 0600))
				path = alias + string(filepath.Separator) + ".." + string(filepath.Separator) + ".." + string(filepath.Separator) + "record.json"
				if mode == "relative-link-parent" {
					path = "alias" + string(filepath.Separator) + ".." + string(filepath.Separator) + ".." + string(filepath.Separator) + "record.json"
				}
				refuse = true
			case "case-alias", "data-volume", "separate-git-dir":
				path = filepath.Join(f.Root, "record.json")
				f.Must(os.WriteFile(path, []byte(data), 0600))
				refuse = true
				if mode == "separate-git-dir" {
					reducerGit(t, f, "init", "--quiet", "--separate-git-dir", filepath.Join(t.TempDir(), "git"))
				} else {
					alias := filepath.Join(filepath.Dir(f.Root), strings.ToUpper(filepath.Base(f.Root)), "record.json")
					if mode == "data-volume" {
						if runtime.GOOS != "darwin" {
							t.Skip("requires the macOS data-volume alias")
						}
						physical, err := filepath.EvalSymlinks(path)
						f.Must(err)
						alias = "/System/Volumes/Data" + physical
					}
					a, err := os.Stat(alias)
					if os.IsNotExist(err) {
						t.Skip("filesystem does not provide this alias")
					}
					f.Must(err)
					b, err := os.Stat(path)
					f.Must(err)
					if !os.SameFile(a, b) {
						t.Fatal("alias does not name the same file")
					}
					path = alias
				}
			case "name-prefix":
				dir := f.Root + "-outside"
				f.Must(os.Mkdir(dir, 0700))
				path = filepath.Join(dir, "record.json")
				f.Must(os.WriteFile(path, []byte(data), 0600))
			}
			before := actorRecord(t, f, id)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path})
			after := actorRecord(t, f, id)
			if refuse {
				if err == nil || after.Status == "closed" || !jsonEqual(before, after) {
					t.Fatalf("contained or unreadable record accepted or wrote the step: %v", err)
				}
				if mode != "loop-error" && !strings.Contains(err.Error(), "outside every worktree") {
					t.Fatalf("wrong containment refusal: %v", err)
				}
			} else {
				f.Must(err)
				if after.Status != "closed" || after.Metadata[kernel.KeyRecord] != data {
					t.Fatalf("external record did not close: %+v", after)
				}
				f.Must(kernel.ValidReceipt(f.Root, actorReceipt(t, f, id), after.Metadata))
			}
		})
	}
}

func TestS134Submit_RecordReadBound(t *testing.T) {
	f, id, lease, _ := recordFixture(t, "verify")
	path := filepath.Join(t.TempDir(), "large.json")
	file, err := os.Create(path)
	f.Must(err)
	size := int64(kernel.MetadataLimit + 2048)
	f.Must(file.Truncate(size))
	f.Must(file.Close())
	before, err := json.Marshal(actorRecord(t, f, id))
	f.Must(err)
	for _, step := range []string{id, f.ID(id)} {
		_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: step, Lease: lease.Token, Record: path})
		for _, text := range []string{"metadata-too-large", f.Items[0] + ".review", id, kernel.KeyRecord, fmt.Sprintf("record size %d bytes", size)} {
			if err == nil || !strings.Contains(err.Error(), text) {
				t.Fatalf("record size refusal for %s missing %q: %v", step, text, err)
			}
		}
	}
	after, marshalErr := json.Marshal(actorRecord(t, f, id))
	f.Must(marshalErr)
	if string(after) != string(before) {
		t.Fatal("oversized record wrote the step or mindspec_closing")
	}
}
