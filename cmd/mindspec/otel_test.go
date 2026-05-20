package main

// otel_test.go: cobra-level integration tests for `mindspec otel
// setup` / `mindspec otel status`. Spec 084 Bead 1 (hzdh.7-review).
//
// Two layers of coverage:
//
//  1. In-process exit-code tests. We call otelExitCode(err) on the
//     RunE return value of each subcommand to assert the documented
//     0/1/2 exit-code matrix without paying the `go build` cost.
//     These tests run under `go test -short`.
//
//  2. Subprocess tests. Build the mindspec binary and exec it with
//     real argv, asserting the process exit code matches the matrix.
//     Skipped under `-short` (build is ~3s).

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runOtelSetup invokes `mindspec otel setup <args...>` through the
// real rootCmd in-process, returning combined stdout+stderr and the
// exit code the binary would have produced. Using rootCmd (rather
// than otelSetupCmd directly) exercises the full cobra dispatch
// path including flag parsing as the user experiences it.
func runOtelSetup(t *testing.T, args []string) (string, int) {
	t.Helper()
	resetOtelSetupFlags(t)
	full := append([]string{"otel", "setup"}, args...)
	return runRoot(t, full)
}

func runOtelStatus(t *testing.T, args []string) (string, int) {
	t.Helper()
	resetOtelStatusFlags(t)
	full := append([]string{"otel", "status"}, args...)
	return runRoot(t, full)
}

func runRoot(t *testing.T, args []string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return stdout.String() + stderr.String(), otelExitCode(err)
}

