//go:build !windows

package panel

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
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
