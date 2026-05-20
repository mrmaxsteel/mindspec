package recording

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// isolateLockfileHome points HOME and USERPROFILE at a fresh temp dir so tests
// never touch the developer's real ~/.mindspec/agentmind.lock. USERPROFILE is
// set unconditionally because os.UserHomeDir consults it on Windows.
func isolateLockfileHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func TestLockfileWriteRoundTrip(t *testing.T) {
	home := isolateLockfileHome(t)

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

	// Read it back manually (ReadLockfile was unused by mindspec callers and
	// dropped during the Phase 5 move; verifying via plain json.Unmarshal is
	// sufficient to assert the write contract).
	path := filepath.Join(home, LockfileDirName, LockfileBaseName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var got Lockfile
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.PID != want.PID || got.OTLPPort != want.OTLPPort || got.UIPort != want.UIPort || got.Token != want.Token {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
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
		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat file: %v", err)
		}
		if perm := fileInfo.Mode().Perm(); perm != 0o600 {
			t.Fatalf("file perm = %o, want 0600", perm)
		}
	}
}

func TestRemoveLockfile(t *testing.T) {
	isolateLockfileHome(t)
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
