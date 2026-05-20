package main

// cmd/mindspec/record_test.go — spec 084 (mindspec-otel-only) Bead 2.
//
// Covers the spec acceptance criterion at spec.md line 562-566:
//
//	With OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:65535 (a port
//	with nothing listening), `mindspec record start --spec test --
//	echo hi` exits 0, prints `hi`, and writes the expected manifest
//	+ recording/ skeleton. No mindspec stderr mentions OTEL,
//	agentmind, or telemetry.
//
// Also covers spec Test F (exit-code propagation): a workload
// exiting non-zero propagates its exit code verbatim with empty
// mindspec-emitted stderr.
//
// These are subprocess tests — the cobra command must be invoked
// against a freshly built binary so SilenceErrors + os.Exit interplay
// is exercised under the same flag the user sees.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRecordStart_WritesManifestAndPropagatesExitCode is the literal
// implementation of the spec AC at spec.md:562-566.
//
// CONSENSUS revision #6 strengthens this test: in addition to asserting
// `mindspec record start` exits 0 with `hi` on stdout, the workload's
// view of its OWN environment is captured (the child bash script
// writes its $OTEL_EXPORTER_OTLP_ENDPOINT to a temp file) and the test
// asserts the workload actually saw the OTEL endpoint mindspec injected
// from on-disk config. This closes the loop on what the spec AC is
// trying to prove — that the workload subprocess receives the OTEL env,
// not just that mindspec believes it injected one.
func TestRecordStart_WritesManifestAndPropagatesExitCode(t *testing.T) {
	t.Parallel()
	bin := buildMindspecBinary(t)
	workspace := mkWorkspace(t, true)

	// Pre-seed .claude/settings.local.json via `mindspec otel setup`
	// so loadConfiguredOtel has something to inject. A port with
	// nothing listening, per the spec AC. record start MUST NOT
	// care: it never speaks OTLP itself.
	const wantEndpoint = "http://127.0.0.1:65535"
	setup := exec.Command(bin, "otel", "setup",
		"--endpoint", wantEndpoint,
		"--protocol", "http/protobuf")
	setup.Dir = workspace
	setup.Env = strippedEnv(t)
	var setupStderr bytes.Buffer
	setup.Stderr = &setupStderr
	if err := setup.Run(); err != nil {
		t.Fatalf("otel setup failed: %v\nstderr=%q", err, setupStderr.String())
	}

	// CONSENSUS revision #6: capture the child workload's view of its
	// own env to a temp file. The workload echoes "hi" to stdout to
	// satisfy the spec AC, then writes its $OTEL_EXPORTER_OTLP_ENDPOINT
	// to /tmp-file so the test can verify what the child actually saw.
	envCapture := filepath.Join(workspace, "child-env.txt")
	workload := `echo hi; echo "$OTEL_EXPORTER_OTLP_ENDPOINT" > ` + envCapture

	cmd := exec.Command(bin, "record", "start", "--spec", "999-test-record",
		"--", "/bin/bash", "-c", workload)
	cmd.Dir = workspace
	cmd.Env = strippedEnv(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected exit 0 with workload `echo hi`; got %v\nstdout=%q\nstderr=%q",
			err, stdout.String(), stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "hi" {
		t.Errorf("expected stdout %q; got %q", "hi", got)
	}
	// Stderr must not name OTEL/agentmind/telemetry on success.
	for _, banned := range []string{"OTEL", "agentmind", "telemetry"} {
		if strings.Contains(stderr.String(), banned) {
			t.Errorf("stderr must not mention %q; got %q", banned, stderr.String())
		}
	}

	// CONSENSUS revision #6: assert the child subprocess actually
	// observed the OTEL endpoint mindspec injected — not merely that
	// mindspec believes it injected one.
	childEnv, err := os.ReadFile(envCapture)
	if err != nil {
		t.Fatalf("workload did not write captured env file at %s: %v", envCapture, err)
	}
	gotEndpoint := strings.TrimSpace(string(childEnv))
	if gotEndpoint != wantEndpoint {
		t.Errorf("child subprocess saw OTEL_EXPORTER_OTLP_ENDPOINT=%q; want %q",
			gotEndpoint, wantEndpoint)
	}

	// Manifest + recording/ skeleton present on disk (canonical
	// .mindspec/docs/specs/<id>/recording/ layout per ADR-0022).
	manifestPath := filepath.Join(workspace, ".mindspec", "docs", "specs", "999-test-record", "recording", "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written at %s: %v", manifestPath, err)
	}
	var m struct {
		SpecID string `json:"spec_id"`
		Status string `json:"status"`
		Phases []struct {
			Phase string `json:"phase"`
		} `json:"phases"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest parse: %v\nraw=%s", err, raw)
	}
	if m.SpecID != "999-test-record" {
		t.Errorf("manifest spec_id = %q; want %q", m.SpecID, "999-test-record")
	}
	if len(m.Phases) == 0 || m.Phases[0].Phase != "spec" {
		t.Errorf("expected manifest to open with phase=spec; got %+v", m.Phases)
	}
}

// TestRecordStart_CallerEnvWinsOverDiskConfig is CONSENSUS revision #2's
// behavioral test: when the caller exports OTEL_EXPORTER_OTLP_ENDPOINT
// explicitly, that value MUST win over whatever `mindspec otel setup`
// previously wrote to disk. Closes the precedence question that all
// six round-1 reviewers surfaced.
func TestRecordStart_CallerEnvWinsOverDiskConfig(t *testing.T) {
	t.Parallel()
	bin := buildMindspecBinary(t)
	workspace := mkWorkspace(t, true)

	// Disk config says http://disk-config:4318.
	setup := exec.Command(bin, "otel", "setup",
		"--endpoint", "http://disk-config:4318",
		"--protocol", "http/protobuf")
	setup.Dir = workspace
	setup.Env = strippedEnv(t)
	if err := setup.Run(); err != nil {
		t.Fatalf("otel setup failed: %v", err)
	}

	// Caller exports a different endpoint — must win.
	const callerEndpoint = "http://caller-env:9999"
	env := strippedEnv(t)
	env = append(env, "OTEL_EXPORTER_OTLP_ENDPOINT="+callerEndpoint)

	envCapture := filepath.Join(workspace, "child-env.txt")
	workload := `echo "$OTEL_EXPORTER_OTLP_ENDPOINT" > ` + envCapture

	cmd := exec.Command(bin, "record", "start", "--spec", "999-test-precedence",
		"--", "/bin/bash", "-c", workload)
	cmd.Dir = workspace
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("record start failed: %v\nstdout=%q\nstderr=%q",
			err, stdout.String(), stderr.String())
	}

	childEnv, err := os.ReadFile(envCapture)
	if err != nil {
		t.Fatalf("workload did not capture env: %v", err)
	}
	got := strings.TrimSpace(string(childEnv))
	if got != callerEndpoint {
		t.Errorf("caller-exported OTEL_EXPORTER_OTLP_ENDPOINT must win over on-disk config; got %q, want %q",
			got, callerEndpoint)
	}
}

// TestRecordStart_MalformedConfigSurfacesError is CONSENSUS revision #1's
// behavioral test: a malformed .claude/settings.local.json must cause
// `record start` to exit non-zero with a real Error: diagnostic on
// stderr, NOT silently degrade to the parent env.
func TestRecordStart_MalformedConfigSurfacesError(t *testing.T) {
	t.Parallel()
	bin := buildMindspecBinary(t)
	workspace := mkWorkspace(t, true)

	// Write malformed JSON to .claude/settings.local.json.
	claudeDir := filepath.Join(workspace, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"),
		[]byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cmd := exec.Command(bin, "record", "start", "--spec", "999-test-bad-config",
		"--", "/bin/bash", "-c", "echo should-not-run")
	cmd.Dir = workspace
	cmd.Env = strippedEnv(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit on malformed config; got nil (stdout=%q)",
			stdout.String())
	}
	if strings.Contains(stdout.String(), "should-not-run") {
		t.Errorf("workload must not run when on-disk OTEL config is malformed; stdout=%q",
			stdout.String())
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Errorf("expected Error: diagnostic on stderr for malformed config; got stderr=%q",
			stderr.String())
	}
	if !strings.Contains(stderr.String(), "malformed Claude OTEL config") {
		t.Errorf("expected stderr to identify malformed Claude OTEL config; got stderr=%q",
			stderr.String())
	}
}

// TestRecordStart_ExitCodePropagation is Test F's exit-code half: the
// workload's non-zero exit is propagated verbatim.
func TestRecordStart_ExitCodePropagation(t *testing.T) {
	t.Parallel()
	bin := buildMindspecBinary(t)
	workspace := mkWorkspace(t, true)

	cmd := exec.Command(bin, "record", "start", "--spec", "999-test-record-exit",
		"--", "/bin/bash", "-c", "exit 7")
	cmd.Dir = workspace
	cmd.Env = strippedEnv(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError; got %T: %v\nstdout=%q\nstderr=%q",
			err, err, stdout.String(), stderr.String())
	}
	if got := exitErr.ExitCode(); got != 7 {
		t.Fatalf("expected workload exit code 7 to propagate; got %d", got)
	}
	// Stderr must remain empty on workload exit (spec Test F).
	if s := stderr.String(); s != "" {
		t.Errorf("expected empty stderr on workload exit; got %q", s)
	}
}

// TestRecordStart_RequiresWorkloadArgs asserts the new shape: omitting
// `-- <workload-cmd>` produces a real usage error (not a silent no-op).
func TestRecordStart_RequiresWorkloadArgs(t *testing.T) {
	t.Parallel()
	bin := buildMindspecBinary(t)
	workspace := mkWorkspace(t, true)

	cmd := exec.Command(bin, "record", "start", "--spec", "999-test-record-nocmd")
	cmd.Dir = workspace
	cmd.Env = strippedEnv(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected non-zero exit when no workload args given; got nil")
	}
	if !strings.Contains(stderr.String(), "workload command is required") {
		t.Errorf("expected usage error on stderr; got %q", stderr.String())
	}
}
