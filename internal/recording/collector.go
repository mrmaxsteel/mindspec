package recording

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// StartCollector launches a detached background collector process.
// It runs `mindspec record collect` as a subprocess that survives session boundaries.
func StartCollector(root, specID string) error {
	eventsPath := EventsPath(root, specID)
	port := defaultRecordingPort

	// Find mindspec binary
	binPath, err := findMindspecBinary(root)
	if err != nil {
		return fmt.Errorf("finding mindspec binary: %w", err)
	}

	cmd := exec.Command(binPath, "record", "collect",
		"--port", fmt.Sprintf("%d", port),
		"--output", eventsPath,
	)

	// Detach: new session, no stdin/stdout/stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting collector: %w", err)
	}

	pid := cmd.Process.Pid

	// Release the process so it isn't reaped when we exit
	cmd.Process.Release() //nolint:errcheck

	// Update manifest with PID and port
	m, err := ReadManifest(root, specID)
	if err != nil {
		return fmt.Errorf("reading manifest for PID update: %w", err)
	}
	m.CollectorPID = pid
	m.CollectorPort = port
	m.Status = "recording"
	if err := WriteManifest(root, specID, m); err != nil {
		return fmt.Errorf("writing manifest with PID: %w", err)
	}

	// Brief wait for collector to start listening
	time.Sleep(500 * time.Millisecond)

	return nil
}

// StopCollector sends SIGTERM to the collector process and updates the manifest.
func StopCollector(root, specID string) error {
	m, err := ReadManifest(root, specID)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	if m.CollectorPID > 0 {
		proc, err := os.FindProcess(m.CollectorPID)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			// Give it a moment to shut down gracefully
			time.Sleep(500 * time.Millisecond)
		}
	}

	m.Status = "complete"
	m.CollectorPID = 0

	// Close the last phase
	if len(m.Phases) > 0 {
		last := &m.Phases[len(m.Phases)-1]
		if last.EndedAt == "" {
			last.EndedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}

	return WriteManifest(root, specID, m)
}

// findMindspecBinary locates the mindspec binary.
func findMindspecBinary(root string) (string, error) {
	// Try ./bin/mindspec relative to root
	binPath := root + "/bin/mindspec"
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	// Try PATH
	path, err := exec.LookPath("mindspec")
	if err == nil {
		return path, nil
	}

	return "", fmt.Errorf("mindspec binary not found in %s/bin/ or PATH", root)
}
