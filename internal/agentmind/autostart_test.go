package agentmind

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// listenLocal opens a TCP listener on a free localhost port and returns the
// listener plus the port. The listener is closed by t.Cleanup.
func listenLocal(t *testing.T) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln, ln.Addr().(*net.TCPAddr).Port
}

func TestIsRunningHappyPath(t *testing.T) {
	isolateHome(t)
	_, port := listenLocal(t)
	if err := WriteLockfile(Lockfile{
		PID:       os.Getpid(),
		OTLPPort:  port,
		UIPort:    port + 1,
		Token:     "abc",
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}
	if !IsRunning(port) {
		t.Fatal("IsRunning returned false on happy path")
	}
}

func TestIsRunningForeignListenerNoLockfile(t *testing.T) {
	isolateHome(t)
	_, port := listenLocal(t)
	if IsRunning(port) {
		t.Fatal("IsRunning returned true for foreign listener with no lockfile")
	}
	if !PortInUseByForeign(port) {
		t.Fatal("PortInUseByForeign returned false for foreign listener")
	}
}

func TestIsRunningPortMismatch(t *testing.T) {
	isolateHome(t)
	_, port := listenLocal(t)
	if err := WriteLockfile(Lockfile{
		PID:      os.Getpid(),
		OTLPPort: 9999,
		Token:    "x",
	}); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}
	if IsRunning(port) {
		t.Fatal("IsRunning returned true despite port mismatch")
	}
}

func TestIsRunningStaleLockfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test harness")
	}
	isolateHome(t)
	_, port := listenLocal(t)

	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting child: %v", err)
	}
	deadPID := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting on child: %v", err)
	}
	if isPIDAlive(deadPID) {
		t.Skip("PID reuse race; cannot reliably exercise stale-lockfile path")
	}

	if err := WriteLockfile(Lockfile{
		PID:      deadPID,
		OTLPPort: port,
		Token:    "stale",
	}); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	if IsRunning(port) {
		t.Fatal("IsRunning returned true for stale lockfile")
	}
	// After the call, the stale lockfile should have been removed.
	lf, err := ReadLockfile()
	if err != nil {
		t.Fatalf("ReadLockfile after stale cleanup: %v", err)
	}
	if lf != nil {
		t.Fatalf("expected stale lockfile to be removed; got %+v", *lf)
	}
}

func TestPortInUseByForeignNothingListening(t *testing.T) {
	isolateHome(t)
	// Grab a free port, then close so nothing listens.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	if PortInUseByForeign(port) {
		t.Fatal("PortInUseByForeign returned true when nothing is listening")
	}
}

func TestTokenEmptyWhenNoLockfile(t *testing.T) {
	isolateHome(t)
	if tok := Token(); tok != "" {
		t.Fatalf("Token() = %q on absent lockfile; want empty", tok)
	}
}

func TestTokenFromLockfile(t *testing.T) {
	isolateHome(t)
	if err := WriteLockfile(Lockfile{
		PID:      os.Getpid(),
		OTLPPort: 4318,
		Token:    "hex-token-value",
	}); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}
	if got := Token(); got != "hex-token-value" {
		t.Fatalf("Token() = %q, want %q", got, "hex-token-value")
	}
}

func TestAutoStartRefusesForeignListener(t *testing.T) {
	isolateHome(t)
	_, port := listenLocal(t)

	// Use a root that has no bin/mindspec so we'd fail later anyway — but the
	// foreign-listener check should fire *before* binary lookup, so we
	// expect the "unknown process" error, not a "binary not found" error.
	tmp := t.TempDir()
	_, err := AutoStart(tmp, port, port+1, "")
	if err == nil {
		t.Fatal("AutoStart returned nil error for foreign listener")
	}
	if !strings.Contains(err.Error(), "unknown process") {
		t.Fatalf("AutoStart error = %q; want substring 'unknown process'", err.Error())
	}
}
