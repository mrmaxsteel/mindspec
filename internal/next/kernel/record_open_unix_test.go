//go:build !windows

package kernel

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestS134Submit_OpenedRecordFIFO(t *testing.T) {
	if name := os.Getenv("S134_RECORD_FIFO"); name != "" {
		if err := os.WriteFile(name, []byte("record"), 0600); err != nil {
			t.Fatal(err)
		}
		checked, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(name, name+"-original"); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(name, 0600); err != nil {
			t.Fatal(err)
		}
		_, err = readCheckedRecord("test-1.1.review.r1.lens", name, checked)
		if err == nil || !strings.Contains(err.Error(), "opened record is not a regular file") {
			t.Fatalf("opened FIFO was not refused as a nonregular file: %v", err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestS134Submit_OpenedRecordFIFO$")
	cmd.Env = append(os.Environ(), "S134_RECORD_FIFO="+filepath.Join(t.TempDir(), "record"))
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("opening a replaced FIFO blocked: %v %s", ctx.Err(), out)
	}
	if err != nil {
		t.Fatalf("FIFO probe failed: %v %s", err, out)
	}
}
