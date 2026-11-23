package main

// Shared test helpers extracted in spec 084 Bead 3 when the prior host
// (degradation_test.go) was deleted along with the agentmind/client and
// internal/bench imports it exercised. The helpers themselves are
// agentmind-independent — they build the mindspec binary, set up a
// hermetic env (no inherited OTEL_*, no AGENTMIND_BIN, fresh HOME), and
// scaffold a tmp workspace — so they remain useful for record_test,
// help_golden_test, deprecated_commands_test, and any future Bead 3-or-
// later subprocess test.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The binary is shared read-only; tests needing to modify it must copy it first.
// TestMain owns its lifetime so parallel tests can use it until m.Run returns.
var mindspecTestBinary struct {
	sync.Once
	dir, path string
	err       error
	stderr    bytes.Buffer
}

func buildMindspecBinary(t *testing.T) string {
	t.Helper()
	repoRoot := repoRootFromTestDir(t)

	mindspecTestBinary.Do(func() {
		mindspecTestBinary.dir, mindspecTestBinary.err = os.MkdirTemp("", "mindspec-test-binary-")
		if mindspecTestBinary.err != nil {
			return
		}
		mindspecTestBinary.path = filepath.Join(mindspecTestBinary.dir, "mindspec")
		cmd := exec.Command("go", "build", "-o", mindspecTestBinary.path, "./cmd/mindspec")
		cmd.Dir = repoRoot
		cmd.Stderr = &mindspecTestBinary.stderr
		mindspecTestBinary.err = cmd.Run()
		if mindspecTestBinary.err == nil {
			mindspecTestBinary.err = os.Chmod(mindspecTestBinary.path, 0o555)
		}
	})
	if mindspecTestBinary.err != nil {
		t.Fatalf("build mindspec test binary: %v\nstderr: %s", mindspecTestBinary.err, mindspecTestBinary.stderr.String())
	}
	return mindspecTestBinary.path
}

// strippedEnv returns an environment with:
//   - $AGENTMIND_BIN unset
//   - PATH set to an empty temp dir (no agentmind binary discoverable)
//   - HOME pointed at a fresh temp dir (so config probes can't
//     accidentally pick up developer-host state)
//   - All OTEL_* / CLAUDE_CODE_ENABLE_TELEMETRY env vars removed (per
//     spec 084 Bead 2 caller-env-wins precedence; without stripping
//     OTEL_* the developer's host shell vars leak into hermetic
//     record-start tests).
//
// Other env vars (HOME-derived ones, GOCACHE, etc.) are preserved.
func strippedEnv(t *testing.T) []string {
	t.Helper()
	emptyDir := t.TempDir()
	homeDir := t.TempDir()

	out := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "AGENTMIND_BIN=") {
			continue
		}
		if strings.HasPrefix(kv, "PATH=") {
			continue
		}
		if strings.HasPrefix(kv, "HOME=") {
			continue
		}
		if strings.HasPrefix(kv, "OTEL_") {
			continue
		}
		if strings.HasPrefix(kv, "CLAUDE_CODE_ENABLE_TELEMETRY=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "PATH="+emptyDir)
	out = append(out, "HOME="+homeDir)
	out = append(out, "AGENTMIND_BIN=")
	return out
}

func mkWorkspace(t *testing.T, enableRecording bool) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	// Marker for findLocalRoot.
	if err := os.WriteFile(filepath.Join(dir, ".mindspec", ".keep"), []byte(""), 0o644); err != nil {
		t.Fatalf("write .keep: %v", err)
	}
	if enableRecording {
		cfg := "recording:\n  enabled: true\n"
		if err := os.WriteFile(filepath.Join(dir, ".mindspec", "config.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatalf("write config.yaml: %v", err)
		}
	}
	return dir
}

// repoRootFromTestDir returns the mindspec repository root by walking
// up from the test's working directory until it finds go.mod with the
// mindspec module path.
func repoRootFromTestDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		gm := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(gm); err == nil {
			if strings.Contains(string(data), "module github.com/mrmaxsteel/mindspec") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find mindspec go.mod walking up from %s", wd)
		}
		dir = parent
	}
}
