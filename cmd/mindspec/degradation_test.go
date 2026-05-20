package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDegradation_PerClass exercises spec 083 Hard Constraint #4 / Test C
// for the command classes addressable from Bead 3a (telemetry-as-output
// and batch). The interactive class is exercised by Bead 4.
//
// Each subtest builds the mindspec binary into a temp dir, sets up a
// minimal workspace, strips agentmind from the environment (PATH
// contains no agentmind binary, $AGENTMIND_BIN is unset, and no
// <root>/bin/agentmind exists), then invokes the command under test
// and asserts the documented exit code and stderr contents.
func TestDegradation_PerClass(t *testing.T) {
	if testing.Short() && os.Getenv("MINDSPEC_RUN_DEGRADATION") == "" {
		// `go test -short` skips by default; the build is expensive.
		// Set MINDSPEC_RUN_DEGRADATION=1 to opt in under -short.
		t.Skip("skipping integration test under -short; set MINDSPEC_RUN_DEGRADATION=1 to run")
	}

	bin := buildMindspecBinary(t)

	const warnLine = "WARN: agentmind binary not found; telemetry export will drop silently"

	t.Run("telemetry_as_output_record_start_exits_nonzero", func(t *testing.T) {
		t.Parallel()
		workspace := mkWorkspace(t, true)

		cmd := exec.Command(bin, "record", "start", "--spec", "999-test-bead3a")
		cmd.Dir = workspace
		cmd.Env = strippedEnv(t)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()

		if err == nil {
			t.Fatalf("expected non-zero exit for telemetry-as-output with binary absent; got nil\nstdout=%q\nstderr=%q", stdout.String(), stderr.String())
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected ExitError; got %T: %v", err, err)
		}
		if exitErr.ExitCode() == 0 {
			t.Fatalf("expected non-zero exit code; got 0")
		}

		if got := strings.Count(stderr.String(), warnLine); got != 1 {
			t.Fatalf("expected exactly one warn line in stderr; got %d\nstderr=%q", got, stderr.String())
		}
	})

	t.Run("batch_bench_setup_exits_zero_no_warn", func(t *testing.T) {
		// `mindspec bench setup` does not call AutoStart — it only prints
		// env-var configuration. It is satisfied-by-design: exits 0 with
		// no warn line, and Test C batch class is met trivially.
		t.Parallel()
		workspace := mkWorkspace(t, false)

		cmd := exec.Command(bin, "bench", "setup")
		cmd.Dir = workspace
		cmd.Env = strippedEnv(t)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("bench setup failed: %v\nstderr=%q", err, stderr.String())
		}
		if strings.Contains(stderr.String(), warnLine) {
			t.Fatalf("bench setup must not emit warn line (satisfied-by-design); got stderr=%q", stderr.String())
		}
	})

	t.Run("batch_agentmind_setup_exits_zero_no_warn", func(t *testing.T) {
		// `mindspec agentmind setup` is also a docs/config command that
		// does not call AutoStart. Same satisfied-by-design contract as
		// bench setup.
		t.Parallel()
		workspace := mkWorkspace(t, false)

		// `agentmind setup` is a parent command with no own RunE; print --help
		// is the closest invocation that proves the subtree exists without
		// triggering an unrelated codex-OTEL config write. The point of
		// this test is that the command does not attempt AutoStart and
		// does not emit the warn line.
		cmd := exec.Command(bin, "agentmind", "setup", "--help")
		cmd.Dir = workspace
		cmd.Env = strippedEnv(t)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("agentmind setup --help failed: %v\nstderr=%q", err, stderr.String())
		}
		if strings.Contains(stderr.String(), warnLine) {
			t.Fatalf("agentmind setup must not emit warn line (satisfied-by-design); got stderr=%q", stderr.String())
		}
	})
}

// TestDegradation_TypedSentinelDetection is a static/positive assertion:
// every swapped consumer must detect the absent-binary condition via
// errors.Is(err, client.ErrBinaryNotFound), never via substring matching
// on error text. The negative assertion (no strings.Contains on
// err.Error()) lives alongside in package-level grep tests.
func TestDegradation_TypedSentinelDetection(t *testing.T) {
	t.Parallel()
	// The positive assertion is enforced by compile-time: every call
	// site uses errors.Is. If a future regression switches to
	// strings.Contains(err.Error(), "not found"), this grep catches it.
	repoRoot := repoRootFromTestDir(t)
	swappedFiles := []string{
		filepath.Join(repoRoot, "internal", "recording", "collector.go"),
		filepath.Join(repoRoot, "internal", "bench", "runner.go"),
		filepath.Join(repoRoot, "cmd", "mindspec", "record.go"),
	}

	for _, f := range swappedFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(raw)

		// Positive: errors.Is(err, client.ErrBinaryNotFound) appears.
		if !strings.Contains(src, "errors.Is(err, client.ErrBinaryNotFound)") {
			t.Errorf("%s: missing positive assertion errors.Is(err, client.ErrBinaryNotFound)", f)
		}

		// Negative: no substring-matching on err.Error() for "binary not found"
		// or "agentmind".
		badPatterns := []string{
			`strings.Contains(err.Error(), "binary not found"`,
			`strings.Contains(err.Error(), "agentmind"`,
			`strings.HasPrefix(err.Error(), "agentmind binary not found"`,
		}
		for _, bad := range badPatterns {
			if strings.Contains(src, bad) {
				t.Errorf("%s: contains banned substring-matching pattern %q (use errors.Is(err, client.ErrBinaryNotFound) instead)", f, bad)
			}
		}
	}
}

// --- helpers -------------------------------------------------------------

func buildMindspecBinary(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	out := filepath.Join(tmp, "mindspec")
	repoRoot := repoRootFromTestDir(t)

	cmd := exec.Command("go", "build", "-o", out, "./cmd/mindspec")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build mindspec: %v\nstderr: %s", err, stderr.String())
	}
	return out
}

// strippedEnv returns an environment with:
//   - $AGENTMIND_BIN unset
//   - PATH set to an empty temp dir (no agentmind binary discoverable)
//   - HOME pointed at a fresh temp dir (so lockfile/config probes can't
//     accidentally pick up a real agentmind running on the host)
//
// Other env vars (HOME-derived ones, GOCACHE, etc.) are preserved.
func strippedEnv(t *testing.T) []string {
	t.Helper()
	emptyDir := t.TempDir()
	homeDir := t.TempDir()

	out := []string{}
	for _, kv := range os.Environ() {
		// Drop AGENTMIND_BIN. Drop PATH (we'll replace). Drop HOME
		// (we'll replace).
		if strings.HasPrefix(kv, "AGENTMIND_BIN=") {
			continue
		}
		if strings.HasPrefix(kv, "PATH=") {
			continue
		}
		if strings.HasPrefix(kv, "HOME=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "PATH="+emptyDir)
	out = append(out, "HOME="+homeDir)
	// Explicitly clear AGENTMIND_BIN so any inherited value is gone.
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
// up from this test file until it finds go.mod with the mindspec module
// path. Works whether the test is run from the repo root, a worktree,
// or a different working directory.
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
