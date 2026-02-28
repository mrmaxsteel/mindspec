//go:build !windows

package recording

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func signalTerminate(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}

// processName returns the command name for the given PID using ps.
// Works on both Linux and macOS.
func processName(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return "", fmt.Errorf("querying process %d: %w", pid, err)
	}
	return strings.TrimSpace(string(out)), nil
}
