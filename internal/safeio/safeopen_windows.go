//go:build windows

package safeio

// nofollowFlag is a no-op on Windows: syscall.O_NOFOLLOW is not defined and
// NTFS reparse-point handling is out of scope. The Lstat pre-check in
// refuseIfSymlink remains the only line of defence here, which catches the
// common case of a plain NTFS symlink.
func nofollowFlag() int { return 0 }
