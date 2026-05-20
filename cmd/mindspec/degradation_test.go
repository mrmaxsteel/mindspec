package main

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/agentmind/client"
	"github.com/mrmaxsteel/mindspec/internal/bench"
)

// TestDegradation_PerClass exercises spec 083 Hard Constraint #4 / Test C
// for the command classes addressable from Bead 3a (telemetry-as-output
// and batch). The interactive class is exercised by Bead 4.
//
// Panel bead-3a-v1, REV-1: the test no longer self-skips under
// `go test -short`. The cost of one `go build` of the mindspec binary
// is the price of HC#6 evidence. Genuinely network-touching subtests
// (currently: none) would be moved behind `-short` individually rather
// than gating the whole suite.
//
// Panel bead-3a-v1, REV-2: a `batch_bench_run_collector_degrades` subtest
// exercises the named batch-class exemplar via the testable
// startBenchCollector helper in internal/bench (the full `bench run`
// subprocess path is unsuitable because its prerequisite check
// requires a clean git tree, claude on PATH, and bin/mindspec).
//
// Panel bead-3a-v1, REV-5: the `agentmind setup` subtest invokes the
// real subcommand without `--help`, so any future regression that
// wired AutoStart into the parent or PersistentPreRunE would be
// caught.
//
// Each subprocess subtest builds the mindspec binary into a temp dir,
// sets up a minimal workspace, strips agentmind from the environment
// (PATH contains no agentmind binary, $AGENTMIND_BIN is unset, and no
// <root>/bin/agentmind exists), then invokes the command under test
// and asserts the documented exit code and stderr contents.
func TestDegradation_PerClass(t *testing.T) {
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

	t.Run("batch_bench_run_collector_degrades", func(t *testing.T) {
		// Panel bead-3a-v1, REV-2: the spec's named batch-class
		// exemplar. The full `bench run` subprocess path cannot be
		// invoked in a hermetic test (it requires claude on PATH,
		// bin/mindspec, and a clean git tree). Instead we exercise
		// the AutoStart switch directly through the
		// internal/bench.startBenchCollector helper that runner.Run
		// now delegates to.
		//
		// NOTE: this subtest cannot run with t.Parallel — it uses
		// t.Setenv to neutralize PATH/AGENTMIND_BIN/HOME so AutoStart's
		// findBinary cannot resolve a real agentmind binary on the
		// host, and t.Setenv is incompatible with t.Parallel.
		emptyDir := t.TempDir()
		repoRoot := t.TempDir()
		workDir := t.TempDir()
		homeDir := t.TempDir()

		t.Setenv("PATH", emptyDir)
		t.Setenv("AGENTMIND_BIN", "")
		t.Setenv("HOME", homeDir)

		eventsPath := filepath.Join(workDir, "bench-events.jsonl")

		var stdout, stderr bytes.Buffer
		// Round-trip through the exported test helper. Note: in this
		// test binary's process, the warnOnce in agentmind/client may
		// have already fired in another subtest, so we tolerate 0 or 1
		// warn lines here. The strict count==1 assertion lives in the
		// internal/bench package test (see autostart_test.go) which
		// runs in its own process.
		err := bench.RunStartCollectorForTest(repoRoot, workDir, eventsPath, &stdout, &stderr)
		if err != nil {
			t.Fatalf("startBenchCollector returned error: %v (stderr=%q)", err, stderr.String())
		}

		// At most one canonical warn line.
		if got := strings.Count(stderr.String(), warnLine); got > 1 {
			t.Fatalf("expected at most one canonical warn line in stderr; got %d\nstderr=%q", got, stderr.String())
		}

		// No PID file should have been written (nothing was spawned).
		pidFile := filepath.Join(workDir, bench.BenchCollectorPIDFile)
		if _, statErr := os.Stat(pidFile); statErr == nil {
			t.Fatalf("bench-run degraded path must not write a PID file; found %s", pidFile)
		}
	})

	t.Run("batch_agentmind_setup_exits_zero_no_warn", func(t *testing.T) {
		// `mindspec agentmind setup` is a parent command with no RunE
		// that prints usage and exits 0. Per REV-5 (panel bead-3a-v1)
		// we invoke the bare subcommand — NOT `--help`, which used to
		// short-circuit cobra before any RunE / PersistentPreRunE
		// could even run. The bare invocation traverses the entire
		// command chain that a future regression might wire AutoStart
		// into.
		t.Parallel()
		workspace := mkWorkspace(t, false)

		cmd := exec.Command(bin, "agentmind", "setup")
		cmd.Dir = workspace
		cmd.Env = strippedEnv(t)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("agentmind setup failed: %v\nstdout=%q\nstderr=%q", err, stdout.String(), stderr.String())
		}
		if strings.Contains(stderr.String(), warnLine) {
			t.Fatalf("agentmind setup must not emit warn line (satisfied-by-design); got stderr=%q", stderr.String())
		}
	})
}

