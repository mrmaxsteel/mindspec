//go:build windows

package recording

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds; Kill with nil signal isn't
	// supported, so we just assume the process exists.
	_ = proc
	return true
}

func signalTerminate(proc *os.Process) error {
	return proc.Kill()
}

// processName returns the command name for the given PID using tasklist.
func processName(pid int) (string, error) {
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return "", fmt.Errorf("querying process %d: %w", pid, err)
	}
	line := strings.TrimSpace(string(out))
	if strings.HasPrefix(line, "\"") {
		parts := strings.SplitN(line, ",", 2)
		return strings.Trim(parts[0], "\""), nil
	}
	return "", fmt.Errorf("process %d not found", pid)
}
