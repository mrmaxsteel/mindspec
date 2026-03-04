package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newCmd(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	return exec.Command(name, args...)
}

func TestInstallShimsCreatesScripts(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	logPath := filepath.Join(t.TempDir(), "events.jsonl")

	if err := InstallShims(binDir, logPath); err != nil {
		t.Fatalf("InstallShims- %v", err)
	}

	// At least one shim should exist (git is usually available)
	entries, err := os.ReadDir(binDir)
	if err != nil {
		t.Fatalf("reading shim dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no shims created (git should be available)")
	}

	// Check that shim scripts are executable and contain the log path
	for _, entry := range entries {
		shimPath := filepath.Join(binDir, entry.Name())
		fi, err := os.Stat(shimPath)
		if err != nil {
			t.Fatalf("stat %s: %v", shimPath, err)
		}
		if fi.Mode()&0o111 == 0 {
			t.Errorf("%s is not executable", entry.Name())
		}

		content, err := os.ReadFile(shimPath)
		if err != nil {
			t.Fatalf("reading %s: %v", shimPath, err)
		}
		if !strings.Contains(string(content), logPath) {
			t.Errorf("%s does not contain log path %q", entry.Name(), logPath)
		}
	}
}

func TestShimEnvPrependsPath(t *testing.T) {
	binDir := "/tmp/test-shims"
	env := ShimEnv(binDir)

	if len(env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(env))
	}
	if !strings.HasPrefix(env[0], "PATH=") {
		t.Fatalf("expected PATH=..., got %q", env[0])
	}
	if !strings.Contains(env[0], binDir) {
		t.Errorf("PATH does not contain binDir %q: %s", binDir, env[0])
	}

	// binDir should be first in PATH
	pathVal := strings.TrimPrefix(env[0], "PATH=")
	parts := strings.SplitN(pathVal, ":", 2)
	if parts[0] != binDir {
		t.Errorf("binDir should be first in PATH, got %q", parts[0])
	}
}

func TestFindRealBinaryExcludesShimDir(t *testing.T) {
	// git should be findable
	path, err := findRealBinary("git", "/nonexistent-dir")
	if err != nil {
		t.Fatalf("findRealBinary(git): %v", err)
	}
	if !strings.Contains(path, "git") {
		t.Errorf("expected path containing git, got %q", path)
	}

	// Should not find a nonexistent binary
	_, err = findRealBinary("this-binary-does-not-exist-xyz", "/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestRecorderShimCapturesInvocation(t *testing.T) {
	// Create a real shim for "echo" (universally available)
	binDir := filepath.Join(t.TempDir(), "bin")
	logPath := filepath.Join(t.TempDir(), "events.jsonl")

	// Find real echo
	echoPath, err := findRealBinary("echo", binDir)
	if err != nil {
		// echo might be a shell builtin; use /bin/echo
		echoPath = "/bin/echo"
		if _, err := os.Stat(echoPath); err != nil {
			t.Skip("no /bin/echo available")
		}
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeShim(binDir, logPath, "echo", echoPath, "/nonexistent/mindspec", os.Getenv("PATH")); err != nil {
		t.Fatalf("writeShim: %v", err)
	}

	// Run the shim
	shimPath := filepath.Join(binDir, "echo")
	cmd := newCmd(t, shimPath, "hello", "world")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shim execution failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "hello world") {
		t.Errorf("shim output = %q, expected to contain 'hello world'", out)
	}

	// Check the log
	events, err := ParseEvents(logPath)
	if err != nil {
		t.Fatalf("ParseEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("no events recorded")
	}
	if events[0].Command != "echo" {
		t.Errorf("event.Command = %q, want echo", events[0].Command)
	}
	if events[0].ExitCode != 0 {
		t.Errorf("event.ExitCode = %d, want 0", events[0].ExitCode)
	}
}
