//go:build !windows

package phase

// phaselock_unix.go — the unix (darwin/linux) kernel-held advisory lock
// for the phase write lock: syscall.Flock with LOCK_EX. flock(2)
// associates the lock with the OPEN FILE DESCRIPTION, so a crashed
// holder's lock is released by the kernel when its file descriptors
// close (process exit) — no PID/timestamp content, no takeover, no
// deletion step. Mirrors internal/journal/lock_unix.go's primitive
// exactly; this file adds the BOUNDED poll-with-deadline wait that
// journal's blocking LOCK_EX does not need (journal's reports lock has
// no operator-facing timeout contract).

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// acquirePhaseFileLock opens (creating 0600 if absent) the lockfile at
// path and takes an exclusive, cross-process advisory lock (LOCK_EX),
// polling with LOCK_NB at pollInterval until either the lock is acquired
// or wait elapses. On timeout it returns an error wrapping
// ErrPhaseLockTimeout. It returns an unlock func that releases the lock
// and closes the descriptor.
func acquirePhaseFileLock(path string, wait, pollInterval time.Duration) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lockfile: %w", err)
	}
	// Re-assert 0600 in case the lockfile pre-existed under a permissive
	// umask.
	_ = f.Chmod(0o600)

	deadline := time.Now().Add(wait)
	for {
		if acquireAttemptHook != nil {
			acquireAttemptHook()
		}
		// gosec G115 (pre-existing class, internal/journal/lock_unix.go's
		// identical flock idiom): os.File.Fd() on a freshly-opened file
		// descriptor is always a small non-negative int value in
		// practice; the uintptr->int conversion cannot meaningfully
		// overflow for a real process fd table.
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) //nolint:gosec // G115: real fd values never approach int overflow range
		if err == nil {
			return func() {
				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // G115: see acquisition above
				_ = f.Close()
			}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			_ = f.Close()
			return nil, fmt.Errorf("flock LOCK_EX: %w", err)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("%w: %s", ErrPhaseLockTimeout, path)
		}
		time.Sleep(pollInterval)
	}
}
