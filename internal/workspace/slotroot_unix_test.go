//go:build !windows

package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFindSlotRoot_UnsafeMetadata(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "directory", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "scratch", "test", "p", "r1", "R1")
			if err := os.MkdirAll(filepath.Join(dir, "tree", "sub"), 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "slot.json")
			raw, err := json.Marshal(SlotRoot{Spec: "test", Slug: "p", Round: 1, Slot: "R1", MainRoot: root})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "fifo":
				err = syscall.Mkfifo(path, 0o600)
			case "symlink":
				target := filepath.Join(root, "metadata.json")
				if err := os.WriteFile(target, raw, 0o600); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(target, path)
			case "directory":
				err = os.Mkdir(path, 0o700)
			case "oversize":
				err = os.WriteFile(path, append(raw, []byte(strings.Repeat(" ", 64<<10))...), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			type result struct {
				found bool
				err   error
			}
			done := make(chan result, 1)
			go func() {
				_, found, err := FindSlotRoot(filepath.Join(dir, "tree", "sub"), func(string) (string, error) { return "scratch", nil })
				done <- result{found, err}
			}()
			select {
			case got := <-done:
				if !got.found || got.err == nil || !strings.Contains(got.err.Error(), "recovery:") {
					t.Fatalf("unsafe metadata needs refusal with recovery: found=%v err=%v", got.found, got.err)
				}
			case <-time.After(300 * time.Millisecond):
				// Release and join the old blocking implementation before failing.
				pipe, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = pipe.Write(raw)
				_ = pipe.Close()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("metadata reader did not stop")
				}
				t.Fatal("slot discovery blocked on FIFO metadata")
			}
		})
	}
}
