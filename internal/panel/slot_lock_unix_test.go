//go:build !windows

package panel

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlotLock_UnixProbeNoDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".locks", "R1.lock")
	if held, err := ProbeSlotLock(path); err != nil || held {
		t.Fatalf("absent probe: %v %v", held, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("probe created a file")
	}
	lock, err := AcquireSlotLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if held, err := ProbeSlotLock(path); err != nil || !held {
		t.Fatalf("held probe: %v %v", held, err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("probe waited")
	}
	if other, err := TrySlotLock(path); !errors.Is(err, ErrSlotLocked) {
		if other != nil {
			_ = other.Release()
		}
		t.Fatalf("second holder: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if held, err := ProbeSlotLock(path); err != nil || held {
		t.Fatalf("released probe: %v %v", held, err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("lock inode was deleted: %v", err)
	}
	other, err := AcquireSlotLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Release(); err != nil {
		t.Fatal(err)
	}
}
