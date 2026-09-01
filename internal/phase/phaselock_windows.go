//go:build windows

package phase

// phaselock_windows.go — the Windows kernel-held advisory lock for the
// phase write lock: LockFileEx (exclusive byte-range lock) on a lockfile
// handle WITHOUT FILE_SHARE_DELETE, held for the whole critical section.
// Atomicity and ownership are the OS's; release is owner-only
// (UnlockFileEx + CloseHandle), and a crashed holder's lock releases with
// its process — mirroring flock semantics. This moots the stale-
// lockfile-content problem class entirely: the FILE is not the lock (the
// byte-range lock is), so a pre-existing empty/partial leftover lockfile
// never blocks acquisition — there is NO PID/timestamp content, NO
// takeover, and NO deletion step.
//
// Go's os.OpenFile on Windows (via syscall.Open) opens with share mode
// FILE_SHARE_READ|FILE_SHARE_WRITE — FILE_SHARE_DELETE is NOT included —
// so the plain os.OpenFile call below already satisfies the "retained
// handle without FILE_SHARE_DELETE" design requirement without a raw
// CreateFile call.
//
// Deliberately NOT internal/journal's O_EXCL-lockfile-with-retry idiom
// (lock_windows.go there creates then removes a lockfile as the mutual-
// exclusion primitive): that idiom leaves a stale lockfile behind a
// crashed holder, exactly the class this lock avoids by using a
// kernel-held byte-range lock instead. No golang.org/x/sys dependency —
// LockFileEx/UnlockFileEx are called directly via syscall.NewLazyDLL,
// the same house pattern used elsewhere for Win32-only primitives not
// exposed by the standard syscall package.

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	lockfileExclusiveLock   = 0x00000002
	lockfileFailImmediately = 0x00000001
	lockAllBytesLow         = 0xFFFFFFFF
	lockAllBytesHigh        = 0xFFFFFFFF
)

// win32Overlapped mirrors the Win32 OVERLAPPED struct. Only the fields
// LockFileEx/UnlockFileEx require are populated (all zero — this is a
// whole-file byte-range lock with no associated event).
type win32Overlapped struct {
	Internal     uintptr
	InternalHigh uintptr
	Offset       uint32
	OffsetHigh   uint32
	HEvent       syscall.Handle
}

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

// acquirePhaseFileLock opens (creating 0600 if absent) the lockfile at
// path and takes an exclusive LockFileEx byte-range lock covering the
// whole file, polling with LOCKFILE_FAIL_IMMEDIATELY at pollInterval
// until either the lock is acquired or wait elapses. On timeout it
// returns an error wrapping ErrPhaseLockTimeout. It returns an unlock
// func that releases the lock and closes the handle.
func acquirePhaseFileLock(path string, wait, pollInterval time.Duration) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lockfile: %w", err)
	}

	handle := syscall.Handle(f.Fd())
	deadline := time.Now().Add(wait)
	var ov win32Overlapped
	for {
		r, _, callErr := procLockFileEx.Call(
			uintptr(handle),
			uintptr(lockfileExclusiveLock|lockfileFailImmediately),
			0,
			uintptr(lockAllBytesLow),
			uintptr(lockAllBytesHigh),
			uintptr(unsafe.Pointer(&ov)),
		)
		if r != 0 {
			return func() {
				var releaseOv win32Overlapped
				_, _, _ = procUnlockFileEx.Call(
					uintptr(handle),
					0,
					uintptr(lockAllBytesLow),
					uintptr(lockAllBytesHigh),
					uintptr(unsafe.Pointer(&releaseOv)),
				)
				_ = f.Close()
			}, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("%w: %s (%v)", ErrPhaseLockTimeout, path, callErr)
		}
		time.Sleep(pollInterval)
	}
}
