//go:build !windows

package kernel

import (
	"os"
	"syscall"
	"testing"
)

type tempOwnerInfo struct {
	os.FileInfo
	stat syscall.Stat_t
}

func (i tempOwnerInfo) Sys() any { return &i.stat }

func TestS134Store_TempDirectoryOwner(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	current := uint32(os.Geteuid())
	if !privateTempDir(tempOwnerInfo{FileInfo: info, stat: syscall.Stat_t{Uid: current}}) {
		t.Fatal("current owner refused")
	}
	if privateTempDir(tempOwnerInfo{FileInfo: info, stat: syscall.Stat_t{Uid: current + 1}}) {
		t.Fatal("other owner accepted")
	}
}
