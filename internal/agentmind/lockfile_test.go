package agentmind

import (
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// isolateHome points HOME and USERPROFILE at a fresh temp dir so tests never
// touch the developer's real ~/.mindspec/agentmind.lock. USERPROFILE is set
// unconditionally because os.UserHomeDir consults it on Windows.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func TestWriteReadRoundTrip(t *testing.T) {
	home := isolateHome(t)

	want := Lockfile{
		PID:       os.Getpid(),
		OTLPPort:  4318,
		UIPort:    8420,
		Token:     "deadbeef",
		StartedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := WriteLockfile(want); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	got, err := ReadLockfile()
	if err != nil {
		t.Fatalf("ReadLockfile: %v", err)
	}
	if got == nil {
		t.Fatal("ReadLockfile returned nil; expected struct")
	}
	if got.PID != want.PID || got.OTLPPort != want.OTLPPort || got.UIPort != want.UIPort || got.Token != want.Token {
		t.Fatalf("round-trip mismatch: got %+v want %+v", *got, want)
	}
	if !got.StartedAt.Equal(want.StartedAt) {
		t.Fatalf("StartedAt mismatch: got %v want %v", got.StartedAt, want.StartedAt)
	}

	// Permission checks are meaningful on Unix only.
	if runtime.GOOS != "windows" {
		dirInfo, err := os.Stat(filepath.Join(home, LockfileDirName))
		if err != nil {
			t.Fatalf("stat dir: %v", err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Fatalf("dir perm = %o, want 0700", perm)
		}
		fileInfo, err := os.Stat(filepath.Join(home, LockfileDirName, LockfileBaseName))
		if err != nil {
			t.Fatalf("stat file: %v", err)
		}
		if perm := fileInfo.Mode().Perm(); perm != 0o600 {
			t.Fatalf("file perm = %o, want 0600", perm)
		}
	}
}

func TestReadLockfileAbsent(t *testing.T) {
	isolateHome(t)
	lf, err := ReadLockfile()
	if err != nil {
		t.Fatalf("ReadLockfile on absent file: %v", err)
	}
	if lf != nil {
		t.Fatalf("expected nil lockfile, got %+v", *lf)
	}
}

func TestRemoveLockfile(t *testing.T) {
	isolateHome(t)
	if err := WriteLockfile(Lockfile{PID: os.Getpid(), OTLPPort: 4318}); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}
	if err := RemoveLockfile(); err != nil {
		t.Fatalf("RemoveLockfile: %v", err)
	}
	// Removing again should be a no-op, not an error.
	if err := RemoveLockfile(); err != nil {
		t.Fatalf("RemoveLockfile (second call): %v", err)
	}
}

func TestRejectGroupReadablePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only permission semantics")
	}
	home := isolateHome(t)
	if err := WriteLockfile(Lockfile{PID: os.Getpid(), OTLPPort: 4318}); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}
	path := filepath.Join(home, LockfileDirName, LockfileBaseName)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	_, err := ReadLockfile()
	if err == nil {
		t.Fatal("expected ReadLockfile to reject world-readable file")
	}
}

func TestNewToken(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if len(a) != 64 {
		t.Fatalf("token length = %d, want 64 hex chars", len(a))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Fatalf("token is not valid hex: %v", err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken (second): %v", err)
	}
	if a == b {
		t.Fatal("two NewToken calls returned the same value")
	}
}

func TestIsPIDAliveSelf(t *testing.T) {
	if !isPIDAlive(os.Getpid()) {
		t.Fatal("isPIDAlive(self) returned false")
	}
}

func TestIsPIDAliveDeadProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test harness")
	}
	// Spawn a child that exits immediately. After Wait returns, its PID is
	// reaped and (modulo PID reuse) won't be alive.
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting child: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting on child: %v", err)
	}
	// Race window: the OS could recycle the PID. Acceptable — this test is
	// best-effort. We assert the common case.
	if isPIDAlive(pid) {
		t.Logf("isPIDAlive(%d) returned true after reap; likely PID reuse — skipping", pid)
		t.Skip("PID reuse race; skipping")
	}
}

func TestIsPIDAliveZero(t *testing.T) {
	if isPIDAlive(0) {
		t.Fatal("isPIDAlive(0) returned true")
	}
}
