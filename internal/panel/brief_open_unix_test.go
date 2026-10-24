//go:build !windows

package panel

import (
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
