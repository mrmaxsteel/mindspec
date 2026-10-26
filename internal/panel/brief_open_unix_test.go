//go:build !windows

package panel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestReadBriefFileDescriptor(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBriefFile(fifo); err == nil {
		t.Fatal("accepted FIFO")
	}
	target := filepath.Join(dir, "body.md")
	body := "body\r\n\tunchanged\n"
	if err := os.WriteFile(target, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readBriefFile(link); err == nil {
		t.Fatal("accepted symlink")
	}
	if data, err := readBriefFile(target); err != nil || string(data) != body {
		t.Fatalf("regular body changed: %q %v", data, err)
	}
}

func TestReadFilledBriefFIFO(t *testing.T) {
	file := filepath.Join(t.TempDir(), "BRIEF.md")
	if err := syscall.Mkfifo(file, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ReadFilledBrief(file)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("expected regular-file refusal, got %v", err)
		}
	case <-time.After(3 * time.Second):
		// Release an old blocking reader and join it before failing.
		pipe, err := os.OpenFile(file, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		pipe.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("brief reader did not stop")
		}
		t.Fatal("panel brief reader blocked on FIFO")
	}
}

// Calling the open helper directly bypasses any preceding Lstat check. A
// helper implemented as os.Open would follow this link and fail this test.
func TestOpenBriefFileNoFollow(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "body"), filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("authored body"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	f, err := openBriefFile(link)
	if f != nil {
		f.Close()
	}
	if !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("open followed symlink: %v", err)
	}
}
