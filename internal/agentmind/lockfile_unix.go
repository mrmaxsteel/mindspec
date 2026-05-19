//go:build !windows

package agentmind

import "syscall"

// isPIDAlive reports whether a process with the given PID is currently alive.
// On Unix we use signal 0 — kill(pid, 0) performs the permission/existence
// check but does not actually deliver a signal.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but we can't signal it (different user).
	// Treat as alive — the lockfile contract assumes same-user processes, so
	// EPERM is unusual but indicates *some* process holds that PID.
	if err == syscall.EPERM {
		return true
	}
	return false
}

// isStrictPermsPlatform indicates whether Unix-style file permissions are
// meaningful on the current OS. Windows synthesizes mode bits and would
// spuriously fail the 0o077 check.
func isStrictPermsPlatform() bool { return true }