func resetOtelSetupFlags(t *testing.T) {
	t.Helper()
	for _, name := range []string{"endpoint", "protocol", "service-name", "headers", "target", "codex-config"} {
		if f := otelSetupCmd.Flags().Lookup(name); f != nil {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	}
	if f := otelSetupCmd.Flags().Lookup("codex"); f != nil {
		_ = f.Value.Set("false")
		f.Changed = false
	}
}

func resetOtelStatusFlags(t *testing.T) {
	t.Helper()
	if f := otelStatusCmd.Flags().Lookup("codex-config"); f != nil {
		_ = f.Value.Set("")
		f.Changed = false
	}
}

// withCwd chdirs to dir for the duration of the test and restores
// after, and seeds a .mindspec marker so findRoot() picks the
// tempdir instead of walking up into the real mindspec checkout
// the tests are running inside.
func withCwd(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mindspec", ".keep"), nil, 0o644); err != nil {
		t.Fatalf("write .mindspec/.keep: %v", err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// === in-process tests ======================================================

func TestOtelSetup_ExitCode0_WriteAndIdempotent(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)

	out, code := runOtelSetup(t, []string{"--endpoint", "http://collector:4318"})
	if code != 0 {
		t.Fatalf("first setup: code=%d out=%s", code, out)
	}
	// File should exist.
	settings := filepath.Join(root, ".claude", "settings.local.json")
	if _, err := os.Stat(settings); err != nil {
		t.Fatalf("settings file not written: %v", err)
	}

	// Second run with identical args is a no-op + exit 0.
	out, code = runOtelSetup(t, []string{"--endpoint", "http://collector:4318"})
	if code != 0 {
		t.Fatalf("second setup: code=%d out=%s", code, out)
	}
	if !strings.Contains(out, "unchanged") {
		t.Errorf("expected 'unchanged' on idempotent re-run, got: %s", out)
	}
}

func TestOtelSetup_ExitCode2_MissingEndpoint(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	_, code := runOtelSetup(t, []string{})
	if code != 2 {
		t.Errorf("missing --endpoint should return exit 2, got %d", code)
	}
}

func TestOtelSetup_ExitCode2_UnknownTarget(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	_, code := runOtelSetup(t, []string{"--endpoint", "http://x:4318", "--target", "bogus"})
	if code != 2 {
		t.Errorf("unknown --target should return exit 2, got %d", code)
	}
}

func TestOtelSetup_ExitCode2_BadProtocol(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	_, code := runOtelSetup(t, []string{"--endpoint", "http://x:4318", "--protocol", "weird"})
	if code != 2 {
		t.Errorf("bad --protocol should return exit 2, got %d", code)
	}
}

func TestOtelSetup_ExitCode2_BadHeaders(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	_, code := runOtelSetup(t, []string{"--endpoint", "http://x:4318", "--headers", "k"})
	if code != 2 {
		t.Errorf("malformed --headers should return exit 2, got %d", code)
	}
}

func TestOtelSetup_ExitCode1_PreexistingMalformedJSON(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, ".claude", "settings.local.json")
	if err := os.WriteFile(settings, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, code := runOtelSetup(t, []string{"--endpoint", "http://x:4318"})
	if code != 1 {
		t.Errorf("pre-existing malformed JSON should return exit 1, got %d", code)
	}
}

func TestOtelSetup_TargetEnv_PrintsExports(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	out, code := runOtelSetup(t, []string{"--endpoint", "http://x:4318", "--target", "env"})
	if code != 0 {
		t.Fatalf("--target=env should exit 0, got %d", code)
	}
	if !strings.Contains(out, "export OTEL_EXPORTER_OTLP_ENDPOINT='http://x:4318'") {
		t.Errorf("--target=env did not print exports: %s", out)
	}
	// Must NOT have written .claude/settings.local.json
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.local.json")); err == nil {
		t.Errorf("--target=env should not write settings file")
	}
}

func TestOtelStatus_ExitCode0_Configured(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	// Pre-write a Claude settings file via setup itself.
	if _, code := runOtelSetup(t, []string{"--endpoint", "http://collector:4318"}); code != 0 {
		t.Fatalf("setup failed: %d", code)
	}
	codex := filepath.Join(t.TempDir(), "config.toml")
	out, code := runOtelStatus(t, []string{"--codex-config", codex})
	if code != 0 {
		t.Fatalf("status when configured should exit 0, got %d\nout=%s", code, out)
	}
	if !strings.Contains(out, "http://collector:4318") {
		t.Errorf("status output missing endpoint: %s", out)
	}
}

func TestOtelStatus_ExitCode1_NoConfig(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	codex := filepath.Join(t.TempDir(), "config.toml")
	_, code := runOtelStatus(t, []string{"--codex-config", codex})
	if code != 1 {
		t.Errorf("status with no config should exit 1, got %d", code)
	}
}

func TestOtelStatus_ExitCode2_MalformedFile(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.local.json"),
		[]byte("not valid json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	codex := filepath.Join(t.TempDir(), "config.toml")
	_, code := runOtelStatus(t, []string{"--codex-config", codex})
	if code != 2 {
		t.Errorf("status with malformed file should exit 2, got %d", code)
	}
}

// TestOtelSetup_StaleOtelKeysReplaced verifies a previously-set
// OTEL_EXPORTER_OTLP_ENDPOINT key with a different value is REPLACED
// (not appended-alongside) when the user re-runs setup with a new
// endpoint. Per the consensus revision #5: merge must replace, not
// append, the canonical OTEL keys.
func TestOtelSetup_StaleOtelKeysReplaced(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root)

	if _, code := runOtelSetup(t, []string{"--endpoint", "http://OLD:4318"}); code != 0 {
		t.Fatalf("first setup: %d", code)
	}
	if _, code := runOtelSetup(t, []string{"--endpoint", "http://NEW:4318"}); code != 0 {
		t.Fatalf("second setup: %d", code)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	env := doc["env"].(map[string]any)
	if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://NEW:4318" {
		t.Errorf("endpoint not replaced: %v", env["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	if strings.Contains(string(raw), "OLD:4318") {
		t.Errorf("stale OLD endpoint not removed from file:\n%s", string(raw))
	}
}

// === subprocess test (skipped under -short) =================================

// TestOtelExitCodes_Subprocess builds the real binary and execs it
// to assert the documented 0/1/2 exit codes survive cobra's
// process-exit path end-to-end (not just otelExitCode unwrapping).
func TestOtelExitCodes_Subprocess(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess test skipped under -short (go build is slow)")
	}
	bin := buildMindspecBinary(t)

	type tc struct {
		name     string
		args     []string
		setup    func(t *testing.T, root string)
		wantCode int
	}
	cases := []tc{
		{
			name:     "setup_missing_endpoint_exit_2",
			args:     []string{"otel", "setup"},
			wantCode: 2,
		},
		{
			name:     "setup_unknown_target_exit_2",
			args:     []string{"otel", "setup", "--endpoint", "http://x:4318", "--target", "bogus"},
			wantCode: 2,
		},
		{
			name:     "setup_ok_exit_0",
			args:     []string{"otel", "setup", "--endpoint", "http://x:4318"},
			wantCode: 0,
		},
		{
			name: "setup_malformed_preexisting_exit_1",
			args: []string{"otel", "setup", "--endpoint", "http://x:4318"},
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".claude", "settings.local.json"),
					[]byte("not valid json {{{"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCode: 1,
		},
		{
			name:     "status_no_config_exit_1",
			args:     []string{"otel", "status", "--codex-config", "/nonexistent/codex.toml"},
			wantCode: 1,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			// Seed a .mindspec marker so findRoot picks this dir.
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
				t.Fatal(err)
			}
			if c.setup != nil {
				c.setup(t, root)
			}
			cmd := exec.Command(bin, c.args...)
			cmd.Dir = root
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			gotCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					gotCode = exitErr.ExitCode()
				} else {
					t.Fatalf("exec failed (not an ExitError): %v\nstderr=%s", err, stderr.String())
				}
			}
			if gotCode != c.wantCode {
				t.Errorf("exit code = %d, want %d\nstdout=%s\nstderr=%s",
					gotCode, c.wantCode, stdout.String(), stderr.String())
			}
		})
	}
}
