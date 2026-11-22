//go:build windows

package kernel

import (
	"math"
	"syscall"
)

func processRunning(pid int) bool {
	if pid < 1 || uint64(pid) > math.MaxUint32 {
		return true
	}
	process, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		const invalidParameter syscall.Errno = 87
		return err != invalidParameter
	}
	defer syscall.CloseHandle(process)
	status, err := syscall.WaitForSingleObject(process, 0)
	return err != nil || status == syscall.WAIT_TIMEOUT
}
