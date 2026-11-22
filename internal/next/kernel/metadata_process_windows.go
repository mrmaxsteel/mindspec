//go:build windows

package kernel

import (
	"math"
	"os"
	"syscall"
)

// Windows directories inherit access controls; Unix permission bits do not apply.
func privateTempDir(_ os.FileInfo) bool { return true }

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
