//go:build !windows

package journal

// lock_unix.go — the unix (darwin/linux, the install-script targets) OS-visible
// advisory lock for the reports.jsonl read-modify-write: syscall.Flock with
// LOCK_EX. This is a kernel-level, cross-process advisory lock held on the
// reports.lock descriptor for the whole read-modify-write; LOCK_UN + Close
// release it. It mirrors the safeio package's unix/windows syscall split.

import (
	"fmt"
	"os"
	"syscall"
)

// acquireFileLock opens (creating 0600 if absent) the lockfile at path and
// takes an exclusive, cross-process advisory lock (LOCK_EX, blocking until any
// other holder releases). It returns an unlock func that releases the lock and
// closes the descriptor. The lockfile is created with the same owner-only mode
// as every other store file (HC-8).
func acquireFileLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, fmt.Errorf("open lockfile: %w", err)
	}
	// Re-assert 0600 in case the lockfile pre-existed under a permissive umask.
	_ = f.Chmod(fileMode)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock LOCK_EX: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
