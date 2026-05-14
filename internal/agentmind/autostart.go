// Package agentmind provides utilities for managing the AgentMind process —
// the unified OTLP collector and live visualization server.
package agentmind

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const (
	// DefaultOTLPPort is the default OTLP/HTTP receiver port for AgentMind.
	DefaultOTLPPort = 4318
	// DefaultUIPort is the default web UI port for AgentMind.
	DefaultUIPort = 8420
)

// IsRunning returns true only if a TCP listener on `port` is *demonstrably*
// AgentMind: a valid lockfile exists, its PID is alive, its otlp_port
// matches, and the port is reachable. A bare TCP listener with no matching
// lockfile is explicitly NOT trusted.
func IsRunning(port int) bool {
	lf, err := ReadLockfile()
	if err != nil || lf == nil {
		return false
	}
	if lf.OTLPPort != port {
		return false
	}
	if !isPIDAlive(lf.PID) {
		_ = RemoveLockfile() // stale, clean up
		return false
	}
	// Confirm the listener is really up.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// PortInUseByForeign returns true if `port` is bound but no valid lockfile
// claims it. AutoStart uses this to refuse silent reuse of an unknown
// listener. "Valid" here means: lockfile exists, claims this port, and its
// PID is alive.
func PortInUseByForeign(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
	if err != nil {
		// Nothing is listening — definitely not "foreign".
		return false
	}
	conn.Close()

	lf, err := ReadLockfile()
	if err != nil || lf == nil {
		return true
	}
	if lf.OTLPPort != port {
		return true
	}
	if !isPIDAlive(lf.PID) {
		return true
	}
	return false
}

// Token returns the current AgentMind handoff token, or "" if no valid
// lockfile is present. Callers performing privileged handoff (recording
// collector, bench runner) pass this in an `X-AgentMind-Token` header.
func Token() string {
	lf, err := ReadLockfile()
	if err != nil || lf == nil {
		return ""
	}
	return lf.Token
}

// AutoStart ensures AgentMind is running. If already listening on otlpPort
// AND the listener is verifiably AgentMind, returns 0 (reusing existing
// instance). If the port is bound by an unknown process, returns an error
// rather than silently trusting it. Otherwise starts AgentMind as a detached
// background process and returns the PID.
func AutoStart(root string, otlpPort, uiPort int, outputPath string) (int, error) {
	if IsRunning(otlpPort) {
		return 0, nil
	}
	if PortInUseByForeign(otlpPort) {
		return 0, fmt.Errorf("port %d in use by an unknown process; refusing to reuse", otlpPort)
	}

	binPath, err := findBinary(root)
	if err != nil {
		return 0, err
	}

	args := []string{"agentmind", "serve",
		"--otlp-port", fmt.Sprintf("%d", otlpPort),
		"--ui-port", fmt.Sprintf("%d", uiPort),
	}
	if outputPath != "" {
		args = append(args, "--output", outputPath)
	}

	cmd := exec.Command(binPath, args...)
	detachProcess(cmd)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting AgentMind: %w", err)
	}

	pid := cmd.Process.Pid
	cmd.Process.Release() //nolint:errcheck

	if err := WaitForPort(otlpPort, 5*time.Second); err != nil {
		return pid, fmt.Errorf("AgentMind started (PID %d) but not responding: %w", pid, err)
	}

	// Re-verify identity: the freshly-spawned process should have written a
	// lockfile claiming this port. If it didn't (or the lockfile points
	// somewhere else), fail loudly rather than silently trusting whatever is
	// on the port now.
	if !IsRunning(otlpPort) {
		return pid, fmt.Errorf("AgentMind started (PID %d) but lockfile verification failed; check ~/%s/%s", pid, LockfileDirName, LockfileBaseName)
	}

	fmt.Fprintf(os.Stderr, "AgentMind started — watch live at http://localhost:%d\n", uiPort)
	return pid, nil
}

// WaitForPort polls until a TCP connection to the given port succeeds or timeout expires.
func WaitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("port %d not ready after %s", port, timeout)
}

// Probe sends a lightweight HTTP request to check if the OTLP receiver is responding.
func Probe(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/v1/logs", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	// Any response (even 405 Method Not Allowed) means the server is up
	return true
}

// findBinary locates the mindspec binary.
func findBinary(root string) (string, error) {
	binPath := root + "/bin/mindspec"
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	path, err := exec.LookPath("mindspec")
	if err == nil {
		return path, nil
	}

	return "", fmt.Errorf("mindspec binary not found in %s/bin/ or PATH", root)
}
