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
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Store_MergedMetadataBound(t *testing.T) {
	for _, cas := range []bool{false, true} {
		t.Run(fmt.Sprintf("cas=%t", cas), func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
			gate := f.Items[0] + ".review"
			id := gate + ".r1.verify"
			f.Create(id, gate)
			f.Must(f.Store.WriteMetadata(id, map[string]any{"first": strings.Repeat("x", 9<<20)}))
			updates := map[string]any{"second": strings.Repeat("y", 9<<20)}
			r := actorRecord(t, f, id)
			merged := map[string]any{}
			for k, v := range r.Metadata {
				merged[k] = v
			}
			for k, v := range updates {
				merged[k] = v
			}
			raw, err := json.Marshal(merged)
			f.Must(err)
			calls := len(f.Fake.Calls)
			if cas {
				err = f.Store.SetStatusMetadataIf(id, "open", "in_progress", updates)
			} else {
				err = f.Store.WriteMetadata(id, updates)
			}
			for _, word := range []string{"metadata-too-large", gate, id, "first", fmt.Sprintf("size %d bytes", len(raw)), "\nrecovery: halt the spec"} {
				if err == nil || !strings.Contains(err.Error(), word) {
					t.Fatalf("merged bound missing %q: %v", word, err)
				}
			}
			if !strings.HasSuffix(err.Error(), "\nrecovery: halt the spec") || strings.Count(err.Error(), "recovery:") != 1 {
				t.Fatalf("wrong recovery for stored metadata: %v", err)
			}
			for _, args := range f.Fake.Calls[calls:] {
				if args[0] != "list" {
					t.Fatalf("merged metadata bound called bd write: %v", args)
				}
			}
			if !jsonEqual(r, actorRecord(t, f, id)) {
				t.Fatal("merged bound changed stored metadata or status")
			}
		})
	}
}

func TestS134Store_TempDirectory(t *testing.T) {
	for _, mode := range []string{"symlink", "permissions", "file", "inside", "case-alias", "data-volume", "link-in-tree", "link-parent-spelling", "link-target", "missing"} {
		t.Run(mode, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "repo")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			base := t.TempDir()
			t.Setenv("TMPDIR", base)
			dir, err := kernel.KernelTempDir(root)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "permissions":
				if runtime.GOOS == "windows" {
					t.Skip("Unix permission bits")
				}
				if err := os.Chmod(dir, 0750); err != nil {
					t.Fatal(err)
				}
			case "symlink", "file":
				if err := os.Rename(dir, dir+"-original"); err != nil {
					t.Fatal(err)
				}
				if mode == "symlink" {
					err = os.Symlink(root, dir)
				} else {
					err = os.WriteFile(dir, []byte("file"), 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "missing":
				t.Setenv("TMPDIR", filepath.Join(base, "missing"))
			case "inside", "case-alias", "data-volume", "link-in-tree", "link-parent-spelling", "link-target":
				inside := filepath.Join(root, "temp")
				if err := os.Mkdir(inside, 0700); err != nil {
					t.Fatal(err)
				}
				supplied := inside
				if mode == "case-alias" {
					supplied = filepath.Join(filepath.Dir(root), "REPO", "temp")
				}
				if mode == "data-volume" {
					if runtime.GOOS != "darwin" {
						t.Skip("macOS data-volume alias")
					}
					physical, err := filepath.EvalSymlinks(inside)
					if err != nil {
						t.Fatal(err)
					}
					supplied = "/System/Volumes/Data" + physical
				}
				if mode == "link-target" {
					supplied = filepath.Join(base, "alias")
					if err := os.Symlink(inside, supplied); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "link-in-tree" {
					supplied = filepath.Join(root, "external-temp")
					if err := os.Symlink(base, supplied); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "link-parent-spelling" {
					child := filepath.Join(base, "one", "two")
					if err := os.MkdirAll(child, 0700); err != nil {
						t.Fatal(err)
					}
					alias := filepath.Join(root, "alias")
					if err := os.Symlink(child, alias); err != nil {
						t.Fatal(err)
					}
					supplied = alias + string(filepath.Separator) + ".." + string(filepath.Separator) + ".."
				}
				if _, err := os.Stat(supplied); os.IsNotExist(err) {
					t.Skip("filesystem does not provide this alias")
				}
				t.Setenv("TMPDIR", supplied)
			}
			_, err = kernel.KernelTempDir(root)
			if mode == "missing" && (err == nil || !strings.Contains(err.Error(), "missing or unreadable")) {
				t.Fatalf("missing TMPDIR cause: %v", err)
			}
			if err == nil || !strings.Contains(err.Error(), "recovery: set TMPDIR to") || strings.Contains(err.Error(), "mindspec next-action") {
				t.Fatalf("unsafe temporary directory accepted or unusable recovery: %v", err)
			}
		})
	}
}

func TestS134Store_SweepFailureAllowsPass(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	dir, err := kernel.KernelTempDir(f.Root)
	f.Must(err)
	f.Must(os.Rename(dir, filepath.Join(t.TempDir(), "original")))
	f.Must(os.WriteFile(dir, []byte("unusable"), 0600))
	_, err = kernel.Pass(kernel.NewRegistry(), f.State(), true)
	if err != nil {
		t.Fatalf("cleanup failure blocked apply pass: %v", err)
	}
	calls := len(f.Fake.Calls)
	err = f.Store.WriteMetadata(f.Epic, map[string]any{"write": "must refuse"})
	if err == nil {
		t.Fatal("metadata write accepted unusable temporary directory")
	}
	for _, args := range f.Fake.Calls[calls:] {
		if args[0] != "list" {
			t.Fatalf("unusable directory allowed bd write: %v", args)
		}
	}
}
