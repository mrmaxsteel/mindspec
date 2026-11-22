//go:build !windows

package kernel

import (
	"os"
	"syscall"
)

func privateTempDir(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return info.Mode().Perm()&0077 == 0 && ok && int64(stat.Uid) == int64(os.Geteuid())
}

func processRunning(pid int) bool { return syscall.Kill(pid, 0) != syscall.ESRCH }
