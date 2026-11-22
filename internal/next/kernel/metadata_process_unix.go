//go:build !windows

package kernel

import "syscall"

func processRunning(pid int) bool { return syscall.Kill(pid, 0) != syscall.ESRCH }
