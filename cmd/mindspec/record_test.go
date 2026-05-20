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
func TestRecordStart_WritesManifestAndPropagatesExitCode(t *testing.T) {
	t.Parallel()
	bin := buildMindspecBinary(t)
	workspace := mkWorkspace(t, true)

	// A port with nothing listening, per the spec AC. record start
	// MUST NOT care: it never speaks OTLP itself.
	env := strippedEnv(t)
	env = append(env, "OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:65535")

	cmd := exec.Command(bin, "record", "start", "--spec", "999-test-record",
		"--", "/bin/bash", "-c", "echo hi")
	cmd.Dir = workspace
	cmd.Env = env
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