// TestDegradation_TypedSentinelDetection is a repo-wide grep guardrail
// against future regressions that try to detect the absent-binary
// condition via substring matching on err.Error() instead of
// errors.Is(err, client.ErrBinaryNotFound).
//
// Panel bead-3a-v1, REV-7: broadened from a fixed three-file list to a
// recursive scan over `cmd/mindspec/...` and `internal/...` (excluding
// `_test.go` files and vendored / generated trees). The negative net is
// also widened to flag `strings.Contains` used in proximity to
// ErrBinaryNotFound text — see banPatterns below.
//
// Positive assertion: at least one production file must use
// errors.Is(err, client.ErrBinaryNotFound) so that the typed-sentinel
// contract is exercised somewhere. (This is anchored at the cmd/mindspec
// level — record.go is the canonical telemetry-as-output detection
// site.)
func TestDegradation_TypedSentinelDetection(t *testing.T) {
	t.Parallel()
	repoRoot := repoRootFromTestDir(t)
	roots := []string{
		filepath.Join(repoRoot, "cmd", "mindspec"),
		filepath.Join(repoRoot, "internal"),
	}

	// Banned substring-matching patterns. HC#4 forbids substring
	// matching on err.Error() to detect the absent-binary condition.
	banPatterns := []string{
		// Classic forms of the legacy bug.
		`strings.Contains(err.Error(), "binary not found"`,
		`strings.Contains(err.Error(), "agentmind"`,
		`strings.HasPrefix(err.Error(), "agentmind binary not found"`,
		`strings.Contains(err.Error(), "AGENTMIND"`,
		`strings.Contains(err.Error(), "not found"`,
		`strings.Contains(err.Error(), "no such file"`,
		// Lowercase / case-folded variants.
		`strings.Contains(strings.ToLower(err.Error()), "agentmind"`,
		`strings.Contains(strings.ToLower(err.Error()), "binary not found"`,
	}

	positiveFound := false

	walkErr := filepath.WalkDir(roots[0], func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		positiveFound = inspectFile(t, path, d, banPatterns) || positiveFound
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", roots[0], walkErr)
	}
	walkErr = filepath.WalkDir(roots[1], func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		positiveFound = inspectFile(t, path, d, banPatterns) || positiveFound
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", roots[1], walkErr)
	}

	if !positiveFound {
		t.Errorf("no file under cmd/mindspec or internal/ uses errors.Is(err, client.ErrBinaryNotFound); the typed sentinel contract must be exercised at least once in production code")
	}
}

// inspectFile reads a single .go file (skipping _test.go and non-Go)
// and asserts:
//
//   - no banned substring-matching pattern appears (REV-7);
//   - if the file references client.ErrBinaryNotFound, every reference
//     except the package-import line must be inside an errors.Is(...)
//     call or a comment — never inside a strings.Contains (REV-7,
//     additional clause requested by panel).
//
// Returns true if the file uses errors.Is(err, client.ErrBinaryNotFound)
// — the caller aggregates this across all files for the positive
// assertion.
func inspectFile(t *testing.T, path string, d fs.DirEntry, banPatterns []string) bool {
	t.Helper()
	if d.IsDir() {
		return false
	}
	name := d.Name()
	if !strings.HasSuffix(name, ".go") {
		return false
	}
	if strings.HasSuffix(name, "_test.go") {
		return false
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := string(raw)

	// REV-7 negative: no banned substring patterns.
	for _, bad := range banPatterns {
		if strings.Contains(src, bad) {
			t.Errorf("%s: contains banned substring-matching pattern %q (use errors.Is(err, client.ErrBinaryNotFound) instead)", path, bad)
		}
	}

	// REV-7 additional: assert strings.Contains is never used in
	// proximity to ErrBinaryNotFound. We scan line-by-line for any
	// line that mentions both "strings.Contains" and
	// "ErrBinaryNotFound" — a near-miss regression that wraps the
	// typed sentinel inside a string-matching helper would be caught.
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, "strings.Contains") && strings.Contains(line, "ErrBinaryNotFound") {
			t.Errorf("%s:%d: strings.Contains used together with ErrBinaryNotFound — use errors.Is, not substring matching", path, i+1)
		}
	}

	// Positive: this file uses the typed sentinel via errors.Is.
	return strings.Contains(src, "errors.Is(err, client.ErrBinaryNotFound)")
}

// --- helpers -------------------------------------------------------------

// Make sure the unused-import lint stays happy in case future
// refactors remove all references to client from this test file.
var _ = client.ErrBinaryNotFound

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
