//go:build windows

package agentmind

import (
	"golang.org/x/sys/windows"
)

// isPIDAlive reports whether a process with the given PID is currently alive.
// On Windows we ask for the lightest-weight handle available; success implies
// the PID maps to a live process. We always close any handle we obtain.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	return true
}

// isStrictPermsPlatform indicates whether Unix-style file permissions are
// meaningful on the current OS. Windows synthesizes mode bits, so we skip
// the 0o077 check there.
func isStrictPermsPlatform() bool { return false }
